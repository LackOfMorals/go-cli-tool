// Copyright (c) "Neo4j"
// Neo4j Sweden AB [http://neo4j.com]

package analytics

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/user"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cli/go-cli-tool/internal/logger"
	"github.com/denisbrodbeck/machineid"
	"github.com/google/uuid"
	mixpanel "github.com/mixpanel/mixpanel-go"
)

// httpClientTransport adapts our HTTPClient interface into an http.RoundTripper,
// allowing the Mixpanel SDK to use our injectable client (including mocks in tests).
// The endpoint is stored here so we can rewrite the URL on every request —
// the SDK resolves its own internal URL before hitting the transport, which
// would otherwise bypass our configured proxy endpoint.
type httpClientTransport struct {
	client   HTTPClient
	endpoint string
}

func (t *httpClientTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	path := strings.TrimLeft(req.URL.Path, "/")
	url := t.endpoint + "/" + path
	if req.URL.RawQuery != "" {
		url += "?" + req.URL.RawQuery
	}
	return t.client.Post(url, req.Header.Get("Content-Type"), req.Body)
}

type analyticsConfig struct {
	distinctID  string
	machineID   string
	binaryPath  string
	cliVersion  string
	token       string
	startupTime int64
	isAura      bool
	mp          *mixpanel.ApiClient
}

// eventBufferSize is the capacity of the internal event channel.
// If the buffer fills up (e.g. extended network outage) EmitEvent drops
// the event and logs a warning rather than blocking the caller.
const eventBufferSize = 64

type Analytics struct {
	disabled bool
	cfg      analyticsConfig
	log      logger.Service

	// eventCh carries events to the single background worker.
	// Closed by Flush() to signal the worker to drain and exit.
	eventCh chan TrackEvent

	// closed is set to 1 by Flush() before closing eventCh.
	// EmitEvent checks this to avoid a send-on-closed-channel panic.
	closed atomic.Bool

	// wg tracks the single worker goroutine so Flush() can wait for it.
	wg sync.WaitGroup
}

// NewAnalytics creates an Analytics instance using the default http.Client.
func NewAnalytics(mixPanelToken string, mixpanelEndpoint string, uri string, version string, log logger.Service) *Analytics {
	return NewAnalyticsWithClient(mixPanelToken, mixpanelEndpoint, &http.Client{Timeout: 10 * time.Second}, uri, version, log)
}

// NewAnalyticsWithClient creates an Analytics instance with an injectable HTTPClient,
// allowing tests to intercept outbound Mixpanel calls via a mock.
// log may be nil; in that case analytics logs nothing on the injected logger.
func NewAnalyticsWithClient(mixPanelToken string, mixpanelEndpoint string, client HTTPClient, uri string, version string, log logger.Service) *Analytics {
	endpoint := strings.TrimRight(mixpanelEndpoint, "/")

	var mpClient *mixpanel.ApiClient
	if client != nil {
		httpClient := &http.Client{Transport: &httpClientTransport{client: client, endpoint: endpoint}}
		mpClient = mixpanel.NewApiClient(mixPanelToken,
			mixpanel.HttpClient(httpClient),
		)
	} else {
		mpClient = mixpanel.NewApiClient(mixPanelToken,
			mixpanel.ProxyApiLocation(endpoint),
		)
	}

	a := &Analytics{
		log:     log,
		eventCh: make(chan TrackEvent, eventBufferSize),
		cfg: analyticsConfig{
			// Use the stable, OS-derived machine ID as the distinct ID so that
			// Mixpanel can correlate events across sessions for the same user.
			distinctID:  GetMachineID(),
			machineID:   GetMachineID(),
			binaryPath:  GetBinaryPath(),
			cliVersion:  version,
			token:       mixPanelToken,
			startupTime: time.Now().Unix(),
			isAura:      isAura(uri),
			mp:          mpClient,
		},
	}

	// Start the single background worker that serialises all Mixpanel calls.
	a.wg.Add(1)
	go a.worker()

	return a
}

// auraURIPattern matches the host patterns used by Neo4j Aura:
// databases.neo4j.io (classic) and instances.neo4j.io (multi-DB).
var auraURIPattern = regexp.MustCompile(`(databases|instances)\.neo4j\.io\b`)

// IsAuraURI reports whether uri points at a Neo4j Aura-managed instance.
// Exported so that tests and other packages can use it without duplicating
// the pattern.
func IsAuraURI(uri string) bool {
	return auraURIPattern.MatchString(uri)
}

// isAura is the internal alias used during construction.
func isAura(uri string) bool { return IsAuraURI(uri) }

// EmitToolEvent records a tool invocation outcome with all standard
// base properties. It is the preferred way to emit tool events from the
// shell because it ensures the correct event name and property set.
func (a *Analytics) EmitToolEvent(toolName string, success bool) {
	a.EmitEvent(a.NewToolEvent(toolName, success))
}

// EmitEvent queues an analytics event for the background worker.
// It never blocks: if the internal buffer is full the event is dropped
// and a warning is logged. Safe to call after Flush() — it is a no-op.
func (a *Analytics) EmitEvent(event TrackEvent) {
	if a.disabled || a.closed.Load() {
		return
	}
	select {
	case a.eventCh <- event:
		a.logDebug("queued analytics event", logger.Field{Key: "event", Value: event.Event})
	default:
		a.logWarn("analytics buffer full — dropping event",
			logger.Field{Key: "event", Value: event.Event},
			logger.Field{Key: "buffer_size", Value: eventBufferSize},
		)
	}
}

