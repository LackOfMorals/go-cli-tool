package tools

import (
	"fmt"
	"sync"

	"github.com/cli/go-cli-tool/internal/core"
	"github.com/cli/go-cli-tool/internal/tool"
)

// ToolRegistry manages registration and discovery of tools
type ToolRegistry struct {
	tools map[string]tool.Tool
	mutex sync.RWMutex
}

// NewToolRegistry creates a new tool registry
func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{
		tools: make(map[string]tool.Tool),
	}
}

// Register adds a tool to the registry
func (r *ToolRegistry) Register(t tool.Tool) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	name := t.Name()
	if _, exists := r.tools[name]; exists {
		return core.NewToolError(name, "ALREADY_EXISTS", "tool already registered", core.ErrToolAlreadyExists)
	}

	r.tools[name] = t
	return nil
}

// RegisterWithConfig adds a tool with its configuration
func (r *ToolRegistry) RegisterWithConfig(t tool.Tool, config core.ToolConfig) error {
	if err := r.Register(t); err != nil {
		return err
	}

	if !config.Enabled {
		return nil
	}

	if len(config.Params) > 0 {
		if err := t.Configure(config.Params); err != nil {
			return fmt.Errorf("failed to configure tool %s: %w", t.Name(), err)
		}
	}

	return nil
}

// Get retrieves a tool by name
func (r *ToolRegistry) Get(name string) (tool.Tool, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	t, exists := r.tools[name]
	if !exists {
		return nil, core.NewToolError(name, "NOT_FOUND", "tool not found", core.ErrToolNotFound)
	}

	return t, nil
}

// List returns all registered tools
func (r *ToolRegistry) List() []tool.Tool {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	tools := make([]tool.Tool, 0, len(r.tools))
	for _, t := range r.tools {
		tools = append(tools, t)
	}
	return tools
}

// ListNames returns all registered tool names
func (r *ToolRegistry) ListNames() []string {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	names := make([]string, 0, len(r.tools))
	for name := range r.tools {
		names = append(names, name)
	}
	return names
}

// Unregister removes a tool from the registry
func (r *ToolRegistry) Unregister(name string) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if _, exists := r.tools[name]; !exists {
		return core.NewToolError(name, "NOT_FOUND", "tool not found", core.ErrToolNotFound)
	}

	delete(r.tools, name)
	return nil
}

// Count returns the number of registered tools
func (r *ToolRegistry) Count() int {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	return len(r.tools)
}

// Exists checks if a tool exists in the registry
func (r *ToolRegistry) Exists(name string) bool {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	_, exists := r.tools[name]
	return exists
}

// Clear removes all tools from the registry
func (r *ToolRegistry) Clear() {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	r.tools = make(map[string]tool.Tool)
}

// Clone creates a copy of the registry
func (r *ToolRegistry) Clone() *ToolRegistry {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	clone := NewToolRegistry()
	for name, t := range r.tools {
		// Clone the tool's params
		baseTool, ok := t.(*tool.BaseTool)
		if ok {
			params := baseTool.GetParams()
			newTool := tool.NewBaseTool(t.Name(), t.Description(), t.Version())
			newTool.Configure(params)
			clone.tools[name] = newTool
		} else {
			clone.tools[name] = t
		}
	}
	return clone
}

// Filter returns tools matching a predicate
func (r *ToolRegistry) Filter(predicate func(tool.Tool) bool) []tool.Tool {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	matched := make([]tool.Tool, 0)
	for _, t := range r.tools {
		if predicate(t) {
			matched = append(matched, t)
		}
	}
	return matched
}

// SearchByTag finds tools with a specific tag
func (r *ToolRegistry) SearchByTag(tag string) []tool.Tool {
	return r.Filter(func(t tool.Tool) bool {
		meta, ok := t.(interface{ GetTags() []string })
		if !ok {
			return false
		}
		tags := meta.GetTags()
		for _, t := range tags {
			if t == tag {
				return true
			}
		}
		return false
	})
}
