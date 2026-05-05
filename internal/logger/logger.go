package logger

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

// ---- Config types -------------------------------------------------------

// LogLevel represents the severity of a log message
type LogLevel string

const (
	LevelDebug LogLevel = "debug"
	LevelInfo  LogLevel = "info"
	LevelWarn  LogLevel = "warn"
	LevelError LogLevel = "error"
	// LevelFatal is accepted in config and flag values for familiarity, but no
	// Fatal() method is exposed on the Service interface. Callers that
	// encounter an unrecoverable error should return it up the call stack so
	// that deferred cleanup (driver close, analytics flush, etc.) runs before
	// the process exits via the os.Exit in main.
	LevelFatal LogLevel = "fatal"
)

// LogFormat represents the output format for logs
type LogFormat string

const (
	FormatText LogFormat = "text"
	FormatJSON LogFormat = "json"
)

// LogOutput controls where log records are written.
type LogOutput string

const (
	// OutputStderr writes all log records to stderr (default). This keeps
	// stdout clean for command output and allows shell redirection of logs
	// independently: neo4j-cli 2>neo4j-cli.log
	OutputStderr LogOutput = "stderr"

	// OutputStdout writes all log records to stdout. Useful when the caller
	// wants a single combined stream (e.g. piping through a log aggregator).
	OutputStdout LogOutput = "stdout"

	// OutputFile writes all log records to a file. The path is provided
	// separately; when it is empty, DefaultLogFilePath() is used.
	OutputFile LogOutput = "file"
)

// DefaultLogFilePath returns the OS-appropriate default log file path when
// log_output = file and no explicit log_file is configured.
//
//	Linux/macOS  ~/.neo4j-cli/neo4j-cli.log
//	Windows      %USERPROFILE%\.neo4j-cli\neo4j-cli.log
func DefaultLogFilePath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(os.TempDir(), "neo4j-cli.log")
	}
	return filepath.Join(home, ".neo4j-cli", "neo4j-cli.log")
}

// Field represents a log field key-value pair
type Field struct {
	Key   string
	Value interface{}
}

// ---- loggerService ------------------------------------------------------

// loggerService is the concrete implementation of the logger service
type loggerService struct {
	logger *slog.Logger
	level  *slog.LevelVar
	format LogFormat
}

// ---- Service ------------------------------------------------------------

// NewLoggerService creates a logger that writes all records to stderr.
// This is the preferred default for interactive CLI use: stderr is separate
// from command output on stdout and can be redirected independently.
func NewLoggerService(format LogFormat, defaultLevel LogLevel) Service {
	return NewLoggerServiceToWriter(format, defaultLevel, os.Stderr)
}

// NewLoggerServiceToWriter creates a logger that writes all records to w.
// Use this when the caller has already resolved the output destination
// (e.g. an open log file, or os.Stdout for pipeline mode). The caller is
// responsible for closing w when it is no longer needed.
func NewLoggerServiceToWriter(format LogFormat, defaultLevel LogLevel, w io.Writer) Service {
	levelVar := &slog.LevelVar{}
	levelVar.Set(toSlogLevel(defaultLevel))

	opts := &slog.HandlerOptions{Level: levelVar}

	var handler slog.Handler
	if format == FormatJSON {
		handler = slog.NewJSONHandler(w, opts)
	} else {
		handler = slog.NewTextHandler(w, opts)
	}

	return &loggerService{
		logger: slog.New(handler),
		level:  levelVar,
		format: format,
	}
}

// OpenLogFile opens (or creates) the log file at path for append writes.
// The caller is responsible for calling Close() on the returned file.
// If path is empty, DefaultLogFilePath() is used.
func OpenLogFile(path string) (*os.File, error) {
	if path == "" {
		path = DefaultLogFilePath()
	}
	if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
		return nil, fmt.Errorf("create log directory: %w", err)
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0640)
	if err != nil {
		return nil, fmt.Errorf("open log file %q: %w", path, err)
	}
	return f, nil
}

func (l *loggerService) Debug(msg string, fields ...Field) {
	l.logger.Debug(msg, toSlogArgs(fields)...)
}

func (l *loggerService) Info(msg string, fields ...Field) {
	l.logger.Info(msg, toSlogArgs(fields)...)
}

func (l *loggerService) Warn(msg string, fields ...Field) {
	l.logger.Warn(msg, toSlogArgs(fields)...)
}

func (l *loggerService) Error(msg string, fields ...Field) {
	l.logger.Error(msg, toSlogArgs(fields)...)
}

func (l *loggerService) SetLevel(level LogLevel) {
	l.level.Set(toSlogLevel(level))
}

func (l *loggerService) GetLevel() LogLevel {
	return fromSlogLevel(l.level.Level())
}

func (l *loggerService) WithFields(fields ...Field) Service {
	return &loggerService{
		logger: l.logger.With(toSlogArgs(fields)...),
		level:  l.level,
		format: l.format,
	}
}

// ---- Helpers ------------------------------------------------------------

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
		// Treated as a level above error for filtering purposes; no Fatal()
		// method exists on Service — see LevelFatal comment above.
		return slog.LevelError + 4
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

// ParseLogLevel converts a string to LogLevel.
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

// ParseLogFormat converts a string to LogFormat.
func ParseLogFormat(format string) LogFormat {
	if strings.ToLower(format) == "json" {
		return FormatJSON
	}
	return FormatText
}

// ParseLogOutput converts a string to LogOutput.
// Unknown values fall back to OutputStderr.
func ParseLogOutput(output string) LogOutput {
	switch strings.ToLower(strings.TrimSpace(output)) {
	case "stdout":
		return OutputStdout
	case "file":
		return OutputFile
	default:
		return OutputStderr
	}
}

// WriterFor returns the io.Writer for the given output destination.
// For OutputFile it returns nil — the caller must open a file and use
// NewLoggerServiceToWriter directly. For stdout/stderr the returned writer
// is one of the os standard streams.
func WriterFor(output LogOutput) io.Writer {
	switch output {
	case OutputStdout:
		return os.Stdout
	default: // OutputStderr and unknown values
		return os.Stderr
	}
}


