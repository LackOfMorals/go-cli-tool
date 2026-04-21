# Go CLI Tool Architecture Specification

## Project Overview

**Project Name:** go-cli-tool
**Project Type:** Go-based CLI Application
**Core Functionality:** A modular, extensible CLI framework with shell interface, logging, and configuration management
**Target Users:** Developers building CLI tools requiring a consistent architecture pattern

---

## Architecture Design

### Core Principles

1. **Interface-Driven Design**: All tools and components implement Go interfaces
2. **Dependency Injection**: Components receive dependencies through interfaces, not concrete types
3. **Composability**: Tools can be composed and chained together
4. **Extensibility**: New tools can be added without modifying existing code

---

## Module Structure

### 1. Core Package (`/core`)

#### 1.1 Logger Interface (`/core/logger.go`)

```go
// Logger defines the interface for logging operations
type Logger interface {
    Debug(msg string, fields ...Field)
    Info(msg string, fields ...Field)
    Warn(msg string, fields ...Field)
    Error(msg string, fields ...Field)
    Fatal(msg string, fields ...Field)
}

// Field represents a log field key-value pair
type Field struct {
    Key   string
    Value interface{}
}

// TextLogger implements Logger for text output
// JSONLogger implements Logger for JSON output
```

#### 1.2 Config Interface (`/core/config.go`)

```go
// ConfigLoader defines the interface for loading configuration
type ConfigLoader interface {
    Load(path string) (Config, error)
    Save(path string, config Config) error
}

// Config represents the application configuration
type Config struct {
    LogLevel   string
    LogFormat  string // "text" or "json"
    Tools      []ToolConfig
    Shell      ShellConfig
}

// ToolConfig represents individual tool configuration
type ToolConfig struct {
    Name   string
    Enabled bool
    Params map[string]interface{}
}
```

---

### 2. Shell Package (`/shell`)

#### 2.1 Shell Interface (`/shell/shell.go`)

```go
// Shell defines the interface for CLI shell operations
type Shell interface {
    Start() error
    Stop() error
    Execute(cmd string) (string, error)
    RegisterCommand(name string, handler CommandHandler)
    SetLogger(logger core.Logger)
}

// CommandHandler is a function that handles shell commands
type CommandHandler func(args []string, ctx ShellContext) (string, error)

// ShellContext provides context for command execution
type ShellContext struct {
    Config  core.Config
    Logger  core.Logger
    Tools   map[string]Tool
}
```

#### 2.2 Interactive Shell (`/shell/interactive.go`)

```go
// InteractiveShell implements Shell with readline support
type InteractiveShell struct {
    reader     *readline.Reader
    handlers   map[string]CommandHandler
    logger     core.Logger
    config     core.Config
    tools      map[string]Tool
}
```

---

### 3. Tool Package (`/tool`)

#### 3.1 Tool Interface (`/tool/tool.go`)

```go
// Tool defines the interface all CLI tools must implement
type Tool interface {
    // Metadata
    Name() string
    Description() string
    Version() string

    // Execution
    Execute(ctx Context) (Result, error)
    Validate(ctx Context) error

    // Configuration
    Configure(params map[string]interface{}) error
    DefaultParams() map[string]interface{}
}

// Context provides execution context for tools
type Context struct {
    Args      []string
    Flags     map[string]string
    EnvVars   map[string]string
    Config    map[string]interface{}
    Logger    core.Logger
    IO        IOHandler
}

// Result represents the output of a tool execution
type Result struct {
    Success bool
    Output  string
    Error   error
    Data    map[string]interface{}
    Logs    []string
}

// IOHandler defines the interface for input/output operations
type IOHandler interface {
    Read() (string, error)
    Write(format string, args ...interface{})
    WriteError(err error)
}
```

#### 3.2 Base Tool (`/tool/base.go`)

```go
// BaseTool provides common functionality for all tools
type BaseTool struct {
    name        string
    description string
    version     string
    params      map[string]interface{}
    logger      core.Logger
}

// NewBaseTool creates a new base tool with common functionality
func NewBaseTool(name, description, version string) *BaseTool
```

---

### 4. Tools Package (`/tools`)

Example tool implementations demonstrating the pattern:

#### 4.1 Echo Tool (`/tools/echo.go`)

```go
// EchoTool implements a simple echo functionality
type EchoTool struct {
    *tool.BaseTool
    uppercase bool
    repeat    int
}

// NewEchoTool creates a new echo tool
func NewEchoTool() *EchoTool

// Execute implements Tool interface
func (t *EchoTool) Execute(ctx tool.Context) (tool.Result, error)

// Validate implements Tool interface
func (t *EchoTool) Validate(ctx tool.Context) error

// Configure implements Tool interface
func (t *EchoTool) Configure(params map[string]interface{}) error

// DefaultParams implements Tool interface
func (t *EchoTool) DefaultParams() map[string]interface{}
```

#### 4.2 Help Tool (`/tools/help.go`)

```go
// HelpTool provides help information for all registered tools
type HelpTool struct {
    *tool.BaseTool
    tools map[string]tool.Tool
}

// NewHelpTool creates a new help tool
func NewHelpTool(registry *ToolRegistry) *HelpTool
```

---

### 5. Registry Package (`/registry`)

#### 5.1 Tool Registry (`/registry/registry.go`)

