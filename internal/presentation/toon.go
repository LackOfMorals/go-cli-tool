package presentation

// This file currently provides the normalizeForTOON helper used by the
// upcoming TOONFormatter (task-004). The helper rewrites Neo4j entity maps
// (those produced by repository.convertValue, identified by the sentinel
// keys "_labels" and "_type") into a clean, encoder-friendly shape:
//
//	node     {labels: [...], properties: {...}}
//	rel      {type: "...",   properties: {...}}
//
// All underscore-prefixed sentinel keys (_labels, _type, _id, ...) are
// stripped from the encoded output. Plain maps and slices are recursed
// into; everything else passes through unchanged.
//
// The helper is pure (no I/O, no state) and safe for concurrent use.
//
// Planned encoder usage (see tasks-add-toon-to-output-formats.yml task-004):
//   - Top-level encoder: toon.MarshalString(v) — returns the TOON document
//     as a string, mirroring how JSONFormatter calls json.Marshal.
//   - Options: library defaults (Core Profile, comma delimiter, two-space
//     indent). No non-default toon.WithIndent / toon.WithArrayDelimiter /
//     toon.WithLengthMarkers tuning is planned for v1.
//   - Errors are wrapped as fmt.Errorf("encode TOON: %w", err) for parity
//     with JSONFormatter's "marshal JSON: %w".

import (
	"strings"

	// Blank import: keeps github.com/toon-format/toon-go pinned in go.mod /
	// go.sum across `go mod tidy` runs until task-004 lands the real import
	// (and the TOONFormatter that calls toon.MarshalString). Once that
	// happens this blank import line should be removed.
	_ "github.com/toon-format/toon-go"
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
