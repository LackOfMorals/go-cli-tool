package tools

import (
	"fmt"
	"strings"

	"github.com/cli/go-cli-tool/internal/tool"
)

// EchoTool echoes its arguments back to the console with optional
// transformations (uppercase, repeat).
type EchoTool struct {
	*tool.BaseTool
	uppercase bool
	repeat    int
}

func NewEchoTool() *EchoTool {
	return &EchoTool{
		BaseTool: tool.NewBaseTool(
			"echo",
			"Echo text back to the console with optional transformations",
			"1.0.0",
		),
		repeat: 1,
	}
}

func (t *EchoTool) Execute(ctx tool.Context) (tool.Result, error) {
	if err := t.Validate(ctx); err != nil {
		return tool.ErrorResult("validation failed"), err
	}

	message := t.getMessage(ctx.Args)

	if t.uppercase {
		message = strings.ToUpper(message)
	}

	var out strings.Builder
	for i := 0; i < t.repeat; i++ {
		if i > 0 {
			out.WriteByte('\n')
		}
		out.WriteString(message)
	}

	return tool.SuccessResult(out.String()), nil
}

func (t *EchoTool) Validate(_ tool.Context) error {
	if t.repeat < 1 || t.repeat > 100 {
		return fmt.Errorf("repeat must be between 1 and 100 (got %d)", t.repeat)
	}
	return nil
}

func (t *EchoTool) Configure(params map[string]interface{}) error {
	if err := t.BaseTool.Configure(params); err != nil {
		return err
	}

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
			if _, err := fmt.Sscanf(v, "%d", &t.repeat); err != nil {
				return fmt.Errorf("repeat: expected integer, got %q", v)
			}
		}
	}

	return nil
}

func (t *EchoTool) DefaultParams() map[string]interface{} {
	return map[string]interface{}{
		"uppercase": false,
		"repeat":    1,
	}
}

func (t *EchoTool) getMessage(args []string) string {
	if len(args) > 0 {
		return strings.Join(args, " ")
	}
	if val, ok := t.GetParam("message"); ok {
		if msg, ok := val.(string); ok {
			return msg
		}
	}
	return ""
}
