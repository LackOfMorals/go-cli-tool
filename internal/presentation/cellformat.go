package presentation

import (
	"fmt"
	"sort"
	"strings"
)

// FormatCellValue converts a query result value into a human-readable string
// for display in tables, property lists, and graph renderers.
//
// Special map shapes produced by repository.convertValue are rendered in
// standard Cypher notation so that RETURN * output is as readable as
// explicit RETURN n.name, n.born output:
//
//	Node:         (:Person {born: 1964, name: "Keanu Reeves"})
//	Relationship: [:ACTED_IN {roles: ["Neo"]}]
//	List:         [v1, v2, ...]
//	String:       value          (no quotes in table cells — cleaner to read)
//	Null:         null
//	Other:        fmt.Sprintf default
func FormatCellValue(v interface{}) string {
	if v == nil {
		return "null"
	}

	switch t := v.(type) {
	case map[string]interface{}:
		if _, isNode := t["_labels"]; isNode {
			return formatNodeValue(t)
		}
		if _, isRel := t["_type"]; isRel {
			return formatRelValue(t)
		}
		return formatMapValue(t)

	case []interface{}:
		parts := make([]string, len(t))
		for i, item := range t {
			parts[i] = FormatCellValue(item)
		}
		return "[" + strings.Join(parts, ", ") + "]"

	case string:
		return t // no quotes in table cells — cleaner to read

	default:
		return fmt.Sprintf("%v", t)
	}
}

// formatNodeValue renders a node map as (:Label {prop: val, ...}).
func formatNodeValue(m map[string]interface{}) string {
	var sb strings.Builder
	sb.WriteString("(")

	if raw, ok := m["_labels"]; ok {
		switch labels := raw.(type) {
		case []string:
			for _, l := range labels {
				sb.WriteString(":")
				sb.WriteString(l)
			}
		case []interface{}:
			for _, l := range labels {
				sb.WriteString(":")
				fmt.Fprintf(&sb, "%v", l)
			}
		}
	}

	props := entityProps(m)
	if len(props) > 0 {
		sb.WriteString(" ")
		sb.WriteString(FormatPropsInline(props))
	}
	sb.WriteString(")")
	return sb.String()
}

// formatRelValue renders a relationship map as [:TYPE {prop: val, ...}].
func formatRelValue(m map[string]interface{}) string {
	var sb strings.Builder
	sb.WriteString("[:")
	if relType, ok := m["_type"]; ok {
		fmt.Fprintf(&sb, "%v", relType)
	}
	props := entityProps(m)
	if len(props) > 0 {
		sb.WriteString(" ")
		sb.WriteString(FormatPropsInline(props))
	}
	sb.WriteString("]")
	return sb.String()
}

// formatMapValue renders a plain map as {k: v, ...} (alphabetical key order).
func formatMapValue(m map[string]interface{}) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s: %s", k, FormatPropValue(m[k])))
	}
	return "{" + strings.Join(parts, ", ") + "}"
}

// entityProps returns the non-internal properties of a node or rel map
// (skips keys that start with "_").
func entityProps(m map[string]interface{}) map[string]interface{} {
	props := make(map[string]interface{}, len(m))
	for k, v := range m {
		if !strings.HasPrefix(k, "_") {
			props[k] = v
		}
	}
	return props
}

// FormatPropsInline renders a property map as {key: val, ...} (alphabetical).
// Exported so the graph renderer in commands can call it directly if needed.
func FormatPropsInline(props map[string]interface{}) string {
	if len(props) == 0 {
		return ""
	}
	keys := make([]string, 0, len(props))
	for k := range props {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s: %s", k, FormatPropValue(props[k])))
	}
	return "{" + strings.Join(parts, ", ") + "}"
}

// FormatPropValue renders a single property value with Cypher-style quoting.
// Strings are double-quoted; lists are bracketed; maps recurse; others use %v.
func FormatPropValue(v interface{}) string {
	if v == nil {
		return "null"
	}
	switch t := v.(type) {
	case string:
		return `"` + t + `"`
	case []interface{}:
		parts := make([]string, len(t))
		for i, item := range t {
			parts[i] = FormatPropValue(item)
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case map[string]interface{}:
		return formatMapValue(t)
	default:
		return fmt.Sprintf("%v", t)
	}
}
