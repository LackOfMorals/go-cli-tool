# lom

A command-line tool for Neo4j with an interactive shell that can be used by people and by AI agents. Connect to a Neo4j database and the Neo4j Aura management API from a single binary — run Cypher queries, manage cloud instances, and perform administrative operations.

---
## Installation

lom is a Go binary that has been wrapper as a Python package.  You can use standard Python package tooling for installation, upgrades and removal 

### pip
`pip install lom` 

### pipx

`pipx install lom`

### uv
`uv tool install lom`

### uvx
uvx will install and execute lom.  This means that you must include the commands you want to use with uvx 

`uvx -i lom YOUR_COMMANDS `

Check the installation by running `lom --help` and you will see the help information. 

---
## Getting started


### Run

```bash
# Run a subcommand directly
./bin/lom cypher "MATCH (n:Person) RETURN n.name LIMIT 5"
./bin/lom cloud instances list
./bin/lom admin show-databases
./bin/lom config list

# Point at a specific config file
./bin/lom --config-file ~/.lom/config.json cloud instances list

# Control output format
./bin/lom cypher --format json "MATCH (n) RETURN n LIMIT 10"
./bin/lom cloud instances list --format json
./bin/lom cloud instances list --format toon
```

---

## Commands

All functionality is exposed as top-level subcommands. Running `lom` with no arguments prints help.

| Subcommand | Description |
|---|---|
| `cypher [query]` | Execute a Cypher query against the connected database |
| `cloud` | Manage Neo4j Aura cloud resources |
| `admin` | Administrative operations against the connected database |
| `config` | Manage CLI configuration |

### cypher

Executes a Cypher query against the connected database.

```bash
lom cypher "MATCH (n) RETURN n LIMIT 5"
lom cypher --param name=Alice "MATCH (n:Person {name:\$name}) RETURN n"
lom cypher --format json "MATCH (n) RETURN n"
lom cypher --format toon "MATCH (n)-[r]->(m) RETURN n,r,m"
lom cypher --limit 100 "MATCH (n) RETURN n"
```

Flags (placed before the query):

| Flag | Description |
|---|---|
| `--param key=value` | Add a query parameter (repeatable). Values are auto-typed: int, float, bool, string. |
| `--format table\|toon\|json\|pretty-json\|graph` | Override the output format for this query. |
| `--limit N` | Override the auto-injected row limit. |

> **Requires a Neo4j connection.** If credentials are not configured, you are prompted to enter them on first use and they are saved to the config file.

### cloud

Manages Neo4j Aura cloud resources.

> **Requires Aura credentials.** If `aura.client_id` or `aura.client_secret` are not configured, you are prompted to enter them on first use.

```bash
lom cloud instances list
lom cloud instances ls                                                  # alias
lom cloud instances get <id>
lom cloud instances create name=<n> tenant=<id> [cloud=<p>] [region=<r>] [type=<t>] [version=<v>] [memory=<size>]
lom cloud instances update <id> [name=<new-name>] [memory=<size>]
lom cloud instances pause <id>
lom cloud instances resume <id>
lom cloud instances delete <id>
lom cloud instances rm <id>                                             # alias

lom cloud projects list
lom cloud projects get <id>
```

`instances create` requires `name` and `tenant`. All other fields fall back to `aura.instance_defaults` in the config. Set defaults to avoid repeating them on every invocation:

```bash
lom config set aura.instance_defaults.tenant_id abc-123
lom config set aura.instance_defaults.cloud_provider aws
lom cloud instances create name=my-db
```

> **Save your password.** When `instances create` succeeds, the initial password is shown exactly once and cannot be recovered.

### admin

Runs administrative commands against the connected database.

> **Requires a Neo4j connection.** Same prerequisite as `cypher`.

```bash
lom admin show-users
lom admin show-databases
```

### config

Manages CLI configuration. Changes made with `set`, `delete`, and `reset` are persisted to the config file immediately and take effect in the current session.

```bash
lom config list                                # show all keys, values, and descriptions
lom config list --format json
lom config set neo4j.uri bolt://myhost:7687
lom config set cypher.output_format json
lom config set aura.instance_defaults.region us-east-1
lom config delete neo4j.password              # reset a key to its default (prompts)
lom config reset                              # wipe config file, restore all defaults (prompts)
```

