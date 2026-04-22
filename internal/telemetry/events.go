package telemetry
import (
	"runtime"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/cli/go-cli-tool/internal/logger"

	
)

const eventNamePrefix = "NEO4JCLI"

type TrackEvent struct {
	Event      string      `json:"event"`
	Properties interface{} `json:"properties"`
}

// baseProperties are the base properties attached to a MixPanel "track" event.
// DistinctID is a distinct ID used to identify unique users, we do not use this information, therefore for us it will be distinct different executions.
// InsertID is used to deduplicate duplicate messages.
type baseProperties struct {
	Token      string `json:"token"`
	Time       int64  `json:"time"`
	DistinctID string `json:"distinct_id"`
	InsertID   string `json:"$insert_id"`
	Uptime     int64  `json:"uptime"`
	OS         string `json:"$os"`
	OSArch     string `json:"os_arch"`
	IsAura     bool   `json:"isAura"`
	CLIVersion string `json:"cli_version"`
}


// cliStartupProperties contains information available at startup 
type cliStartupProperties struct {
	baseProperties
}


// toolProperties contains tool event properties
type toolProperties struct {
	baseProperties
	ToolUsed string `json:"tools_used"`
	Success  bool   `json:"success"`
}




// NewStartupEvent creates a server startup event with information available immediately (no DB query)
func (t *TelemetryService) NewStartupEvent(transportMode config.TransportMode, tlsEnabled bool, mcpVersion string) TrackEvent {
	props := serverStartupProperties{
		baseProperties: a.getBaseProperties(),
		McpVersion:     mcpVersion,
		TransportMode:  transportMode,
	}

	// Only include TLS field for HTTP mode (omitted for STDIO via omitempty tag with nil pointer)
	if props.TransportMode == config.TransportModeHTTP {
		props.TLSEnabled = &tlsEnabled
	}

	return TrackEvent{
		Event:      strings.Join([]string{eventNamePrefix, "MCP_STARTUP"}, "_"),
		Properties: props,
	}
}


