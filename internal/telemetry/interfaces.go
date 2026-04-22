package telemetry



// TelemetryService defines the interface for tracking events
type Service interface {
	Enable()
	Disable()
	Track(ctx context.Context, eventName string, properties map[string]any) error
	TrackStartup(ctx context.Context) error
	TrackShutdown(ctx context.Context) error
	TrackToolUsed(ctx context.Context, toolName string, args []string) error
	TrackToolSuccess(ctx context.Context, toolName string, duration float64) error
	TrackToolError(ctx context.Context, toolName string, err error) error
}