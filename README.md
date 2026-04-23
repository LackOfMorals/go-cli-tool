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

For longer queries, end a line with `\` to continue on the next line. The shell collects all continuation lines before executing:

```
neo4j> cypher MATCH (n:Person) \
...>         WHERE n.age > 30 \
...>         RETURN n.name, n.age \
...>         ORDER BY n.age DESC LIMIT 10
```

> **Requires a Neo4j connection.** If `neo4j.uri` or `neo4j.username` are not configured, the shell returns a clear error before attempting the query. Typing `cypher` alone always shows usage regardless of connection state.

#### cloud

Manages Neo4j Aura cloud resources.

> **Requires Aura credentials.** If `aura.client_id` or `aura.client_secret` are not configured, the shell returns a clear error before making any API call. Typing `cloud` alone always shows usage regardless of credential state.

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

> **Requires a Neo4j connection.** Same prerequisite as `cypher` — `neo4j.uri` and `neo4j.username` must be configured.

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
        prerequisites.go   Neo4jPrerequisite and AuraPrerequisite factories
    shell/
        category.go     Category type, Command type, routing (Dispatch/Find/SetPrerequisite)
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
        return tool.ErrorResult("usage: mytool <text>"), fmt.Errorf("no input")
    }

    output := fmt.Sprintf("%s %s", t.prefix, ctx.Args[0])
    return tool.SuccessResult(output), nil
}

// Validate runs automatically before Execute — the shell calls it and
// will not proceed to Execute if it returns an error. Use it to check
// prerequisites (required config, service availability) rather than
// duplicating those checks inside Execute.
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
            Aliases:     []string{"idx"},          // optional: short-form names
            Usage:       "show-indexes",
            Description: "List all indexes in the current database",
            Handler: func(args []string, ctx shell.ShellContext) (string, error) {
                // call your service here
                return "indexes output", nil
            },
        })
}
```

Aliases are registered automatically in dispatch and tab completion, and appear in parentheses in `help` output:

```
neo4j> admin show-indexes
neo4j> admin idx            # same command
```

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

In `cmd/neo4-cli/app.go`, add the service field, construct it in `newApp`, and register the category. If the category requires a database or API connection, attach a prerequisite so users get a clear error message instead of a raw driver error:

```go
// In the App struct:
gdsSvc service.GDSService

// In newApp(), after the repo is created:
gdsSvc := service.NewGDSService(repo)

// In buildCategories():
func (a *App) buildCategories() map[string]*shell.Category {
    return map[string]*shell.Category{
        "cypher": commands.BuildCypherCategory(a.cypherSvc).
            SetPrerequisite(commands.Neo4jPrerequisite(&a.cfg.Neo4j)),
        "cloud":  commands.BuildCloudCategory(a.cloudSvc).
            SetPrerequisite(commands.AuraPrerequisite(&a.cfg.Aura)),
        "admin":  commands.BuildAdminCategory(a.adminSvc).
            SetPrerequisite(commands.Neo4jPrerequisite(&a.cfg.Neo4j)),
        "gds":    commands.BuildGDSCategory(a.gdsSvc).        // ← add here
            SetPrerequisite(commands.Neo4jPrerequisite(&a.cfg.Neo4j)),
    }
}
```

Add the prerequisite factory to `internal/commands/prerequisites.go` if you need a check that doesn't fit the existing `Neo4jPrerequisite` or `AuraPrerequisite` patterns.

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

**Prerequisite checks belong in `prerequisites.go`** — if a category requires an external dependency (database connection, API credentials), declare it with `SetPrerequisite` in `app.go`. Write the factory function in `internal/commands/prerequisites.go` so it is independently testable. The check runs before every real dispatch, but bare category invocations (e.g. `cypher` alone) always show help regardless. Use `fmt.Errorf("%w: ...", tool.ErrPrerequisite)` so callers can identify prerequisite errors with `errors.Is`.

**Tool `Validate` runs before `Execute`** — the shell calls `Validate` automatically before every `Execute`. Use `Validate` for tool-level readiness checks (required arguments, service availability) rather than repeating the check inside `Execute`. `BaseTool.Validate` is a no-op, so only override it when needed.

**Command aliases are first-class** — add short-form names via the `Aliases []string` field on `Command`. Aliases are resolved in dispatch and tab completion, and shown in `help` output. Register canonical names as the `Name` field; aliases are supplementary.

**Secrets stay out of flags** — passwords and API secrets must not be CLI flags. Accept them only via environment variables or the config file.

**Errors are the caller's responsibility** — return `fmt.Errorf("context: %w", err)` and let the shell print it. Don't call `os.Exit` or `log.Fatal` from inside a service or command handler.

**No imports up the stack** — `shell` does not import `commands` or `service`. `commands` does not import `tools`. Keep the dependency graph acyclic and flowing in one direction toward `cmd`.
