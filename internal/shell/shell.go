package shell

import (
	"github.com/cli/go-cli-tool/internal/config"

	"github.com/cli/go-cli-tool/internal/logger"
	"github.com/cli/go-cli-tool/internal/presentation"
	"github.com/cli/go-cli-tool/internal/telemetry"
	"github.com/cli/go-cli-tool/internal/tool"
)

// Shell defines the interface for CLI shell operations
type Shell interface {
	// Start starts the shell
	Start() error

	// Stop stops the shell
	Stop() error

	// Execute executes a command string
	Execute(cmd string) (string, error)

	// RegisterCommand registers a command handler
	RegisterCommand(name string, handler CommandHandler)

	// SetLogger sets the logger
	SetLogger(logger logger.LoggerService)

	// SetConfig sets the configuration
	SetConfig(config config.Config)

	// SetTelemetry sets the telemetry service
	SetTelemetry(telemetry telemetry.TelemetryService)

	// SetPresenter sets the presentation service
	SetPresenter(presenter *presentation.PresentationService)

	// SetRegistry sets the tool registry
	SetRegistry(registry interface {
		Get(name string) (tool.Tool, error)
		ListNames() []string
	})

	// IsRunning returns whether the shell is running
	IsRunning() bool
}

// CommandHandler is a function that handles shell commands
type CommandHandler func(args []string, ctx ShellContext) (string, error)

// ShellContext provides context for command execution
type ShellContext struct {
	// Config is the application configuration
	Config config.Config

	// Logger is the logger instance
	Logger logger.LoggerService

	// Telemetry is the telemetry service
	Telemetry telemetry.TelemetryService

	// Presenter is the presentation service
	Presenter *presentation.PresentationService

	// Tools is a reference to the tool registry
	Registry interface {
		Get(name string) (tool.Tool, error)
		ListNames() []string
	}

	// IO is the IO handler
	IO tool.IOHandler
}

// NewShellContext creates a new shell context
func NewShellContext() *ShellContext {
	return &ShellContext{}
}

// WithConfig sets the configuration
func (c *ShellContext) WithConfig(config *config.Config) *ShellContext {
	c.Config = *config
	return c
}

// WithLogger sets the logger
func (c *ShellContext) WithLogger(logger *logger.LoggerService) *ShellContext {
	c.Logger = *logger
	return c
}

// WithTelemetry sets the telemetry service
func (c *ShellContext) WithTelemetry(telemetry *telemetry.TelemetryService) *ShellContext {
	c.Telemetry = *telemetry
	return c
}

// WithPresenter sets the presentation service
func (c *ShellContext) WithPresenter(presenter *presentation.PresentationService) *ShellContext {
	c.Presenter = presenter
	return c
}

// WithRegistry sets the tool registry
func (c *ShellContext) WithRegistry(registry interface {
	Get(name string) (tool.Tool, error)
	ListNames() []string
}) *ShellContext {
	c.Registry = registry
	return c
}

// WithIO sets the IO handler
func (c *ShellContext) WithIO(io tool.IOHandler) *ShellContext {
	c.IO = io
	return c
}

// BuiltinCommands returns all built-in command names
func BuiltinCommands() []string {
	return []string{
		"exit",
		"quit",
		"help",
		"list",
		"exec",
		"config",
		"set",
		"log-level",
		"clear",
		"version",
	}
}

// IsBuiltinCommand checks if a command is built-in
func IsBuiltinCommand(name string) bool {
	for _, cmd := range BuiltinCommands() {
		if cmd == name {
			return true
		}
	}
	return false
}
