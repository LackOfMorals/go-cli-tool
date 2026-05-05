package tools

import (
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/cli/go-cli-tool/internal/tool"
)

// ToolLister is the interface HelpTool needs from the registry.
type ToolLister interface {
	List() []tool.Tool
	ListNames() []string
}

// HelpTool displays help information for registered tools.
type HelpTool struct {
	*tool.BaseTool
	registry ToolLister
}

func NewHelpTool(registry ToolLister) *HelpTool {
	return &HelpTool{
		BaseTool: tool.NewBaseTool(
			"help",
			"Display help information for tools and commands",
			"1.0.0",
		),
		registry: registry,
	}
}

func (t *HelpTool) Execute(ctx tool.Context) (tool.Result, error) {
	if len(ctx.Args) > 0 {
		help, err := t.getToolHelp(ctx.Args[0])
		if err != nil {
			return tool.ErrorResult("tool not found"), err
		}
		return tool.SuccessResult(help), nil
	}

	var output strings.Builder
	writer := tabwriter.NewWriter(&output, 0, 8, 2, ' ', 0)

	fmt.Fprintln(writer, "Available Tools:")
	fmt.Fprintln(writer, "================")
	fmt.Fprintln(writer)

	if t.registry != nil {
		for _, listed := range t.registry.List() {
			fmt.Fprintf(writer, "  %s\t%s\n", listed.Name(), listed.Description())
		}
	}
	writer.Flush()

	output.WriteString("\nUsage:\n")
	output.WriteString("======\n")
	output.WriteString("  help [tool-name]  - Show help for a specific tool or list all tools\n")

	return tool.SuccessResult(output.String()), nil
}

func (t *HelpTool) Validate(_ tool.Context) error        { return nil }
func (t *HelpTool) DefaultParams() map[string]interface{} { return map[string]interface{}{} }
func (t *HelpTool) Configure(params map[string]interface{}) error {
	return t.BaseTool.Configure(params)
}

func (t *HelpTool) getToolHelp(toolName string) (string, error) {
	if t.registry == nil {
		return "", fmt.Errorf("no tools registered")
	}

	var found tool.Tool
	for _, t := range t.registry.List() {
		if t.Name() == toolName {
			found = t
			break
		}
	}
	if found == nil {
		return "", fmt.Errorf("tool not found: %s", toolName)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "%s (v%s)\n", found.Name(), found.Version())
	b.WriteString(strings.Repeat("=", len(found.Name())+10))
	b.WriteString("\n\nDescription:\n")
	b.WriteString("-------------\n")
	b.WriteString(found.Description())
	b.WriteString("\n\n")

	if defaults := found.DefaultParams(); len(defaults) > 0 {
		b.WriteString("Default Parameters:\n")
		b.WriteString("-------------------\n")
		for key, value := range defaults {
			fmt.Fprintf(&b, "  %s: %v\n", key, value)
		}
	}

	return b.String(), nil
}
