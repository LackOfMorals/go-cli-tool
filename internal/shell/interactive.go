package shell

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"

	"github.com/chzyer/readline"
	"github.com/cli/go-cli-tool/internal/analytics"
	"github.com/cli/go-cli-tool/internal/config"
	"github.com/cli/go-cli-tool/internal/logger"
	"github.com/cli/go-cli-tool/internal/presentation"
	"github.com/cli/go-cli-tool/internal/tool"
	"github.com/google/shlex"
)

// InteractiveShell is the concrete REPL implementation of the Shell interface.
type InteractiveShell struct {
	log       logger.Service
	cfg       *config.Config
	registry  Registry
	telemetry analytics.Service
	presenter presentation.Service
	version   string

	categories map[string]*Category
	handlers   map[string]CommandHandler
	handlersMu sync.RWMutex // protects handlers; separate from mu to avoid deadlock

	io          tool.IOHandler
	prompt      string
	historyFile string

	// rl is the active readline instance. It is set inside Start() before
	// runLoop is entered and cleared after runLoop returns. Protected by mu.
	rl *readline.Instance

	running bool
	mu      sync.RWMutex
}

// NewInteractiveShell creates a shell with sensible defaults.
func NewInteractiveShell() *InteractiveShell {
	return &InteractiveShell{
		categories:  make(map[string]*Category),
		handlers:    make(map[string]CommandHandler),
		io:          tool.NewDefaultIOHandler(),
		prompt:      "lom> ",
		historyFile: ".neo4j_history",
		version:     "development",
	}
}

// ---- Setters ------------------------------------------------------------

func (s *InteractiveShell) SetLogger(log logger.Service)            { s.log = log }
func (s *InteractiveShell) SetTelemetry(tel analytics.Service)      { s.telemetry = tel }
func (s *InteractiveShell) SetPresenter(p presentation.Service)     { s.presenter = p }
func (s *InteractiveShell) SetCategories(cats map[string]*Category) { s.categories = cats }
func (s *InteractiveShell) SetVersion(v string)                     { s.version = v }
func (s *InteractiveShell) SetRegistry(r Registry)                  { s.registry = r }

func (s *InteractiveShell) SetConfig(cfg config.Config) {
	s.cfg = &cfg
	if cfg.Shell.Prompt != "" {
		s.prompt = cfg.Shell.Prompt
	}
	if cfg.Shell.HistoryFile != "" {
		s.historyFile = cfg.Shell.HistoryFile
	}
}

// ---- Shell interface ----------------------------------------------------

// Start initialises readline and runs the REPL, blocking until the user exits
// or a termination signal is received.
//
// readline handles SIGINT (Ctrl+C) internally — it returns ErrInterrupt
// immediately so the REPL is never stuck waiting for a newline. We only
// intercept SIGTERM for a clean teardown.
func (s *InteractiveShell) Start() error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return fmt.Errorf("shell is already running")
	}

	rl, err := readline.NewEx(&readline.Config{
		Prompt:          s.prompt,
		HistoryFile:     s.historyFile,
		HistoryLimit:    500,
		InterruptPrompt: "^C",
		EOFPrompt:       "exit",
		AutoComplete:    s.buildCompleter(),
	})
	if err != nil {
		s.mu.Unlock()
		return fmt.Errorf("init readline: %w", err)
	}

	s.running = true
	s.rl = rl
	s.mu.Unlock()

	// Defer cleanup so it always runs, even if runLoop panics.
	defer func() {
		_ = rl.Close()
		s.mu.Lock()
		s.rl = nil
		s.mu.Unlock()
	}()

	// SIGTERM: close readline so Readline() unblocks and the loop exits.
	// Do NOT intercept SIGINT here — readline owns it.
	//
	// ctx/cancel ensures the signal goroutine exits cleanly when Start()
	// returns, preventing a goroutine leak across multiple Start() calls
	// (common in tests).
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGTERM)
	go func() {
		select {
		case <-sigChan:
			// Grab the pointer under the lock in case Stop() has already cleared it.
			s.mu.RLock()
			rl := s.rl
			s.mu.RUnlock()
			if rl != nil {
				_ = rl.Close()
			}
		case <-ctx.Done():
			// Start() is returning; exit cleanly.
		}
	}()

	s.printWelcome()
	s.runLoop(rl)

	signal.Stop(sigChan)
	_ = s.Stop() // ensure running = false however the loop exited
	return nil
}

