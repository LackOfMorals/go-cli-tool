package tools

import (
	"fmt"
	"testing"

	"github.com/cli/go-cli-tool/internal/config"
	"github.com/cli/go-cli-tool/internal/tool"
)

func TestNewToolRegistry(t *testing.T) {
	registry := NewToolRegistry()

	if registry == nil {
		t.Fatal("Expected non-nil registry")
	}

	if registry.Count() != 0 {
		t.Errorf("Expected 0 tools, got %d", registry.Count())
	}
}

func TestToolRegistryRegister(t *testing.T) {
	registry := NewToolRegistry()
	testTool := tool.NewBaseTool("test", "Test tool", "1.0.0")

	err := registry.Register(testTool)
	if err != nil {
		t.Errorf("Failed to register tool: %v", err)
	}

	if registry.Count() != 1 {
		t.Errorf("Expected 1 tool, got %d", registry.Count())
	}
}

func TestToolRegistryGet(t *testing.T) {
	registry := NewToolRegistry()
	testTool := tool.NewBaseTool("test", "Test tool", "1.0.0")
	if err := registry.Register(testTool); err != nil {
		t.Fatalf("Register: %v", err)
	}

	retrieved, err := registry.Get("test")
	if err != nil {
		t.Errorf("Failed to get tool: %v", err)
	}

	if retrieved.Name() != "test" {
		t.Errorf("Expected name 'test', got %s", retrieved.Name())
	}

	if retrieved.Version() != "1.0.0" {
		t.Errorf("Expected version '1.0.0', got %s", retrieved.Version())
	}
}

func TestToolRegistryGetNotFound(t *testing.T) {
	registry := NewToolRegistry()

	_, err := registry.Get("nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent tool")
	}
}

func TestToolRegistryList(t *testing.T) {
	registry := NewToolRegistry()

	for _, tt := range []tool.Tool{
		tool.NewBaseTool("tool1", "Tool 1", "1.0.0"),
		tool.NewBaseTool("tool2", "Tool 2", "1.0.0"),
		tool.NewBaseTool("tool3", "Tool 3", "1.0.0"),
	} {
		if err := registry.Register(tt); err != nil {
			t.Fatalf("Register: %v", err)
		}
	}

	tools := registry.List()
	if len(tools) != 3 {
		t.Errorf("Expected 3 tools, got %d", len(tools))
	}
}

func TestToolRegistryListNames(t *testing.T) {
	registry := NewToolRegistry()

	for _, tt := range []tool.Tool{
		tool.NewBaseTool("echo", "Echo tool", "1.0.0"),
		tool.NewBaseTool("help", "Help tool", "1.0.0"),
	} {
		if err := registry.Register(tt); err != nil {
			t.Fatalf("Register: %v", err)
		}
	}

	names := registry.ListNames()
	if len(names) != 2 {
		t.Errorf("Expected 2 names, got %d", len(names))
	}

	found := false
	for _, name := range names {
		if name == "echo" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected 'echo' to be in names list")
	}
}

func TestToolRegistryUnregister(t *testing.T) {
	registry := NewToolRegistry()
	testTool := tool.NewBaseTool("test", "Test tool", "1.0.0")
	if err := registry.Register(testTool); err != nil {
		t.Fatalf("Register: %v", err)
	}

	err := registry.Unregister("test")
	if err != nil {
		t.Errorf("Failed to unregister tool: %v", err)
	}

	if registry.Count() != 0 {
		t.Errorf("Expected 0 tools after unregister, got %d", registry.Count())
	}

	_, err = registry.Get("test")
	if err == nil {
		t.Error("Expected error after unregister")
	}
}

func TestToolRegistryExists(t *testing.T) {
	registry := NewToolRegistry()
	testTool := tool.NewBaseTool("test", "Test tool", "1.0.0")

	if registry.Exists("test") {
		t.Error("Expected tool to not exist before registration")
	}

	if err := registry.Register(testTool); err != nil {
		t.Fatalf("Register: %v", err)
	}

	if !registry.Exists("test") {
		t.Error("Expected tool to exist after registration")
	}
}

func TestToolRegistryClear(t *testing.T) {
	registry := NewToolRegistry()

	for _, tt := range []tool.Tool{
		tool.NewBaseTool("tool1", "Tool 1", "1.0.0"),
		tool.NewBaseTool("tool2", "Tool 2", "1.0.0"),
	} {
		if err := registry.Register(tt); err != nil {
			t.Fatalf("Register: %v", err)
		}
	}

	registry.Clear()

	if registry.Count() != 0 {
		t.Errorf("Expected 0 tools after clear, got %d", registry.Count())
	}
}

func TestToolRegistryDuplicateRegistration(t *testing.T) {
	registry := NewToolRegistry()
	testTool := tool.NewBaseTool("test", "Test tool", "1.0.0")

	_ = registry.Register(testTool) // first registration succeeds; ignore error
	err := registry.Register(testTool)

	if err == nil {
		t.Error("Expected error for duplicate registration")
	}
}

func TestRegisterWithConfig_RollbackOnConfigureError(t *testing.T) {
	// If Configure fails the tool must not remain in the registry.
	registry := NewToolRegistry()

	badTool := &configurableErrorTool{
		BaseTool: tool.NewBaseTool("bad", "will fail", "1.0.0"),
	}

	cfg := config.ToolConfig{
		Enabled: true,
		Params:  map[string]interface{}{"trigger_error": true},
	}

	err := registry.RegisterWithConfig(badTool, cfg)
	if err == nil {
		t.Fatal("expected error from Configure")
	}

	// The tool must have been rolled back — registry should be empty.
	if registry.Exists("bad") {
		t.Error("tool should have been unregistered after Configure failure")
	}
	if registry.Count() != 0 {
		t.Errorf("registry should be empty after rollback, got count %d", registry.Count())
	}
}

// configurableErrorTool implements tool.Tool and returns an error from Configure.
type configurableErrorTool struct {
	*tool.BaseTool
}

func (t *configurableErrorTool) Configure(params map[string]interface{}) error {
	if _, ok := params["trigger_error"]; ok {
		return fmt.Errorf("configure: intentional test error")
	}
	return nil
}

func TestToolRegistryFilter(t *testing.T) {
	registry := NewToolRegistry()

	for _, tt := range []tool.Tool{
		tool.NewBaseTool("echo", "Echo tool", "1.0.0"),
		tool.NewBaseTool("help", "Help tool", "1.0.0"),
		tool.NewBaseTool("exit", "Exit tool", "1.0.0"),
	} {
		if err := registry.Register(tt); err != nil {
			t.Fatalf("Register: %v", err)
		}
	}

	// Filter tools with 'e' in name
	filtered := registry.Filter(func(t tool.Tool) bool {
		name := t.Name()
		return len(name) > 0 && name[0] == 'e'
	})

	if len(filtered) != 2 {
		t.Errorf("Expected 2 filtered tools, got %d", len(filtered))
	}
}
