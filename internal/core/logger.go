package core

import (
	"log/slog"
	"os"
	"strings"
)

// LogLevel represents the severity of a log message
type LogLevel string

const (
	LevelDebug LogLevel = "debug"
	LevelInfo  LogLevel = "info"
	LevelWarn  LogLevel = "warn"
	LevelError LogLevel = "error"
	LevelFatal LogLevel = "fatal"
)

// LogFormat represents the output format for logs
type LogFormat string

const (
	FormatText LogFormat = "text"
	FormatJSON LogFormat = "json"
)

// Field represents a log field key-value pair
type Field struct {
	Key   string
	Value interface{}
}

// Logger defines the interface for logging operations
type Logger interface {
	Debug(msg string, fields ...Field)
	Info(msg string, fields ...Field)
	Warn(msg string, fields ...Field)
	Error(msg string, fields ...Field)
	Fatal(msg string, fields ...Field)
	SetLevel(level LogLevel)
	GetLevel() LogLevel
	WithFields(fields ...Field) Logger
}

// slogLogger implements Logger using log/slog
type slogLogger struct {
	logger *slog.Logger
	level  *slog.LevelVar
	format LogFormat
}

// NewLogger creates a logger based on the specified format
func NewLogger(format LogFormat, level LogLevel) Logger {
	levelVar := &slog.LevelVar{}
	levelVar.Set(toSlogLevel(level))

	var handler slog.Handler
	opts := &slog.HandlerOptions{
		Level: levelVar,
	}

	if format == FormatJSON {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	return &slogLogger{
		logger: slog.New(handler),
		level:  levelVar,
		format: format,
	}
}

func (l *slogLogger) Debug(msg string, fields ...Field) {
	l.logger.Debug(msg, toSlogArgs(fields)...)
}

func (l *slogLogger) Info(msg string, fields ...Field) {
	l.logger.Info(msg, toSlogArgs(fields)...)
}

func (l *slogLogger) Warn(msg string, fields ...Field) {
	l.logger.Warn(msg, toSlogArgs(fields)...)
}

func (l *slogLogger) Error(msg string, fields ...Field) {
	l.logger.Error(msg, toSlogArgs(fields)...)
}

func (l *slogLogger) Fatal(msg string, fields ...Field) {
	l.logger.Error(msg, append(toSlogArgs(fields), slog.String("fatal", "true"))...)
	os.Exit(1)
}

func (l *slogLogger) SetLevel(level LogLevel) {
	l.level.Set(toSlogLevel(level))
}

func (l *slogLogger) GetLevel() LogLevel {
	return fromSlogLevel(l.level.Level())
}

func (l *slogLogger) WithFields(fields ...Field) Logger {
	return &slogLogger{
		logger: l.logger.With(toSlogArgs(fields)...),
		level:  l.level,
		format: l.format,
	}
}

// Helper functions for conversion

func toSlogLevel(level LogLevel) slog.Level {
	switch strings.ToLower(string(level)) {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	case "fatal":
		return slog.LevelError + 4 // Custom fatal level
	default:
		return slog.LevelInfo
	}
}

func fromSlogLevel(level slog.Level) LogLevel {
	switch level {
	case slog.LevelDebug:
		return LevelDebug
	case slog.LevelInfo:
		return LevelInfo
	case slog.LevelWarn:
		return LevelWarn
	case slog.LevelError:
		return LevelError
	default:
		if level > slog.LevelError {
			return LevelFatal
		}
		return LevelInfo
	}
}

func toSlogArgs(fields []Field) []any {
	args := make([]any, len(fields))
	for i, f := range fields {
		args[i] = slog.Any(f.Key, f.Value)
	}
	return args
}

// ParseLogLevel converts a string to LogLevel
func ParseLogLevel(level string) LogLevel {
	switch strings.ToLower(level) {
	case "debug":
		return LevelDebug
	case "info":
		return LevelInfo
	case "warn", "warning":
		return LevelWarn
	case "error":
		return LevelError
	case "fatal":
		return LevelFatal
	default:
		return LevelInfo
	}
}

// ParseLogFormat converts a string to LogFormat
func ParseLogFormat(format string) LogFormat {
	if strings.ToLower(format) == "json" {
		return FormatJSON
	}
	return FormatText
}
