package logger

import (
	"testing"
)

func TestLoggerLevel(t *testing.T) {
	logger := NewLoggerService(FormatText, LevelInfo)

	// Test SetLevel
	logger.SetLevel(LevelDebug)
	if logger.GetLevel() != LevelDebug {
		t.Errorf("Expected level %v, got %v", LevelDebug, logger.GetLevel())
	}

	// Set back to info level
	logger.SetLevel(LevelInfo)
	if logger.GetLevel() != LevelInfo {
		t.Errorf("Expected level %v, got %v", LevelInfo, logger.GetLevel())
	}
}

func TestParseLogLevel(t *testing.T) {
	tests := []struct {
		input    string
		expected LogLevel
	}{
		{"debug", LevelDebug},
		{"DEBUG", LevelDebug},
		{"Debug", LevelDebug},
		{"info", LevelInfo},
		{"INFO", LevelInfo},
		{"warn", LevelWarn},
		{"WARN", LevelWarn},
		{"warning", LevelWarn},
		{"error", LevelError},
		{"ERROR", LevelError},
		{"fatal", LevelFatal},
		{"FATAL", LevelFatal},
		{"unknown", LevelInfo}, // Default
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := ParseLogLevel(tt.input)
			if result != tt.expected {
				t.Errorf("ParseLogLevel(%s) = %v, expected %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestParseLogFormat(t *testing.T) {
	tests := []struct {
		input    string
		expected LogFormat
	}{
		{"json", FormatJSON},
		{"JSON", FormatJSON},
		{"Json", FormatJSON},
		{"text", FormatText},
		{"TEXT", FormatText},
		{"unknown", FormatText}, // Default
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := ParseLogFormat(tt.input)
			if result != tt.expected {
				t.Errorf("ParseLogFormat(%s) = %v, expected %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestNewLogger(t *testing.T) {
	textLogger := NewLoggerService(FormatText, LevelInfo)
	if textLogger == nil {
		t.Error("Expected non-nil text logger")
	}

	jsonLogger := NewLoggerService(FormatJSON, LevelInfo)
	if jsonLogger == nil {
		t.Error("Expected non-nil json logger")
	}
}

func TestWithFields(t *testing.T) {
	logger := NewLoggerService(FormatText, LevelInfo)
	loggerWithFields := logger.WithFields(Field{Key: "app", Value: "test"})

	// Should return a Logger interface
	if loggerWithFields == nil {
		t.Error("Expected non-nil logger from WithFields")
	}
}

func TestField(t *testing.T) {
	field := Field{Key: "test", Value: "value"}

	if field.Key != "test" {
		t.Errorf("Expected key 'test', got %s", field.Key)
	}

	if field.Value != "value" {
		t.Errorf("Expected value 'value', got %v", field.Value)
	}
}
