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

	"github.com/cli/go-cli-tool/internal/config"
	"github.com/cli/go-cli-tool/internal/logger"
	"github.com/cli/go-cli-tool/internal/presentation"
	"github.com/cli/go-cli-tool/internal/telemetry"
	"github.com/cli/go-cli-tool/internal/tool"
	"github.com/cli/go-cli-tool/internal/tools"
)

// InteractiveShell implements Shell with basic input support
type InteractiveShell struct {
	// Core components
	logger    *logger.LoggerService
	config    *config.Config
	registry  *tools.ToolRegistry
	telemetry *telemetry.TelemetryService
	presenter *presentation.PresentationService

	// Shell state
	running bool
	mu      sync.RWMutex

	// Command handlers
	handlers map[string]CommandHandler

	// IO handler
	io tool.IOHandler

	// History file
	historyFile string

	// Prompt
	prompt string
}

// NewInteractiveShell creates a new interactive shell
func NewInteractiveShell() *InteractiveShell {
	return &InteractiveShell{
		handlers:    make(map[string]CommandHandler),
		running:     false,
		prompt:      "cli> ",
		historyFile: ".cli_history",
		io:          tool.NewDefaultIOHandler(),
	}
}

// SetLogger sets the logger
func (s *InteractiveShell) SetLogger(logger *logger.LoggerService) {
	s.logger = logger
}

// SetConfig sets the configuration
func (s *InteractiveShell) SetConfig(config config.Config) {
	s.config = &config
	if config.Shell.Prompt != "" {
		s.prompt = config.Shell.Prompt
	}
	if config.Shell.HistoryFile != "" {
		s.historyFile = config.Shell.HistoryFile
	}
}

// SetTelemetry sets the telemetry service
func (s *InteractiveShell) SetTelemetry(telemetry *telemetry.TelemetryService) {
	s.telemetry = telemetry
}

// SetPresenter sets the presentation service
func (s *InteractiveShell) SetPresenter(presenter *presentation.PresentationService) {
	s.presenter = presenter
}

// SetRegistry sets the tool registry
func (s *InteractiveShell) SetRegistry(registry interface {
	Get(name string) (tool.Tool, error)
	ListNames() []string
}) {
	// Type assertion to get the concrete type
	if reg, ok := registry.(*tools.ToolRegistry); ok {
		s.registry = reg
	}
}

// Start starts the interactive shell
func (s *InteractiveShell) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return fmt.Errorf("shell is already running")
	}

	s.running = true

	// Handle signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		for {
			select {
			case <-sigChan:
				s.Stop()
				return
			}
		}
	}()

	// Main loop
	go s.runLoop()

	return nil
}

// Stop stops the shell
func (s *InteractiveShell) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.running = false
	return nil
}

// IsRunning returns whether the shell is running
func (s *InteractiveShell) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

// Execute executes a command string
func (s *InteractiveShell) Execute(cmd string) (string, error) {
	if cmd == "" {
		return "", nil
	}

	// Parse command
	args := parseCommand(cmd)
	if len(args) == 0 {
		return "", nil
	}

	command := args[0]
	cmdArgs := args[1:]

	// Check for built-in commands first
	if IsBuiltinCommand(command) {
		return s.executeBuiltin(command, cmdArgs)
	}

	// Check custom handlers
	if handler, ok := s.handlers[command]; ok {
		ctx := s.createContext()
		return handler(cmdArgs, *ctx)
	}

	// Check for tool execution
	if s.registry != nil {
		if t, err := s.registry.Get(command); err == nil {
			return s.executeTool(t, cmdArgs)
		}
	}

	return "", fmt.Errorf("command not found: %s", command)
}

// RegisterCommand registers a command handler
func (s *InteractiveShell) RegisterCommand(name string, handler CommandHandler) {
	s.handlers[name] = handler
}

// runLoop runs the main shell loop
func (s *InteractiveShell) runLoop() {
	reader := bufio.NewReader(os.Stdin)

	for s.IsRunning() {
		fmt.Print(s.prompt)
		line, _, err := reader.ReadLine()
		if err != nil {
			break
		}

		cmd := strings.TrimSpace(string(line))
		if cmd == "" {
			continue
		}

		output, err := s.Execute(cmd)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		}
		if output != "" {
			fmt.Println(output)
		}
	}
}

// createContext creates a shell context
func (s *InteractiveShell) createContext() *ShellContext {
	return NewShellContext().
		WithConfig(s.config).
		WithLogger(s.logger).
		WithTelemetry(s.telemetry).
		WithPresenter(s.presenter).
		WithRegistry(s.registry).
		WithIO(s.io)
}

// executeBuiltin executes a built-in command
func (s *InteractiveShell) executeBuiltin(command string, args []string) (string, error) {
	switch command {
	case "exit", "quit":
		s.Stop()
		return "Goodbye!", nil

	case "help":
		return s.builtinHelp(args)

	case "list":
		return s.builtinList(args)

	case "exec":
		return s.builtinExec(args)

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
		return "", fmt.Errorf("unknown built-in command: %s", command)
	}
}