---

## Configuration

Settings are resolved in this order, highest priority first:

```
CLI flags  >  environment variables  >  config file  >  defaults
```

### Config file

The default config file path is `~/.lom/config.json`. The directory and file are created automatically when credentials are first saved via an interactive prompt. Pass `--config-file <path>` to use a different location.

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
  "telemetry": {
    "metrics": true
  }
}
```

### Environment variables

All variables use the `LOM_` prefix. Nested keys use underscores.

| Variable | Default | Description |
|---|---|---|
| `LOM_LOG_LEVEL` | `info` | Log level: `debug`, `info`, `warn`, `error` |
| `LOM_LOG_FORMAT` | `text` | Log format: `text`, `json` |
| `LOM_LOG_OUTPUT` | `stderr` | Log destination: `stderr`, `stdout`, `file` |
| `LOM_LOG_FILE` | _(empty)_ | Log file path (used when `LOM_LOG_OUTPUT=file`) |
| `LOM_NEO4J_URI` | `bolt://localhost:7687` | Neo4j bolt URI |
| `LOM_NEO4J_USERNAME` | `neo4j` | Neo4j username |
| `LOM_NEO4J_PASSWORD` | _(empty)_ | Neo4j password — prefer env over config file |
| `LOM_NEO4J_DATABASE` | `neo4j` | Neo4j database name |
| `LOM_AURA_CLIENT_ID` | _(empty)_ | Aura API client ID |
| `LOM_AURA_CLIENT_SECRET` | _(empty)_ | Aura API client secret — prefer env over config file |
| `LOM_AURA_TIMEOUT_SECONDS` | `30` | Aura API request timeout |
| `LOM_AURA_INSTANCE_DEFAULTS_TENANT_ID` | _(empty)_ | Default tenant ID for new instances |
| `LOM_AURA_INSTANCE_DEFAULTS_CLOUD_PROVIDER` | `gcp` | Default cloud provider: `aws`, `gcp`, `azure` |
| `LOM_AURA_INSTANCE_DEFAULTS_REGION` | `europe-west1` | Default region for new instances |
| `LOM_AURA_INSTANCE_DEFAULTS_TYPE` | `enterprise-db` | Default instance type |
| `LOM_AURA_INSTANCE_DEFAULTS_VERSION` | `5` | Default Neo4j version |
| `LOM_AURA_INSTANCE_DEFAULTS_MEMORY` | `8GB` | Default instance memory |
| `LOM_CYPHER_SHELL_LIMIT` | `25` | Default LIMIT injected into cypher queries |
| `LOM_CYPHER_EXEC_LIMIT` | `100` | Default LIMIT in non-interactive mode |
| `LOM_CYPHER_OUTPUT_FORMAT` | `table` | Default output format: `table`, `json`, `pretty-json`, `graph`, `toon` |
| `LOM_TELEMETRY_METRICS` | `true` | Send anonymous usage metrics to Neo4j |

> **Security note:** `LOM_NEO4J_PASSWORD` and `LOM_AURA_CLIENT_SECRET` are intentionally not available as CLI flags. Passing secrets as flags exposes them in shell history and `ps` output.

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

--agent                   Enable agent mode (see Agent mode section)
--rw                      Permit write/mutating operations in agent mode
--request-id string       Correlation ID for agent-mode JSON responses
--timeout duration        Maximum command execution time (e.g. 30s)
```

### Interactive credential prompts

When Neo4j or Aura credentials are missing, the CLI prompts for them interactively on first use and saves them to the config file. To skip prompts in automated or agent contexts, pre-populate credentials via environment variables or the config file.

---

## Agent mode

The CLI is designed to be driven by AI agents, CI pipelines, and orchestration tools. Use `--agent` (or `LOM_AGENT=true`) to activate a safe, machine-readable operating mode.

### Activating agent mode

```bash
# Via flag
lom --agent cloud instances list

