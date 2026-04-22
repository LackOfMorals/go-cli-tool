package shell

import (
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
)

// InteractiveShell is the concrete REPL implementation of the Shell interface.
type InteractiveShell struct {
	log       logger.Service
	cfg       *config.Config
	registry  Registry
	telemetry analytics.Service
	presenter *presentation.PresentationService
	version   string

	categories map[string]*Category
	handlers   map[string]CommandHandler

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
		prompt:      "neo4j> ",
		historyFile: ".neo4j_history",
		version:     "development",
	}
}

// ---- Setters ------------------------------------------------------------

func (s *InteractiveShell) SetLogger(log logger.Service)                     { s.log = log }
func (s *InteractiveShell) SetTelemetry(tel analytics.Service)               { s.telemetry = tel }
func (s *InteractiveShell) SetPresenter(p *presentation.PresentationService) { s.presenter = p }
func (s *InteractiveShell) SetCategories(cats map[string]*Category)          { s.categories = cats }
func (s *InteractiveShell) SetVersion(v string)                              { s.version = v }
func (s *InteractiveShell) SetRegistry(r Registry)                           { s.registry = r }

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
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGTERM)
	go func() {
		<-sigChan
		// Grab the pointer under the lock in case Stop() has already cleared it.
		s.mu.RLock()
		rl := s.rl
		s.mu.RUnlock()
		if rl != nil {
			_ = rl.Close()
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
	s.handlers[name] = handler
}

// Execute parses and dispatches a single command line.
//
// Routing order:
//  1. Built-in commands (exit, help, config, …)
//  2. Top-level categories — routing within is handled by Category.Dispatch
//  3. Custom handlers registered via RegisterCommand
//  4. Tool registry
func (s *InteractiveShell) Execute(cmd string) (string, error) {
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
		return cat.Dispatch(rest, s.makeContext())
	}

	if handler, ok := s.handlers[name]; ok {
		return handler(rest, s.makeContext())
	}

	if s.registry != nil {
		if t, err := s.registry.Get(name); err == nil {
			return s.executeTool(t, rest)
		}
	}

	return "", fmt.Errorf("unknown command: %q  (type 'help' to see available commands)", name)
}

// ---- REPL loop ----------------------------------------------------------

func (s *InteractiveShell) runLoop(rl *readline.Instance) {
	for {
		line, err := rl.Readline()

		if err == readline.ErrInterrupt {
			// Ctrl+C with a partial line: discard input and continue.
			// Ctrl+C on an empty line: show a hint so the user knows how to exit.
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

		cmd := strings.TrimSpace(line)
		if cmd == "" {
			continue
		}

		output, execErr := s.Execute(cmd)
		if execErr != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", execErr)
		}
		if output != "" {
			fmt.Println(output)
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

		for _, cmdName := range cat.CommandNames() {
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

func (s *InteractiveShell) makeContext() ShellContext {
	cfg := config.Config{}
	if s.cfg != nil {
		cfg = *s.cfg
	}
	return ShellContext{
		Config:    cfg,
		Logger:    s.log,
		Telemetry: s.telemetry,
		Presenter: s.presenter,
		Registry:  s.registry,
		IO:        s.io,
	}
}

func (s *InteractiveShell) printWelcome() {
	fmt.Println("Neo4j CLI — type 'help' for commands, 'exit' to quit.")
	fmt.Println()
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
	return fmt.Sprintf("Log Level:    %s\nLog Format:   %s\nPrompt:       %s\nHistory File: %s",
		s.cfg.LogLevel, s.cfg.LogFormat, s.cfg.Shell.Prompt, s.cfg.Shell.HistoryFile), nil
}

func (s *InteractiveShell) builtinSet(args []string) (string, error) {
	if len(args) < 2 {
		return "", fmt.Errorf("usage: set <key> <value>")
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
	default:
		return "", fmt.Errorf("unknown config key: %s", args[0])
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
	return fmt.Sprintf("neo4j-cli %s", s.version), nil
}

// ---- Tool execution -----------------------------------------------------

func (s *InteractiveShell) executeTool(t tool.Tool, args []string) (string, error) {
	if s.telemetry != nil {
		s.telemetry.EmitEvent(analytics.TrackEvent{Event: "tool_used"})
	}

	toolCtx := tool.NewContext().
		WithArgs(args).
		WithLogger(s.log).
		WithIO(s.io).
		WithPresenter(s.presenter)

	result, err := t.Execute(*toolCtx)
	if err != nil {
		return "", err
	}
	if !result.Success {
		return result.Output, fmt.Errorf("tool execution failed")
	}
	return result.Output, nil
}

// ---- Input parsing ------------------------------------------------------

// parseCommand splits a raw input line into tokens, honouring single and
// double quotes so arguments with spaces can be passed. Both spaces and tabs
// are treated as whitespace.
func parseCommand(cmd string) []string {
	var args []string
	var current strings.Builder
	inQuote := false
	quoteChar := rune(0)

	for _, ch := range cmd {
		switch {
		case (ch == '"' || ch == '\'') && !inQuote:
			inQuote = true
			quoteChar = ch
		case ch == quoteChar && inQuote:
			inQuote = false
			quoteChar = 0
		case (ch == ' ' || ch == '\t') && !inQuote:
			if current.Len() > 0 {
				args = append(args, current.String())
				current.Reset()
			}
		default:
			current.WriteRune(ch)
		}
	}
	if current.Len() > 0 {
		args = append(args, current.String())
	}
	return args
}
