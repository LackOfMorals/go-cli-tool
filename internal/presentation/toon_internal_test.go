package presentation

import (
	"reflect"
	"testing"
)

// Tests for normalizeForTOON live in package presentation (internal test
// file) so they can exercise the unexported helper directly. Per AGENTS.md
// convention, unexported helper tests use the *_internal_test.go suffix.

func TestNormalizeForTOON_Scalar(t *testing.T) {
	cases := []any{
		"hello",
		int64(42),
		3.14,
		true,
		nil,
	}
	for _, c := range cases {
		if got := normalizeForTOON(c); got != c {
			t.Errorf("normalizeForTOON(%v) = %v, want unchanged", c, got)
		}
	}
}

func TestNormalizeForTOON_NodeMap(t *testing.T) {
	in := map[string]any{
		"_labels": []string{"Person", "Actor"},
		"_id":     int64(123),
		"name":    "Keanu Reeves",
		"born":    int64(1964),
	}

	got := normalizeForTOON(in)

	want := map[string]any{
		"labels": []any{"Person", "Actor"},
		"properties": map[string]any{
			"name": "Keanu Reeves",
			"born": int64(1964),
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("node map mismatch:\n got: %#v\nwant: %#v", got, want)
	}

	// Sentinel keys must not appear anywhere in the output.
	gotMap := got.(map[string]any)
	if _, ok := gotMap["_labels"]; ok {
		t.Error("output still contains _labels")
	}
	if _, ok := gotMap["_id"]; ok {
		t.Error("output still contains _id")
	}
	props := gotMap["properties"].(map[string]any)
	for k := range props {
		if len(k) > 0 && k[0] == '_' {
			t.Errorf("properties contain underscore-prefixed key %q", k)
		}
	}
}

func TestNormalizeForTOON_NodeMapLabelsAsInterfaceSlice(t *testing.T) {
	// Some Neo4j driver versions surface labels as []any rather than
	// []string. Both must produce the same []any output.
	in := map[string]any{
		"_labels": []any{"Person"},
		"name":    "Trinity",
	}
	got := normalizeForTOON(in)
	want := map[string]any{
		"labels":     []any{"Person"},
		"properties": map[string]any{"name": "Trinity"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v, want %#v", got, want)
	}
}

func TestNormalizeForTOON_RelationshipMap(t *testing.T) {
	in := map[string]any{
		"_type":  "ACTED_IN",
		"_id":    int64(7),
		"_start": int64(1),
		"_end":   int64(2),
		"roles":  []any{"Neo"},
	}

	got := normalizeForTOON(in)

	want := map[string]any{
		"type": "ACTED_IN",
		"properties": map[string]any{
			"roles": []any{"Neo"},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("rel map mismatch:\n got: %#v\nwant: %#v", got, want)
	}

	gotMap := got.(map[string]any)
	for _, sentinel := range []string{"_type", "_id", "_start", "_end"} {
		if _, ok := gotMap[sentinel]; ok {
			t.Errorf("output still contains sentinel key %q", sentinel)
		}
	}
}

func TestNormalizeForTOON_PlainMapPassThroughKeys(t *testing.T) {
	// A plain map (no _labels, no _type) keeps all keys, but values are
	// still recursively normalised.
	in := map[string]any{
		"name":   "Alice",
		"age":    int64(30),
		"active": true,
	}
	got := normalizeForTOON(in).(map[string]any)
	if !reflect.DeepEqual(got, in) {
		t.Errorf("plain map should pass through unchanged: got %#v want %#v", got, in)
	}
}

func TestNormalizeForTOON_NestedNodeInMap(t *testing.T) {
	in := map[string]any{
		"count": int64(1),
		"actor": map[string]any{
			"_labels": []string{"Person"},
			"_id":     int64(1),
			"name":    "Keanu Reeves",
		},
	}

	got := normalizeForTOON(in).(map[string]any)

	wantActor := map[string]any{
		"labels":     []any{"Person"},
		"properties": map[string]any{"name": "Keanu Reeves"},
	}
	if !reflect.DeepEqual(got["actor"], wantActor) {
		t.Errorf("nested actor mismatch:\n got: %#v\nwant: %#v", got["actor"], wantActor)
	}
	if got["count"] != int64(1) {
		t.Errorf("count not preserved: got %#v", got["count"])
	}
}

func TestNormalizeForTOON_SliceOfNodes(t *testing.T) {
	in := []any{
		map[string]any{
			"_labels": []string{"Person"},
			"name":    "Alice",
		},
		map[string]any{
			"_labels": []string{"Person"},
			"name":    "Bob",
		},
	}

	got := normalizeForTOON(in).([]any)

	if len(got) != 2 {
		t.Fatalf("expected 2 elements, got %d", len(got))
	}
	for i, item := range got {
		m, ok := item.(map[string]any)
		if !ok {
			t.Fatalf("element %d not a map: %#v", i, item)
		}
		if _, ok := m["_labels"]; ok {
			t.Errorf("element %d still contains _labels", i)
		}
		if _, ok := m["labels"]; !ok {
			t.Errorf("element %d missing labels key", i)
		}
		if _, ok := m["properties"]; !ok {
			t.Errorf("element %d missing properties key", i)
		}
	}
}

func TestNormalizeForTOON_SliceOfMapsTyped(t *testing.T) {
	// Some callers may produce []map[string]any directly (rather than
	// []any of maps). Both shapes must normalise to a []any of maps.
	in := []map[string]any{
		{"_type": "ACTED_IN", "roles": []any{"Neo"}},
		{"_type": "DIRECTED"},
	}

	got := normalizeForTOON(in)
	gotSlice, ok := got.([]any)
	if !ok {
		t.Fatalf("expected []any, got %T", got)
	}
	if len(gotSlice) != 2 {
		t.Fatalf("expected 2 elements, got %d", len(gotSlice))
	}
	first := gotSlice[0].(map[string]any)
	if first["type"] != "ACTED_IN" {
		t.Errorf("first element type = %v, want ACTED_IN", first["type"])
	}
	if _, ok := first["_type"]; ok {
		t.Error("first element still contains _type")
	}
}

func TestNormalizeForTOON_DoesNotMutateInput(t *testing.T) {
	in := map[string]any{
		"_labels": []string{"Person"},
		"_id":     int64(1),
		"name":    "Alice",
	}

	_ = normalizeForTOON(in)

	// Original map must still contain its sentinel keys after
	// normalization — the helper produces a new map.
	if _, ok := in["_labels"]; !ok {
		t.Error("input map was mutated: _labels removed")
	}
	if _, ok := in["_id"]; !ok {
		t.Error("input map was mutated: _id removed")
	}
}
