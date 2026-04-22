package logger

// Defines the public interface for logging operations
type Service interface {
	Debug(msg string, fields ...Field)
	Info(msg string, fields ...Field)
	Warn(msg string, fields ...Field)
	Error(msg string, fields ...Field)
	Fatal(msg string, fields ...Field)
	SetLevel(level LogLevel)
	GetLevel() LogLevel
	WithFields(fields ...Field) Service
}