# Via environment variable (recommended for pipelines — all invocations inherit it)
export LOM_AGENT=true
lom cloud instances list
```

### What --agent does

| Behaviour | Human mode (default) | Agent mode |
|---|---|---|
| Output format | `table` | `json` |
| Missing credentials | Interactive prompt | Structured JSON error on stdout, exit non-zero |
| Errors | Written to stderr | JSON envelope on stdout |
| Write operations | Allowed with confirmation prompt | **Blocked** unless `--rw` is also passed |

### The --rw flag

In agent mode, all operations that modify state are blocked by default. Pass `--rw` (or `LOM_RW=true`) to explicitly permit mutations:

```bash
# Blocked — returns READ_ONLY error
lom --agent cloud instances delete <id>

# Allowed — no prompt, executes immediately
lom --agent --rw cloud instances delete <id>
```

`--rw` governs **all** mutation categories uniformly:

| Category | Read operations | Write operations (require `--rw`) |
|---|---|---|
| `cypher` | Any read-only query | Queries classified as write by Neo4j EXPLAIN |
| `cloud instances` | `list`, `get` | `create`, `update`, `pause`, `resume`, `delete` |
| `cloud projects` | `list`, `get` | _(none currently)_ |
| `admin` | `show-users`, `show-databases` | _(none currently)_ |
| `config` | `list` | `set`, `delete`, `reset` |

### Cypher write detection

For `cypher` commands in agent mode without `--rw`, the CLI automatically runs `EXPLAIN` on the query before execution. If Neo4j classifies the statement as a write (`rw`, `w`, or `s`), it is blocked:

```bash
# EXPLAIN detects a write — blocked
lom --agent cypher "CREATE (n:Person {name:'Alice'}) RETURN n"
# → {"status":"error","error":{"code":"WRITE_BLOCKED","message":"..."},...}

# Read query — EXPLAIN confirms safe, executes
lom --agent cypher "MATCH (n:Person) RETURN n.name LIMIT 10"

# EXPLAIN or PROFILE queries run as-is — no pre-check
lom --agent cypher "EXPLAIN MATCH (n) RETURN n"
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
| `MISSING_CREDENTIALS` | Required credentials absent (no prompt in agent mode) |
| `TIMEOUT` | Command exceeded `--timeout` duration |
| `EXECUTION_ERROR` | Any other failure |

### Additional agent flags

```
--request-id string   Correlation ID included in JSON responses.
                      Auto-generated (UUID) if not supplied.
                      Env: LOM_REQUEST_ID

--timeout duration    Maximum time for a command to run (e.g. 30s, 2m).
                      Exit code 1 + TIMEOUT error on expiry.
```

### Recommended orchestrator setup

```bash
export LOM_AGENT=true
export LOM_REQUEST_ID="pipeline-run-${RUN_ID}"  # inject your trace ID
export LOM_NEO4J_URI="bolt+s://your-instance.databases.neo4j.io"
export LOM_NEO4J_USERNAME="neo4j"
export LOM_NEO4J_PASSWORD="${NEO4J_PASSWORD}"

# Read operations work with no further flags
lom cypher "MATCH (n:Person) RETURN count(n)"

# Write operations require explicit --rw
lom --rw cypher "CREATE (n:Event {ts: datetime()}) RETURN n"
```

---

## Contributing

### Project structure

```
cmd/lom/
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
        interfaces.go       CypherService, CloudService, AdminService interfaces
        cypher_service.go
        cloud_service.go
        admin_service.go
        graph_service.go
    commands/           Category builders (pure wiring — no business logic)
        cypher.go
        cloud.go
        admin.go
        config.go           config list/set/delete/reset category
        prerequisites.go    Neo4jPrerequisite, AuraPrerequisite, interactive variants
    dispatch/           Command routing primitives
        dispatch.go         Registry interface, CommandHandler type, Context struct
        category.go         Category and Command types — Dispatch, Find, SetPrerequisite
```

The dependency direction is strict:

```
cmd  →  commands  →  service  →  repository
cmd  →  dispatch
commands  →  dispatch   (for Category / Command / Context types)
```

No package imports its own parent. `dispatch` does not import `commands` or `service`.

---


### Adding a command

A command sits inside an existing category or sub-category. It takes positional arguments and returns a formatted string.

**Example: adding `admin show-indexes` to the `admin` category**

Open `internal/commands/admin.go` and chain another `AddCommand` call:

```go
func BuildAdminCategory(svc service.AdminService) *dispatch.Category {
    return dispatch.NewCategory("admin", "Administrative operations...").
        AddCommand(&dispatch.Command{
            Name:        "show-users",
            // ... existing command ...
        }).
        AddCommand(&dispatch.Command{
            Name:        "show-indexes",
            Aliases:     []string{"idx"},
            Usage:       "show-indexes",
            Description: "List all indexes in the current database",
            Handler: func(args []string, ctx dispatch.Context) (string, error) {
                return svc.ShowIndexes(ctx.Context)
            },
        })
}
```

Aliases are registered automatically in dispatch and appear in parentheses in `--help` output:

```bash
lom admin show-indexes
lom admin idx           # same command
```

**Example: adding a command to a sub-category**

To add `cloud instances clone <id>`, open `internal/commands/cloud.go` and chain another `AddCommand` in `buildInstancesCategory`:

```go
func buildInstancesCategory(svc service.CloudService) *dispatch.Category {
    return dispatch.NewCategory("instances", "Manage Aura DB instances").
        AddCommand(instanceListCmd(svc)).
        // ...existing commands...
        AddCommand(&dispatch.Command{
            Name:        "clone",
            Usage:       "clone <id>",
            Description: "Clone an existing instance",
            Handler: func(args []string, ctx dispatch.Context) (string, error) {
                if len(args) == 0 {
                    return "", fmt.Errorf("usage: cloud instances clone <id>")
                }
                return fmt.Sprintf("Instance %s cloned.", args[0]), nil
            },
        })
}
```

---

### Adding a category

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
            Handler: func(args []string, ctx dispatch.Context) (string, error) {
                algos, err := svc.ListAlgorithms(ctx.Context)
                if err != nil {
                    return "", err
                }
                return strings.Join(algos, "\n"), nil
            },
        })
}
```

**Step 4 — Wire it into App**

In `cmd/lom/app.go`, add the service field, construct it in `newApp`, and register the category and its Cobra subcommand:

```go
// In newApp(), after repo is created:
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
        SilenceUsage:  true,
        SilenceErrors: true,
    }
}
```

Use `InteractiveNeo4jPrerequisite` for database-connected categories and `InteractiveAuraPrerequisite` for Aura API categories. Add new prerequisite factories to `internal/commands/prerequisites.go` if neither fits.

The category is then available as a direct subcommand:

```bash
lom gds list-algorithms
lom gds --help
```

---

### Code conventions

**Service pattern** — business logic lives in `internal/service`. Command handlers in `internal/commands` call services; they contain no logic of their own beyond argument validation and output formatting.

**Interfaces before implementations** — new behaviour starts with an interface in `internal/service/interfaces.go`. This keeps the command layer decoupled from concrete implementations and makes testing straightforward.

**Prerequisite checks belong in `prerequisites.go`** — if a category requires an external dependency (database connection, API credentials), declare it with `SetPrerequisite` in `buildCategories`. Write the factory function in `internal/commands/prerequisites.go` so it is independently testable. The check runs before every real dispatch, but bare category invocations (e.g. `lom cypher --help`) always show help regardless.

**Command aliases are first-class** — add short-form names via the `Aliases []string` field on `dispatch.Command`. Aliases are resolved in dispatch and shown in `--help` output. Register the canonical name as `Name`; aliases are supplementary.

**Secrets stay out of flags** — passwords and API secrets must not be CLI flags. Accept them only via environment variables, the config file, or interactive prompts.

**Errors are the caller's responsibility** — return `fmt.Errorf("context: %w", err)` and let the caller handle it. Don't call `os.Exit` or `log.Fatal` from inside a service or command handler.

**No imports up the stack** — `dispatch` does not import `commands` or `service`. `commands` does not import `tools`. Keep the dependency graph acyclic and flowing in one direction toward `cmd`.

**Declare MutationMode on every command and tool** — every `dispatch.Command` and `tool.Tool` must declare its `MutationMode`. Use `ModeRead` (default) for read-only operations, `ModeWrite` for operations that always modify state, and `ModeConditional` for operations where mutability depends on runtime input (Cypher queries). The dispatcher enforces the read-only contract in agent mode automatically, so individual handlers do not need to check `ctx.AgentMode` for blocking. Handlers *should* check `ctx.AgentMode` only to suppress interactive prompts (e.g. confirmation dialogs).

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
