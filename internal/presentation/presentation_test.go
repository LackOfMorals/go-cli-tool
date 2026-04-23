package presentation_test

import (
	"strings"
	"sync"
	"testing"

	"github.com/cli/go-cli-tool/internal/logger"
	"github.com/cli/go-cli-tool/internal/presentation"
)

func newTestService(t *testing.T, format presentation.OutputFormat) *presentation.PresentationService {
	t.Helper()
	log := logger.NewLoggerService(logger.FormatText, logger.LevelError)
	svc, err := presentation.NewPresentationService(format, log)
	if err != nil {
		t.Fatalf("NewPresentationService: %v", err)
	}
	return svc
}

func sampleTableData() *presentation.TableData {
	return presentation.NewTableData(
		[]string{"name", "age"},
		[][]interface{}{
			{"Alice", int64(30)},
			{"Bob", int64(25)},
		},
	)
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

// ---- Format: table -------------------------------------------------------

func TestFormat_Table_ContainsColumnsAndRows(t *testing.T) {
	svc := newTestService(t, presentation.OutputFormatTable)
	out, err := svc.Format(sampleTableData())
	if err != nil {
		t.Fatalf("Format: %v", err)
	}
	for _, want := range []string{"name", "age", "Alice", "Bob", "30", "25"} {
		if !strings.Contains(out, want) {
			t.Errorf("output should contain %q:\n%s", want, out)
		}
	}
}

func TestFormat_Table_NoResults(t *testing.T) {
	svc := newTestService(t, presentation.OutputFormatTable)
	out, err := svc.Format(presentation.NewTableData([]string{"name"}, nil))
	if err != nil {
		t.Fatalf("Format: %v", err)
	}
	if !strings.Contains(out, "no results") {
		t.Errorf("expected no-results message, got: %q", out)
	}
}

func TestFormat_Table_DetailData(t *testing.T) {
	svc := newTestService(t, presentation.OutputFormatTable)
	detail := presentation.NewDetailData("Instance", []presentation.DetailField{
		{Label: "ID", Value: "abc-123"},
		{Label: "Name", Value: "prod-db"},
	})
	out, err := svc.Format(detail)
	if err != nil {
		t.Fatalf("Format: %v", err)
	}
	if !strings.Contains(out, "abc-123") {
		t.Errorf("output should contain ID value, got: %q", out)
	}
	if !strings.Contains(out, "prod-db") {
		t.Errorf("output should contain name value, got: %q", out)
	}
}

// ---- Format: JSON -------------------------------------------------------

func TestFormat_JSON_ProducesArray(t *testing.T) {
	svc := newTestService(t, presentation.OutputFormatJSON)
	out, err := svc.Format(sampleTableData())
	if err != nil {
		t.Fatalf("Format: %v", err)
	}
	if !strings.HasPrefix(out, "[") {
		t.Errorf("expected JSON array, got: %s", out)
	}
	if !strings.Contains(out, `"name"`) {
		t.Errorf("expected JSON to contain column names, got: %s", out)
	}
}

func TestFormat_PrettyJSON_IsIndented(t *testing.T) {
	svc := newTestService(t, presentation.OutputFormatPrettyJSON)
	out, err := svc.Format(sampleTableData())
	if err != nil {
		t.Fatalf("Format: %v", err)
	}
	if !strings.Contains(out, "\n") {
		t.Errorf("expected indented JSON output, got: %s", out)
	}
}

func TestFormat_JSON_DetailData_ProducesObject(t *testing.T) {
	svc := newTestService(t, presentation.OutputFormatJSON)
	detail := presentation.NewDetailData("", []presentation.DetailField{
		{Label: "ID", Value: "abc"},
	})
	out, err := svc.Format(detail)
	if err != nil {
		t.Fatalf("Format: %v", err)
	}
	if !strings.HasPrefix(out, "{") {
		t.Errorf("expected JSON object for DetailData, got: %s", out)
	}
}

// ---- Format: graph -------------------------------------------------------

func TestFormat_Graph_ScalarRendersPropertyList(t *testing.T) {
	svc := newTestService(t, presentation.OutputFormatGraph)
	out, err := svc.Format(sampleTableData())
	if err != nil {
		t.Fatalf("Format: %v", err)
	}
	if !strings.Contains(out, "Alice") {
		t.Errorf("graph output should contain value Alice:\n%s", out)
	}
	if !strings.Contains(out, "○") {
		t.Errorf("graph output should use ○ bullets:\n%s", out)
	}
}

func TestFormat_Graph_NodeRendersBox(t *testing.T) {
	svc := newTestService(t, presentation.OutputFormatGraph)
	nodeData := presentation.NewTableData(
		[]string{"n"},
		[][]interface{}{{
			map[string]interface{}{
				"_labels": []string{"Person"},
				"_id":     "4:...:1",
				"name":    "Alice",
				"born":    1964,
			},
		}},
	)
	out, err := svc.Format(nodeData)
	if err != nil {
		t.Fatalf("Format: %v", err)
	}
	if !strings.Contains(out, ":Person") {
		t.Errorf("graph output should show label :Person:\n%s", out)
	}
	if !strings.Contains(out, "Alice") {
		t.Errorf("graph output should show property value:\n%s", out)
	}
}

// ---- FormatAs -----------------------------------------------------------

func TestFormatAs_OverridesDefault(t *testing.T) {
	// Default is table, but FormatAs with JSON should produce JSON.
	svc := newTestService(t, presentation.OutputFormatTable)
	out, err := svc.FormatAs(sampleTableData(), presentation.OutputFormatJSON)
	if err != nil {
		t.Fatalf("FormatAs: %v", err)
	}
	if !strings.HasPrefix(out, "[") {
		t.Errorf("expected JSON output from FormatAs, got: %s", out)
	}
}

func TestFormatAs_UnknownFormat_ReturnsError(t *testing.T) {
	svc := newTestService(t, presentation.OutputFormatTable)
	_, err := svc.FormatAs(sampleTableData(), "nonexistent")
	if err == nil {
		t.Error("expected error for unknown format in FormatAs")
	}
}

// ---- Text format --------------------------------------------------------

func TestFormat_Text_DelegatesToTable(t *testing.T) {
	svc := newTestService(t, presentation.OutputFormatText)
	out, err := svc.Format(sampleTableData())
	if err != nil {
		t.Fatalf("Format: %v", err)
	}
	// Text format delegates to table, so should contain column names.
	if !strings.Contains(out, "name") {
		t.Errorf("text format should show column names, got: %s", out)
	}
}

func TestFormat_Text_PlainString(t *testing.T) {
	svc := newTestService(t, presentation.OutputFormatText)
	out, err := svc.Format("hello")
	if err != nil {
		t.Fatalf("Format: %v", err)
	}
	if out != "hello" {
		t.Errorf("got %q, want %q", out, "hello")
	}
}

// ---- SetFormat ----------------------------------------------------------

func TestSetFormat_Valid(t *testing.T) {
	svc := newTestService(t, presentation.OutputFormatText)
	if err := svc.SetFormat(presentation.OutputFormatJSON); err != nil {
		t.Fatalf("SetFormat(JSON): %v", err)
	}
	out, err := svc.Format(sampleTableData())
	if err != nil {
		t.Fatalf("Format after SetFormat: %v", err)
	}
	if !strings.Contains(out, `"name"`) {
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

type upperFormatter struct{}

func (f *upperFormatter) Format(data any) (string, error) {
	if s, ok := data.(string); ok {
		return strings.ToUpper(s), nil
	}
	return "", nil
}

// ---- Cell formatting ----------------------------------------------------

func TestFormatCellValue_Node(t *testing.T) {
	node := map[string]interface{}{
		"_labels": []string{"Person"},
		"_id":     "4:...:1",
		"name":    "Alice",
		"born":    1964,
	}
	out := presentation.FormatCellValue(node)
	if !strings.Contains(out, ":Person") {
		t.Errorf("should contain label, got: %s", out)
	}
	if !strings.Contains(out, `"Alice"`) {
		t.Errorf("should contain quoted name, got: %s", out)
	}
	if strings.Contains(out, "_id") {
		t.Errorf("should not contain internal _id key, got: %s", out)
	}
}

func TestFormatCellValue_Relationship(t *testing.T) {
	rel := map[string]interface{}{
		"_type":  "ACTED_IN",
		"_id":    "...",
		"_start": "...",
		"_end":   "...",
		"roles":  []interface{}{"Neo"},
	}
	out := presentation.FormatCellValue(rel)
	if !strings.Contains(out, "ACTED_IN") {
		t.Errorf("should contain rel type, got: %s", out)
	}
	if !strings.Contains(out, "Neo") {
		t.Errorf("should contain roles value, got: %s", out)
	}
}

func TestFormatCellValue_Nil(t *testing.T) {
	if out := presentation.FormatCellValue(nil); out != "null" {
		t.Errorf("nil should render as 'null', got: %q", out)
	}
}

// ---- Concurrency --------------------------------------------------------

func TestPresentationServiceConcurrent(t *testing.T) {
	svc := newTestService(t, presentation.OutputFormatText)
	var wg sync.WaitGroup

	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = svc.Format(sampleTableData())
		}()
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = svc.RegisterFormatter(presentation.OutputFormatJSON, &presentation.JSONFormatter{Indent: false})
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = svc.SetFormat(presentation.OutputFormatJSON)
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = svc.SetFormat(presentation.OutputFormatText)
	}()
	wg.Wait()
}
