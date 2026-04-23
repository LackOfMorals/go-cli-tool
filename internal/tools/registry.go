package tools

import (
	"errors"
	"fmt"
	"sync"

	"github.com/cli/go-cli-tool/internal/config"
	"github.com/cli/go-cli-tool/internal/tool"
)

// ---- Sentinel errors ----------------------------------------------------

var (
	ErrToolNotFound      = errors.New("tool not found")
	ErrToolAlreadyExists = errors.New("tool already registered")
)

// ---- ToolError ----------------------------------------------------------

// ToolError carries the tool name and a short error code alongside the
// underlying error, making registry errors easy to identify in logs.
type ToolError struct {
	ToolName string
	Code     string
	Message  string
	Err      error
}

func (e *ToolError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("[%s] %s: %v", e.ToolName, e.Message, e.Err)
	}
	return fmt.Sprintf("[%s] %s", e.ToolName, e.Message)
}

func (e *ToolError) Unwrap() error { return e.Err }

func newToolError(toolName, code, message string, err error) *ToolError {
	return &ToolError{ToolName: toolName, Code: code, Message: message, Err: err}
}

// ---- ToolRegistry -------------------------------------------------------

// ToolRegistry manages registration and discovery of tools.
type ToolRegistry struct {
	tools map[string]tool.Tool
	mutex sync.RWMutex
}

// NewToolRegistry creates a new, empty tool registry.
func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{tools: make(map[string]tool.Tool)}
}

// Register adds a tool to the registry. Returns ErrToolAlreadyExists (wrapped
// in a ToolError) if a tool with the same name is already registered.
func (r *ToolRegistry) Register(t tool.Tool) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	name := t.Name()
	if _, exists := r.tools[name]; exists {
		return newToolError(name, "ALREADY_EXISTS", "tool already registered", ErrToolAlreadyExists)
	}
	r.tools[name] = t
	return nil
}

// RegisterWithConfig adds a tool and applies its configuration. If Configure
// fails the tool is unregistered so the registry is never left in a
// partially-configured state.
func (r *ToolRegistry) RegisterWithConfig(t tool.Tool, cfg config.ToolConfig) error {
	if err := r.Register(t); err != nil {
		return err
	}
	if !cfg.Enabled || len(cfg.Params) == 0 {
		return nil
	}
	if err := t.Configure(cfg.Params); err != nil {
		// Rollback: remove the tool so we never leave a misconfigured entry.
		_ = r.Unregister(t.Name())
		return fmt.Errorf("configure tool %q: %w", t.Name(), err)
	}
	return nil
}

// Get retrieves a tool by name. Returns ErrToolNotFound (wrapped in a
// ToolError) if the tool is not registered.
func (r *ToolRegistry) Get(name string) (tool.Tool, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	t, exists := r.tools[name]
	if !exists {
		return nil, newToolError(name, "NOT_FOUND", "tool not found", ErrToolNotFound)
	}
	return t, nil
}

// List returns all registered tools in an unspecified order.
func (r *ToolRegistry) List() []tool.Tool {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	ts := make([]tool.Tool, 0, len(r.tools))
	for _, t := range r.tools {
		ts = append(ts, t)
	}
	return ts
}

// ListNames returns the names of all registered tools in an unspecified order.
func (r *ToolRegistry) ListNames() []string {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	names := make([]string, 0, len(r.tools))
	for name := range r.tools {
		names = append(names, name)
	}
	return names
}

// Unregister removes a tool from the registry.
func (r *ToolRegistry) Unregister(name string) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if _, exists := r.tools[name]; !exists {
		return newToolError(name, "NOT_FOUND", "tool not found", ErrToolNotFound)
	}
	delete(r.tools, name)
	return nil
}

// Count returns the number of registered tools.
func (r *ToolRegistry) Count() int {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	return len(r.tools)
}

// Exists reports whether a tool with the given name is registered.
func (r *ToolRegistry) Exists(name string) bool {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	_, exists := r.tools[name]
	return exists
}

// Clear removes all tools from the registry.
func (r *ToolRegistry) Clear() {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	r.tools = make(map[string]tool.Tool)
}

// Filter returns all tools for which the predicate returns true.
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

// SearchByTag returns all tools that expose a GetTags() method containing tag.
func (r *ToolRegistry) SearchByTag(tag string) []tool.Tool {
	return r.Filter(func(t tool.Tool) bool {
		tagger, ok := t.(interface{ GetTags() []string })
		if !ok {
			return false
		}
		for _, tt := range tagger.GetTags() {
			if tt == tag {
				return true
			}
		}
		return false
	})
}
