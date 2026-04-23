// Copyright (c) "Neo4j"
// Neo4j Sweden AB [http://neo4j.com]

package analytics

//go:generate mockgen -destination=mocks/mock_analytics.go -package=analytics_mocks -typed github.com/cli/go-cli-tool/internal/analytics Service,HTTPClient

import (
	"io"
	"net/http"
)

// HTTPClient is the subset of *http.Client used by Analytics, allowing injection of a mock in tests.
type HTTPClient interface {
	Post(url, contentType string, body io.Reader) (*http.Response, error)
}

type Service interface {
	Disable()
	Enable()
	IsEnabled() bool
	// EmitEvent queues a pre-built event. Prefer EmitToolEvent for tool
	// invocations — it attaches all standard properties automatically.
	EmitEvent(event TrackEvent)
	// EmitToolEvent records a tool invocation with the correct event name and
	// all standard base properties (OS, machine ID, uptime, etc.).
	EmitToolEvent(toolName string, success bool)
	// Flush blocks until all in-flight async EmitEvent goroutines have completed.
	// Call it during shutdown to avoid dropping events.
	Flush()
}