// Flush closes the event channel and blocks until the worker has sent every
// queued event. Call it once during application shutdown.
// After Flush returns, EmitEvent is a safe no-op.
func (a *Analytics) Flush() {
	// Mark as closed before closing the channel so EmitEvent's guard fires
	// first and we never race a send against a close.
	if a.closed.CompareAndSwap(false, true) {
		close(a.eventCh)
	}
	a.wg.Wait()
}

// worker is the single goroutine that drains eventCh and forwards events to
// Mixpanel. It exits when the channel is closed and fully drained (i.e. after
// Flush() is called).
func (a *Analytics) worker() {
	defer a.wg.Done()
	for event := range a.eventCh {
		if err := a.sendTrackEvent([]TrackEvent{event}); err != nil {
			a.logError("error sending analytics event",
				logger.Field{Key: "event", Value: event.Event},
				logger.Field{Key: "error", Value: err.Error()},
			)
		}
	}
}

func (a *Analytics) Enable()         { a.disabled = false }
func (a *Analytics) Disable()        { a.disabled = true }
func (a *Analytics) IsEnabled() bool { return !a.disabled }

func (a *Analytics) sendTrackEvent(events []TrackEvent) error {
	sdkEvents := make([]*mixpanel.Event, 0, len(events))
	for _, e := range events {
		props, err := toPropertiesMap(e.Properties)
		if err != nil {
			return fmt.Errorf("marshal properties for event %q: %w", e.Event, err)
		}
		sdkEvents = append(sdkEvents, a.cfg.mp.NewEvent(e.Event, a.cfg.distinctID, props))
	}

	if err := a.cfg.mp.Track(context.Background(), sdkEvents); err != nil {
		return fmt.Errorf("mixpanel track error: %w", err)
	}
	a.logDebug("sent event to Mixpanel", logger.Field{Key: "event", Value: sdkEvents[0].Name})
	return nil
}

// logDebug logs at debug level if a logger has been injected.
// Use for internal pipeline messages that are only relevant when diagnosing issues.
func (a *Analytics) logDebug(msg string, fields ...logger.Field) {
	if a.log != nil {
		a.log.Debug(msg, fields...)
	}
}

// logWarn logs at warn level if a logger has been injected.
func (a *Analytics) logWarn(msg string, fields ...logger.Field) {
	if a.log != nil {
		a.log.Warn(msg, fields...)
	}
}

// logInfo logs at info level if a logger has been injected.
func (a *Analytics) logInfo(msg string, fields ...logger.Field) {
	if a.log != nil {
		a.log.Info(msg, fields...)
	}
}

// logError logs at error level if a logger has been injected.
func (a *Analytics) logError(msg string, fields ...logger.Field) {
	if a.log != nil {
		a.log.Error(msg, fields...)
	}
}

// toPropertiesMap converts any properties struct to map[string]any via JSON
// so it's compatible with the SDK without duplicating field mappings.
func toPropertiesMap(props any) (map[string]any, error) {
	b, err := json.Marshal(props)
	if err != nil {
		return nil, err
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	return m, nil
}

// GetBinaryPath returns the absolute path of the running binary via os.Executable.
// Symlinks are resolved so the real on-disk path is reported.
// Any occurrence of the user's home directory or username is redacted.
// Returns an empty string on failure.
func GetBinaryPath() string {
	path, err := os.Executable()
	if err != nil {
		slog.Warn("Could not determine binary path for analytics", "error", err)
		return ""
	}

	path, err = filepath.EvalSymlinks(path)
	if err != nil {
		slog.Warn("Could not resolve binary path symlinks for analytics", "error", err)
		// Continue with the unresolved path rather than returning empty.
	}

	return redactPath(path)
}

// redactPath removes personally identifiable segments from a file path.
// It replaces the home directory prefix first (most specific), then falls back
// to replacing any remaining occurrences of the username.
func redactPath(path string) string {
	// 1. Replace home directory prefix — works on Linux (/home/user),
	//    macOS (/Users/user) and Windows (C:\Users\user).
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		// filepath.Rel gives us the portion after the home dir, without
		// needing to worry about slash style differences on Windows.
		if rel, err := filepath.Rel(home, path); err == nil && !strings.HasPrefix(rel, "..") {
			return filepath.Join("<home>", rel)
		}
	}

	// 2. Fallback: replace the username directly in case the home dir lookup
	//    failed or the binary lives outside the home dir but still contains
	//    the username (e.g. /tmp/username/bin).
	if user, err := user.Current(); err == nil && user.Username != "" {
		// On Windows, Current().Username may be "DOMAIN\user" — strip the domain.
		username := user.Username
		if idx := strings.LastIndex(username, `\`); idx != -1 {
			username = username[idx+1:]
		}
		path = strings.ReplaceAll(path, username, "<user>")
	}

	return path
}

// GetMachineID returns a stable, privacy-safe machine identifier using the OS-provided
// hardware UUID, HMAC-hashed with the app name so the raw system UUID is never exposed.
// Returns an empty string on failure (e.g. insufficient permissions on some Linux configs).
func GetMachineID() string {
	id, err := machineid.ProtectedID("neo4j-mcp-canary")
	if err != nil {
		slog.Warn("Could not retrieve machine ID for analytics", "error", err)
		return ""
	}
	return id
}

func GetDistinctID() string {
	id, err := uuid.NewV6()
	if err != nil {
		slog.Error("Error generating distinct ID for analytics", "error", err.Error())
		return ""
	}
	return id.String()
}
