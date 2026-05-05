# AGENTS.md

Project documentation for automated agents working on go-cli-tool.

## Feedback Instructions

### TEST COMMANDS
```
go test ./...
go test ./internal/shell/...          # shell package only
go test -run TestName ./internal/...  # single test
```

### BUILD COMMANDS
```
go build ./...
go build -o bin/nctl ./cmd/nctl
```

### LINT COMMANDS
```
golangci-lint run
```

### FORMAT COMMANDS
```
gofmt -w .
```

### GENERATE COMMANDS
```
go generate ./...
```
Generated files must be committed. CI fails if they are stale.

## Architecture Notes

- Dependency flow: `cmd → commands → service → repository` (one direction only)
- `internal/shell` package is self-contained; bridge adapters for dispatch live within it
- `presentation.Service` is the interface; concrete impl is unexported `presentationService`

## Gotchas

- `go generate ./...` must be re-run and committed when the Cobra tree or mocked interfaces change
- The shell package already uses `presentation.Service` (interface) — do not reference the concrete struct
- `golangci-lint run` must pass with zero suppressions
- `dispatch.Category` private fields (commands, prerequisite) are exposed via `Commands()` and `Prerequisite()` accessors added for the bridge adapter — use these, not reflection
- Bridge tests live in `package shell_test` (external) like all other shell tests — use only exported API and a local stub presenter; no mock framework needed
- `stubPresenter` pattern: implement all four `presentation.Service` methods, capture `lastData`/`lastFormat` for assertions, return configurable `returnString`/`returnErr`
- When adding a bool flag that maps to `*bool` in Overrides, set the flag default to match the viper default and use `cmd.Flags().Changed("flag-name")` to only apply the override when explicitly set — this prevents the flag default from silently clobbering config-file or env-var values
- `CLI_SHELL_ENABLED` env var is handled automatically by viper's AutomaticEnv via the `shell.enabled` → `CLI_SHELL_ENABLED` key remapping (prefix CLI, `.` → `_`)
- Root command `RunE` for the shell: check `agentMode` first (fall back to `cmd.Help()`), then check `cfg.Shell.Enabled` (fall back to `cmd.Help()`), then call `startShell`. Split into two functions (`launchShell` + `startShell`) for testability.
- `shell.BridgeCategory` ctxFor closure: set `AgentMode=false` and `AllowWrites=true` for the interactive shell — human sessions are not agent sessions and write operations should not be blocked.
- First-time banner detection: URI empty-or-equals `"bolt://localhost:7687"` AND password empty — use the package-level `isUnconfigured(*config.Config) bool` helper; do not inline this check in `printWelcome` so it can be tested independently.
- Tests for unexported shell helpers (e.g. `isUnconfigured`) must be in `package shell` (not `shell_test`); name the file `*_internal_test.go` to distinguish from the external `*_test.go` files in the same directory.
- In bridge loops that call `cat.Commands()`, snapshot the map once before the loop (`cmds := cat.Commands()`) to avoid O(n) repeated map allocations.
- Agent-mode env vars are `NCTL_AGENT`, `NCTL_RW`, `NCTL_REQUEST_ID` (renamed from `NEO4J_CLI_*` in task-002).
- Viper env prefix is now `NCTL` (was `CLI`), so config env vars are `NCTL_NEO4J_URI`, `NCTL_AURA_CLIENT_SECRET`, `NCTL_SHELL_ENABLED`, etc. (renamed in task-003).
- Default config path is now `~/.nctl/config.json` (was `~/.neo4j-cli/config.json`).
- Default log file path is now `~/.nctl/nctl.log` (was `~/.neo4j-cli/neo4j-cli.log`); tmpdir fallback is `nctl.log`.