// Stop marks the shell as stopped. If readline is active it is closed so that
// any in-progress Readline() call unblocks immediately.
func (s *InteractiveShell) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.running = false
	if s.rl != nil {
		_ = s.rl.Close()
	}
	return nil
}

func (s *InteractiveShell) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

func (s *InteractiveShell) RegisterCommand(name string, handler CommandHandler) {
	s.handlersMu.Lock()
	defer s.handlersMu.Unlock()
	s.handlers[name] = handler
}

// Execute implements Shell. For callers outside the REPL (e.g. tests or
// programmatic use) it runs the command with a background context —
// cancellation is not available. The REPL loop uses executeWithContext
// directly so that Ctrl+C can cancel in-flight service calls.
func (s *InteractiveShell) Execute(cmd string) (string, error) {
	return s.executeWithContext(context.Background(), cmd)
}

// executeWithContext is the internal dispatch entry-point. ctx should be the
// per-command context created in runLoop so that Ctrl+C propagates.
func (s *InteractiveShell) executeWithContext(ctx context.Context, cmd string) (string, error) {
	args := parseCommand(cmd)
	if len(args) == 0 {
		return "", nil
	}

	name := args[0]
	rest := args[1:]

	if IsBuiltinCommand(name) {
		return s.executeBuiltin(name, rest)
	}

	if cat, ok := s.categories[name]; ok {
		return cat.Dispatch(rest, s.makeContext(ctx))
	}

	s.handlersMu.RLock()
	handler, hasHandler := s.handlers[name]
	s.handlersMu.RUnlock()
	if hasHandler {
		return handler(rest, s.makeContext(ctx))
	}

	if s.registry != nil {
		if t, err := s.registry.Get(name); err == nil {
			return s.executeTool(ctx, t, rest)
		}
	}

	return "", fmt.Errorf("unknown command: %q  (type 'help' to see available commands)", name)
}

// ---- REPL loop ----------------------------------------------------------

func (s *InteractiveShell) runLoop(rl *readline.Instance) {
	for {
		line, err := rl.Readline()

		if err == readline.ErrInterrupt {
			// Ctrl+C at the prompt: discard any partial input.
			// Ctrl+C on an empty line: show a hint.
			if strings.TrimSpace(line) == "" {
				fmt.Fprintln(os.Stderr, "(type 'exit' or press Ctrl+D to quit)")
			}
			continue
		}

		if err == io.EOF || err != nil {
			// Ctrl+D or readline closed (SIGTERM / Stop() called).
			fmt.Println()
			break
		}

		// Collect continuation lines when the input ends with \.
		collected, contErr := s.collectInput(rl, line)
		if contErr == readline.ErrInterrupt {
			// Ctrl+C mid-continuation: discard accumulated input.
			continue
		}
		if contErr != nil {
			// EOF or readline closed during continuation.
			fmt.Println()
			break
		}

		cmd := strings.TrimSpace(collected)
		if cmd == "" {
			continue
		}

		// Create a per-command context. While the command runs we register a
		// SIGINT handler so that Ctrl+C cancels the in-flight context rather
		// than killing the process. readline owns SIGINT at the prompt; we
		// take it back for the duration of each command and restore it after.
		cmdCtx, cmdCancel := context.WithCancel(context.Background())
		interruptCh := make(chan os.Signal, 1)
		signal.Notify(interruptCh, os.Interrupt)
		go func() {
			select {
			case <-interruptCh:
				fmt.Fprintln(os.Stderr, "^C")
				cmdCancel()
			case <-cmdCtx.Done():
				// Command finished normally; exit the goroutine cleanly.
			}
		}()

		output, execErr := s.executeWithContext(cmdCtx, cmd)

		cmdCancel() // always release resources, even on normal return
		signal.Stop(interruptCh)

		// Suppress the generic "context canceled" message — the interrupt
		// goroutine already printed "^C" to stderr.
		if !errors.Is(execErr, context.Canceled) {
			if execErr != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", execErr)
			}
			if output != "" {
				fmt.Println(output)
			}
		}

		// Break if Execute() triggered Stop() (e.g. 'exit' / 'quit' command).
		if !s.IsRunning() {
			break
		}
	}
}

