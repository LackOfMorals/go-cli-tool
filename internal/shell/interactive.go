package shell

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/cli/go-cli-tool/internal/analytics"
	"github.com/cli/go-cli-tool/internal/config"
	"github.com/cli/go-cli-tool/internal/logger"
	"github.com/cli/go-cli-tool/internal/presentation"
	"github.com/cli/go-cli-tool/internal/tool"
	"github.com/cli/go-cli-tool/internal/tools"
)

// InteractiveShell is the concrete REPL implementation of the Shell interface.
type InteractiveShell struct {
	log       logger.Service
	cfg       *config.Config
	registry  *tools.ToolRegistry
	telemetry analytics.Service
	presenter *presentation.PresentationService

	// categories is the top-level command hierarchy (cypher, cloud, admin).
	categories map[string]*Category

	// handlers holds custom one-off command handlers registered at runtime.
	handlers map[string]CommandHandler

	io          tool.IOHandler
	prompt      string
	historyFile string

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
	}
}

// ---- Setters ------------------------------------------------------------

func (s *InteractiveShell) SetLogger(log logger.Service)                     { s.log = log }
func (s *InteractiveShell) SetTelemetry(tel analytics.Service)               { s.telemetry = tel }
func (s *InteractiveShell) SetPresenter(p *presentation.PresentationService) { s.presenter = p }
func (s *InteractiveShell) SetCategories(cats map[string]*Category)          { s.categories = cats }

func (s *InteractiveShell) SetConfig(cfg config.Config) {
	s.cfg = &cfg
	if cfg.Shell.Prompt != "" {
		s.prompt = cfg.Shell.Prompt
	}
	if cfg.Shell.HistoryFile != "" {
		s.historyFile = cfg.Shell.HistoryFile
	}
}

func (s *InteractiveShell) SetRegistry(registry interface {
	Get(name string) (tool.Tool, error)
	ListNames() []string
}) {
	if reg, ok := registry.(*tools.ToolRegistry); ok {
		s.registry = reg
	}
}

// ---- Shell interface ----------------------------------------------------

// Start runs the REPL and blocks until the user exits or a signal fires.
func (s *InteractiveShell) Start() error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return fmt.Errorf("shell is already running")
	}
	s.running = true
	s.mu.Unlock()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		_ = s.Stop()
	}()

	s.printWelcome()
	s.runLoop()

	signal.Stop(sigChan)
	return nil
}

func (s *InteractiveShell) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.running = false
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
//  2. Top-level categories (cypher, cloud, admin) — routing inside the
//     category tree is handled by Category.Dispatch
//  3. Custom handlers registered via RegisterCommand
//  4. Tool registry (legacy flat tools)
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
		return cat.Dispatch(rest, *s.createContext())
	}

	if handler, ok := s.handlers[name]; ok {
		return handler(rest, *s.createContext())
	}

	if s.registry != nil {
		if t, err := s.registry.Get(name); err == nil {
			return s.executeTool(t, rest)
		}
	}

	return "", fmt.Errorf("unknown command: %q  (type 'help' to see available commands)", name)
}

// ---- REPL loop ----------------------------------------------------------

func (s *InteractiveShell) runLoop() {
	reader := bufio.NewReader(os.Stdin)

	for s.IsRunning() {
		fmt.Print(s.prompt)

		line, err := reader.ReadString('\n')
		if err != nil {
			fmt.Println()
			break
		}

		cmd := strings.TrimSpace(line)
		if cmd == "" {
			continue
		}

		output, err := s.Execute(cmd)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
		}
		if output != "" {
			fmt.Println(output)
		}
	}

	_ = s.Stop()
}

func (s *InteractiveShell) createContext() *ShellContext {
	return NewShellContext().
		WithConfig(s.cfg).
		WithLogger(s.log).
		WithTelemetry(s.telemetry).
		WithPresenter(s.presenter).
		WithRegistry(s.registry).
		WithIO(s.io)
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

// builtinHelp handles:
//
//	help                   — full overview of categories + builtins
//	help cypher            — category help
//	help cloud instances   — sub-category help (uses Category.Find)
func (s *InteractiveShell) builtinHelp(args []string) (string, error) {
	if len(args) == 0 {
		return CategoryHelpOverview(s.categories), nil
	}

	cat, ok := s.categories[args[0]]
	if !ok {
		return "", fmt.Errorf("unknown category: %s", args[0])
	}

	// Deeper navigation (e.g. "help cloud instances") uses Category.Find.
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
	return "neo4j-cli (version injected at build time via -ldflags)", nil
}

// ---- Tool execution -----------------------------------------------------

func (s *InteractiveShell) executeTool(t tool.Tool, args []string) (string, error) {
	if s.telemetry != nil {
		s.telemetry.EmitEvent(analytics.TrackEvent{Event: "tool_used"})
	}

	start := time.Now()
	_ = context.Background()

	toolCtx := tool.NewContext().
		WithArgs(args).
		WithLogger(s.log).
		WithIO(s.io).
		WithPresenter(s.presenter)

	result, err := t.Execute(*toolCtx)
	_ = time.Since(start)

	if err != nil {
		return "", err
	}
	if !result.Success {
		if result.Error != nil {
			return result.Output, result.Error
		}
		return result.Output, fmt.Errorf("tool execution failed")
	}
	return result.Output, nil
}

// ---- Input parsing ------------------------------------------------------

func parseCommand(cmd string) []string {
	var args []string
	var current strings.Builder
	inQuote := false
	quoteChar := ' '

	for _, ch := range cmd {
		switch {
		case (ch == '"' || ch == '\'') && !inQuote:
			inQuote = true
			quoteChar = ch
		case ch == quoteChar && inQuote:
			inQuote = false
		case ch == ' ' && !inQuote:
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
