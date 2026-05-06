package logger

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ---- Level --------------------------------------------------------------

func TestLoggerLevel(t *testing.T) {
	log := NewLoggerService(FormatText, LevelInfo)
	log.SetLevel(LevelDebug)
	if log.GetLevel() != LevelDebug {
		t.Errorf("got %v, want %v", log.GetLevel(), LevelDebug)
	}
	log.SetLevel(LevelInfo)
	if log.GetLevel() != LevelInfo {
		t.Errorf("got %v, want %v", log.GetLevel(), LevelInfo)
	}
}

// ---- ParseLogLevel ------------------------------------------------------

func TestParseLogLevel(t *testing.T) {
	tests := []struct {
		input string
		want  LogLevel
	}{
		{"debug", LevelDebug}, {"DEBUG", LevelDebug}, {"Debug", LevelDebug},
		{"info", LevelInfo}, {"INFO", LevelInfo},
		{"warn", LevelWarn}, {"WARN", LevelWarn}, {"warning", LevelWarn},
		{"error", LevelError}, {"ERROR", LevelError},
		{"fatal", LevelFatal}, {"FATAL", LevelFatal},
		{"unknown", LevelInfo},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := ParseLogLevel(tt.input); got != tt.want {
				t.Errorf("ParseLogLevel(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

// ---- ParseLogFormat -----------------------------------------------------

func TestParseLogFormat(t *testing.T) {
	tests := []struct {
		input string
		want  LogFormat
	}{
		{"json", FormatJSON}, {"JSON", FormatJSON}, {"Json", FormatJSON},
		{"text", FormatText}, {"TEXT", FormatText}, {"unknown", FormatText},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := ParseLogFormat(tt.input); got != tt.want {
				t.Errorf("ParseLogFormat(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

// ---- ParseLogOutput -----------------------------------------------------

func TestParseLogOutput(t *testing.T) {
	tests := []struct {
		input string
		want  LogOutput
	}{
		{"stderr", OutputStderr},
		{"STDERR", OutputStderr},
		{"stdout", OutputStdout},
		{"STDOUT", OutputStdout},
		{"file", OutputFile},
		{"FILE", OutputFile},
		{"", OutputStderr},    // empty → default
		{"unknown", OutputStderr}, // unknown → default
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := ParseLogOutput(tt.input); got != tt.want {
				t.Errorf("ParseLogOutput(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

// ---- WriterFor ----------------------------------------------------------

func TestWriterFor(t *testing.T) {
	if WriterFor(OutputStderr) != os.Stderr {
		t.Error("WriterFor(stderr) should return os.Stderr")
	}
	if WriterFor(OutputStdout) != os.Stdout {
		t.Error("WriterFor(stdout) should return os.Stdout")
	}
	// Unknown values fall back to stderr.
	if WriterFor("unknown") != os.Stderr {
		t.Error("WriterFor(unknown) should fall back to os.Stderr")
	}
}

// ---- NewLoggerServiceToWriter -------------------------------------------

func TestNewLoggerServiceToWriter_WritesToProvidedWriter(t *testing.T) {
	var buf bytes.Buffer
	log := NewLoggerServiceToWriter(FormatText, LevelDebug, &buf)

	log.Debug("hello debug")
	log.Info("hello info")
	log.Warn("hello warn")
	log.Error("hello error")

	out := buf.String()
	for _, msg := range []string{"hello debug", "hello info", "hello warn", "hello error"} {
		if !strings.Contains(out, msg) {
			t.Errorf("expected %q in output, got:\n%s", msg, out)
		}
	}
}

func TestNewLoggerServiceToWriter_JSONFormat(t *testing.T) {
	var buf bytes.Buffer
	log := NewLoggerServiceToWriter(FormatJSON, LevelInfo, &buf)
	log.Info("json test", Field{Key: "k", Value: "v"})

	out := buf.String()
	if !strings.Contains(out, `"msg":"json test"`) {
		t.Errorf("expected JSON-formatted output, got: %s", out)
	}
	if !strings.Contains(out, `"k":"v"`) {
		t.Errorf("expected field k=v in JSON output, got: %s", out)
	}
}

func TestNewLoggerServiceToWriter_LevelFiltering(t *testing.T) {
	var buf bytes.Buffer
	log := NewLoggerServiceToWriter(FormatText, LevelWarn, &buf)

	log.Debug("should not appear")
	log.Info("should not appear")
	log.Warn("should appear")
	log.Error("should appear")

	out := buf.String()
	if strings.Contains(out, "should not appear") {
		t.Errorf("Debug/Info messages should be suppressed at Warn level, got:\n%s", out)
	}
	if !strings.Contains(out, "should appear") {
		t.Errorf("Warn/Error messages should appear at Warn level, got:\n%s", out)
	}
}

// ---- NewLoggerService default (stderr) ----------------------------------

func TestNewLoggerService_NonNil(t *testing.T) {
	if NewLoggerService(FormatText, LevelInfo) == nil {
		t.Error("NewLoggerService returned nil")
	}
	if NewLoggerService(FormatJSON, LevelInfo) == nil {
		t.Error("NewLoggerService(JSON) returned nil")
	}
}

// ---- WithFields ---------------------------------------------------------

func TestWithFields_AttrsAppearInOutput(t *testing.T) {
	var buf bytes.Buffer
	log := NewLoggerServiceToWriter(FormatText, LevelInfo, &buf)
	log.WithFields(Field{Key: "app", Value: "test"}).Info("msg")

	if !strings.Contains(buf.String(), "app=test") {
		t.Errorf("expected field in output, got: %s", buf.String())
	}
}

// ---- OpenLogFile --------------------------------------------------------

func TestOpenLogFile_CreatesFileAndDirectory(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "subdir", "nctl.log")

	f, err := OpenLogFile(path)
	if err != nil {
		t.Fatalf("OpenLogFile: %v", err)
	}
	t.Cleanup(func() { _ = f.Close() })

	// File must be writable.
	if _, err := f.WriteString("test entry\n"); err != nil {
		t.Errorf("WriteString: %v", err)
	}

	// Verify the file exists on disk.
	if _, err := os.Stat(path); err != nil {
		t.Errorf("file not found after open: %v", err)
	}
}

func TestOpenLogFile_DefaultPath(t *testing.T) {
	// When path is empty, DefaultLogFilePath() is used.
	// We don't actually open it to avoid side effects in tests;
	// just verify DefaultLogFilePath returns a non-empty string.
	p := DefaultLogFilePath()
	if p == "" {
		t.Error("DefaultLogFilePath returned empty string")
	}
	if !strings.Contains(p, "nctl") {
		t.Errorf("DefaultLogFilePath should contain 'nctl', got: %s", p)
	}
}

func TestOpenLogFile_AppendsToExistingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "append.log")

	// Write first entry.
	f1, _ := OpenLogFile(path)
	_, _ = f1.WriteString("line1\n")
	_ = f1.Close()

	// Reopen and write second entry — must not truncate.
	f2, err := OpenLogFile(path)
	if err != nil {
		t.Fatalf("second OpenLogFile: %v", err)
	}
	_, _ = f2.WriteString("line2\n")
	_ = f2.Close()

	data, _ := os.ReadFile(path)
	content := string(data)
	if !strings.Contains(content, "line1") || !strings.Contains(content, "line2") {
		t.Errorf("expected both lines in file, got:\n%s", content)
	}
}

// ---- Field --------------------------------------------------------------

func TestField(t *testing.T) {
	f := Field{Key: "k", Value: "v"}
	if f.Key != "k" || f.Value != "v" {
		t.Errorf("Field: got {%q, %v}", f.Key, f.Value)
	}
}
