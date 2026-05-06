package presentation

// This file provides:
//   - normalizeForTOON: a pure helper that rewrites Neo4j entity maps into
//     an encoder-friendly shape before delegating to the toon-go encoder.
//     Nodes (identified by the "_labels" sentinel) become
//     {labels: [...], properties: {...}}; relationships ("_type") become
//     {type: "...", properties: {...}}; all underscore-prefixed sentinel
//     keys (_labels, _type, _id, ...) are stripped from the output. Plain
//     maps and slices are recursed into; everything else passes through
//     unchanged. The helper is pure (no I/O, no state) and safe for
//     concurrent use.
//   - TOONFormatter: an OutputFormatter that serialises Tabular,
//     *DetailData, strings and arbitrary scalars as TOON documents using
//     toon-go's library defaults (Core Profile, comma delimiter,
//     two-space indent). Encoder errors are wrapped as
//     fmt.Errorf("encode TOON: %w", err) for parity with the JSON
//     formatter's "marshal JSON: %w" prefix.

import (
	"fmt"
	"strings"

	"github.com/toon-format/toon-go"
)

// normalizeForTOON recursively rewrites a value so it encodes cleanly as
// TOON. Neo4j node maps (those carrying a "_labels" key) become
// {labels: [...], properties: {...}}; relationship maps (carrying a
// "_type" key) become {type: "...", properties: {...}}; plain maps and
// slices are recursed into; scalars and unknown types pass through
// unchanged.
//
// The function does not mutate its input.
func normalizeForTOON(v any) any {
	switch t := v.(type) {
	case map[string]any:
		return normalizeMap(t)
	case []any:
		out := make([]any, len(t))
		for i, item := range t {
			out[i] = normalizeForTOON(item)
		}
		return out
	case []map[string]any:
		out := make([]any, len(t))
		for i, item := range t {
			out[i] = normalizeMap(item)
		}
		return out
	default:
		return v
	}
}

// normalizeMap handles the three map cases: node, relationship, plain.
func normalizeMap(m map[string]any) map[string]any {
	if _, ok := m["_labels"]; ok {
		return normalizeNode(m)
	}
	if _, ok := m["_type"]; ok {
		return normalizeRel(m)
	}
	out := make(map[string]any, len(m))
	for k, val := range m {
		out[k] = normalizeForTOON(val)
	}
	return out
}

// normalizeNode rewrites a node map into {labels: [...], properties: {...}}.
// Underscore-prefixed sentinel keys are excluded from properties.
func normalizeNode(m map[string]any) map[string]any {
	return map[string]any{
		"labels":     normalizeLabels(m["_labels"]),
		"properties": entityPropsNormalized(m),
	}
}

// normalizeRel rewrites a relationship map into {type: "...", properties: {...}}.
// Underscore-prefixed sentinel keys are excluded from properties.
func normalizeRel(m map[string]any) map[string]any {
	return map[string]any{
		"type":       m["_type"],
		"properties": entityPropsNormalized(m),
	}
}

// normalizeLabels coerces the various label encodings into a []any so the
// TOON encoder sees a plain array regardless of upstream typing.
func normalizeLabels(v any) []any {
	switch t := v.(type) {
	case []string:
		out := make([]any, len(t))
		for i, l := range t {
			out[i] = l
		}
		return out
	case []any:
		return t
	default:
		// Unknown shape — wrap into a single-element array so output is
		// always an array, matching the {labels: [...]} contract.
		return []any{v}
	}
}

// entityPropsNormalized returns the non-internal properties of a node or
// relationship map (skipping keys that start with "_") with each value
// recursively normalised so nested entities render cleanly.
func entityPropsNormalized(m map[string]any) map[string]any {
	props := make(map[string]any, len(m))
	for k, val := range m {
		if strings.HasPrefix(k, "_") {
			continue
		}
		props[k] = normalizeForTOON(val)
	}
	return props
}

// ---- TOONFormatter ------------------------------------------------------

// TOONFormatter serialises data as a TOON document using the toon-go
// encoder with library defaults (Core Profile, comma delimiter, two-space
// indent).
//
// Tabular data is converted to a slice of row maps (mirroring the
// behaviour of JSONFormatter's tabularToJSONSlice). *DetailData is
// converted to a flat map keyed by field label. Strings and arbitrary
// scalars are passed through to the encoder unchanged. All values are
// routed through normalizeForTOON beforehand so Neo4j entity maps emit
// cleanly without their underscore-prefixed sentinel keys.
//
// TOONFormatter has no mutable state and is safe for concurrent use,
// matching the contract of the other built-in formatters.
type TOONFormatter struct{}

// Format renders data as a TOON document.
func (f *TOONFormatter) Format(data any) (string, error) {
	var v any

	switch d := data.(type) {
	case Tabular:
		v = tabularToJSONSlice(d)
	case *DetailData:
		v = detailToJSONMap(d)
	default:
		v = data
	}

	v = normalizeForTOON(v)

	out, err := toon.MarshalString(v)
	if err != nil {
		return "", fmt.Errorf("encode TOON: %w", err)
	}
	return out, nil
}
