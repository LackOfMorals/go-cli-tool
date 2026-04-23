// Package presentation formats command output in a chosen style
// (table, graph, json, pretty-json, text).
//
// All output flows through PresentationService.Format or FormatAs. Commands
// and tools build typed data objects (TableData, DetailData) and hand them
// to the service — they never format strings directly.
package presentation

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"

	"github.com/cli/go-cli-tool/internal/logger"
)

// ---- Output format constants --------------------------------------------

// OutputFormat identifies a registered output style.
type OutputFormat string

const (
	OutputFormatText       OutputFormat = "text"
	OutputFormatJSON       OutputFormat = "json"
	OutputFormatPrettyJSON OutputFormat = "pretty-json"
	OutputFormatTable      OutputFormat = "table"
	OutputFormatGraph      OutputFormat = "graph"
)

// IsValid reports whether f is one of the recognised formats.
func (f OutputFormat) IsValid() bool {
	switch f {
	case OutputFormatText, OutputFormatJSON, OutputFormatPrettyJSON,
		OutputFormatTable, OutputFormatGraph:
		return true
	}
	return false
}

// ---- PresentationService ------------------------------------------------

// PresentationService is the default Service implementation.
// All exported methods are safe for concurrent use.
type PresentationService struct {
	mu         sync.RWMutex
	format     OutputFormat
	formatters map[OutputFormat]OutputFormatter
	logger     logger.Service
}

// NewPresentationService builds a service with all built-in formatters
// registered. format is the default; use FormatAs for per-call overrides.
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

	_ = s.RegisterFormatter(OutputFormatText, &TextFormatter{})
	_ = s.RegisterFormatter(OutputFormatTable, &TableFormatter{})
	_ = s.RegisterFormatter(OutputFormatGraph, &GraphFormatter{})
	_ = s.RegisterFormatter(OutputFormatJSON, &JSONFormatter{Indent: false})
	_ = s.RegisterFormatter(OutputFormatPrettyJSON, &JSONFormatter{Indent: true})

	return s, nil
}

func (s *PresentationService) RegisterFormatter(format OutputFormat, formatter OutputFormatter) error {
	if formatter == nil {
		return fmt.Errorf("formatter for %q cannot be nil", format)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.formatters[format] = formatter
	return nil
}

func (s *PresentationService) SetFormat(format OutputFormat) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.formatters[format]; !ok {
		return fmt.Errorf("no formatter registered for format %q", format)
	}
	s.format = format
	return nil
}

