package shell

import (
	"fmt"
	"sort"
	"strings"

	"github.com/cli/go-cli-tool/internal/analytics"
	"github.com/cli/go-cli-tool/internal/config"
	"github.com/cli/go-cli-tool/internal/logger"
	"github.com/cli/go-cli-tool/internal/presentation"
	"github.com/cli/go-cli-tool/internal/tool"
)

// ---- Shell interface ----------------------------------------------------

// Shell defines the interface for CLI shell operations.
type Shell interface {
	Start() error
	Stop() error
	IsRunning() bool
	Execute(cmd string) (string, error)
	RegisterCommand(name string, handler CommandHandler)
	SetCategories(categories map[string]*Category)

	SetLogger(log logger.Service)
	SetConfig(cfg config.Config)
	SetTelemetry(tel analytics.Service)
	SetPresenter(p *presentation.PresentationService)
	SetRegistry(registry interface {
		Get(name string) (tool.Tool, error)
		ListNames() []string
	})
}

// CommandHandler is a function that handles a shell command.
type CommandHandler func(args []string, ctx ShellContext) (string, error)

// ---- ShellContext -------------------------------------------------------

// ShellContext carries the services a CommandHandler may need.
type ShellContext struct {
	Config    config.Config
	Logger    logger.Service
	Telemetry analytics.Service
	Presenter *presentation.PresentationService
	Registry  interface {
		Get(name string) (tool.Tool, error)
		ListNames() []string
	}
	IO tool.IOHandler
}

func NewShellContext() *ShellContext { return &ShellContext{} }

func (c *ShellContext) WithConfig(cfg *config.Config) *ShellContext {
	if cfg != nil {
		c.Config = *cfg
	}
	return c
}

func (c *ShellContext) WithLogger(log logger.Service) *ShellContext {
	c.Logger = log
	return c
}

func (c *ShellContext) WithTelemetry(tel analytics.Service) *ShellContext {
	c.Telemetry = tel
	return c
}

func (c *ShellContext) WithPresenter(p *presentation.PresentationService) *ShellContext {
	c.Presenter = p
	return c
}

func (c *ShellContext) WithRegistry(r interface {
	Get(name string) (tool.Tool, error)
	ListNames() []string
}) *ShellContext {
	c.Registry = r
	return c
}

func (c *ShellContext) WithIO(io tool.IOHandler) *ShellContext {
	c.IO = io
	return c
}

// ---- Built-in command names ---------------------------------------------

// BuiltinCommands returns the names of commands reserved by the shell itself.
// These take precedence over categories and the tool registry.
func BuiltinCommands() []string {
	return []string{"exit", "quit", "help", "config", "set", "log-level", "clear", "version"}
}

// IsBuiltinCommand reports whether name is a built-in shell command.
func IsBuiltinCommand(name string) bool {
	for _, cmd := range BuiltinCommands() {
		if cmd == name {
			return true
		}
	}
	return false
}

// ---- helpOverview -------------------------------------------------------

// CategoryHelpOverview builds the top-level help string given a map of
// categories. Pulled here so it can be called from both the shell and tests.
func CategoryHelpOverview(categories map[string]*Category) string {
	var b strings.Builder

	if len(categories) > 0 {
		fmt.Fprintln(&b, "Categories:")
		names := make([]string, 0, len(categories))
		for n := range categories {
			names = append(names, n)
		}
		sort.Strings(names)
		for _, n := range names {
			fmt.Fprintf(&b, "  %-18s %s\n", n, categories[n].Description)
		}
		fmt.Fprintln(&b)
	}

	fmt.Fprintln(&b, "Built-in commands:")
	fmt.Fprintln(&b, "  exit / quit        Exit the shell")
	fmt.Fprintln(&b, "  help [cat [sub]]   Show this help or help for a category")
	fmt.Fprintln(&b, "  config             Show current configuration")
	fmt.Fprintln(&b, "  set <key> <val>    Set a config value (prompt, log-level)")
	fmt.Fprintln(&b, "  log-level [level]  Get or set the log level")
	fmt.Fprintln(&b, "  clear              Clear the screen")
	fmt.Fprintln(&b, "  version            Show version")

	return strings.TrimRight(b.String(), "\n")
}
