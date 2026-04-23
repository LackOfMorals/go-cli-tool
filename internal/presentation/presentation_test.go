package presentation_test

import (
	"strings"
	"sync"
	"testing"

	"github.com/cli/go-cli-tool/internal/logger"
	"github.com/cli/go-cli-tool/internal/presentation"
)

// newTestService creates a PresentationService for tests, failing immediately
// if construction fails.
func newTestService(t *testing.T, format presentation.OutputFormat) *presentation.PresentationService {
	t.Helper()
	log := logger.NewLoggerService(logger.FormatText, logger.LevelError)
	svc, err := presentation.NewPresentationService(format, log)
	if err != nil {
		t.Fatalf("NewPresentationService: %v", err)
	}
	return svc
}

// ---- Construction -------------------------------------------------------

func TestNewPresentationService_InvalidFormat(t *testing.T) {
	log := logger.NewLoggerService(logger.FormatText, logger.LevelError)
	_, err := presentation.NewPresentationService("invalid", log)
	if err == nil {
		t.Error("expected error for invalid format, got nil")
	}
}

func TestNewPresentationService_NilLogger(t *testing.T) {
	_, err := presentation.NewPresentationService(presentation.OutputFormatText, nil)
	if err == nil {
		t.Error("expected error for nil logger, got nil")
	}
}

// ---- Format -------------------------------------------------------------

func TestFormat_Text_String(t *testing.T) {
	svc := newTestService(t, presentation.OutputFormatText)
	out, err := svc.Format("hello")
	if err != nil {
		t.Fatalf("Format: %v", err)
	}
	if out != "hello" {
		t.Errorf("got %q, want %q", out, "hello")
	}
}

func TestFormat_Text_Nil(t *testing.T) {
	svc := newTestService(t, presentation.OutputFormatText)
	out, err := svc.Format(nil)
	if err != nil {
		t.Fatalf("Format(nil): %v", err)
	}
	if out != "" {
		t.Errorf("got %q, want empty string", out)
	}
}

func TestFormat_JSON(t *testing.T) {
	svc := newTestService(t, presentation.OutputFormatJSON)
	out, err := svc.Format(map[string]string{"key": "value"})
	if err != nil {
		t.Fatalf("Format: %v", err)
	}
	if !strings.Contains(out, `"key"`) {
		t.Errorf("expected JSON output to contain key, got: %s", out)
	}
}

// ---- SetFormat ----------------------------------------------------------

func TestSetFormat_Valid(t *testing.T) {
	svc := newTestService(t, presentation.OutputFormatText)
	if err := svc.SetFormat(presentation.OutputFormatJSON); err != nil {
		t.Fatalf("SetFormat(JSON): %v", err)
	}
	// Verify the format switch took effect.
	out, err := svc.Format(map[string]int{"n": 1})
	if err != nil {
		t.Fatalf("Format after SetFormat: %v", err)
	}
	if !strings.Contains(out, `"n"`) {
		t.Errorf("expected JSON output after SetFormat, got: %s", out)
	}
}

func TestSetFormat_Unregistered(t *testing.T) {
	svc := newTestService(t, presentation.OutputFormatText)
	if err := svc.SetFormat("nonexistent"); err == nil {
		t.Error("expected error when switching to unregistered format")
	}
}

// ---- RegisterFormatter --------------------------------------------------

func TestRegisterFormatter_NilFormatter(t *testing.T) {
	svc := newTestService(t, presentation.OutputFormatText)
	if err := svc.RegisterFormatter(presentation.OutputFormatText, nil); err == nil {
		t.Error("expected error for nil formatter")
	}
}

func TestRegisterFormatter_CustomFormatter(t *testing.T) {
	svc := newTestService(t, presentation.OutputFormatText)

	// Register a custom formatter under an arbitrary format name.
	const customFmt presentation.OutputFormat = "upper"
	_ = svc.RegisterFormatter(customFmt, &upperFormatter{})
	if err := svc.SetFormat(customFmt); err != nil {
		t.Fatalf("SetFormat custom: %v", err)
	}

	out, err := svc.Format("hello")
	if err != nil {
		t.Fatalf("Format with custom formatter: %v", err)
	}
	if out != "HELLO" {
		t.Errorf("got %q, want %q", out, "HELLO")
	}
}

// upperFormatter is a test-only OutputFormatter that upper-cases strings.
type upperFormatter struct{}

func (f *upperFormatter) Format(data any) (string, error) {
	if s, ok := data.(string); ok {
		return strings.ToUpper(s), nil
	}
	return "", nil
}

// ---- Concurrency --------------------------------------------------------

// TestPresentationServiceConcurrent verifies that concurrent calls to Format,
// RegisterFormatter, and SetFormat do not trigger the race detector.
//
// Run with: go test -race ./internal/presentation/...
func TestPresentationServiceConcurrent(t *testing.T) {
	svc := newTestService(t, presentation.OutputFormatText)

	var wg sync.WaitGroup
	const goroutines = 20

	// Concurrent reads via Format.
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = svc.Format("concurrent data")
		}()
	}

	// Concurrent write via RegisterFormatter.
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = svc.RegisterFormatter(presentation.OutputFormatJSON, &presentation.JSONFormatter{Indent: false})
	}()

	// Concurrent format switch via SetFormat.
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = svc.SetFormat(presentation.OutputFormatJSON)
	}()

	// Concurrent switch back.
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = svc.SetFormat(presentation.OutputFormatText)
	}()

	wg.Wait()
}
