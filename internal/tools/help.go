package tools

import (
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/cli/go-cli-tool/internal/tool"
)

// ToolLister interface for listing tools
type ToolLister interface {
	List() []tool.Tool
	ListNames() []string
}

// HelpTool provides help information for all registered tools
type HelpTool struct {
	*tool.BaseTool
	registry ToolLister
}

// NewHelpTool creates a new help tool
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

// Execute implements Tool interface
func (t *HelpTool) Execute(ctx tool.Context) (tool.Result, error) {
	result := tool.NewResult()

	var output strings.Builder
	writer := tabwriter.NewWriter(&output, 0, 8, 2, ' ', 0)

	// Get tool name from args if provided
	if len(ctx.Args) > 0 {
		toolName := ctx.Args[0]
		help, err := t.getToolHelp(toolName)
		if err != nil {
			result.SetError("tool not found", err)
			return *result, err
		}
		result.SetSuccess(help)
		return *result, nil
	}

	// Show general help
	fmt.Fprintln(writer, "Available Tools:")
	fmt.Fprintln(writer, "================")
	fmt.Fprintln(writer)

	if t.registry != nil {
		for _, name := range t.registry.ListNames() {
			tools := t.registry.List()
			for _, t := range tools {
				if t.Name() == name {
					fmt.Fprintf(writer, "  %s\t%s\n", name, t.Description())
					break
				}
			}
		}
	}

	writer.Flush()

	// Add usage information
	output.WriteString("\nUsage:\n")
	output.WriteString("======\n")
	output.WriteString("  help [tool-name]  - Show help for a specific tool or list all tools\n")

	result.SetSuccess(output.String())
	return *result, nil
}

// Validate implements Tool interface
func (t *HelpTool) Validate(ctx tool.Context) error {
	return nil
}

// Configure implements Tool interface
func (t *HelpTool) Configure(params map[string]interface{}) error {
	return t.BaseTool.Configure(params)
}

// DefaultParams implements Tool interface
func (t *HelpTool) DefaultParams() map[string]interface{} {
	return map[string]interface{}{}
}

// getToolHelp returns help text for a specific tool
func (t *HelpTool) getToolHelp(toolName string) (string, error) {
	if t.registry == nil {
		return "", fmt.Errorf("no tools registered")
	}

	tools := t.registry.List()
	var foundTool tool.Tool
	for _, t := range tools {
		if t.Name() == toolName {
			foundTool = t
			break
		}
	}

	if foundTool == nil {
		return "", fmt.Errorf("tool not found: %s", toolName)
	}

	var output strings.Builder

	// Tool header
	output.WriteString(fmt.Sprintf("%s (v%s)\n", foundTool.Name(), foundTool.Version()))
	output.WriteString(strings.Repeat("=", len(foundTool.Name())+10))
	output.WriteString("\n\n")

	// Description
	output.WriteString("Description:\n")
	output.WriteString("-------------\n")
	output.WriteString(foundTool.Description())
	output.WriteString("\n\n")

	// Parameters
	defaults := foundTool.DefaultParams()
	if len(defaults) > 0 {
		output.WriteString("Default Parameters:\n")
		output.WriteString("-------------------\n")
		for key, value := range defaults {
			output.WriteString(fmt.Sprintf("  %s: %v\n", key, value))
		}
		output.WriteString("\n")
	}

	return output.String(), nil
}