// builtinHelp shows help information
func (s *InteractiveShell) builtinHelp(args []string) (string, error) {
	if len(args) > 0 {
		toolName := args[0]
		if s.registry != nil {
			if t, err := s.registry.Get(toolName); err == nil {
				return fmt.Sprintf("%s (v%s)\n\n%s", t.Name(), t.Version(), t.Description()), nil
			}
		}
		return "", fmt.Errorf("tool not found: %s", toolName)
	}

	return "Available commands:\n" +
		"  exit, quit  - Exit the shell\n" +
		"  help [cmd]  - Show help information\n" +
		"  list        - List all available tools\n" +
		"  exec <tool> [args] - Execute a tool\n" +
		"  config      - Show current configuration\n" +
		"  set <key> <value> - Set configuration value\n" +
		"  log-level <level>  - Change log level\n" +
		"  clear       - Clear the screen\n" +
		"  version     - Show version information", nil
}

// builtinList lists all available tools
func (s *InteractiveShell) builtinList(args []string) (string, error) {
	if s.registry == nil {
		return "", fmt.Errorf("no tools registered")
	}

	tools := s.registry.ListNames()
	if len(tools) == 0 {
		return "No tools available", nil
	}

	var builder strings.Builder
	builder.WriteString("Available tools:\n")
	for _, name := range tools {
		builder.WriteString(fmt.Sprintf("  - %s\n", name))
	}
	return builder.String(), nil
}

// builtinExec executes a tool
func (s *InteractiveShell) builtinExec(args []string) (string, error) {
	if len(args) == 0 {
		return "", fmt.Errorf("usage: exec <tool> [args]")
	}

	toolName := args[0]
	toolArgs := args[1:]

	if s.registry == nil {
		return "", fmt.Errorf("no tools registered")
	}

	t, err := s.registry.Get(toolName)
	if err != nil {
		return "", err
	}

	return s.executeTool(t, toolArgs)
}

// builtinConfig shows configuration
func (s *InteractiveShell) builtinConfig(args []string) (string, error) {
	return fmt.Sprintf("Log Level: %s\nLog Format: %s\nPrompt: %s\nHistory File: %s",
		s.config.LogLevel, s.config.LogFormat, s.config.Shell.Prompt, s.config.Shell.HistoryFile), nil
}

// builtinSet sets a configuration value
func (s *InteractiveShell) builtinSet(args []string) (string, error) {
	if len(args) < 2 {
		return "", fmt.Errorf("usage: set <key> <value>")
	}

	key := args[0]
	value := args[1]

	switch key {
	case "prompt":
		s.config.Shell.Prompt = value
		s.prompt = value
		return fmt.Sprintf("Prompt set to: %s", value), nil
	case "log-level":
		s.config.LogLevel = value
		if s.logger != nil {
			s.logger.SetLevel(logger.ParseLogLevel(value))
		}
		return fmt.Sprintf("Log level set to: %s", value), nil
	default:
		return "", fmt.Errorf("unknown config key: %s", key)
	}
}

// builtinLogLevel changes the log level
func (s *InteractiveShell) builtinLogLevel(args []string) (string, error) {
	if len(args) == 0 {
		return fmt.Sprintf("Current log level: %s", s.config.LogLevel), nil
	}

	level := args[0]
	s.config.LogLevel = level
	if s.logger != nil {
		s.logger.SetLevel(logger.ParseLogLevel(level))
	}
	return fmt.Sprintf("Log level set to: %s", level), nil
}

// builtinClear clears the screen
func (s *InteractiveShell) builtinClear(args []string) (string, error) {
	fmt.Print("\033[2J\033[H")
	return "", nil
}

// builtinVersion shows version
func (s *InteractiveShell) builtinVersion(args []string) (string, error) {
	return "go-cli-tool v1.0.0", nil
}

// executeTool executes a tool with given arguments
func (s *InteractiveShell) executeTool(t tool.Tool, args []string) (string, error) {
	ctx := context.Background()
	if s.telemetry != nil {
		s.telemetry.TrackToolUsed(ctx, t.Name(), args)
	}
	start := time.Now()

	toolCtx := tool.NewContext().
		WithArgs(args).
		WithLogger(s.logger).
		WithIO(s.io).
		WithPresenter(s.presenter)

	result, err := t.Execute(*toolCtx)
	duration := time.Since(start).Seconds()

	if err != nil {
		if s.telemetry != nil {
			s.telemetry.TrackToolError(ctx, t.Name(), err)
		}
		return "", err
	}

	if !result.Success {
		if s.telemetry != nil {
			if result.Error != nil {
				s.telemetry.TrackToolError(ctx, t.Name(), result.Error)
			} else {
				s.telemetry.TrackToolError(ctx, t.Name(), fmt.Errorf("tool execution failed"))
			}
		}
		if result.Error != nil {
			return result.Output, result.Error
		}
		return result.Output, fmt.Errorf("tool execution failed")
	}

	if s.telemetry != nil {
		s.telemetry.TrackToolSuccess(ctx, t.Name(), duration)
	}

	return result.Output, nil
}

// parseCommand parses a command string into arguments
func parseCommand(cmd string) []string {
	var args []string
	var current strings.Builder
	inQuote := false
	quoteChar := ' '

	for _, ch := range cmd {
		if (ch == '"' || ch == '\'') && !inQuote {
			inQuote = true
			quoteChar = ch
		} else if ch == quoteChar && inQuote {
			inQuote = false
		} else if ch == ' ' && !inQuote {
			if current.Len() > 0 {
				args = append(args, current.String())
				current.Reset()
			}
		} else {
			current.WriteRune(ch)
		}
	}

	if current.Len() > 0 {
		args = append(args, current.String())
	}

	return args
}
