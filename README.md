# neo4j-cli

A CLI for Neo4j with an interactive shell. Connect to a Neo4j database and the Neo4j Aura management API from a single tool — run Cypher queries, manage cloud instances, and perform administrative operations.

---

## Contents

- [Getting started](#getting-started)
- [Configuration](#configuration)
- [Shell usage](#shell-usage)
- [Contributing](#contributing)
  - [Project structure](#project-structure)
  - [Adding a tool](#adding-a-tool)
  - [Adding a shell command](#adding-a-shell-command)
  - [Adding a shell category](#adding-a-shell-category)
  - [Code conventions](#code-conventions)

---

## Getting started

### Prerequisites

- Go 1.23 or later

### Build

```bash
git clone <repo-url>
cd go-cli-tool
go build -o bin/neo4j-cli ./cmd/neo4-cli
```

### Run

```bash
# Interactive shell (default)
./bin/neo4j-cli

# Execute a single tool and exit
./bin/neo4j-cli --exec echo --args "hello world"

# Point at a config file
./bin/neo4j-cli --config-file ~/.neo4j-cli.json
```

---

## Configuration

Settings are resolved in this order, highest priority first:

```
CLI flags  >  environment variables  >  config file  >  defaults
```

### Config file

Pass `--config-file <path>` to load a JSON file. A minimal example:

```json
{
  "log_level": "info",
  "log_format": "text",
  "neo4j": {
    "uri": "bolt://localhost:7687",
    "username": "neo4j",
    "password": "secret",
    "database": "neo4j"
  },
  "aura": {
    "client_id": "your-client-id",
    "client_secret": "your-client-secret",
    "timeout_seconds": 30
  },
  "shell": {
    "prompt": "neo4j> ",
    "history_file": ".neo4j_history"
  }
}
```

### Environment variables

All variables use the `CLI_` prefix. Nested keys use underscores.

| Variable | Default | Description |
|---|---|---|
| `CLI_LOG_LEVEL` | `info` | Log level: `debug`, `info`, `warn`, `error` |
| `CLI_LOG_FORMAT` | `text` | Log format: `text`, `json` |
| `CLI_NEO4J_URI` | `bolt://localhost:7687` | Neo4j bolt URI |
| `CLI_NEO4J_USERNAME` | `neo4j` | Neo4j username |
| `CLI_NEO4J_PASSWORD` | _(empty)_ | Neo4j password — prefer env over config file |
| `CLI_NEO4J_DATABASE` | `neo4j` | Neo4j database name |
| `CLI_AURA_CLIENT_ID` | _(empty)_ | Aura API client ID |
| `CLI_AURA_CLIENT_SECRET` | _(empty)_ | Aura API client secret — prefer env over config file |
| `CLI_AURA_TIMEOUT_SECONDS` | `30` | Aura API request timeout |

> **Security note:** `CLI_NEO4J_PASSWORD` and `CLI_AURA_CLIENT_SECRET` are intentionally not available as CLI flags. Passing secrets as flags exposes them in shell history and `ps` output.

### CLI flags

```
--config-file string      Path to a JSON configuration file
--neo4j-uri string        Neo4j bolt URI
--neo4j-username string   Neo4j username
--neo4j-database string   Neo4j database name
--aura-client-id string   Aura API client ID
--aura-timeout int        Aura API timeout in seconds
--log-level string        Log level
--log-format string       Log format
--exec string             Execute a named tool directly and exit
--args strings            Arguments for --exec
```

---

## Shell usage

Start the shell with no arguments, or with `--shell`:

```
$ ./bin/neo4j-cli

Neo4j CLI — type 'help' for commands, 'exit' to quit.

neo4j>
```

### Categories

The shell is organised into three top-level categories.

#### cypher

Executes a Cypher query against the connected database. The entire remainder of the line is sent as the query — no quoting needed.

```
neo4j> cypher MATCH (n) RETURN n LIMIT 5
neo4j> cypher MATCH (n:Person {name: "Alice"})-[:KNOWS]->(m) RETURN m
neo4j> cypher CREATE (n:Person {name: "Bob"}) RETURN n
```

#### cloud

Manages Neo4j Aura cloud resources. Requires `CLI_AURA_CLIENT_ID` and `CLI_AURA_CLIENT_SECRET` to be set.

```
neo4j> cloud instances list
neo4j> cloud instances get <id>
neo4j> cloud instances pause <id>
neo4j> cloud instances resume <id>
neo4j> cloud instances delete <id>

neo4j> cloud projects list
neo4j> cloud projects get <id>
```

#### admin

Runs administrative commands against the connected database.

```
neo4j> admin show-users
neo4j> admin show-databases
```

### Built-in commands

```
neo4j> help                    Overview of all categories
neo4j> help cloud              Help for the cloud category
neo4j> help cloud instances    Help for cloud > instances
neo4j> config                  Show current configuration
neo4j> set prompt "db> "       Change the prompt
neo4j> set log-level debug     Change the log level
neo4j> log-level               Show current log level
neo4j> clear                   Clear the screen
neo4j> version                 Show version
neo4j> exit                    Exit the shell
```

---

## Contributing

### Project structure

```
cmd/neo4-cli/
    main.go             Entry point — calls run() and os.Exit
    app.go              App struct, startup wiring, flag definitions

internal/
    config/             Config structs, Viper loader, flag overrides
    logger/             Logger interface and slog implementation
    analytics/          Mixpanel analytics service
    presentation/       Output formatters (text, JSON, table)
    repository/         GraphRepository interface and Neo4j implementation
    service/
        interfaces.go   CypherService, CloudService, AdminService interfaces
        cypher_service.go
        cloud_service.go
        admin_service.go
        graph_service.go
    commands/           Shell category builders (pure wiring, no logic)
        cypher.go
        cloud.go
        admin.go
    shell/
        category.go     Category type, Command type, routing (Dispatch/Find)
        shell.go        Shell interface, ShellContext, built-in command names
        interactive.go  InteractiveShell — the REPL implementation
    tool/               Tool interface, BaseTool, Context, Result
    tools/              Concrete tool implementations + ToolRegistry
```

The dependency direction is strict:

```
cmd  →  commands  →  service  →  repository
cmd  →  shell
cmd  →  tools
commands  →  shell   (for Category / Command types)
```

No package imports its own parent. `shell` does not import `commands` or `service`.

---

### Adding a tool

Tools are flat, named executables that can be invoked from the shell with `exec <name>` or from the command line with `--exec <name>`. They sit alongside the category hierarchy and are independent of it.

**Step 1 — Create the file**

Create `internal/tools/mytool.go`. Embed `*tool.BaseTool` to get the parameter helpers for free:

```go
package tools

import (
    "fmt"
    "github.com/cli/go-cli-tool/internal/tool"
)

type MyTool struct {
    *tool.BaseTool
    // add any tool-specific fields here
    prefix string
}

func NewMyTool() *MyTool {
    return &MyTool{
        BaseTool: tool.NewBaseTool(
            "mytool",                        // name — must be unique in the registry
            "Does something interesting",    // description shown in help
            "1.0.0",                         // version
        ),
        prefix: ">>",
    }
}
```

**Step 2 — Implement the interface**

You must implement all four methods. `BaseTool` provides no-op defaults for `Validate` and `Configure`, so you only need to override the ones that matter:

```go
// Execute is the only method you must override.
func (t *MyTool) Execute(ctx tool.Context) (tool.Result, error) {
    if len(ctx.Args) == 0 {
        return tool.ErrorResult("usage: mytool <text>", fmt.Errorf("no input")), nil
    }

    output := fmt.Sprintf("%s %s", t.prefix, ctx.Args[0])
    return tool.SuccessResult(output), nil
}

// Validate runs before Execute. Return an error to abort.
func (t *MyTool) Validate(ctx tool.Context) error {
    if len(ctx.Args) == 0 {
        return fmt.Errorf("at least one argument is required")
    }
    return nil
}

// Configure applies parameters from the config file.
func (t *MyTool) Configure(params map[string]interface{}) error {
    t.BaseTool.Configure(params) // always call through first
    if v, ok := params["prefix"].(string); ok {
        t.prefix = v
    }
    return nil
}

// DefaultParams declares the default configuration values.
func (t *MyTool) DefaultParams() map[string]interface{} {
    return map[string]interface{}{
        "prefix": ">>",
    }
}
```

**Step 3 — Register it**

In `cmd/neo4-cli/app.go`, add your tool to the slice inside `buildRegistry`:

```go
for _, t := range []tool.Tool{
    tools.NewEchoTool(),
    tools.NewHelpTool(registry),
    tools.NewQueryTool(graphSvc),
    tools.NewMyTool(),           // ← add here
} {
    registerTool(registry, t, cfg, log)
}
```

**Step 4 — (Optional) Configure via config file**

If your tool has configurable behaviour, users can set it in their JSON config file:

```json
{
  "tools": {
    "mytool": {
      "enabled": true,
      "params": {
        "prefix": "---"
      }
    }
  }
}
```

**Step 5 — Test it**

```bash
go build ./... && ./bin/neo4j-cli --exec mytool --args "hello"
```

Or from the shell:

```
neo4j> exec mytool hello
```

---

### Adding a shell command

A shell command sits inside an existing category or sub-category. It takes named arguments and returns a formatted string.

**Example: adding `admin show-indexes` to the `admin` category**

Open `internal/commands/admin.go` and call `AddCommand` on the returned category:

```go
func BuildAdminCategory(svc service.AdminService) *shell.Category {
    return shell.NewCategory("admin", "Administrative operations...").
        AddCommand(&shell.Command{
            Name:        "show-users",
            // ... existing command ...
        }).
        AddCommand(&shell.Command{
            Name:        "show-indexes",
            Usage:       "show-indexes",
            Description: "List all indexes in the current database",
            Handler: func(args []string, ctx shell.ShellContext) (string, error) {
                // call your service here
                return "indexes output", nil
            },
        })
}
```

If the command needs data from a service that `AdminService` doesn't yet expose, add the method to the interface in `internal/service/interfaces.go` first, then implement it in `internal/service/admin_service.go`.

**Example: adding a command to a sub-category**

To add `cloud instances clone <id>` open `internal/commands/cloud.go` and chain another `AddCommand` onto the `instances` category builder:

```go
func buildInstancesCategory(svc service.InstancesService) *shell.Category {
    return shell.NewCategory("instances", "Manage Aura DB instances").
        AddCommand(&shell.Command{
            Name:        "list",
            // ...
        }).
        AddCommand(&shell.Command{
            Name:        "clone",
            Usage:       "clone <id>",
            Description: "Clone an existing instance",
            Handler: func(args []string, ctx shell.ShellContext) (string, error) {
                if len(args) == 0 {
                    return "", fmt.Errorf("usage: cloud instances clone <id>")
                }
                // call svc.Clone(context.Background(), args[0])
                return fmt.Sprintf("Instance %s cloned.", args[0]), nil
            },
        })
}
```

---

### Adding a shell category

To add a new top-level category (e.g. `gds` for Graph Data Science):

**Step 1 — Add a service interface**

In `internal/service/interfaces.go`:

```go
// GDSService runs Graph Data Science operations.
type GDSService interface {
    ListAlgorithms(ctx context.Context) ([]string, error)
    RunPageRank(ctx context.Context, graphName string) (string, error)
}
```

**Step 2 — Implement the service**

Create `internal/service/gds_service.go`:

```go
package service

import (
    "context"
    "github.com/cli/go-cli-tool/internal/repository"
)

type GDSServiceImpl struct {
    repo repository.GraphRepository
}

func NewGDSService(repo repository.GraphRepository) GDSService {
    return &GDSServiceImpl{repo: repo}
}

func (s *GDSServiceImpl) ListAlgorithms(ctx context.Context) ([]string, error) {
    // implement using CALL gds.list()
    return nil, nil
}

func (s *GDSServiceImpl) RunPageRank(ctx context.Context, graphName string) (string, error) {
    // implement
    return "", nil
}
```

**Step 3 — Build the category**

Create `internal/commands/gds.go`:

```go
package commands

import (
    "context"
    "strings"

    "github.com/cli/go-cli-tool/internal/service"
    "github.com/cli/go-cli-tool/internal/shell"
)

func BuildGDSCategory(svc service.GDSService) *shell.Category {
    return shell.NewCategory("gds", "Graph Data Science operations").
        AddCommand(&shell.Command{
            Name:        "list-algorithms",
            Usage:       "list-algorithms",
            Description: "List available GDS algorithms",
            Handler: func(args []string, ctx shell.ShellContext) (string, error) {
                algos, err := svc.ListAlgorithms(context.Background())
                if err != nil {
                    return "", err
                }
                return strings.Join(algos, "\n"), nil
            },
        }).
        AddCommand(&shell.Command{
            Name:        "pagerank",
            Usage:       "pagerank <graph-name>",
            Description: "Run PageRank on a projected graph",
            Handler: func(args []string, ctx shell.ShellContext) (string, error) {
                if len(args) == 0 {
                    return "", fmt.Errorf("usage: gds pagerank <graph-name>")
                }
                return svc.RunPageRank(context.Background(), args[0])
            },
        })
}
```

**Step 4 — Wire it into App**

In `cmd/neo4-cli/app.go`, add the service field, construct it in `newApp`, and register the category:

```go
// In the App struct:
gdsSvc service.GDSService

// In newApp(), after the repo is created:
gdsSvc := service.NewGDSService(repo)

// In buildCategories():
func (a *App) buildCategories() map[string]*shell.Category {
    return map[string]*shell.Category{
        "cypher": commands.BuildCypherCategory(a.cypherSvc),
        "cloud":  commands.BuildCloudCategory(a.cloudSvc),
        "admin":  commands.BuildAdminCategory(a.adminSvc),
        "gds":    commands.BuildGDSCategory(a.gdsSvc),   // ← add here
    }
}
```

The category is now live in the shell:

```
neo4j> gds list-algorithms
neo4j> gds pagerank my-graph
neo4j> help gds
```

---

### Code conventions

**Service pattern** — business logic lives in `internal/service`. Command handlers in `internal/commands` call services; they contain no logic of their own beyond argument validation and output formatting.

**Interfaces before implementations** — new behaviour starts with an interface in `internal/service/interfaces.go`. This keeps the command layer and the shell decoupled from concrete implementations and makes testing straightforward.

**Secrets stay out of flags** — passwords and API secrets must not be CLI flags. Accept them only via environment variables or the config file.

**Errors are the caller's responsibility** — return `fmt.Errorf("context: %w", err)` and let the shell print it. Don't call `os.Exit` or `log.Fatal` from inside a service or command handler.

**No imports up the stack** — `shell` does not import `commands` or `service`. `commands` does not import `tools`. Keep the dependency graph acyclic and flowing in one direction toward `cmd`.