// ---- Tab completion -----------------------------------------------------

// buildCompleter constructs a readline AutoCompleter from the registered
// categories, commands, and tool registry. It is called once when Start()
// initialises readline and reflects the state at that point.
func (s *InteractiveShell) buildCompleter() readline.AutoCompleter {
	var items []readline.PrefixCompleterInterface

	// Built-in commands
	for name := range builtins {
		items = append(items, readline.PcItem(name))
	}

	// Category tree: category → sub-category → commands
	for catName, cat := range s.categories {
		var catChildren []readline.PrefixCompleterInterface

		for _, subName := range cat.SubcategoryNames() {
			sub := cat.Subcat(subName)
			if sub == nil {
				continue
			}
			var cmdItems []readline.PrefixCompleterInterface
			for _, cmdName := range sub.CommandNames() {
				cmdItems = append(cmdItems, readline.PcItem(cmdName))
			}
			catChildren = append(catChildren, readline.PcItem(subName, cmdItems...))
		}

		for _, cmdName := range cat.AllCommandNames() {
			catChildren = append(catChildren, readline.PcItem(cmdName))
		}

		items = append(items, readline.PcItem(catName, catChildren...))
	}

	// Tool registry
	if s.registry != nil {
		for _, name := range s.registry.ListNames() {
			items = append(items, readline.PcItem(name))
		}
	}

	return readline.NewPrefixCompleter(items...)
}

// ---- Context ------------------------------------------------------------

func (s *InteractiveShell) makeContext(ctx context.Context) Context {
	cfg := config.Config{}
	if s.cfg != nil {
		cfg = *s.cfg
	}
	return Context{
		Context:   ctx,
		Config:    cfg,
		Logger:    s.log,
		Telemetry: s.telemetry,
		Presenter: s.presenter,
		Registry:  s.registry,
		IO:        s.io,
	}
}

const defaultNeo4jURI = "bolt://localhost:7687"

// isUnconfigured reports true when no meaningful Neo4j connection config has
// been provided — i.e. the URI is absent or still the default sentinel AND no
// password has been set.
func isUnconfigured(cfg *config.Config) bool {
	if cfg == nil {
		return true
	}
	uri := cfg.Neo4j.URI
	uriIsDefault := uri == "" || uri == defaultNeo4jURI
	return uriIsDefault && cfg.Neo4j.Password == ""
}

func (s *InteractiveShell) printWelcome() {
	fmt.Println("lom , a CLI for Neo4j — type 'help' for commands, 'exit' to quit.")
	fmt.Println()

	if isUnconfigured(s.cfg) {
		fmt.Println("Welcome! No connection configuration detected.")
		fmt.Println("To get started, set your Neo4j connection details:")
		fmt.Println()
		fmt.Println("  config set neo4j.uri    bolt://<host>:7687")
		fmt.Println("  config set neo4j.username <username>")
		fmt.Println("  config set neo4j.password <password>")
		fmt.Println()
		fmt.Println("Then run 'cypher' commands to query your database.")
		fmt.Println("Type 'help' to see all available commands.")
		fmt.Println()
	}
}

// ---- Built-in commands --------------------------------------------------

func (s *InteractiveShell) executeBuiltin(command string, args []string) (string, error) {
	switch command {
	case "exit", "quit":
		_ = s.Stop()
		return "Goodbye!", nil
	case "help":
		return s.builtinHelp(args)
	case "config":
		// With subcommand args, delegate to the config category if registered.
		// Without args, show the built-in configuration summary.
		if len(args) > 0 {
			if cat, ok := s.categories["config"]; ok {
				return cat.Dispatch(args, s.makeContext(context.Background()))
			}
		}
		return s.builtinConfig(args)
	case "set":
		return s.builtinSet(args)
	case "log-level":
		return s.builtinLogLevel(args)
	case "clear":
		return s.builtinClear(args)
	case "version":
		return s.builtinVersion(args)
	default:
		return "", fmt.Errorf("unknown built-in: %s", command)
	}
}

