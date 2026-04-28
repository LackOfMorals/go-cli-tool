package dispatch

import (
	"context"

	"github.com/cli/go-cli-tool/internal/analytics"
	"github.com/cli/go-cli-tool/internal/config"
	"github.com/cli/go-cli-tool/internal/logger"
	"github.com/cli/go-cli-tool/internal/presentation"
	"github.com/cli/go-cli-tool/internal/tool"
)

// Registry is the interface used for tool lookup.
// *tools.ToolRegistry satisfies it; any other implementation does too.
type Registry interface {
	Get(name string) (tool.Tool, error)
	ListNames() []string
}

// CommandHandler is a function that handles a dispatched command.
type CommandHandler func(args []string, ctx Context) (string, error)

// Context carries the services a CommandHandler may need.
// It is a plain value type built fresh on each dispatch — no builder chain.
//
// Context.Context is the per-command context. Handlers should pass it to
// every service call instead of using context.Background().
type Context struct {
	Context   context.Context
	Config    config.Config
	Logger    logger.Service
	Telemetry analytics.Service
	Presenter *presentation.PresentationService
	Registry  Registry
	IO        tool.IOHandler
}
