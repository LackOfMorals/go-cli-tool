# neo4j-cli

A CLI for Neo4j with an interactive shell. Connect to a Neo4j database and the Neo4j Aura management API from a single tool — run Cypher queries, manage cloud instances, and perform administrative operations.

---

## Contents

- [Getting started](#getting-started)
- [Usage modes](#usage-modes)
- [Configuration](#configuration)
- [Shell usage](#shell-usage)
- [Contributing](#contributing)
  - [Project structure](#project-structure)
  - [Adding a tool](#adding-a-tool)
  - [Adding a shell command](#adding-a-shell-command)
  - [Adding a shell category](#adding-a-shell-category)
  - [Code conventions](#code-conventions)
- [CI & Releases](#ci--releases)

---

## Getting started

### Prerequisites

- Go 1.24 or later

### Build

```bash
git clone <repo-url>
cd go-cli-tool
go build -o bin/neo4j-cli ./cmd/neo4j-cli
```

### Run

```bash
# Interactive shell (default when no subcommand is given)
./bin/neo4j-cli

# Run a subcommand directly
./bin/neo4j-cli cypher "MATCH (n:Person) RETURN n.name LIMIT 5"
./bin/neo4j-cli cloud instances list
./bin/neo4j-cli admin show-databases
./bin/neo4j-cli config list

# Point at a config file
./bin/neo4j-cli --config-file ~/.neo4j-cli/config.json cloud instances list
```

---

## Usage modes

`neo4j-cli` has two modes of operation:

**Interactive shell** — run with no arguments (or with `--shell`) to get a REPL prompt. Readline history, tab completion, multi-line input, and session-level settings are all available.

**Direct subcommand** — pass a subcommand and its arguments directly on the command line for use in scripts or CI pipelines. All four categories are available as top-level subcommands:

| Subcommand | Description |
|---|---|
| `cypher [query]` | Execute a Cypher query against the connected database |
| `cloud` | Manage Neo4j Aura cloud resources |
| `admin` | Administrative operations against the connected database |
| `config` | Manage CLI configuration |

---

## Configuration

Settings are resolved in this order, highest priority first:

```
CLI flags  >  environment variables  >  config file  >  defaults
```

### Config file

The default config file path is `~/.neo4j-cli/config.json`. The directory and file are created automatically when credentials are first saved via an interactive prompt. Pass `--config-file <path>` to use a different location.

A full example:

```json
{
  "log_level": "info",
  "log_format": "text",
  "log_output": "stderr",
  "log_file": "",
  "neo4j": {
    "uri": "bolt://localhost:7687",
    "username": "neo4j",
    "password": "secret",
    "database": "neo4j"
  },
  "aura": {
    "client_id": "your-client-id",
    "client_secret": "your-client-secret",
    "timeout_seconds": 30,
    "instance_defaults": {
      "tenant_id": "your-tenant-id",
      "cloud_provider": "gcp",
      "region": "europe-west1",
      "type": "enterprise-db",
      "version": "5",
      "memory": "8GB"
    }
  },
  "cypher": {
    "shell_limit": 25,
    "exec_limit": 100,
    "output_format": "table"
  },
  "shell": {
    "prompt": "neo4j> ",
    "history_file": ".neo4j_history"
  },
  "telemetry": {
    "metrics": true
  }
}
```

### Environment variables

All variables use the `CLI_` prefix. Nested keys use underscores.

| Variable | Default | Description |
|---|---|---|
| `CLI_LOG_LEVEL` | `info` | Log level: `debug`, `info`, `warn`, `error` |
| `CLI_LOG_FORMAT` | `text` | Log format: `text`, `json` |
| `CLI_LOG_OUTPUT` | `stderr` | Log destination: `stderr`, `stdout`, `file` |
| `CLI_LOG_FILE` | _(empty)_ | Log file path (used when `CLI_LOG_OUTPUT=file`) |
| `CLI_NEO4J_URI` | `bolt://localhost:7687` | Neo4j bolt URI |
| `CLI_NEO4J_USERNAME` | `neo4j` | Neo4j username |
| `CLI_NEO4J_PASSWORD` | _(empty)_ | Neo4j password — prefer env over config file |
| `CLI_NEO4J_DATABASE` | `neo4j` | Neo4j database name |
| `CLI_AURA_CLIENT_ID` | _(empty)_ | Aura API client ID |
| `CLI_AURA_CLIENT_SECRET` | _(empty)_ | Aura API client secret — prefer env over config file |
| `CLI_AURA_TIMEOUT_SECONDS` | `30` | Aura API request timeout |
| `CLI_AURA_INSTANCE_DEFAULTS_TENANT_ID` | _(empty)_ | Default tenant ID for new instances |
| `CLI_AURA_INSTANCE_DEFAULTS_CLOUD_PROVIDER` | `gcp` | Default cloud provider: `aws`, `gcp`, `azure` |
| `CLI_AURA_INSTANCE_DEFAULTS_REGION` | `europe-west1` | Default region for new instances |
| `CLI_AURA_INSTANCE_DEFAULTS_TYPE` | `enterprise-db` | Default instance type |
| `CLI_AURA_INSTANCE_DEFAULTS_VERSION` | `5` | Default Neo4j version |
| `CLI_AURA_INSTANCE_DEFAULTS_MEMORY` | `8GB` | Default instance memory |
| `CLI_CYPHER_SHELL_LIMIT` | `25` | Default LIMIT injected in interactive shell mode |
| `CLI_CYPHER_EXEC_LIMIT` | `100` | Default LIMIT injected in non-interactive mode |
| `CLI_CYPHER_OUTPUT_FORMAT` | `table` | Default output format: `table`, `json`, `pretty-json`, `graph` |
| `CLI_TELEMETRY_METRICS` | `true` | Send anonymous usage metrics to Neo4j |

> **Security note:** `CLI_NEO4J_PASSWORD` and `CLI_AURA_CLIENT_SECRET` are intentionally not available as CLI flags. Passing secrets as flags exposes them in shell history and `ps` output.

### CLI flags

```
--config-file string      Path to a JSON configuration file
--neo4j-uri string        Neo4j bolt URI
--neo4j-username string   Neo4j username
--neo4j-database string   Neo4j database name
--aura-client-id string   Aura API client ID
--aura-timeout int        Aura API timeout in seconds
--log-level string        Log level: debug, info, warn, error
--log-format string       Log format: text, json
--log-output string       Log destination: stderr, stdout, file
--log-file string         Log file path (when --log-output=file)
--format string           Output format: table, json, pretty-json, graph
--no-metrics              Disable anonymous usage metrics
--shell, -s               Force interactive shell mode
```

### Interactive credential prompts

When Neo4j or Aura credentials are missing, the shell prompts for them interactively on first use and saves them to the config file for future sessions. To skip prompts, pre-populate credentials via environment variables or the config file.

---

## Shell usage

Start the shell with no arguments or with `--shell`:

```
$ ./bin/neo4j-cli

Neo4j CLI — type 'help' for commands, 'exit' to quit.

neo4j>
```

### Categories

The shell is organised into four top-level categories.

#### cypher

Executes a Cypher query against the connected database. Supports inline flags for parameters, output format, and row limit.

```
neo4j> cypher MATCH (n) RETURN n LIMIT 5
neo4j> cypher MATCH (n:Person {name: "Alice"})-[:KNOWS]->(m) RETURN m
neo4j> cypher --param name=Alice MATCH (n:Person {name:$name}) RETURN n
neo4j> cypher --format json MATCH (n) RETURN n
neo4j> cypher --format graph MATCH (n)-[r]->(m) RETURN n,r,m
neo4j> cypher --limit 100 MATCH (n) RETURN n
```

Flags (placed before the query):

| Flag | Description |
|---|---|
| `--param key=value` | Add a query parameter (repeatable). Values are auto-typed: int, float, bool, string. |
| `--format table\|json\|pretty-json\|graph` | Override the output format for this query. |
| `--limit N` | Override the auto-injected row limit. |

Typing `cypher` alone (no query) drops into an interactive multi-line prompt. End the statement with `;` to execute.

> **Requires a Neo4j connection.** If credentials are not configured, you are prompted to enter them on first use.

#### cloud

Manages Neo4j Aura cloud resources.

> **Requires Aura credentials.** If `aura.client_id` or `aura.client_secret` are not configured, you are prompted to enter them on first use.

```
neo4j> cloud instances list
neo4j> cloud instances ls                                                  # alias for list
neo4j> cloud instances get <id>
neo4j> cloud instances create name=<n> tenant=<id> [cloud=<p>] [region=<r>] [type=<t>] [version=<v>] [memory=<size>]
neo4j> cloud instances update <id> [name=<new-name>] [memory=<size>]
neo4j> cloud instances pause <id>
neo4j> cloud instances resume <id>
neo4j> cloud instances delete <id>
neo4j> cloud instances rm <id>                                             # alias for delete

neo4j> cloud projects list
neo4j> cloud projects ls                                                   # alias for list
neo4j> cloud projects get <id>
```

`instances create` requires `name` and `tenant`. All other fields fall back to `aura.instance_defaults` in the config. Set defaults to avoid re-typing them on every invocation:

```
neo4j> config set aura.instance_defaults.tenant_id abc-123
neo4j> config set aura.instance_defaults.cloud_provider aws
neo4j> cloud instances create name=my-db
```

> **Save your password.** When `instances create` succeeds, the initial password is shown exactly once and cannot be recovered.

#### admin

Runs administrative commands against the connected database.

> **Requires a Neo4j connection.** Same prerequisite as `cypher`.

```
neo4j> admin show-users
neo4j> admin show-databases
```

#### config

Manages CLI configuration. Changes made with `set`, `delete`, and `reset` are persisted to the config file immediately and take effect in the current session.

```
neo4j> config list                             # show all keys, values, and descriptions
neo4j> config ls                               # alias for list
neo4j> config set neo4j.uri bolt://myhost:7687
neo4j> config set cypher.output_format json
neo4j> config set aura.instance_defaults.region us-east-1
neo4j> config delete neo4j.password            # reset a key to its default (prompts)
neo4j> config reset                            # wipe config file, restore all defaults (prompts)
```

All settable keys (use `config list` to see current values):

| Key | Default | Description |
|---|---|---|
| `log.level` | `info` | Log verbosity |
| `log.format` | `text` | Log format |
| `log.output` | `stderr` | Log destination |
| `log.file` | _(empty)_ | Log file path |
| `shell.prompt` | `neo4j> ` | Shell prompt string |
| `shell.history_file` | `.neo4j_history` | History file path |
| `telemetry.metrics` | `true` | Send anonymous usage metrics |
| `neo4j.uri` | `bolt://localhost:7687` | Bolt connection URI |
| `neo4j.username` | `neo4j` | Neo4j username |
| `neo4j.password` | _(empty)_ | Neo4j password |
| `neo4j.database` | `neo4j` | Default database name |
| `aura.client_id` | _(empty)_ | Aura API client ID |
| `aura.client_secret` | _(empty)_ | Aura API client secret |
| `aura.timeout_seconds` | `30` | Aura API request timeout |
| `aura.instance_defaults.tenant_id` | _(empty)_ | Default tenant for new instances |
| `aura.instance_defaults.cloud_provider` | `gcp` | Default cloud provider |
| `aura.instance_defaults.region` | `europe-west1` | Default region |
| `aura.instance_defaults.type` | `enterprise-db` | Default instance type |
| `aura.instance_defaults.version` | `5` | Default Neo4j version |
| `aura.instance_defaults.memory` | `8GB` | Default instance memory |
| `cypher.shell_limit` | `25` | Default LIMIT in shell mode |
| `cypher.exec_limit` | `100` | Default LIMIT in non-interactive mode |
| `cypher.output_format` | `table` | Default output format |

### Built-in commands

```
neo4j> help                        Overview of all categories
neo4j> help cloud                  Help for the cloud category
neo4j> help cloud instances        Help for cloud > instances
neo4j> set prompt "db> "           Session-only prompt change
neo4j> set log-level debug         Session-only log level change
neo4j> set cypher-format json      Session-only output format change
neo4j> set cypher-limit 50         Session-only row limit change
neo4j> log-level                   Show current log level
neo4j> clear                       Clear the screen
neo4j> version                     Show version
neo4j> exit / quit                 Exit the shell
```

> `set` changes apply to the current session only. Use `config set` to persist a change across sessions.

---

## Contributing

### Project structure

```
cmd/neo4j-cli/
    main.go             Entry point — calls run() and os.Exit
    app.go              App struct, Cobra root command, startup wiring, flag definitions,
                        subcommand builders (buildCloudCommand, buildCypherCommand, etc.)

internal/
    config/             Config structs, Viper loader, Overrides, SaveConfiguration
    logger/             Logger interface and slog implementation
    analytics/          Mixpanel analytics service
    presentation/       Output formatters (text, JSON, pretty-JSON, table, graph)
    repository/         GraphRepository interface and Neo4j driver implementation
    service/
        interfaces.go   CypherService, CloudService, AdminService interfaces
        cypher_service.go
        cloud_service.go
        admin_service.go
        graph_service.go
    commands/           Shell category builders (pure wiring, no business logic)
        cypher.go
        cloud.go
        admin.go
        config.go       config list/set/delete/reset category
        prerequisites.go   Neo4jPrerequisite, AuraPrerequisite, interactive variants
    shell/
        category.go     Category type, Command type, routing (Dispatch/Find/SetPrerequisite)
        shell.go        Shell interface, ShellContext, built-in command names
        interactive.go  InteractiveShell — the REPL implementation
    tool/               Tool interface, BaseTool, Context, Result, IOHandler
    tools/              Concrete tool implementations (echo, help, query) + ToolRegistry
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

Tools are flat, named executables available in the registry. They can be invoked via the shell's `exec <name>` built-in.

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
    prefix string
}

func NewMyTool() *MyTool {
    return &MyTool{
        BaseTool: tool.NewBaseTool(
            "mytool",                        // name — must be unique in the registry
            "Does something interesting",    // description shown in help
            "1.0.0",
        ),
        prefix: ">>",
    }
}
```

**Step 2 — Implement the interface**

`BaseTool` provides no-op defaults for `Validate` and `Configure`. Override only the ones you need:

```go
func (t *MyTool) Execute(ctx tool.Context) (tool.Result, error) {
    if len(ctx.Args) == 0 {
        return tool.ErrorResult("usage: mytool <text>"), fmt.Errorf("no input")
    }
    output := fmt.Sprintf("%s %s", t.prefix, ctx.Args[0])
    return tool.SuccessResult(output), nil
}

func (t *MyTool) Validate(ctx tool.Context) error {
    if len(ctx.Args) == 0 {
        return fmt.Errorf("at least one argument is required")
    }
    return nil
}

func (t *MyTool) Configure(params map[string]interface{}) error {
    t.BaseTool.Configure(params) // always call through first
    if v, ok := params["prefix"].(string); ok {
        t.prefix = v
    }
    return nil
}

func (t *MyTool) DefaultParams() map[string]interface{} {
    return map[string]interface{}{"prefix": ">>"}
}
```

**Step 3 — Register it**

In `cmd/neo4j-cli/app.go`, add your tool to the slice inside `buildRegistry`:

```go
for _, t := range []tool.Tool{
    tools.NewEchoTool(),
    tools.NewHelpTool(registry),
    tools.NewQueryTool(cypherSvc),
    tools.NewMyTool(),           // ← add here
} {
    registerTool(registry, t, cfg, log)
}
```

**Step 4 — (Optional) Configure via config file**

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

---

### Adding a shell command

A shell command sits inside an existing category or sub-category. It takes positional arguments and returns a formatted string.

**Example: adding `admin show-indexes` to the `admin` category**

Open `internal/commands/admin.go` and chain another `AddCommand` call:

```go
func BuildAdminCategory(svc service.AdminService) *shell.Category {
    return shell.NewCategory("admin", "Administrative operations...").
        AddCommand(&shell.Command{
            Name:        "show-users",
            // ... existing command ...
        }).
        AddCommand(&shell.Command{
            Name:        "show-indexes",
            Aliases:     []string{"idx"},
            Usage:       "show-indexes",
            Description: "List all indexes in the current database",
            Handler: func(args []string, ctx shell.ShellContext) (string, error) {
                return svc.ShowIndexes(ctx.Context)
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

To add `cloud instances clone <id>`, open `internal/commands/cloud.go` and chain another `AddCommand` in `buildInstancesCategory`:

```go
func buildInstancesCategory(svc service.CloudService) *shell.Category {
    return shell.NewCategory("instances", "Manage Aura DB instances").
        AddCommand(instanceListCmd(svc)).
        // ...existing commands...
        AddCommand(&shell.Command{
            Name:        "clone",
            Usage:       "clone <id>",
            Description: "Clone an existing instance",
            Handler: func(args []string, ctx shell.ShellContext) (string, error) {
                if len(args) == 0 {
                    return "", fmt.Errorf("usage: cloud instances clone <id>")
                }
                // call svc.Instances().Clone(ctx.Context, args[0])
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
type GDSService interface {
    ListAlgorithms(ctx context.Context) ([]string, error)
    RunPageRank(ctx context.Context, graphName string) (string, error)
}
```

**Step 2 — Implement the service**

Create `internal/service/gds_service.go`:

```go
package service

type GDSServiceImpl struct {
    repo repository.GraphRepository
}

func NewGDSService(repo repository.GraphRepository) GDSService {
    return &GDSServiceImpl{repo: repo}
}

func (s *GDSServiceImpl) ListAlgorithms(ctx context.Context) ([]string, error) {
    // CALL gds.list()
    return nil, nil
}

func (s *GDSServiceImpl) RunPageRank(ctx context.Context, graphName string) (string, error) {
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
        })
}
```

**Step 4 — Wire it into App**

In `cmd/neo4j-cli/app.go`, add the service field and construct it in `newApp`. Then add the category to `buildCategories` and a Cobra subcommand in `buildRootCommand`:

```go
// In newApp(), after the repo is created:
gdsSvc := service.NewGDSService(repo)

// In buildCategories():
"gds": commands.BuildGDSCategory(a.gdsSvc).
    SetPrerequisite(commands.InteractiveNeo4jPrerequisite(&a.cfg.Neo4j, a.cfg, configPath)),

// In buildRootCommand():
rootCmd.AddCommand(buildGDSCommand())

func buildGDSCommand() *cobra.Command {
    return &cobra.Command{
        Use:   "gds",
        Short: "Graph Data Science operations",
        RunE:  runCategory("gds"),
        // ...
    }
}
```

If the category requires a database connection, use `InteractiveNeo4jPrerequisite`. For Aura API access, use `InteractiveAuraPrerequisite`. Add new prerequisite factories to `internal/commands/prerequisites.go` if neither fits.

The category is then available both in the shell and as a direct subcommand:

```
neo4j> gds list-algorithms
$ neo4j-cli gds list-algorithms
$ neo4j-cli gds --help
```

---

### Code conventions

**Service pattern** — business logic lives in `internal/service`. Command handlers in `internal/commands` call services; they contain no logic of their own beyond argument validation and output formatting.

**Interfaces before implementations** — new behaviour starts with an interface in `internal/service/interfaces.go`. This keeps the command layer and the shell decoupled from concrete implementations and makes testing straightforward.

**Prerequisite checks belong in `prerequisites.go`** — if a category requires an external dependency (database connection, API credentials), declare it with `SetPrerequisite` in `buildCategories`. Write the factory function in `internal/commands/prerequisites.go` so it is independently testable. Use `InteractiveNeo4jPrerequisite` / `InteractiveAuraPrerequisite` in the live app so missing credentials trigger a prompt rather than a hard error. The check runs before every real dispatch, but bare category invocations (e.g. `cypher` alone) always show help regardless.

**Tool `Validate` runs before `Execute`** — the shell calls `Validate` automatically before every `Execute`. Use it for tool-level readiness checks rather than repeating them inside `Execute`. `BaseTool.Validate` is a no-op, so only override it when needed.

**Command aliases are first-class** — add short-form names via the `Aliases []string` field on `Command`. Aliases are resolved in dispatch and tab completion, and shown in `help` output. Register canonical names as the `Name` field; aliases are supplementary.

**Secrets stay out of flags** — passwords and API secrets must not be CLI flags. Accept them only via environment variables, the config file, or interactive prompts.

**Errors are the caller's responsibility** — return `fmt.Errorf("context: %w", err)` and let the shell print it. Don't call `os.Exit` or `log.Fatal` from inside a service or command handler.

**No imports up the stack** — `shell` does not import `commands` or `service`. `commands` does not import `tools`. Keep the dependency graph acyclic and flowing in one direction toward `cmd`.

---

## CI & Releases

One GitHub Actions workflow manages the release process.

### Workflows

| Workflow | Trigger | What it does |
|---|---|---|
| **Release** | Push of a `vX.Y.Z` tag | Gates on tests, builds cross-platform binaries via GoReleaser, extracts the changelog section, creates a GitHub Release |

### Making a release

Releases follow a four-step process. `changie` collects unreleased fragment files and determines the correct semver bump automatically from the change kinds (`Added` → minor, `Fixed`/`Security` → patch, `Changed`/`Removed` → major).

**1. Batch and merge the changelog**

```bash
changie batch   # collects .changes/unreleased/*.yaml → .changes/vX.Y.Z.md
changie merge   # folds that file into CHANGELOG.md
```

**2. Commit and tag**

```bash
git add CHANGELOG.md .changes/
git commit -m "chore: release v0.2.0"
git tag v0.2.0
git push origin main --tags
```

### Adding a changelog entry

Every PR that changes Go source files needs a changie fragment. Run:

```bash
changie new
```

Choose a kind and write a one-line summary, then commit the generated `.yaml` file alongside your code changes.
