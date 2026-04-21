package service

import (
	"context"
	"github.com/mixpanel/mixpanel-go"
	"github.com/cli/go-cli-tool/internal/core"
)

// TelemetryService defines the interface for tracking events
type TelemetryService interface {
	Track(ctx context.Context, eventName string, properties map[string]any) error
	TrackStartup(ctx context.Context) error
	TrackShutdown(ctx context.Context) error
	TrackToolUsed(ctx context.Context, toolName string, args []string) error
	TrackToolSuccess(ctx context.Context, toolName string, duration float64) error
	TrackToolError(ctx context.Context, toolName string, err error) error
}

// MixpanelService implements TelemetryService using the Mixpanel Go SDK
type MixpanelService struct {
	client *mixpanel.ApiClient
	logger core.Logger
	userID string
}

// NewMixpanelService creates a new Mixpanel telemetry service
func NewMixpanelService(token string, logger core.Logger) *MixpanelService {
	client := mixpanel.NewApiClient(token)
	return &MixpanelService{
		client: client,
		logger: logger,
		userID: "anonymous", // In a real app, this might be a machine ID or user ID
	}
}

func (s *MixpanelService) Track(ctx context.Context, eventName string, properties map[string]any) error {
	event := s.client.NewEvent(eventName, s.userID, properties)
	err := s.client.Track(ctx, []*mixpanel.Event{event})
	if err != nil {
		s.logger.Warn("Failed to send telemetry to Mixpanel", core.Field{Key: "error", Value: err.Error()})
	}
	return err
}

func (s *MixpanelService) TrackStartup(ctx context.Context) error {
	return s.Track(ctx, "app_startup", map[string]any{
		"os": "darwin", // Hardcoded for this example or use runtime.GOOS
	})
}

func (s *MixpanelService) TrackShutdown(ctx context.Context) error {
	return s.Track(ctx, "app_shutdown", nil)
}

func (s *MixpanelService) TrackToolUsed(ctx context.Context, toolName string, args []string) error {
	return s.Track(ctx, "tool_used", map[string]any{
		"tool": toolName,
		"args": args,
	})
}

func (s *MixpanelService) TrackToolSuccess(ctx context.Context, toolName string, duration float64) error {
	return s.Track(ctx, "tool_success", map[string]any{
		"tool":     toolName,
		"duration": duration,
	})
}

func (s *MixpanelService) TrackToolError(ctx context.Context, toolName string, err error) error {
	return s.Track(ctx, "tool_error", map[string]any{
		"tool":  toolName,
		"error": err.Error(),
	})
}
