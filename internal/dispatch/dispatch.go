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

// CommandResult is the typed return value from every CommandHandler.
//
// Presentation and Item/Items/Message serve separate rendering channels:
//   - Presentation: used exclusively for human-mode table/detail rendering,
//     passed to PresentationService.Format or FormatAs.
//   - Item, Items, Message: used exclusively for JSON-mode output, assembled
//     into {"status":"ok","data":...} by the dispatcher.
//
// Exactly one of Item, Items, or Message should carry the JSON payload.
// Presentation may be nil for action-only results that carry only a Message.
type CommandResult struct {
	// Presentation is the typed data for human-mode rendering.
	// Pass *presentation.TableData for lists, *presentation.DetailData for
	// single records. Leave nil for action-only results.
	Presentation interface{}

	// FormatOverride selects a specific output format for this result,
	// overriding the global --format flag. Used by the cypher handler to
	// honour per-query --format flags (e.g. --format graph).
	FormatOverride presentation.OutputFormat

	// Item is a single structured record for JSON output.
	// Set for get, create, update, and action results that return a record.
	// All keys must be snake_case.
	Item map[string]interface{}

	// Items is a list of structured records for JSON output.
	// Set for list commands. Use a non-nil empty slice for empty lists;
	// nil is treated as "no list data" and falls back to Message.
	Items []map[string]interface{}

	// Message is a human-readable action confirmation.
	// In JSON mode this becomes {"message": "..."} in the data field.
	// Also used as the fallback human-mode output when Presentation is nil.
	Message string
}

// ---- Constructors -------------------------------------------------------

// ItemResult builds a CommandResult for a single record response (get, create, update).
func ItemResult(pres interface{}, item map[string]interface{}) CommandResult {
	return CommandResult{Presentation: pres, Item: item}
}

// ListResult builds a CommandResult for a list response.
// If items is nil it is replaced with an empty slice so JSON always emits [].
func ListResult(pres interface{}, items []map[string]interface{}) CommandResult {
	if items == nil {
		items = []map[string]interface{}{}
	}
	return CommandResult{Presentation: pres, Items: items}
}

// MessageResult builds a CommandResult for an action confirmation that does
// not return a structured record (delete, pause, reset, cancelled).
func MessageResult(msg string) CommandResult {
	return CommandResult{Message: msg}
}

// ---- CommandHandler -----------------------------------------------------

// CommandHandler is a function that handles a dispatched command.
type CommandHandler func(args []string, ctx Context) (CommandResult, error)

// ---- Context ------------------------------------------------------------

// Context carries the services a CommandHandler may need.
// It is a plain value type built fresh on each dispatch — no builder chain.
//
// Context.Context is the per-command context. Handlers should pass it to
// every service call instead of using context.Background().
//
// AgentMode and AllowWrites together implement the agent-mode permission
// model. The dispatcher enforces ModeWrite/ModeConditional commands before
// the handler is called; handlers can also inspect AgentMode directly to
// suppress interactive prompts.
type Context struct {
	Context   context.Context
	Config    config.Config
	Logger    logger.Service
	Telemetry analytics.Service
	Presenter presentation.Service
	Registry  Registry
	IO        tool.IOHandler

	// AgentMode is true when --agent / NEO4J_CLI_AGENT=true is set. It
	// activates non-interactive behaviour, JSON output, and read-only defaults.
	AgentMode bool

	// AllowWrites is true when --rw / NEO4J_CLI_RW=true is set. It grants
	// permission to execute write operations in agent mode. Has no effect
	// outside agent mode.
	AllowWrites bool

	// RequestID is included in every JSON response envelope to allow
	// orchestrators to correlate invocations.
	RequestID string
}