func (s *InteractiveShell) builtinHelp(args []string) (string, error) {
	if len(args) == 0 {
		return CategoryHelpOverview(s.categories), nil
	}

	cat, ok := s.categories[args[0]]
	if !ok {
		return "", fmt.Errorf("unknown category: %s", args[0])
	}

	if len(args) > 1 {
		sub := cat.Find(args[1:])
		if sub == nil {
			return "", fmt.Errorf("unknown: %s", strings.Join(args, " "))
		}
		return sub.Help(), nil
	}

	return cat.Help(), nil
}

func (s *InteractiveShell) builtinConfig(_ []string) (string, error) {
	if s.cfg == nil {
		return "no configuration loaded", nil
	}

	c := s.cfg
	var b strings.Builder

	sec := func(name string) {
		if b.Len() > 0 {
			b.WriteByte('\n')
		}
		fmt.Fprintf(&b, "%s\n%s\n", name, strings.Repeat("─", len(name)))
	}
	row := func(label, value string) {
		fmt.Fprintf(&b, "  %-14s  %s\n", label, value)
	}
	secret := func(v string) string {
		if v == "" {
			return "(not set)"
		}
		return "(set)"
	}
	orNotSet := func(v string) string {
		if v == "" {
			return "(not set)"
		}
		return v
	}

	sec("Logging")
	row("Level", c.LogLevel)
	row("Format", c.LogFormat)
	row("Output", func() string {
		if c.LogOutput == "" {
			return "stderr"
		}
		return c.LogOutput
	}())
	if c.LogOutput == "file" {
		row("Log file", func() string {
			if c.LogFile == "" {
				return "(default: ~/.lom/lom.log)"
			}
			return c.LogFile
		}())
	}

	sec("Shell")
	row("Prompt", c.Shell.Prompt)
	row("History file", c.Shell.HistoryFile)

	sec("Neo4j")
	row("URI", orNotSet(c.Neo4j.URI))
	row("Username", orNotSet(c.Neo4j.Username))
	row("Database", orNotSet(c.Neo4j.Database))
	row("Password", secret(c.Neo4j.Password))

	sec("Aura")
	row("Client ID", orNotSet(c.Aura.ClientID))
	row("Client secret", secret(c.Aura.ClientSecret))
	row("Timeout", fmt.Sprintf("%ds", c.Aura.TimeoutSeconds))

	sec("Cypher")
	cypherFmt := c.Cypher.OutputFormat
	if cypherFmt == "" {
		cypherFmt = "table"
	}
	row("Output format", cypherFmt)
	shellLim := c.Cypher.ShellLimit
	if shellLim == 0 {
		shellLim = 25
	}
	execLim := c.Cypher.ExecLimit
	if execLim == 0 {
		execLim = 100
	}
	row("Shell limit", fmt.Sprintf("%d rows", shellLim))
	row("Exec limit", fmt.Sprintf("%d rows", execLim))

	sec("Telemetry")
	metricsStatus := "enabled"
	if !c.Telemetry.Metrics {
		metricsStatus = "disabled"
	}
	row("Metrics", metricsStatus)

	return strings.TrimRight(b.String(), "\n"), nil
}

