package tool

import (
	"errors"
	"fmt"
)

// ErrPrerequisite is a sentinel that prerequisite checks wrap so callers can
// distinguish a missing-dependency error from an execution error.
//
//	// Test for it:
//	if errors.Is(err, tool.ErrPrerequisite) { ... }
var ErrPrerequisite = errors.New("prerequisite not met")

// MutationMode declares whether a tool modifies state in the connected
// database or cloud platform. The dispatcher uses this to enforce the
// agent-mode read-only contract: ModeWrite is blocked without --rw;
// ModeConditional triggers an EXPLAIN pre-check.
type MutationMode int

const (
	// ModeRead means the tool never modifies state. This is the safe default;
	// BaseTool.MutationMode() returns ModeRead so existing tools are unaffected.
	ModeRead MutationMode = iota

	// ModeWrite means the tool always modifies state. In agent mode the
	// dispatcher blocks the call and returns ErrReadOnly before Execute() is
	// ever called, unless --rw / LOM_RW is also set.
	ModeWrite

	// ModeConditional means whether the tool mutates state depends on its
	// input at runtime (e.g. a Cypher query). The dispatcher triggers an
	// EXPLAIN pre-check in agent mode without --rw rather than statically
	// blocking or blindly allowing.
	ModeConditional
)

// ErrReadOnly is returned by the dispatcher when a ModeWrite or
// ModeConditional tool is blocked in agent mode because --rw is not set.
var ErrReadOnly = errors.New("read only")

// AgentError is a structured error that carries a machine-readable code and
// is rendered as a JSON error envelope when the CLI runs in --agent mode.
type AgentError struct {
	Code    string // e.g. "READ_ONLY", "TIMEOUT", "MISSING_CREDENTIALS"
	Message string
	Err     error  // optional wrapped cause
}

func (e *AgentError) Error() string { return e.Message }
func (e *AgentError) Unwrap() error { return e.Err }

// NewAgentError constructs an AgentError with the given code and message.
func NewAgentError(code, message string) *AgentError {
	return &AgentError{Code: code, Message: message}
}

// NewAgentErrorf constructs an AgentError wrapping an existing error.
func NewAgentErrorf(code string, err error, format string, args ...interface{}) *AgentError {
	return &AgentError{Code: code, Message: fmt.Sprintf(format, args...), Err: err}
}

// Tool defines the interface all CLI tools must implement.
type Tool interface {
	Name() string
	Description() string
	Version() string

	Execute(ctx Context) (Result, error)
	Validate(ctx Context) error

	Configure(params map[string]interface{}) error
	DefaultParams() map[string]interface{}

	// MutationMode declares whether this tool reads or writes state.
	// BaseTool returns ModeRead by default; override in write tools.
	MutationMode() MutationMode
}

// BaseTool provides common implementations for the Tool interface.
// Embed it in concrete tools and override only the methods you need.
type BaseTool struct {
	name        string
	description string
	version     string
	params      map[string]interface{}
}

func NewBaseTool(name, description, version string) *BaseTool {
	return &BaseTool{
		name:        name,
		description: description,
		version:     version,
		params:      make(map[string]interface{}),
	}
}

func (t *BaseTool) Name() string        { return t.name }
func (t *BaseTool) Description() string { return t.description }
func (t *BaseTool) Version() string     { return t.version }

func (t *BaseTool) GetParams() map[string]interface{} { return t.params }

func (t *BaseTool) SetParam(key string, value interface{}) { t.params[key] = value }

func (t *BaseTool) GetParam(key string) (interface{}, bool) {
	val, ok := t.params[key]
	return val, ok
}

// DefaultParams returns an empty map. Override in concrete tools.
func (t *BaseTool) DefaultParams() map[string]interface{} {
	return map[string]interface{}{}
}

// Configure stores all params. Override in concrete tools to apply them.
func (t *BaseTool) Configure(params map[string]interface{}) error {
	for k, v := range params {
		t.params[k] = v
	}
	return nil
}

// Validate is a no-op. Override in concrete tools.
func (t *BaseTool) Validate(_ Context) error { return nil }

// Execute returns "not implemented". Override in concrete tools.
func (t *BaseTool) Execute(_ Context) (Result, error) {
	return Result{Success: false, Output: "not implemented"}, nil
}

// MutationMode returns ModeRead. Override in tools that modify state.
func (t *BaseTool) MutationMode() MutationMode { return ModeRead }

// ---- Typed param helpers ------------------------------------------------

func (t *BaseTool) GetStringParam(key, defaultValue string) string {
	if val, ok := t.params[key]; ok {
		if s, ok := val.(string); ok {
			return s
		}
	}
	return defaultValue
}

func (t *BaseTool) GetBoolParam(key string, defaultValue bool) bool {
	if val, ok := t.params[key]; ok {
		if b, ok := val.(bool); ok {
			return b
		}
	}
	return defaultValue
}

func (t *BaseTool) GetIntParam(key string, defaultValue int) int {
	if val, ok := t.params[key]; ok {
		if i, ok := val.(int); ok {
			return i
		}
	}
	return defaultValue
}

func (t *BaseTool) GetFloatParam(key string, defaultValue float64) float64 {
	if val, ok := t.params[key]; ok {
		if f, ok := val.(float64); ok {
			return f
		}
	}
	return defaultValue
}

func (t *BaseTool) GetMapParam(key string) map[string]interface{} {
	if val, ok := t.params[key]; ok {
		if m, ok := val.(map[string]interface{}); ok {
			return m
		}
	}
	return nil
}

func (t *BaseTool) GetSliceParam(key string) []interface{} {
	if val, ok := t.params[key]; ok {
		if s, ok := val.([]interface{}); ok {
			return s
		}
	}
	return nil
}

// ValidateRequiredParams returns an error if any of the named params are absent.
func (t *BaseTool) ValidateRequiredParams(required []string) error {
	for _, key := range required {
		if _, ok := t.params[key]; !ok {
			return fmt.Errorf("required parameter %q is missing", key)
		}
	}
	return nil
}
