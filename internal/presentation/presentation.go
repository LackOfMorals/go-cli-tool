package presentation

import (
	"encoding/json"
	"fmt"

	"github.com/cli/go-cli-tool/internal/logger"
)

// OutputFormat defines the available output formats
type OutputFormat string

const (
	OutputFormatText OutputFormat = "text"
	OutputFormatJSON OutputFormat = "json"
)

// OutputFormatter defines the interface for formatting data
type OutputFormatter interface {
	Format(data any) (string, error)
}

// PresentationService manages the formatting of tool output
type PresentationService struct {
	format     OutputFormat
	formatters map[OutputFormat]OutputFormatter
	logger     logger.LoggerService
}

// NewPresentationService creates a new presentation service
func NewPresentationService(format OutputFormat, logger logger.LoggerService) *PresentationService {
	s := &PresentationService{
		format:     format,
		formatters: make(map[OutputFormat]OutputFormatter),
		logger:     logger,
	}

	// Register default formatters
	s.RegisterFormatter(OutputFormatText, &TextFormatter{})
	s.RegisterFormatter(OutputFormatJSON, &JSONFormatter{})

	return s
}

// RegisterFormatter adds a new formatter to the service
func (s *PresentationService) RegisterFormatter(format OutputFormat, formatter OutputFormatter) {
	s.formatters[format] = formatter
}

// SetFormat changes the current output format
func (s *PresentationService) SetFormat(format OutputFormat) {
	s.format = format
}

// Format transforms data into the current output format
func (s *PresentationService) Format(data any) (string, error) {
	formatter, ok := s.formatters[s.format]
	if !ok {
		s.logger.Warn("Formatter not found, falling back to text", logger.Field{Key: "format", Value: string(s.format)})
		formatter = s.formatters[OutputFormatText]
	}

	return formatter.Format(data)
}

// TextFormatter implements default text formatting
type TextFormatter struct{}

func (f *TextFormatter) Format(data any) (string, error) {
	// Simple string conversion for basic types
	if s, ok := data.(string); ok {
		return s, nil
	}

	// For maps or structs, we might want a more sophisticated text representation
	// For now, use fmt.Sprintf
	return fmt.Sprintf("%v", data), nil
}

// JSONFormatter implements JSON formatting
type JSONFormatter struct{}

func (f *JSONFormatter) Format(data any) (string, error) {
	bytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal to JSON: %w", err)
	}
	return string(bytes), nil
}