func (s *InteractiveShell) builtinSet(args []string) (string, error) {
	if len(args) == 0 {
		return "", fmt.Errorf(
			"usage: set <key> <value>\n" +
				"  keys: prompt, log-level, cypher-format, cypher-limit")
	}
	// Single-arg form: "set <key>" — show current value.
	if len(args) == 1 {
		switch args[0] {
		case "cypher-format":
			fmt := "table"
			if s.cfg != nil && s.cfg.Cypher.OutputFormat != "" {
				fmt = s.cfg.Cypher.OutputFormat
			}
			return fmt, nil
		case "cypher-limit":
			lim := 25
			if s.cfg != nil && s.cfg.Cypher.ShellLimit > 0 {
				lim = s.cfg.Cypher.ShellLimit
			}
			return fmt.Sprintf("%d", lim), nil
		default:
			return "", fmt.Errorf("usage: set <key> <value>  (keys: prompt, log-level, cypher-format, cypher-limit)")
		}
	}
	switch args[0] {
	case "prompt":
		s.cfg.Shell.Prompt = args[1]
		s.prompt = args[1]
		// Update readline's live prompt so the change takes effect immediately.
		s.mu.RLock()
		if s.rl != nil {
			s.rl.SetPrompt(args[1])
		}
		s.mu.RUnlock()
		return fmt.Sprintf("prompt set to: %s", args[1]), nil
	case "log-level":
		s.cfg.LogLevel = args[1]
		if s.log != nil {
			s.log.SetLevel(logger.ParseLogLevel(args[1]))
		}
		return fmt.Sprintf("log level set to: %s", args[1]), nil
	case "cypher-format":
		v := strings.ToLower(args[1])
		if v != "table" && v != "graph" && v != "json" && v != "pretty-json" {
			return "", fmt.Errorf("cypher-format must be table, graph, json, or pretty-json")
		}
		if s.cfg != nil {
			s.cfg.Cypher.OutputFormat = v
		}
		return fmt.Sprintf("cypher output format set to: %s", v), nil
	case "cypher-limit":
		var n int
		if _, err := fmt.Sscanf(args[1], "%d", &n); err != nil || n < 1 {
			return "", fmt.Errorf("cypher-limit must be a positive integer")
		}
		if s.cfg != nil {
			s.cfg.Cypher.ShellLimit = n
		}
		return fmt.Sprintf("cypher row limit set to: %d", n), nil
	default:
		return "", fmt.Errorf("unknown key %q  (keys: prompt, log-level, cypher-format, cypher-limit)", args[0])
	}
}

func (s *InteractiveShell) builtinLogLevel(args []string) (string, error) {
	if len(args) == 0 {
		if s.cfg != nil {
			return fmt.Sprintf("current log level: %s", s.cfg.LogLevel), nil
		}
		return "log level: unknown", nil
	}
	if s.cfg != nil {
		s.cfg.LogLevel = args[0]
	}
	if s.log != nil {
		s.log.SetLevel(logger.ParseLogLevel(args[0]))
	}
	return fmt.Sprintf("log level set to: %s", args[0]), nil
}

func (s *InteractiveShell) builtinClear(_ []string) (string, error) {
	fmt.Print("\033[2J\033[H")
	return "", nil
}

func (s *InteractiveShell) builtinVersion(_ []string) (string, error) {
	return fmt.Sprintf("lom %s", s.version), nil
}

// ---- Tool execution -----------------------------------------------------

func (s *InteractiveShell) executeTool(ctx context.Context, t tool.Tool, args []string) (string, error) {
	toolCtx := tool.NewContext().
		WithContext(ctx).
		WithArgs(args).
		WithLogger(s.log).
		WithIO(s.io).
		WithPresenter(s.presenter)

	// Validate before Execute so that tool-level prerequisite checks
	// (e.g. required config, service availability) produce a clear error
	// before any real work begins. BaseTool.Validate is a no-op by default.
	if err := t.Validate(*toolCtx); err != nil {
		return "", err
	}

	result, err := t.Execute(*toolCtx)

	// Emit after execution so we can record the actual outcome.
	if s.telemetry != nil {
		s.telemetry.EmitToolEvent(t.Name(), err == nil && result.Success)
	}

	if err != nil {
		return "", err
	}
	if !result.Success {
		return result.Output, fmt.Errorf("tool execution failed")
	}
	return result.Output, nil
}

// ---- Input collection --------------------------------------------------

