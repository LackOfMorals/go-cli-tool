package analytics_test

import (
	"encoding/json"
	"io"
	"net/http"
	"runtime"
	"strings"
	"testing"

	"github.com/cli/go-cli-tool/internal/analytics"
	amocks "github.com/cli/go-cli-tool/internal/analytics/mocks"

	"go.uber.org/mock/gomock"
)

// newTestAnalytics creates an Analytics instance wired to a mock HTTP client.
func newTestAnalytics(t *testing.T, client analytics.HTTPClient) *analytics.Analytics {
	t.Helper()
	return analytics.NewAnalyticsWithClient("test-token", "http://localhost", client, "bolt://localhost:7687")
}

// decodeProperties marshals props through JSON and returns a flat map so tests
// can assert individual field values without caring about the concrete struct type.
func decodeProperties(t *testing.T, props interface{}) map[string]interface{} {
	t.Helper()
	b, err := json.Marshal(props)
	if err != nil {
		t.Fatalf("marshal properties: %v", err)
	}
	var m map[string]interface{}
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("unmarshal properties to map: %v", err)
	}
	return m
}

func assertBaseProperties(t *testing.T, props interface{}) map[string]interface{} {
	t.Helper()
	m := decodeProperties(t, props)
	if m["token"] != "test-token" {
		t.Errorf("token: got %v, want test-token", m["token"])
	}
	if _, ok := m["time"].(float64); !ok {
		t.Error("time is not a number")
	}
	if _, ok := m["distinct_id"].(string); !ok {
		t.Error("distinct_id is not a string")
	}
	if _, ok := m["$insert_id"].(string); !ok {
		t.Error("$insert_id is not a string")
	}
	if _, ok := m["uptime"].(float64); !ok {
		t.Error("uptime is not a number")
	}
	if m["$os"] != runtime.GOOS {
		t.Errorf("$os: got %v, want %v", m["$os"], runtime.GOOS)
	}
	if m["os_arch"] != runtime.GOARCH {
		t.Errorf("os_arch: got %v, want %v", m["os_arch"], runtime.GOARCH)
	}
	return m
}

// ---- Emit behaviour -------------------------------------------------------

func TestEmitEvent_Disabled(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockClient := amocks.NewMockHTTPClient(ctrl)
	// No Post calls expected — the mock will fail the test if Post is called.

	svc := newTestAnalytics(t, mockClient)
	svc.Disable()
	svc.EmitEvent(analytics.TrackEvent{Event: "should_not_be_sent"})
}

func TestEmitEvent_Enabled(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockClient := amocks.NewMockHTTPClient(ctrl)

	mockClient.EXPECT().
		Post(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(&http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader("1")),
		}, nil)

	svc := newTestAnalytics(t, mockClient)
	svc.EmitEvent(analytics.TrackEvent{Event: "test_event"})
}

func TestEmitEvent_CorrectURL(t *testing.T) {
	tests := []struct {
		name     string
		endpoint string
		wantURL  string
	}{
		{"trailing slash", "http://localhost/", "http://localhost/track?verbose=1"},
		{"no trailing slash", "http://localhost", "http://localhost/track?verbose=1"},
		{"double trailing slash", "http://localhost//", "http://localhost/track?verbose=1"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockClient := amocks.NewMockHTTPClient(ctrl)

			mockClient.EXPECT().
				Post(tc.wantURL, gomock.Any(), gomock.Any()).
				Return(&http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader("1")),
				}, nil)

			svc := analytics.NewAnalyticsWithClient("test-token", tc.endpoint, mockClient, "")
			svc.EmitEvent(analytics.TrackEvent{Event: "url_test"})
		})
	}
}

func TestEmitEvent_CorrectBody(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockClient := amocks.NewMockHTTPClient(ctrl)

	event := analytics.TrackEvent{
		Event:      "body_test",
		Properties: map[string]interface{}{"key": "value"},
	}

	mockClient.EXPECT().
		Post(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_, _ string, body io.Reader) (*http.Response, error) {
			b, _ := io.ReadAll(body)
			var events []analytics.TrackEvent
			if err := json.Unmarshal(b, &events); err != nil {
				t.Fatalf("unmarshal body: %v", err)
			}
			if len(events) != 1 {
				t.Fatalf("expected 1 event, got %d", len(events))
			}
			if events[0].Event != "body_test" {
				t.Errorf("event name: got %s, want body_test", events[0].Event)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader("1")),
			}, nil
		})

	svc := newTestAnalytics(t, mockClient)
	svc.EmitEvent(event)
}

// ---- Enable / Disable / IsEnabled ----------------------------------------

func TestEnableDisable(t *testing.T) {
	svc := newTestAnalytics(t, nil)

	if !svc.IsEnabled() {
		t.Error("should be enabled by default")
	}

	svc.Disable()
	if svc.IsEnabled() {
		t.Error("should be disabled after Disable()")
	}

	svc.Enable()
	if !svc.IsEnabled() {
		t.Error("should be enabled after Enable()")
	}
}

// ---- Event constructors --------------------------------------------------

func TestNewStartupEvent(t *testing.T) {
	svc := newTestAnalytics(t, nil)
	event := svc.NewStartupEvent()

	if !strings.HasSuffix(event.Event, "STARTUP") {
		t.Errorf("unexpected event name: %s", event.Event)
	}
	assertBaseProperties(t, event.Properties)
}

func TestNewToolEvent(t *testing.T) {
	svc := newTestAnalytics(t, nil)

	t.Run("success", func(t *testing.T) {
		event := svc.NewToolEvent("echo", true)
		if !strings.HasSuffix(event.Event, "TOOL_USED") {
			t.Errorf("unexpected event name: %s", event.Event)
		}
		props := assertBaseProperties(t, event.Properties)
		if props["tool_name"] != "echo" {
			t.Errorf("tool_name: got %v, want echo", props["tool_name"])
		}
		if props["success"] != true {
			t.Errorf("success: got %v, want true", props["success"])
		}
	})

	t.Run("failure", func(t *testing.T) {
		event := svc.NewToolEvent("query", false)
		props := assertBaseProperties(t, event.Properties)
		if props["success"] != false {
			t.Errorf("success: got %v, want false", props["success"])
		}
	})
}
