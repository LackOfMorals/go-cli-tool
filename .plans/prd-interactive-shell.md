# PRD: Interactive Shell REPL

## Overview

Restore the interactive REPL so that running `neo4j-cli` with no subcommand drops the user into a persistent shell session. The shell package (`internal/shell`) already exists and is feature-complete; this work wires it into the application entry point, bridges the existing dispatch command tree into the shell's command model, and exposes the necessary config/flag/env controls.

## Goals

- When `neo4j-cli` is invoked with no arguments (and not in agent mode), launch the interactive REPL.
- All existing commands (cypher, cloud, admin, config) are available inside the shell without duplicating handler logic.
- Users can enable/disable the shell via config file, `--shell` flag, or `CLI_SHELL_ENABLED` env var.
- First-time users with no config file get a helpful welcome message guiding them to configure the tool.

## Non-Goals

- Changing any existing subcommand behaviour when invoked directly (e.g. `neo4j-cli cloud instances list` still works as before).
- Adding new commands or modifying existing command handlers.
- Persistent session state beyond what readline history already provides.

## Requirements

### Functional Requirements

- **REQ-F-001**: `shell.enabled` in the config file defaults to `true`. Existing configs that do not set this key therefore opt in automatically.
- **REQ-F-002**: When `neo4j-cli` is run with no subcommand, no `--agent` flag, and `shell.enabled` is true, the root command's `RunE` launches `InteractiveShell`.
- **REQ-F-003**: A `--shell` boolean flag on the root command overrides `shell.enabled` (wires to `config.Overrides.ShellEnabled`, which already exists).
- **REQ-F-004**: The env var `CLI_SHELL_ENABLED` follows the existing viper `CLI_` prefix pattern and overrides the config-file value.
- **REQ-F-005**: When `--agent` is set (or `NEO4J_CLI_AGENT=true`), the shell is suppressed entirely and Cobra prints normal help.
- **REQ-F-006**: All `dispatch.Category` trees (cypher, cloud, admin, config) are exposed inside the shell via an adapter layer; no handler logic is duplicated.
- **REQ-F-007**: The adapter converts `dispatch.CommandResult` to a display string using the presentation service (same rendering path as non-agent human-mode output).
- **REQ-F-008**: The shell is wired with the same services as the rest of the app: logger, config, telemetry, presentation service, tool registry.
- **REQ-F-009**: When no config file is present, the shell launches and displays a welcome banner that explains how to configure the tool (`config set` commands or config file path).

### Non-Functional Requirements

- **REQ-NF-001**: No new package import cycles. The bridge adapter lives in a package that can import both `internal/shell` and `internal/dispatch` without creating a cycle.
- **REQ-NF-002**: The bridge is covered by unit tests; at minimum, `CommandResult` → string conversion for `Message`, `Presentation` (table/detail), and error cases.
- **REQ-NF-003**: All lint rules (`golangci-lint run`) pass with no new suppressions.

## Technical Considerations

### 1. Config default change

In `internal/config/config.go` line 285, change:
```go
v.SetDefault("shell.enabled", false)
```
to:
```go
v.SetDefault("shell.enabled", true)
```

### 2. Root command `RunE`

`cmd/neo4j-cli/app.go` contains `buildRootCommand()` (around line 165 of `internal/cli/tree.go`). The comment `// No RunE: cobra prints help when called with no subcommand.` must be replaced with a `RunE` that:

```
if agentMode → return nil (cobra help)
app, err := newApp(cfg, overrides)
if !app.cfg.Shell.Enabled → cmd.Help(); return nil
launch InteractiveShell
```

The `--shell` boolean flag is added to the root persistent flags alongside `--agent`:
```go
pf.BoolVar(&shellOverride, "shell", false, "Launch interactive shell (env: CLI_SHELL_ENABLED)")
```
After flag parse, `overrides.ShellEnabled = &shellOverride` if the flag was explicitly set.

### 3. Bridge adapter

Create `internal/shell/bridge.go` (within the `shell` package, so no import cycle):

```go
// BridgeCategory wraps a dispatch.Category as a shell.Category.
// ctxFor is called on each invocation to build the dispatch.Context from
// the shell.ShellContext that arrives at dispatch time.
func BridgeCategory(cat *dispatch.Category, ctxFor func(shell.ShellContext) dispatch.Context) *Category
```

For each `dispatch.Command` in the category tree, a `shell.Command` is synthesised whose `Handler`:
1. Calls `ctxFor(shellCtx)` to build a `dispatch.Context`.
2. Calls `cmd.Handler(args, dispatchCtx)`.
3. Renders `dispatch.CommandResult` to a string:
   - If `result.Presentation != nil` → `ctx.Presenter.Format(result.Presentation)` (captures as string via a `strings.Builder` writer or by having the presenter return a string).
   - Else → `result.Message`.
4. Returns `(string, error)`.

Subcategories are bridged recursively. Prerequisites registered on dispatch categories are wrapped to call `prerequisite()` inside the shell handler (same semantics as the existing `shell.Category.SetPrerequisite`).

### 4. Shell package type fix

`internal/shell/shell.go` and `internal/shell/interactive.go` reference `*presentation.PresentationService` (the concrete struct, now unexported). These must be updated to `presentation.Service` (the interface):

- `Shell` interface method: `SetPresenter(p presentation.Service)`
- `ShellContext.Presenter`: `presentation.Service`
- `InteractiveShell.presenter` field: `presentation.Service`

### 5. App wiring

In `app.go`, add a `launchShell()` method that:
1. Creates `shell.NewInteractiveShell()`.
2. Calls `SetLogger`, `SetConfig`, `SetTelemetry`, `SetPresenter`, `SetRegistry`, `SetVersion`.
3. Bridges all built categories via `shell.BridgeCategory` and calls `SetCategories`.
4. Calls `shell.Start()`.

If no config file was loaded (`app.cfg` has zero-value connection fields), `Start()` prints the welcome banner before entering the read loop.

### 6. Welcome banner (no-config case)

Detected by: `cfg.Neo4j.URI == ""` (or equal to the default sentinel). The banner text:

```
Welcome to neo4j-cli.
No configuration loaded. To get started:

  config set neo4j.uri      bolt://localhost:7687
  config set neo4j.username neo4j
  config set neo4j.password <password>

Type 'help' for a list of commands.
```

## Acceptance Criteria

- [ ] `neo4j-cli` (no args, no agent mode, default config) launches the REPL prompt.
- [ ] `neo4j-cli --agent cloud instances list ...` continues to work without launching the shell.
- [ ] `neo4j-cli --shell=false` suppresses the shell and prints Cobra help.
- [ ] `CLI_SHELL_ENABLED=false neo4j-cli` suppresses the shell.
- [ ] Inside the shell, `cloud instances list` dispatches to the existing cloud handler and renders output.
- [ ] Inside the shell, `cypher MATCH (n) RETURN n LIMIT 1` dispatches to the existing cypher handler.
- [ ] A fresh install (no config file) shows the welcome banner and accepts `config set` commands.
- [ ] `go test ./internal/shell/...` passes, including bridge adapter unit tests.
- [ ] `golangci-lint run` reports zero issues.
- [ ] `go generate ./...` output is committed (generated files unchanged by this feature).

## Out of Scope

- Shell-specific command aliases or shortcuts not present in the dispatch tree.
- Persisting session state (connection, open transactions) across REPL commands.
- Shell autocomplete for Cypher keywords beyond what readline already provides.
- Windows support for readline (deferred — existing readline dependency is already Unix-only).

## Open Questions

None — all questions resolved during PRD Q&A.
