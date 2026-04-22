package tool

import (
	"os"

	"github.com/cli/go-cli-tool/internal/logger"
	"github.com/cli/go-cli-tool/internal/presentation"
)

// Context provides execution context for tools.
type Context struct {
	Args       []string
	Flags      map[string]string
	EnvVars    map[string]string
	Config     map[string]interface{}
	Logger     logger.Service // typed — tools can call ctx.Logger.Info(...) directly
	IO         IOHandler
	Presenter  *presentation.PresentationService
	WorkingDir string
}

func NewContext() *Context {
	return &Context{
		Args:       []string{},
		Flags:      make(map[string]string),
		EnvVars:    make(map[string]string),
		Config:     make(map[string]interface{}),
		WorkingDir: workingDir(),
	}
}

func workingDir() string {
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return wd
}

// ---- Builder methods ----------------------------------------------------

func (c *Context) WithArgs(args []string) *Context          { c.Args = args; return c }
func (c *Context) WithIO(io IOHandler) *Context             { c.IO = io; return c }
func (c *Context) WithPresenter(p *presentation.PresentationService) *Context {
	c.Presenter = p
	return c
}
func (c *Context) WithWorkingDir(dir string) *Context { c.WorkingDir = dir; return c }

func (c *Context) WithLogger(log logger.Service) *Context {
	c.Logger = log
	return c
}

func (c *Context) WithFlags(flags map[string]string) *Context {
	c.Flags = flags
	return c
}

func (c *Context) WithEnvVars(envVars map[string]string) *Context {
	c.EnvVars = envVars
	return c
}

func (c *Context) WithConfig(config map[string]interface{}) *Context {
	c.Config = config
	return c
}

func (c *Context) MergeFlags(flags map[string]string) *Context {
	for k, v := range flags {
		c.Flags[k] = v
	}
	return c
}

func (c *Context) MergeConfig(config map[string]interface{}) *Context {
	for k, v := range config {
		c.Config[k] = v
	}
	return c
}

// ---- Accessors ----------------------------------------------------------

func (c *Context) GetArg(index int) string {
	if index >= 0 && index < len(c.Args) {
		return c.Args[index]
	}
	return ""
}

func (c *Context) GetFlag(name string) string {
	return c.Flags[name]
}

func (c *Context) HasFlag(name string) bool {
	_, ok := c.Flags[name]
	return ok
}

func (c *Context) GetEnvVar(name string) string {
	return c.EnvVars[name]
}

func (c *Context) GetConfig(key string) any {
	return c.Config[key]
}

func (c *Context) GetConfigString(key, defaultValue string) string {
	if val, ok := c.Config[key]; ok {
		if s, ok := val.(string); ok {
			return s
		}
	}
	return defaultValue
}

func (c *Context) GetConfigInt(key string, defaultValue int) int {
	if val, ok := c.Config[key]; ok {
		if i, ok := val.(int); ok {
			return i
		}
	}
	return defaultValue
}

func (c *Context) GetConfigBool(key string, defaultValue bool) bool {
	if val, ok := c.Config[key]; ok {
		if b, ok := val.(bool); ok {
			return b
		}
	}
	return defaultValue
}
