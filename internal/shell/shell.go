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

// Registry is the interface the shell uses for tool lookup.
// *tools.ToolRegistry satisfies it; any other implementation does too.
type Registry interface {
	Get(name string) (tool.Tool, error)
	ListNames() []string
}

// Shell defines the interface for CLI shell operations.
type Shell interface {
	Start() error
	Stop() error
	IsRunning() bool
	Execute(cmd string) (string, error)
	RegisterCommand(name string, handler CommandHandler)
	SetCategories(categories map[string]*Category)
	SetVersion(v string)

	SetLogger(log logger.Service)
	SetConfig(cfg config.Config)
	SetTelemetry(tel analytics.Service)
	SetPresenter(p *presentation.PresentationService)
	SetRegistry(r Registry)
}

// CommandHandler is a function that handles a shell command.
type CommandHandler func(args []string, ctx ShellContext) (string, error)

// ShellContext carries the services a CommandHandler may need.
// It is a plain value type built fresh on each dispatch — no builder chain.
type ShellContext struct {
	Config    config.Config
	Logger    logger.Service
	Telemetry analytics.Service
	Presenter *presentation.PresentationService
	Registry  Registry
	IO        tool.IOHandler
}

// ---- Built-in command lookup --------------------------------------------

// builtins is a set for O(1) membership testing.
var builtins = map[string]struct{}{
	"exit": {}, "quit": {}, "help": {}, "config": {},
	"set": {}, "log-level": {}, "clear": {}, "version": {},
}

// IsBuiltinCommand reports whether name is reserved by the shell.
func IsBuiltinCommand(name string) bool {
	_, ok := builtins[name]
	return ok
}

// ---- Help overview ------------------------------------------------------

// CategoryHelpOverview builds the top-level help string from the category map.
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
