# neo4j-cli

A command-line tool for Neo4j, optimised for use by AI agents and scripts. Connect to a Neo4j database and the Neo4j Aura management API from a single binary — run Cypher queries and manage cloud instances.

---

## Contents

- [Getting started](#getting-started)
- [Commands](#commands)
- [Configuration](#configuration)
- [Agent mode](#agent-mode)
- [Contributing](#contributing)
  - [Project structure](#project-structure)
  - [Adding a tool](#adding-a-tool)
  - [Adding a command](#adding-a-command)
  - [Adding a category](#adding-a-category)
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
# Run a subcommand directly
./bin/neo4j-cli cypher "MATCH (n:Person) RETURN n.name LIMIT 5"
./bin/neo4j-cli cloud instances list

# Control output format
./bin/neo4j-cli cypher --format json "MATCH (n) RETURN n LIMIT 10"
./bin/neo4j-cli cloud instances list --format json
```

Credentials are supplied via environment variables or CLI flags — see [Configuration](#configuration).

---

## Commands

All functionality is exposed as top-level subcommands. Running `neo4j-cli` with no arguments prints help.

| Subcommand | Description |
|---|---|
| `cypher [query]` | Execute a Cypher query against the connected database |
| `cloud` | Manage Neo4j Aura cloud resources |
| `skill` | Install, remove, or list the embedded `neo4j-cli` SKILL.md across detected AI agents |

### cypher

Executes a Cypher query against the connected database.

```bash
neo4j-cli cypher "MATCH (n) RETURN n LIMIT 5"
neo4j-cli cypher --param name=Alice "MATCH (n:Person {name:\$name}) RETURN n"
neo4j-cli cypher --format json "MATCH (n) RETURN n"
neo4j-cli cypher --format graph "MATCH (n)-[r]->(m) RETURN n,r,m"
neo4j-cli cypher --limit 100 "MATCH (n) RETURN n"
```

Flags (placed before the query):

| Flag | Description |
|---|---|
| `--param key=value` | Add a query parameter (repeatable). Values are auto-typed: int, float, bool, string. |
| `--format table\|json\|pretty-json\|graph` | Override the output format for this query. |
| `--limit N` | Override the auto-injected row limit. |

> **Requires a Neo4j connection.** Set `CLI_NEO4J_URI`, `CLI_NEO4J_USERNAME`, and `CLI_NEO4J_PASSWORD` before running.

### cloud

Manages Neo4j Aura cloud resources.

> **Requires Aura credentials.** Set `CLI_AURA_CLIENT_ID` and `CLI_AURA_CLIENT_SECRET` before running.

```bash
neo4j-cli cloud instances list
neo4j-cli cloud instances ls                                                  # alias
neo4j-cli cloud instances get <id>
neo4j-cli cloud instances create name=<n> tenant=<id> [cloud=<p>] [region=<r>] [type=<t>] [version=<v>] [memory=<size>]
neo4j-cli cloud instances update <id> [name=<new-name>] [memory=<size>]
neo4j-cli cloud instances pause <id>
neo4j-cli cloud instances resume <id>
neo4j-cli cloud instances delete <id>
neo4j-cli cloud instances rm <id>                                             # alias

neo4j-cli cloud projects list
neo4j-cli cloud projects get <id>
```

`instances create` requires `name` and `tenant`. All other fields fall back to the `CLI_AURA_INSTANCE_DEFAULTS_*` environment variables.

> **Save your password.** When `instances create` succeeds, the initial password is shown exactly once and cannot be recovered.

### skill

Installs, removes, or lists the embedded `neo4j-cli` SKILL.md for AI agents (Claude Code, Cursor, Windsurf, Copilot, Gemini CLI, Cline, Codex, Pi, OpenCode, Junie). The SKILL.md is generated at build time from this CLI's command tree so agents always see current usage.

```bash
neo4j-cli skill list                                # show all known agents and install status
neo4j-cli skill install                             # install to every detected agent
neo4j-cli skill install claude-code                 # install to a specific agent
neo4j-cli skill remove                              # remove from every agent that has it
neo4j-cli skill remove claude-code                  # remove from a specific agent
```

`install` writes `<agent-skills-dir>/neo4j-cli/SKILL.md`, replacing any existing file or symlink at that path. `list` is read-only; `install` and `remove` are write operations and require `--rw` in agent mode.

> **No prerequisite.** This command only touches local agent skill directories — no Neo4j or Aura credentials needed.

---

## Configuration

Settings are resolved in this order, highest priority first:

```
CLI flags  >  environment variables  >  defaults
```

There is no config file. All values are supplied via environment variables or persistent CLI flags, which is the recommended approach for agent and pipeline use.

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
| `CLI_NEO4J_PASSWORD` | _(empty)_ | Neo4j password |
| `CLI_NEO4J_DATABASE` | `neo4j` | Neo4j database name |
| `CLI_AURA_CLIENT_ID` | _(empty)_ | Aura API client ID |
| `CLI_AURA_CLIENT_SECRET` | _(empty)_ | Aura API client secret |
| `CLI_AURA_TIMEOUT_SECONDS` | `30` | Aura API request timeout |
| `CLI_AURA_INSTANCE_DEFAULTS_TENANT_ID` | _(empty)_ | Default tenant ID for new instances |
| `CLI_AURA_INSTANCE_DEFAULTS_CLOUD_PROVIDER` | `gcp` | Default cloud provider: `aws`, `gcp`, `azure` |
| `CLI_AURA_INSTANCE_DEFAULTS_REGION` | `europe-west1` | Default region for new instances |
| `CLI_AURA_INSTANCE_DEFAULTS_TYPE` | `enterprise-db` | Default instance type |
| `CLI_AURA_INSTANCE_DEFAULTS_VERSION` | `5` | Default Neo4j version |
| `CLI_AURA_INSTANCE_DEFAULTS_MEMORY` | `8GB` | Default instance memory |
| `CLI_CYPHER_SHELL_LIMIT` | `25` | Default LIMIT injected into cypher queries |
| `CLI_CYPHER_EXEC_LIMIT` | `100` | Default LIMIT in non-interactive mode |
| `CLI_CYPHER_OUTPUT_FORMAT` | `table` | Default output format: `table`, `json`, `pretty-json`, `graph` |
| `CLI_TELEMETRY_METRICS` | `true` | Send anonymous usage metrics to Neo4j |

> **Security note:** `CLI_NEO4J_PASSWORD` and `CLI_AURA_CLIENT_SECRET` are intentionally not available as CLI flags. Passing secrets as flags exposes them in shell history and `ps` output.

### CLI flags

```
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

--agent                   Enable agent mode (see Agent mode section)
--rw                      Permit write/mutating operations in agent mode
--request-id string       Correlation ID for agent-mode JSON responses
--timeout duration        Maximum command execution time (e.g. 30s)
```

---

## Agent mode

The CLI is designed to be driven by AI agents, CI pipelines, and orchestration tools. Use `--agent` (or `NEO4J_CLI_AGENT=true`) to activate a safe, machine-readable operating mode.

### Activating agent mode

```bash
# Via flag
neo4j-cli --agent cloud instances list

# Via environment variable (recommended for pipelines — all invocations inherit it)
export NEO4J_CLI_AGENT=true
neo4j-cli cloud instances list
```

### What --agent does

| Behaviour | Human mode (default) | Agent mode |
|---|---|---|
| Output format | `table` | `json` |
| Missing credentials | Error message to stderr | Structured JSON error on stdout, exit non-zero |
| Errors | Written to stderr | JSON envelope on stdout |
| Write operations | Allowed | **Blocked** unless `--rw` is also passed |

### The --rw flag

In agent mode, all operations that modify state are blocked by default. Pass `--rw` (or `NEO4J_CLI_RW=true`) to explicitly permit mutations:

```bash
# Blocked — returns READ_ONLY error
neo4j-cli --agent cloud instances delete <id>

# Allowed — no prompt, executes immediately
neo4j-cli --agent --rw cloud instances delete <id>
```

`--rw` governs **all** mutation categories uniformly:

| Category | Read operations | Write operations (require `--rw`) |
|---|---|---|
| `cypher` | Any read-only query | Queries classified as write by Neo4j EXPLAIN |
| `cloud instances` | `list`, `get` | `create`, `update`, `pause`, `resume`, `delete` |
| `cloud projects` | `list`, `get` | _(none currently)_ |
| `skill` | `list` | `install`, `remove` |

### Cypher write detection

For `cypher` commands in agent mode without `--rw`, the CLI automatically runs `EXPLAIN` on the query before execution. If Neo4j classifies the statement as a write (`rw`, `w`, or `s`), it is blocked:

```bash
# EXPLAIN detects a write — blocked
neo4j-cli --agent cypher "CREATE (n:Person {name:'Alice'}) RETURN n"
# → {"status":"error","error":{"code":"WRITE_BLOCKED","message":"..."},...}

# Read query — EXPLAIN confirms safe, executes
neo4j-cli --agent cypher "MATCH (n:Person) RETURN n.name LIMIT 10"

# EXPLAIN or PROFILE queries run as-is — no pre-check
neo4j-cli --agent cypher "EXPLAIN MATCH (n) RETURN n"
```

With `--rw`, the EXPLAIN pre-check is skipped and queries execute directly.

### Error envelope

All errors in agent mode are written to **stdout** as a JSON envelope so agents reading stdout get machine-readable failures:

```json
{"status":"error","error":{"code":"READ_ONLY","message":"\"delete\" is a write operation; re-run with --rw to permit mutations"},"request_id":"a1b2c3","schema_version":"1"}
```

Error codes:

| Code | Meaning |
|---|---|
| `READ_ONLY` | Write operation attempted without `--rw` |
| `WRITE_BLOCKED` | Cypher write detected by EXPLAIN without `--rw` |
| `MISSING_QUERY` | No cypher statement provided in agent mode |
| `MISSING_CREDENTIALS` | Required credentials absent |
| `TIMEOUT` | Command exceeded `--timeout` duration |
| `EXECUTION_ERROR` | Any other failure |

### Additional agent flags

```
--request-id string   Correlation ID included in JSON responses.
                      Auto-generated (UUID) if not supplied.
                      Env: NEO4J_CLI_REQUEST_ID

--timeout duration    Maximum time for a command to run (e.g. 30s, 2m).
                      Exit code 1 + TIMEOUT error on expiry.
```

### Recommended orchestrator setup

```bash
export NEO4J_CLI_AGENT=true
export NEO4J_CLI_REQUEST_ID="pipeline-run-${RUN_ID}"  # inject your trace ID
export CLI_NEO4J_URI="bolt+s://your-instance.databases.neo4j.io"
export CLI_NEO4J_USERNAME="neo4j"
export CLI_NEO4J_PASSWORD="${NEO4J_PASSWORD}"

# Read operations work with no further flags
neo4j-cli cypher "MATCH (n:Person) RETURN count(n)"

# Write operations require explicit --rw
neo4j-cli --rw cypher "CREATE (n:Event {ts: datetime()}) RETURN n"
```

---

## Contributing

### Project structure

```
cmd/neo4j-cli/
    main.go             Entry point — calls run() and os.Exit
    app.go              App struct, startup wiring, category dispatch

internal/
    config/             Config structs, Viper loader, Overrides (env vars + flags only)
    logger/             Logger interface and slog implementation
    analytics/          Mixpanel analytics service (startup + command events)
    presentation/       Output formatters (text, JSON, pretty-JSON, table, graph)
    repository/         GraphRepository interface and Neo4j driver implementation
    service/
        interfaces.go       CypherService, CloudService, SkillService interfaces
        cypher_service.go
        cloud_service.go
        skill_service.go
        graph_service.go
    commands/           Category builders (pure wiring — no business logic)
        cypher.go
        cloud.go
        skill.go
        prerequisites.go    Neo4jPrerequisite, AuraPrerequisite
    dispatch/           Command routing primitives
        dispatch.go         Registry interface, CommandHandler type, Context struct
        category.go         Category and Command types — Dispatch, Find, SetPrerequisite
    tool/               Tool interface, BaseTool, Context, Result, IOHandler
    tools/              Concrete tool implementations (echo, help, query) + ToolRegistry
```

The dependency direction is strict:

```
cmd  →  commands  →  service  →  repository
cmd  →  dispatch
cmd  →  tools
commands  →  dispatch   (for Category / Command / Context types)
```

No package imports its own parent. `dispatch` does not import `commands` or `service`.

---

### Adding a tool

Tools are flat, named executables registered in the tool registry.

**Step 1 — Create the file**

Create `internal/tools/mytool.go`. Embed `*tool.BaseTool` to get parameter helpers for free:

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
    if err := t.BaseTool.Configure(params); err != nil {
        return err
    }
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

---

### Adding a command

A command sits inside an existing category or sub-category. It takes positional arguments and returns a `dispatch.CommandResult`.

**Example: adding `cloud instances clone` to the instances sub-category**

Open `internal/commands/cloud.go` and chain another `AddCommand` call inside `buildInstancesCategory`:

```go
func buildInstancesCategory(svc service.CloudService) *dispatch.Category {
    return dispatch.NewCategory("instances", "Manage Aura DB instances").
        AddCommand(instanceListCmd(svc)).
        // ...existing commands...
        AddCommand(&dispatch.Command{
            Name:         "clone",
            MutationMode: tool.ModeWrite,
            Usage:        "clone <id>",
            Description:  "Clone an existing instance",
            Handler: func(args []string, ctx dispatch.Context) (dispatch.CommandResult, error) {
                if len(args) == 0 {
                    return dispatch.CommandResult{}, fmt.Errorf("usage: cloud instances clone <id>")
                }
                // call svc...
                return dispatch.MessageResult(fmt.Sprintf("Instance %s cloned.", args[0])), nil
            },
        })
}
```

Aliases are registered automatically in dispatch and appear in parentheses in `--help` output:

```bash
neo4j-cli cloud instances clone <id>
```

---

### Adding a category

To add a new top-level category (e.g. `gds` for Graph Data Science):

**Step 1 — Add a service interface**

In `internal/service/interfaces.go`:

```go
type GDSService interface {
    ListAlgorithms(ctx context.Context) ([]string, error)
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
```

**Step 3 — Build the category**

Create `internal/commands/gds.go`:

```go
package commands

import (
    "strings"

    "github.com/cli/go-cli-tool/internal/dispatch"
    "github.com/cli/go-cli-tool/internal/service"
)

func BuildGDSCategory(svc service.GDSService) *dispatch.Category {
    return dispatch.NewCategory("gds", "Graph Data Science operations").
        AddCommand(&dispatch.Command{
            Name:        "list-algorithms",
            Usage:       "list-algorithms",
            Description: "List available GDS algorithms",
            Handler: func(args []string, ctx dispatch.Context) (dispatch.CommandResult, error) {
                algos, err := svc.ListAlgorithms(ctx.Context)
                if err != nil {
                    return dispatch.CommandResult{}, err
                }
                return dispatch.MessageResult(strings.Join(algos, "\n")), nil
            },
        })
}
```

**Step 4 — Wire it into App**

In `cmd/neo4j-cli/app.go`, add the service field, construct it in `newApp`, register it in `buildCategories`, and add the Cobra subcommand in `internal/cli/tree.go`:

```go
// In newApp(), after repo is created:
gdsSvc := service.NewGDSService(repo)

// In buildCategories():
"gds": commands.BuildGDSCategory(a.gdsSvc).
    SetPrerequisite(commands.Neo4jPrerequisite(&a.cfg.Neo4j)),
```

In `internal/cli/tree.go`, add the Cobra subcommand:

```go
rootCmd.AddCommand(buildGDSCommand(opts.RunFactory))

func buildGDSCommand(rf RunFactory) *cobra.Command {
    return &cobra.Command{
        Use:           "gds",
        Short:         "Graph Data Science operations",
        RunE:          runEFor(rf, "gds"),
        SilenceUsage:  true,
        SilenceErrors: true,
    }
}
```

Use `Neo4jPrerequisite` for database-connected categories and `AuraPrerequisite` for Aura API categories. Add new prerequisite factories to `internal/commands/prerequisites.go` if neither fits.

The category is then available as a direct subcommand:

```bash
neo4j-cli gds list-algorithms
neo4j-cli gds --help
```

---

### Code conventions

**Service pattern** — business logic lives in `internal/service`. Command handlers in `internal/commands` call services; they contain no logic of their own beyond argument validation and output formatting.

**Interfaces before implementations** — new behaviour starts with an interface in `internal/service/interfaces.go`. This keeps the command layer decoupled from concrete implementations and makes testing straightforward.

**Prerequisite checks belong in `prerequisites.go`** — if a category requires an external dependency (database connection, API credentials), declare it with `SetPrerequisite` in `buildCategories`. Write the factory function in `internal/commands/prerequisites.go` so it is independently testable. The check runs before every real dispatch, but bare category invocations (e.g. `neo4j-cli cypher --help`) always show help regardless.

**Tool `Validate` runs before `Execute`** — `Validate` is called automatically before every `Execute`. Use it for tool-level readiness checks rather than repeating them inside `Execute`. `BaseTool.Validate` is a no-op; only override it when needed.

**Command aliases are first-class** — add short-form names via the `Aliases []string` field on `dispatch.Command`. Aliases are resolved in dispatch and shown in `--help` output. Register the canonical name as `Name`; aliases are supplementary.

**Secrets stay out of flags** — passwords and API secrets must not be CLI flags. Accept them only via environment variables.

**Errors are the caller's responsibility** — return `fmt.Errorf("context: %w", err)` and let the caller handle it. Don't call `os.Exit` or `log.Fatal` from inside a service or command handler.

**No imports up the stack** — `dispatch` does not import `commands` or `service`. `commands` does not import `tools`. Keep the dependency graph acyclic and flowing in one direction toward `cmd`.

**Declare MutationMode on every command and tool** — every `dispatch.Command` and `tool.Tool` must declare its `MutationMode`. Use `ModeRead` (default) for read-only operations, `ModeWrite` for operations that always modify state, and `ModeConditional` for operations where mutability depends on runtime input (Cypher queries). The dispatcher enforces the read-only contract in agent mode automatically, so individual handlers do not need to check `ctx.AgentMode` for blocking. Handlers *should* check `ctx.AgentMode` only to suppress interactive prompts (e.g. confirmation dialogs).

---

## CI & Releases

One GitHub Actions workflow manages the release process.

### Workflows

| Workflow | Trigger | What it does |
|---|---|---|
| **CI** | Push / pull request | Runs `go generate`, validates generated files are up to date, builds, vets, and tests |
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

### Generated files

Two files are produced by `go generate ./...` and must be committed alongside any change that affects them:

| File | Generator | Regenerate when |
|---|---|---|
| `internal/skill/skill.md.gen` | `go run ./cmd/gen-skill` | Cobra command tree changes (new commands, flags, descriptions) |
| `internal/analytics/mocks/mock_analytics.go` | `mockgen` | `analytics.Service` interface changes |
| `internal/service/mocks/mock_skill_service.go` | `mockgen` | `service.SkillService` interface changes |

The CI step "Verify generated files are up to date" runs `go generate ./...` and fails if the output differs from what is committed. Always run `go generate ./...` locally and commit the results before pushing.
