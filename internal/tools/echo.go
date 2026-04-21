package tools

import (
	"fmt"
	"strings"

	"github.com/cli/go-cli-tool/internal/tool"
)

// EchoTool implements a simple echo functionality
type EchoTool struct {
	*tool.BaseTool
	uppercase bool
	repeat    int
}

// NewEchoTool creates a new echo tool
func NewEchoTool() *EchoTool {
	return &EchoTool{
		BaseTool: tool.NewBaseTool(
			"echo",
			"Echos text back to the console with optional transformations",
			"1.0.0",
		),
		uppercase: false,
		repeat:    1,
	}
}

// Execute implements Tool interface
func (t *EchoTool) Execute(ctx tool.Context) (tool.Result, error) {
	result := tool.NewResult()

	// Validate arguments
	if err := t.Validate(ctx); err != nil {
		result.SetError("validation failed", err)
		return *result, err
	}

	// Get the message to echo
	message := t.getMessage(ctx.Args)

	// Apply transformations
	if t.uppercase {
		message = strings.ToUpper(message)
	}

	// Repeat the message
	var output strings.Builder
	for i := 0; i < t.repeat; i++ {
		if i > 0 {
			output.WriteString("\n")
		}
		output.WriteString(message)
	}

	result.SetSuccess(output.String())
	return *result, nil
}

// Validate implements Tool interface
func (t *EchoTool) Validate(ctx tool.Context) error {
	// Check if repeat is valid
	if t.repeat < 1 || t.repeat > 100 {
		return fmt.Errorf("repeat must be between 1 and 100")
	}
	return nil
}

// Configure implements Tool interface
func (t *EchoTool) Configure(params map[string]interface{}) error {
	t.BaseTool.Configure(params)

	if val, ok := params["uppercase"]; ok {
		if b, ok := val.(bool); ok {
			t.uppercase = b
		}
	}

	if val, ok := params["repeat"]; ok {
		switch v := val.(type) {
		case int:
			t.repeat = v
		case float64:
			t.repeat = int(v)
		case string:
			var repeat int
			if _, err := fmt.Sscanf(v, "%d", &repeat); err == nil {
				t.repeat = repeat
			}
		}
	}

	return nil
}

// DefaultParams implements Tool interface
func (t *EchoTool) DefaultParams() map[string]interface{} {
	return map[string]interface{}{
		"uppercase": false,
		"repeat":    1,
	}
}

// getMessage extracts the message from arguments or params
func (t *EchoTool) getMessage(args []string) string {
	// First, try to get from args
	if len(args) > 0 {
		return strings.Join(args, " ")
	}

	// Fall back to params
	if val, ok := t.GetParam("message"); ok {
		if msg, ok := val.(string); ok {
			return msg
		}
	}

	return ""
}
