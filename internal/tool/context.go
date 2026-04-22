package tool

import (
	"os"

	"github.com/cli/go-cli-tool/internal/presentation"
)

// Context provides execution context for tools
type Context struct {
	// Args contains command-line arguments
	Args []string

	// Flags contains command-line flags
	Flags map[string]string

	// EnvVars contains environment variables
	EnvVars map[string]string

	// Config contains tool-specific configuration
	Config map[string]interface{}

	// Logger is the logger instance (can be any logger implementing Debug/Info/Warn/Error/Fatal)
	Logger any

	// IO is the IO handler
	IO IOHandler

	// Presenter is the presentation service
	Presenter *presentation.PresentationService

	// WorkingDir is the current working directory
	WorkingDir string
}

// NewContext creates a new execution context
func NewContext() *Context {
	return &Context{
		Args:       []string{},
		Flags:      make(map[string]string),
		EnvVars:    make(map[string]string),
		Config:     make(map[string]interface{}),
		WorkingDir: getWorkingDir(),
	}
}

// getWorkingDir returns the current working directory
func getWorkingDir() string {
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return wd
}

// WithArgs sets the arguments
func (c *Context) WithArgs(args []string) *Context {
	c.Args = args
	return c
}

// WithFlags sets the flags
func (c *Context) WithFlags(flags map[string]string) *Context {
	c.Flags = flags
	return c
}

// WithEnvVars sets the environment variables
func (c *Context) WithEnvVars(envVars map[string]string) *Context {
	c.EnvVars = envVars
	return c
}

// WithConfig sets the configuration
func (c *Context) WithConfig(config map[string]interface{}) *Context {
	c.Config = config
	return c
}

// WithLogger sets the logger
func (c *Context) WithLogger(logger any) *Context {
	c.Logger = logger
	return c
}

// WithIO sets the IO handler
func (c *Context) WithIO(io IOHandler) *Context {
	c.IO = io
	return c
}

// WithPresenter sets the presentation service
func (c *Context) WithPresenter(presenter *presentation.PresentationService) *Context {
	c.Presenter = presenter
	return c
}

// WithWorkingDir sets the working directory
func (c *Context) WithWorkingDir(dir string) *Context {
	c.WorkingDir = dir
	return c
}

// GetArg retrieves an argument by index
func (c *Context) GetArg(index int) string {
	if index >= 0 && index < len(c.Args) {
		return c.Args[index]
	}
	return ""
}

// GetFlag retrieves a flag value
func (c *Context) GetFlag(name string) string {
	if val, ok := c.Flags[name]; ok {
		return val
	}
	return ""
}

// HasFlag checks if a flag exists
func (c *Context) HasFlag(name string) bool {
	_, ok := c.Flags[name]
	return ok
}

// GetEnvVar retrieves an environment variable
func (c *Context) GetEnvVar(name string) string {
	if val, ok := c.EnvVars[name]; ok {
		return val
	}
	return ""
}

// GetConfig retrieves a config value
func (c *Context) GetConfig(key string) any {
	if val, ok := c.Config[key]; ok {
		return val
	}
	return nil
}

// GetConfigString retrieves a config value as string
func (c *Context) GetConfigString(key string, defaultValue string) string {
	if val := c.GetConfig(key); val != nil {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return defaultValue
}

// GetConfigInt retrieves a config value as int
func (c *Context) GetConfigInt(key string, defaultValue int) int {
	if val := c.GetConfig(key); val != nil {
		if i, ok := val.(int); ok {
			return i
		}
	}
	return defaultValue
}

// GetConfigBool retrieves a config value as bool
func (c *Context) GetConfigBool(key string, defaultValue bool) bool {
	if val := c.GetConfig(key); val != nil {
		if b, ok := val.(bool); ok {
			return b
		}
	}
	return defaultValue
}

// MergeFlags merges additional flags into the context
func (c *Context) MergeFlags(flags map[string]string) *Context {
	for k, v := range flags {
		c.Flags[k] = v
	}
	return c
}

// MergeConfig merges additional config into the context
func (c *Context) MergeConfig(config map[string]interface{}) *Context {
	for k, v := range config {
		c.Config[k] = v
	}
	return c
}