// collectInput handles multi-line input for the shell REPL.
//
// Three modes are supported:
//
//  1. Single-line with trailing semicolon — the semicolon is stripped and the
//     line is executed immediately. This works for all commands:
//
//     neo4j> cloud instances list;
//
//  2. Backslash continuation — a trailing \ starts multi-line mode for any
//     command. Input is accumulated until a line without a trailing \ is
//     received, or a trailing ; terminates early:
//
//     neo4j> cloud instances \
//     ...>   list
//
//  3. Cypher mode — when the first token is "cypher", the shell automatically
//     enters multi-line mode and waits for a ; terminator. This lets Cypher
//     queries span as many lines as needed:
//
//     neo4j> cypher MATCH (n:Person)
//     ...>   WHERE n.age > 30
//     ...>   RETURN n.name, n.age;
func (s *InteractiveShell) collectInput(rl *readline.Instance, firstLine string) (string, error) {
	const contPrompt = "...> "

	line := strings.TrimRight(firstLine, " \t")

	// A trailing semicolon on the first line: strip it and execute immediately.
	// This applies to all commands, not just cypher.
	if strings.HasSuffix(line, ";") {
		return strings.TrimRight(strings.TrimSuffix(line, ";"), " \t"), nil
	}

	// Backslash continuation: collect lines until a line has no trailing \
	// or a trailing ; terminates the input early.
	if strings.HasSuffix(line, `\`) {
		rl.SetPrompt(contPrompt)
		defer rl.SetPrompt(s.prompt)

		var buf strings.Builder
		for {
			buf.WriteString(strings.TrimSuffix(line, `\`))
			buf.WriteByte(' ')

			next, err := rl.Readline()
			if err != nil {
				return strings.TrimSpace(buf.String()), err
			}

			line = strings.TrimRight(next, " \t")
			if strings.HasSuffix(line, ";") {
				buf.WriteString(strings.TrimSuffix(line, ";"))
				return strings.TrimSpace(buf.String()), nil
			}
			if !strings.HasSuffix(line, `\`) {
				buf.WriteString(line)
				return buf.String(), nil
			}
		}
	}

	// Cypher mode: wait for a ; terminator before executing.
	// The prompt switches to ...> so the user knows more input is expected.
	if isCypherCommand(line) {
		// If the line already contains a `;` — typically inside a quoted
		// query argument like `cypher "MATCH (n);" --format json` — the
		// cypher input is complete. Execute as-is rather than blocking on a
		// terminator that is already present. The trailing-`;` case is
		// handled above; this catches any `;` followed by flags or other
		// trailing tokens.
		if strings.Contains(line, ";") {
			return line, nil
		}

		rl.SetPrompt(contPrompt)
		defer rl.SetPrompt(s.prompt)

		var buf strings.Builder
		buf.WriteString(line)

		for {
			buf.WriteByte(' ')

			next, err := rl.Readline()
			if err != nil {
				return strings.TrimSpace(buf.String()), err
			}

			line = strings.TrimRight(next, " \t")
			if strings.HasSuffix(line, ";") {
				buf.WriteString(strings.TrimSuffix(line, ";"))
				return strings.TrimSpace(buf.String()), nil
			}
			buf.WriteString(line)
		}
	}

	return line, nil
}

// isCypherCommand reports whether a shell input line begins with the cypher
// command keyword. Used by collectInput to enable automatic ; termination.
func isCypherCommand(line string) bool {
	lower := strings.ToLower(strings.TrimSpace(line))
	return lower == "cypher" || strings.HasPrefix(lower, "cypher ")
}

// ---- Input parsing ------------------------------------------------------

// parseCommand splits a raw input line into POSIX shell tokens using
// github.com/google/shlex. It handles single/double quotes and backslash
// escapes correctly. On a parse error (e.g. unclosed quote) it falls back
// to simple whitespace splitting so the shell never silently swallows input.
func parseCommand(cmd string) []string {
	args, err := shlex.Split(cmd)
	if err != nil {
		// Best-effort fallback: whitespace split preserves the tokens even
		// if quoting is malformed (e.g. an unclosed quote in a Cypher query).
		return strings.Fields(cmd)
	}
	return args
}
