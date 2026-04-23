// Package presentation formats command output for a CLI in a chosen style
// (text, JSON, table, ...).
package presentation

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/cli/go-cli-tool/internal/logger"
)

// OutputFormat identifies a registered output style.
type OutputFormat string

const (
	OutputFormatText  OutputFormat = "text"
	OutputFormatJSON  OutputFormat = "json"
	OutputFormatTable OutputFormat = "table"
)

// IsValid reports whether f is one of the built-in formats. Custom formats
// registered via RegisterFormatter are not checked here — this is a guard
// against typos in config, not a registry lookup.
func (f OutputFormat) IsValid() bool {
	switch f {
	case OutputFormatText, OutputFormatJSON, OutputFormatTable:
		return true
	}
	return false
}

// PresentationService is the default Service implementation.
// All exported methods are safe for concurrent use by multiple goroutines.
type PresentationService struct {
	mu         sync.RWMutex
	format     OutputFormat
	formatters map[OutputFormat]OutputFormatter
	logger     logger.Service
}

// NewPresentationService builds a service with the three built-in formatters
// (text, JSON, table) registered and the given format selected.
//
// Returns an error if the logger is nil or the format is unknown — callers
// should surface these at startup rather than hitting them mid-command.
func NewPresentationService(format OutputFormat, log logger.Service) (*PresentationService, error) {
	if log == nil {
		return nil, fmt.Errorf("logger is required")
	}
	if !format.IsValid() {
		return nil, fmt.Errorf("invalid output format: %q", format)
	}

	s := &PresentationService{
		format:     format,
		formatters: make(map[OutputFormat]OutputFormatter),
		logger:     log,
	}

	// Built-in formatters. Errors here would be programmer errors (nil
	// formatter literal), caught in tests, so ignoring them is safe.
	_ = s.RegisterFormatter(OutputFormatText, &TextFormatter{})
	_ = s.RegisterFormatter(OutputFormatJSON, &JSONFormatter{Indent: true})
	_ = s.RegisterFormatter(OutputFormatTable, &TableFormatter{})

	return s, nil
}

// RegisterFormatter adds or replaces the formatter for a given format.
func (s *PresentationService) RegisterFormatter(format OutputFormat, formatter OutputFormatter) error {
	if formatter == nil {
		return fmt.Errorf("formatter for %q cannot be nil", format)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.formatters[format] = formatter
	return nil
}

// SetFormat switches the active format. The format must already be registered.
func (s *PresentationService) SetFormat(format OutputFormat) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.formatters[format]; !ok {
		return fmt.Errorf("no formatter registered for format %q", format)
	}
	s.format = format
	return nil
}

// Format renders data using the current format. If the current format has no
// registered formatter it falls back to text; if text is also unavailable it
// returns an error rather than panicking.
//
// The formatters map and active format are read under a shared (read) lock.
// The actual formatting work happens outside the lock so a slow formatter
// cannot block concurrent Format or SetFormat calls.
func (s *PresentationService) Format(data any) (string, error) {
	// Capture both the primary formatter and the text fallback atomically so
	// we never observe a partially-updated formatters map.
	s.mu.RLock()
	formatter, ok := s.formatters[s.format]
	var fallback OutputFormatter
	if !ok {
		fallback = s.formatters[OutputFormatText]
	}
	currentFormat := s.format
	s.mu.RUnlock()

	if !ok {
		if fallback == nil {
			return "", fmt.Errorf("no formatter for %q and no text fallback available", currentFormat)
		}
		s.logger.Warn("formatter not found, falling back to text",
			logger.Field{Key: "format", Value: string(currentFormat)})
		formatter = fallback
	}
	return formatter.Format(data)
}

// TextFormatter renders data as human-readable text.
//
// Resolution order:
//  1. nil             -> ""
//  2. fmt.Stringer    -> String()
//  3. string          -> returned unchanged
//  4. Tabular         -> rendered as an aligned table
//  5. anything else   -> fmt.Sprintf("%+v", data) so struct fields are labeled
type TextFormatter struct{}

func (f *TextFormatter) Format(data any) (string, error) {
	if data == nil {
		return "", nil
	}
	if s, ok := data.(fmt.Stringer); ok {
		return s.String(), nil
	}
	if s, ok := data.(string); ok {
		return s, nil
	}
	if t, ok := data.(Tabular); ok {
		return renderTable(t.Columns(), t.Rows()), nil
	}
	return fmt.Sprintf("%+v", data), nil
}

// JSONFormatter renders data as JSON. With Indent true (the default in
// NewPresentationService) output is pretty-printed for humans; set Indent
// false for compact output that pipes cleanly into jq and friends.
type JSONFormatter struct {
	Indent bool
}

func (f *JSONFormatter) Format(data any) (string, error) {
	var (
		b   []byte
		err error
	)
	if f.Indent {
		b, err = json.MarshalIndent(data, "", "  ")
	} else {
		b, err = json.Marshal(data)
	}
	if err != nil {
		return "", fmt.Errorf("failed to marshal to JSON: %w", err)
	}
	return string(b), nil
}

// TableFormatter renders Tabular data as aligned columns. Data that doesn't
// implement Tabular returns an error — the user asked for a table and we
// can't fabricate columns from arbitrary types.
type TableFormatter struct{}

func (f *TableFormatter) Format(data any) (string, error) {
	t, ok := data.(Tabular)
	if !ok {
		return "", fmt.Errorf("table format requires data to implement presentation.Tabular (got %T)", data)
	}
	return renderTable(t.Columns(), t.Rows()), nil
}

// renderTable returns a two-space-padded, left-aligned rendering with a
// dashed separator under the header. Short rows are padded; long rows are
// truncated to the column count.
func renderTable(columns []string, rows [][]string) string {
	widths := make([]int, len(columns))
	for i, c := range columns {
		widths[i] = len(c)
	}
	for _, row := range rows {
		for i := 0; i < len(columns) && i < len(row); i++ {
			if len(row[i]) > widths[i] {
				widths[i] = len(row[i])
			}
		}
	}

	var b strings.Builder
	writeRow := func(cells []string) {
		for i, w := range widths {
			var cell string
			if i < len(cells) {
				cell = cells[i]
			}
			b.WriteString(cell)
			if i < len(widths)-1 {
				b.WriteString(strings.Repeat(" ", w-len(cell)+2))
			}
		}
		b.WriteByte('\n')
	}

	writeRow(columns)
	sep := make([]string, len(columns))
	for i, w := range widths {
		sep[i] = strings.Repeat("-", w)
	}
	writeRow(sep)
	for _, row := range rows {
		writeRow(row)
	}
	return b.String()
}
