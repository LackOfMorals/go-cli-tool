# Go CLI Tool

A modular, extensible CLI framework built with Go, following interface-driven design principles.

## Features

- **Interface-Driven Architecture**: All tools implement Go interfaces for consistent patterns
- **Interactive Shell**: Built-in shell with command history, tab completion, and built-in commands
- **Flexible Logging**: Text and JSON log formats with multiple log levels
- **Configuration Management**: JSON config file, environment variables, and CLI flags
- **Tool Registry**: Dynamic tool registration and discovery system
- **Extensible Design**: Easy to add new tools following the established pattern

## Architecture

### Core Components

```
cmd/main.go          # Application entry point
core/
  logger.go         # Logger interface (TextLogger, JSONLogger)
  config.go         # Config interface and JSON loader
  errors.go        # Common error types
tool/
  tool.go          # Tool interface and BaseTool
  context.go       # Execution context
  result.go        # Result types and IOHandler
shell/
  shell.go         # Shell interface
  interactive.go   # Interactive shell implementation
tools/
  registry.go      # Tool registry
  echo.go          # Echo tool example
  help.go          # Help tool
config/
  config.json      # Default configuration
```

### Interfaces

#### Logger Interface
```go
type Logger interface {
    Debug(msg string, fields ...Field)
    Info(msg string, fields ...Field)
    Warn(msg string, fields ...Field)
    Error(msg string, fields ...Field)
    Fatal(msg string, fields ...Field)
    SetLevel(level LogLevel)
    GetLevel() LogLevel
    WithFields(fields ...Field) Logger
}
```

#### Tool Interface
```go
type Tool interface {
    Name() string
    Description() string
    Version() string
    Execute(ctx Context) (Result, error)
    Validate(ctx Context) error
    Configure(params map[string]interface{}) error
    DefaultParams() map[string]interface{}
}
```

## Usage

### Interactive Shell
```bash
./go-cli-tool shell
```

### Execute Tool Directly
```bash
./go-cli-tool exec echo --args "Hello, World!"
```

### With Configuration File
```bash
./go-cli-tool --config config/config.json shell
```

### With Environment Variables
```bash
CLI_LOG_LEVEL=debug ./go-cli-tool shell
CLI_LOG_FORMAT=json ./go-cli-tool exec echo --args "Test"
```

## Configuration

### JSON Configuration (`config/config.json`)
```json
{
  "log_level": "info",
  "log_format": "text",
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
    }
  }
}
```

### Environment Variables
- `CLI_LOG_LEVEL` - Log level (debug, info, warn, error)
- `CLI_LOG_FORMAT` - Log format (text, json)
- `CLI_SHELL_PROMPT` - Shell prompt string
- `CLI_SHELL_HISTORY` - Shell history file path

### Parameter Resolution Priority
1. Command-line flags (highest)
2. Environment variables
3. Config file
4. Default values (lowest)

## Shell Commands

| Command | Description |
|---------|-------------|
| `exit`, `quit` | Exit the shell |
| `help [tool]` | Show help information |
| `list` | List all available tools |
| `exec <tool> [args]` | Execute a tool |
| `config` | Show current configuration |
| `set <key> <value>` | Set configuration value |
| `log-level <level>` | Change log level |
| `clear` | Clear the screen |
| `version` | Show version information |

## Creating a New Tool

1. Create a new file in `tools/` (e.g., `mytool.go`)
2. Implement the `Tool` interface
3. Register in `cmd/main.go`

```go
package tools

import (
    "strings"
    "github.com/cli/go-cli-tool/tool"
)

type MyTool struct {
    *tool.BaseTool
}

func NewMyTool() *MyTool {
    return &MyTool{
        BaseTool: tool.NewBaseTool(
            "mytool",
            "My custom tool description",
            "1.0.0",
        ),
    }
}

func (t *MyTool) Execute(ctx tool.Context) (tool.Result, error) {
    result := tool.NewResult()
    result.SetSuccess("Done!")
    return *result, nil
}

func (t *MyTool) Validate(ctx tool.Context) error {
    return nil
}

func (t *MyTool) Configure(params map[string]interface{}) error {
    return t.BaseTool.Configure(params)
}

func (t *MyTool) DefaultParams() map[string]interface{} {
    return map[string]interface{}{}
}
```

## Log Formats

### Text Format
```
[2024-01-15 10:30:45] INFO  - Application started | version=1.0.0
```

### JSON Format
```json
{"timestamp":"2024-01-15T10:30:45Z","level":"info","message":"Application started","fields":{"version":"1.0.0"}}
```

## Testing

```bash
go test ./...
```

## Building

```bash
go build -o go-cli-tool ./cmd/main.go
```

## License

MIT