```go
// ToolRegistry manages registration and discovery of tools
type ToolRegistry struct {
    tools  map[string]tool.Tool
    mutex  sync.RWMutex
}

// Register adds a tool to the registry
func (r *ToolRegistry) Register(t tool.Tool) error

// Get retrieves a tool by name
func (r *ToolRegistry) Get(name string) (tool.Tool, error)

// List returns all registered tools
func (r *ToolRegistry) List() []tool.Tool

// Unregister removes a tool from the registry
func (r *ToolRegistry) Unregister(name string) error
```

---

## Configuration System

### JSON Configuration File (`config.json`)

```json
{
    "log_level": "info",
    "log_format": "json",
    "shell": {
        "prompt": "cli> ",
        "history_file": ".cli_history"
    },
    "tools": {
        "echo": {
            "enabled": true,
            "params": {
                "uppercase": false,
                "repeat": 1
            }
        },
        "help": {
            "enabled": true
        }
    },
    "custom_params": {
        "key": "value"
    }
}
```

### Environment Variable Support

- Prefix: `CLI_`
- Example: `CLI_LOG_LEVEL=debug`
- Environment variables override config file settings

### Parameter Resolution Priority

1. Command-line flags (highest)
2. Environment variables
3. Config file
4. Default values (lowest)

---

## Logging System

### Log Levels

- `debug`: Detailed debugging information
- `info`: General operational information
- `warn`: Warning messages
- `error`: Error messages
- `fatal`: Fatal errors that cause application exit

### Log Formats

#### Text Format
```
[2024-01-15 10:30:45] INFO  main.go:42 - Application started
[2024-01-15 10:30:45] DEBUG tool.go:56 - Executing echo tool
```

#### JSON Format
```json
{
    "timestamp": "2024-01-15T10:30:45Z",
    "level": "info",
    "message": "Application started",
    "file": "main.go",
    "line": 42,
    "fields": {
        "version": "1.0.0"
    }
}
```

---

## Shell Interface

### Built-in Commands

| Command | Description |
|---------|-------------|
| `exit` / `quit` | Exit the shell |
| `help [tool]` | Show help information |
| `list` | List all available tools |
| `exec <tool> [args]` | Execute a specific tool |
| `config` | Show current configuration |
| `set <key> <value>` | Set configuration value |
| `log-level <level>` | Change log level |

### Interactive Features

- Command history (up/down arrows)
- Tab completion for tool names
- Ctrl+C to cancel current operation
- Clear screen command

---

## Project Structure

```
go-cli-tool/
├── cmd/
│   └── main.go              # Application entry point
├── core/
│   ├── logger.go            # Logger interface and implementations
│   ├── config.go           # Configuration interface and loader
│   └── errors.go           # Common error types
├── shell/
│   ├── shell.go            # Shell interface
│   ├── interactive.go      # Interactive shell implementation
│   └── commands.go         # Built-in shell commands
├── tool/
│   ├── tool.go             # Tool interface and base implementation
│   ├── context.go          # Execution context
│   └── result.go           # Result types
├── tools/
│   ├── registry.go         # Tool registry
│   ├── echo.go             # Echo tool implementation
│   ├── help.go             # Help tool implementation
│   └── template.go         # Template for new tools
├── config/
│   └── config.json         # Default configuration file
├── go.mod
├── go.sum
├── README.md
└── SPEC.md
```

---

## Implementation Checklist

- [ ] Project structure and go.mod
- [ ] Core logger interface with text and JSON implementations
- [ ] Core config interface with JSON loader
- [ ] Tool interface with base implementation
- [ ] Tool registry
- [ ] Shell interface and interactive implementation
- [ ] Built-in shell commands
- [ ] Echo tool implementation
- [ ] Help tool implementation
- [ ] Main entry point with dependency wiring
- [ ] Unit tests for core components
- [ ] Configuration file example

---

## Dependencies

```go
require (
    github.com/spf13/cobra v1.8.0
    github.com/spf13/viper v1.18.2
    github.com/chzyer/readline v1.4.0
    github.com/stretchr/testify v1.8.4
    go.uber.org/zap v1.26.0
)
```

---

## Usage Examples

### Interactive Mode
```bash
./go-cli-tool shell
```

### Direct Tool Execution
```bash
./go-cli-tool exec echo "Hello, World!"
```

### With Configuration
```bash
./go-cli-tool --config /path/to/config.json exec echo --uppercase "Hello"
```

### With Environment Variables
```bash
CLI_LOG_LEVEL=debug ./go-cli-tool shell
```

---

## Extension Guide

### Creating a New Tool

1. Create a new file in `/tools` (e.g., `mytool.go`)
2. Implement the `Tool` interface
3. Register in main.go using the registry

```go
type MyTool struct {
    *tool.BaseTool
    // Add your tool-specific fields
}

func NewMyTool() *MyTool {
    return &MyTool{
        BaseTool: tool.NewBaseTool("mytool", "My custom tool", "1.0.0"),
    }
}

func (t *MyTool) Execute(ctx tool.Context) (tool.Result, error) {
    // Implementation
    return tool.Result{Success: true, Output: "Done"}, nil
}
```

---

## Version

- Specification Version: 1.0.0
- Created: 2024-01-15
- Last Updated: 2024-01-15