// Format renders data using the current default format.
func (s *PresentationService) Format(data any) (string, error) {
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

// FormatAs renders data using a specific format regardless of the default.
// Use this for per-query format overrides (e.g. cypher --format graph).
func (s *PresentationService) FormatAs(data any, format OutputFormat) (string, error) {
	s.mu.RLock()
	formatter, ok := s.formatters[format]
	s.mu.RUnlock()

	if !ok {
		return "", fmt.Errorf("no formatter registered for format %q", format)
	}
	return formatter.Format(data)
}

// ---- TextFormatter ------------------------------------------------------

// TextFormatter renders data as human-readable text. For Tabular and
// DetailData it delegates to the TableFormatter so all table output looks
// consistent regardless of which format is active.
type TextFormatter struct{}

func (f *TextFormatter) Format(data any) (string, error) {
	if data == nil {
		return "", nil
	}
	// Delegate rich types to the table formatter.
	switch data.(type) {
	case Tabular, *DetailData:
		return (&TableFormatter{}).Format(data)
	}
	if s, ok := data.(fmt.Stringer); ok {
		return s.String(), nil
	}
	if s, ok := data.(string); ok {
		return s, nil
	}
	return fmt.Sprintf("%+v", data), nil
}

// ---- TableFormatter (go-pretty) -----------------------------------------

// TableFormatter renders Tabular and DetailData using go-pretty with a
// rounded Unicode border style.
type TableFormatter struct{}

func (f *TableFormatter) Format(data any) (string, error) {
	switch d := data.(type) {
	case Tabular:
		return f.renderTabular(d), nil
	case *DetailData:
		return f.renderDetail(d), nil
	case string:
		return d, nil
	default:
		return fmt.Sprintf("%v", data), nil
	}
}

func (f *TableFormatter) renderTabular(t Tabular) string {
	rows := t.Rows()
	if len(t.Columns()) == 0 || len(rows) == 0 {
		return "(no results)"
	}

	tw := table.NewWriter()
	tw.SetStyle(table.StyleRounded)
	tw.Style().Options.SeparateRows = false

	// Header
	header := make(table.Row, len(t.Columns()))
	for i, col := range t.Columns() {
		header[i] = col
	}
	tw.AppendHeader(header)

	// Column configs: right-align numeric columns, left-align everything else.
	colCfgs := make([]table.ColumnConfig, len(t.Columns()))
	for i := range t.Columns() {
		colCfgs[i] = table.ColumnConfig{Number: i + 1, Align: text.AlignLeft}
	}
	// Detect numeric columns from the first row.
	if len(rows) > 0 {
		for i, cell := range rows[0] {
			if isNumeric(cell) {
				colCfgs[i].Align = text.AlignRight
			}
		}
	}
	tw.SetColumnConfigs(colCfgs)

	// Rows
	for _, row := range rows {
		tr := make(table.Row, len(row))
		for i, cell := range row {
			tr[i] = FormatCellValue(cell)
		}
		tw.AppendRow(tr)
	}

	rowWord := "rows"
	if len(rows) == 1 {
		rowWord = "row"
	}
	tw.AppendFooter(table.Row{fmt.Sprintf("%d %s", len(rows), rowWord)})

	return tw.Render()
}

func (f *TableFormatter) renderDetail(d *DetailData) string {
	if len(d.Fields) == 0 {
		return ""
	}

	tw := table.NewWriter()
	tw.SetStyle(table.StyleRounded)
	tw.Style().Options.DrawBorder = true
	tw.Style().Options.SeparateHeader = false
	tw.Style().Options.SeparateRows = false

	if d.Title != "" {
		tw.SetTitle(d.Title)
	}

	tw.SetColumnConfigs([]table.ColumnConfig{
		{Number: 1, Align: text.AlignRight, Colors: text.Colors{text.Italic}},
		{Number: 2, Align: text.AlignLeft},
	})

	for _, f := range d.Fields {
		tw.AppendRow(table.Row{f.Label + ":", f.Value})
	}

	return tw.Render()
}

// isNumeric reports whether a value should be right-aligned.
func isNumeric(v interface{}) bool {
	switch v.(type) {
	case int, int8, int16, int32, int64,
		uint, uint8, uint16, uint32, uint64,
		float32, float64:
		return true
	}
	return false
}

// ---- JSONFormatter ------------------------------------------------------

// JSONFormatter serialises data as a JSON array (for Tabular) or object
// (for DetailData). With Indent true the output is pretty-printed.
type JSONFormatter struct {
	Indent bool
}

func (f *JSONFormatter) Format(data any) (string, error) {
	var v interface{}

	switch d := data.(type) {
	case Tabular:
		v = tabularToJSONSlice(d)
	case *DetailData:
		v = detailToJSONMap(d)
	default:
		v = data
	}

	var (
		b   []byte
		err error
	)
	if f.Indent {
		b, err = json.MarshalIndent(v, "", "  ")
	} else {
		b, err = json.Marshal(v)
	}
	if err != nil {
		return "", fmt.Errorf("marshal JSON: %w", err)
	}
	return string(b), nil
}

func tabularToJSONSlice(t Tabular) []map[string]interface{} {
	rows := make([]map[string]interface{}, 0, len(t.Rows()))
	for _, row := range t.Rows() {
		m := make(map[string]interface{}, len(t.Columns()))
		for i, col := range t.Columns() {
			if i < len(row) {
				m[col] = row[i]
			}
		}
		rows = append(rows, m)
	}
	return rows
}

func detailToJSONMap(d *DetailData) map[string]interface{} {
	m := make(map[string]interface{}, len(d.Fields))
	for _, f := range d.Fields {
		m[f.Label] = f.Value
	}
	return m
}

// ---- GraphFormatter -----------------------------------------------------

// GraphFormatter renders Tabular data as ASCII-art graphs or property lists.
//
// When rows contain node-shaped values (maps with "_labels") they are drawn
// as labelled boxes linked by relationship arrows. Scalar results use a
// property-list style that is visually distinct from the table view.
type GraphFormatter struct{}

func (f *GraphFormatter) Format(data any) (string, error) {
	switch d := data.(type) {
	case Tabular:
		return renderGraph(d), nil
	case *DetailData:
		return (&TableFormatter{}).renderDetail(d), nil
	case string:
		return d, nil
	default:
		return fmt.Sprintf("%v", data), nil
	}
}

func renderGraph(t Tabular) string {
	rows := t.Rows()
	if len(t.Columns()) == 0 || len(rows) == 0 {
		return "(no results)"
	}

	// Detect whether any cell is a graph entity.
	hasEntities := false
	for _, row := range rows {
		for _, cell := range row {
			if m, ok := cell.(map[string]interface{}); ok {
				if _, ok := m["_labels"]; ok {
					hasEntities = true
				}
				if _, ok := m["_type"]; ok {
					hasEntities = true
				}
			}
		}
	}

	if hasEntities {
		return renderEntityGraph(t)
	}
	return renderPropertyList(t)
}

// renderPropertyList renders scalar results as a bullet-point property list.
//
//	○ 1
//	├─ name: Keanu Reeves
//	└─ born: 1964
func renderPropertyList(t Tabular) string {
	var b strings.Builder
	cols := t.Columns()

	for idx, row := range t.Rows() {
		fmt.Fprintf(&b, "○ %d\n", idx+1)
		for i, col := range cols {
			val := FormatCellValue(row[i])
			prefix := "├─"
			if i == len(cols)-1 {
				prefix = "└─"
			}
			fmt.Fprintf(&b, "%s %s: %s\n", prefix, col, val)
		}
		b.WriteByte('\n')
	}

	rowWord := "rows"
	if len(t.Rows()) == 1 {
		rowWord = "row"
	}
	fmt.Fprintf(&b, "%d %s", len(t.Rows()), rowWord)
	return b.String()
}

// renderEntityGraph renders rows containing nodes and relationships as
// horizontal chains of labelled boxes.
func renderEntityGraph(t Tabular) string {
	var b strings.Builder
	cols := t.Columns()

	for _, row := range t.Rows() {
		var parts []string
		for i, col := range cols {
			_ = col
			v := row[i]
			m, ok := v.(map[string]interface{})
			if !ok {
				parts = append(parts, FormatCellValue(v))
				continue
			}
			if labels, ok := m["_labels"]; ok {
				parts = append(parts, renderNodeBox(labels, m))
			} else if relType, ok := m["_type"]; ok {
				parts = append(parts, fmt.Sprintf("─[:%v]─▶", relType))
			} else {
				parts = append(parts, FormatCellValue(v))
			}
		}
		b.WriteString(strings.Join(parts, " "))
		b.WriteString("\n\n")
	}

	rowWord := "rows"
	if len(t.Rows()) == 1 {
		rowWord = "row"
	}
	fmt.Fprintf(&b, "%d %s", len(t.Rows()), rowWord)
	return b.String()
}

// renderNodeBox draws a node as a Unicode double-border box.
//
//	╔══════════════════════════╗
//	║ :Person                  ║
//	╟──────────────────────────╢
//	║ born: 1964               ║
//	║ name: "Keanu Reeves"     ║
//	╚══════════════════════════╝
func renderNodeBox(labels interface{}, props map[string]interface{}) string {
	// Build label string: :Label1:Label2
	var labelParts []string
	switch ls := labels.(type) {
	case []string:
		for _, l := range ls {
			labelParts = append(labelParts, ":"+l)
		}
	case []interface{}:
		for _, l := range ls {
			labelParts = append(labelParts, fmt.Sprintf(":%v", l))
		}
	}
	labelStr := strings.Join(labelParts, "")

	// Build property lines (sorted, skipping internal _ keys).
	entityP := entityProps(props)
	propLines := make([]string, 0, len(entityP))
	keys := make([]string, 0, len(entityP))
	for k := range entityP {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		propLines = append(propLines, fmt.Sprintf("%s: %s", k, FormatPropValue(entityP[k])))
	}

	// Compute box width.
	width := len(labelStr)
	for _, l := range propLines {
		if len(l) > width {
			width = len(l)
		}
	}
	width += 2 // padding

	hLine := strings.Repeat("═", width+2)
	pad := func(s string) string { return fmt.Sprintf("║ %-*s ║", width, s) }

	var b strings.Builder
	fmt.Fprintf(&b, "╔%s╗\n", hLine)
	fmt.Fprintf(&b, "%s\n", pad(labelStr))
	if len(propLines) > 0 {
		fmt.Fprintf(&b, "╟%s╢\n", strings.Repeat("─", width+2))
		for _, l := range propLines {
			fmt.Fprintf(&b, "%s\n", pad(l))
		}
	}
	fmt.Fprintf(&b, "╚%s╝", hLine)
	return b.String()
}
