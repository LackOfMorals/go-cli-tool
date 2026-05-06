package presentation_test

import (
	"strings"
	"testing"

	"github.com/cli/go-cli-tool/internal/presentation"
)

// Tests for TOONFormatter live in the external package so they exercise
// only the exported API surface, matching the rest of presentation_test.go.

func TestTOONFormatter_Tabular(t *testing.T) {
	f := &presentation.TOONFormatter{}
	data := presentation.NewTableData(
		[]string{"name", "age"},
		[][]interface{}{
			{"Alice", int64(30)},
			{"Bob", int64(25)},
		},
	)
	out, err := f.Format(data)
	if err != nil {
		t.Fatalf("Format: %v", err)
	}
	for _, want := range []string{"name", "age", "Alice", "Bob", "30", "25"} {
		if !strings.Contains(out, want) {
			t.Errorf("output should contain %q:\n%s", want, out)
		}
	}
}

func TestTOONFormatter_TabularEmpty(t *testing.T) {
	// An empty Tabular should encode as an empty slice without erroring.
	f := &presentation.TOONFormatter{}
	data := presentation.NewTableData([]string{"name"}, nil)
	out, err := f.Format(data)
	if err != nil {
		t.Fatalf("Format empty: %v", err)
	}
	// Should not contain any row data and must not error.
	if strings.Contains(out, "Alice") {
		t.Errorf("empty tabular should not contain row data, got: %q", out)
	}
}

func TestTOONFormatter_DetailData(t *testing.T) {
	f := &presentation.TOONFormatter{}
	detail := presentation.NewDetailData("Instance", []presentation.DetailField{
		{Label: "ID", Value: "abc-123"},
		{Label: "Name", Value: "prod-db"},
	})
	out, err := f.Format(detail)
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

func TestTOONFormatter_ScalarString(t *testing.T) {
	f := &presentation.TOONFormatter{}
	out, err := f.Format("hello")
	if err != nil {
		t.Fatalf("Format string: %v", err)
	}
	if !strings.Contains(out, "hello") {
		t.Errorf("output should contain scalar string, got: %q", out)
	}
}

func TestTOONFormatter_ScalarInt(t *testing.T) {
	f := &presentation.TOONFormatter{}
	out, err := f.Format(int64(42))
	if err != nil {
		t.Fatalf("Format int: %v", err)
	}
	if !strings.Contains(out, "42") {
		t.Errorf("output should contain scalar int, got: %q", out)
	}
}

func TestTOONFormatter_TabularWithNodeStripsSentinels(t *testing.T) {
	// A Tabular cell containing a Neo4j node map must encode without any
	// underscore-prefixed sentinel keys (_labels, _id, ...) — those are
	// rewritten by normalizeForTOON before encoding.
	f := &presentation.TOONFormatter{}
	data := presentation.NewTableData(
		[]string{"n"},
		[][]interface{}{{
			map[string]interface{}{
				"_labels": []string{"Person"},
				"_id":     "4:abc:1",
				"name":    "Keanu Reeves",
				"born":    int64(1964),
			},
		}},
	)
	out, err := f.Format(data)
	if err != nil {
		t.Fatalf("Format: %v", err)
	}
	for _, sentinel := range []string{"_labels", "_id"} {
		if strings.Contains(out, sentinel) {
			t.Errorf("output must not contain sentinel %q:\n%s", sentinel, out)
		}
	}
	for _, want := range []string{"labels", "properties", "Person", "Keanu Reeves", "1964"} {
		if !strings.Contains(out, want) {
			t.Errorf("output should contain %q:\n%s", want, out)
		}
	}
}

func TestTOONFormatter_TabularWithRelationshipStripsSentinels(t *testing.T) {
	f := &presentation.TOONFormatter{}
	data := presentation.NewTableData(
		[]string{"r"},
		[][]interface{}{{
			map[string]interface{}{
				"_type":  "ACTED_IN",
				"_id":    "rel:1",
				"_start": "n:1",
				"_end":   "n:2",
				"roles":  []interface{}{"Neo"},
			},
		}},
	)
	out, err := f.Format(data)
	if err != nil {
		t.Fatalf("Format: %v", err)
	}
	for _, sentinel := range []string{"_type", "_id", "_start", "_end"} {
		if strings.Contains(out, sentinel) {
			t.Errorf("output must not contain sentinel %q:\n%s", sentinel, out)
		}
	}
	for _, want := range []string{"type", "ACTED_IN", "properties", "Neo"} {
		if !strings.Contains(out, want) {
			t.Errorf("output should contain %q:\n%s", want, out)
		}
	}
}

// TOONFormatter is documented as safe for concurrent use; verify by
// running Format from many goroutines simultaneously.
func TestTOONFormatter_ConcurrentSafe(t *testing.T) {
	f := &presentation.TOONFormatter{}
	data := presentation.NewTableData(
		[]string{"name"},
		[][]interface{}{{"Alice"}, {"Bob"}},
	)
	done := make(chan struct{})
	for i := 0; i < 16; i++ {
		go func() {
			defer func() { done <- struct{}{} }()
			if _, err := f.Format(data); err != nil {
				t.Errorf("concurrent Format: %v", err)
			}
		}()
	}
	for i := 0; i < 16; i++ {
		<-done
	}
}
