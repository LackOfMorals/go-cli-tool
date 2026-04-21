package tool

import (
	"fmt"
)

// Tool defines the interface all CLI tools must implement
type Tool interface {
	// Metadata
	Name() string
	Description() string
	Version() string

	// Execution
	Execute(ctx Context) (Result, error)
	Validate(ctx Context) error

	// Configuration
	Configure(params map[string]interface{}) error
	DefaultParams() map[string]interface{}
}

// ToolMetadata contains metadata information for a tool
type ToolMetadata struct {
	Name        string
	Description string
	Version     string
	Author      string
	Tags        []string
}

// BaseTool provides common functionality for all tools
type BaseTool struct {
	name        string
	description string
	version     string
	params      map[string]interface{}
}

// NewBaseTool creates a new base tool with common functionality
func NewBaseTool(name, description, version string) *BaseTool {
	return &BaseTool{
		name:        name,
		description: description,
		version:     version,
		params:      make(map[string]interface{}),
	}
}

// Name returns the tool name
func (t *BaseTool) Name() string {
	return t.name
}

// Description returns the tool description
func (t *BaseTool) Description() string {
	return t.description
}

// Version returns the tool version
func (t *BaseTool) Version() string {
	return t.version
}

// GetParams returns the current tool parameters
func (t *BaseTool) GetParams() map[string]interface{} {
	return t.params
}

// SetParam sets a single parameter
func (t *BaseTool) SetParam(key string, value interface{}) {
	t.params[key] = value
}

// GetParam retrieves a parameter value
func (t *BaseTool) GetParam(key string) (interface{}, bool) {
	val, ok := t.params[key]
	return val, ok
}

// DefaultParams returns default parameters (to be overridden by implementations)
func (t *BaseTool) DefaultParams() map[string]interface{} {
	return map[string]interface{}{}
}

// Configure configures the tool with parameters (to be overridden by implementations)
func (t *BaseTool) Configure(params map[string]interface{}) error {
	for key, value := range params {
		t.params[key] = value
	}
	return nil
}

// Validate validates the tool parameters (to be overridden by implementations)
func (t *BaseTool) Validate(ctx Context) error {
	return nil
}

// Execute executes the tool (to be overridden by implementations)
func (t *BaseTool) Execute(ctx Context) (Result, error) {
	return Result{
		Success: false,
		Output:  "not implemented",
	}, nil
}

// GetStringParam retrieves a string parameter with a default value
func (t *BaseTool) GetStringParam(key string, defaultValue string) string {
	if val, ok := t.params[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return defaultValue
}

// GetBoolParam retrieves a boolean parameter with a default value
func (t *BaseTool) GetBoolParam(key string, defaultValue bool) bool {
	if val, ok := t.params[key]; ok {
		if b, ok := val.(bool); ok {
			return b
		}
	}
	return defaultValue
}

// GetIntParam retrieves an integer parameter with a default value
func (t *BaseTool) GetIntParam(key string, defaultValue int) int {
	if val, ok := t.params[key]; ok {
		if i, ok := val.(int); ok {
			return i
		}
	}
	return defaultValue
}

// GetFloatParam retrieves a float parameter with a default value
func (t *BaseTool) GetFloatParam(key string, defaultValue float64) float64 {
	if val, ok := t.params[key]; ok {
		if f, ok := val.(float64); ok {
			return f
		}
	}
	return defaultValue
}

// GetMapParam retrieves a map parameter
func (t *BaseTool) GetMapParam(key string) map[string]interface{} {
	if val, ok := t.params[key]; ok {
		if m, ok := val.(map[string]interface{}); ok {
			return m
		}
	}
	return nil
}

// GetSliceParam retrieves a slice parameter
func (t *BaseTool) GetSliceParam(key string) []interface{} {
	if val, ok := t.params[key]; ok {
		if s, ok := val.([]interface{}); ok {
			return s
		}
	}
	return nil
}

// ValidateRequiredParams validates that all required parameters are present
func (t *BaseTool) ValidateRequiredParams(required []string) error {
	for _, key := range required {
		if _, ok := t.params[key]; !ok {
			return fmt.Errorf("required parameter '%s' is missing", key)
		}
	}
	return nil
}
