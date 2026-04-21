package tools

import (
	"testing"

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
	registry.Register(testTool)

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

	registry.Register(tool.NewBaseTool("tool1", "Tool 1", "1.0.0"))
	registry.Register(tool.NewBaseTool("tool2", "Tool 2", "1.0.0"))
	registry.Register(tool.NewBaseTool("tool3", "Tool 3", "1.0.0"))

	tools := registry.List()
	if len(tools) != 3 {
		t.Errorf("Expected 3 tools, got %d", len(tools))
	}
}

func TestToolRegistryListNames(t *testing.T) {
	registry := NewToolRegistry()

	registry.Register(tool.NewBaseTool("echo", "Echo tool", "1.0.0"))
	registry.Register(tool.NewBaseTool("help", "Help tool", "1.0.0"))

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
	registry.Register(testTool)

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

	registry.Register(testTool)

	if !registry.Exists("test") {
		t.Error("Expected tool to exist after registration")
	}
}

func TestToolRegistryClear(t *testing.T) {
	registry := NewToolRegistry()

	registry.Register(tool.NewBaseTool("tool1", "Tool 1", "1.0.0"))
	registry.Register(tool.NewBaseTool("tool2", "Tool 2", "1.0.0"))

	registry.Clear()

	if registry.Count() != 0 {
		t.Errorf("Expected 0 tools after clear, got %d", registry.Count())
	}
}

func TestToolRegistryDuplicateRegistration(t *testing.T) {
	registry := NewToolRegistry()
	testTool := tool.NewBaseTool("test", "Test tool", "1.0.0")

	registry.Register(testTool)
	err := registry.Register(testTool)

	if err == nil {
		t.Error("Expected error for duplicate registration")
	}
}

func TestToolRegistryFilter(t *testing.T) {
	registry := NewToolRegistry()

	registry.Register(tool.NewBaseTool("echo", "Echo tool", "1.0.0"))
	registry.Register(tool.NewBaseTool("help", "Help tool", "1.0.0"))
	registry.Register(tool.NewBaseTool("exit", "Exit tool", "1.0.0"))

	// Filter tools with 'e' in name
	filtered := registry.Filter(func(t tool.Tool) bool {
		name := t.Name()
		return len(name) > 0 && name[0] == 'e'
	})

	if len(filtered) != 2 {
		t.Errorf("Expected 2 filtered tools, got %d", len(filtered))
	}
}
