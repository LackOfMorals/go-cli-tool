# PRD: Rename Binary to lom

## Overview

Rename the CLI binary from `neo4j-cli` to `lom` and make every reference consistent across the codebase: the cmd directory, the Cobra root command name, the GoReleaser build config, all env vars (unified under a single `LOM_` prefix), default config/log file paths, and user-facing documentation.

## Goals

- The installed binary is named `lom`.
- All env vars use a single consistent `LOM_` prefix.
- Default config and log directories move from `~/.neo4j-cli/` to `~/.lom/`.
- README and CLAUDE.md reflect the new name.
- No references to `neo4j-cli` remain in any user-facing surface (binary, env vars, paths, help text, error messages).

## Non-Goals

- Changing the Go module path (`github.com/cli/go-cli-tool` is unaffected).
- Providing migration shims or backwards-compatible `NEO4J_CLI_*` / `CLI_*` env var aliases.
- Renaming the GitHub repository itself.

## Requirements

### Functional Requirements

- **REQ-F-001**: `cmd/neo4j-cli/` is renamed to `cmd/lom/`. The Go `main` package inside remains `package main`.
- **REQ-F-002**: The Cobra root command's `Use` field changes from `"neo4j-cli"` to `"lom"`. All `Example:` strings in subcommand builders are updated accordingly.
- **REQ-F-003**: The agent-mode env vars read directly via `os.Getenv` become `LOM_AGENT`, `LOM_RW`, and `LOM_REQUEST_ID`.
- **REQ-F-004**: The viper env prefix changes from `"CLI"` to `"lom"`, making config env vars `LOM_LOG_LEVEL`, `LOM_NEO4J_URI`, `LOM_AURA_CLIENT_SECRET`, `LOM_SHELL_ENABLED`, etc.
- **REQ-F-005**: `DefaultConfigFilePath()` in `internal/config/config.go` returns `~/.lom/config.json` (and falls back to `.lom-config.json` on home-dir error).
- **REQ-F-006**: `DefaultLogFilePath()` in `internal/logger/logger.go` returns `~/.lom/lom.log` (and falls back to `<tmpdir>/lom.log`).
- **REQ-F-007**: The `--log-file` flag description, all help text, error messages, and shell built-in output that reference `neo4j-cli` are updated to `lom`.
- **REQ-F-008**: `.goreleaser.yaml` is updated: `main: ./cmd/lom`, `binary: lom`, `id: lom`.
- **REQ-F-009**: `README.md` is updated: title, build command (`go build -o bin/lom ./cmd/lom`), all usage examples, and any path references (`~/.lom/`).
- **REQ-F-010**: `CLAUDE.md` build command is updated to `go build -o bin/lom ./cmd/lom`.

### Non-Functional Requirements

- **REQ-NF-001**: `go test ./...` passes with no failures after the rename; test assertions that check for `"neo4j-cli"` in output or path strings are updated to `"lom"`.
- **REQ-NF-002**: `golangci-lint run` reports zero issues after the rename.
- **REQ-NF-003**: `go build ./cmd/lom` produces a working binary.

## Technical Considerations

### Files requiring changes

| File | Change |
|---|---|
| `cmd/neo4j-cli/` | Rename directory to `cmd/lom/` |
| `cmd/lom/app.go` | `Use: "lom"`, all example strings, `os.Getenv("LOM_*")`, flag descriptions, `"Run 'lom --help'"` error string |
| `internal/config/config.go` | `SetEnvPrefix("lom")`, `DefaultConfigFilePath()` paths, all `CLI_` comments |
| `internal/logger/logger.go` | `DefaultLogFilePath()` paths and comments |
| `internal/shell/interactive.go` | Version string `"lom %s"`, `~/.lom/lom.log` in config display, `LOM_` references in help |
| `internal/dispatch/category.go` | Error message `"run 'lom %s --help'"` |
| `internal/dispatch/dispatch.go` | Comments referencing `NEO4J_CLI_AGENT`, `NEO4J_CLI_RW` |
| `internal/commands/cypher.go` | `MAINTENANCE:` comments referencing `cmd/neo4j-cli/app.go` |
| `internal/tool/tool.go` | Comment referencing `NEO4J_CLI_RW` |
| `.goreleaser.yaml` | `main`, `binary`, `id` fields |
| `README.md` | Title, build command, all 56+ `neo4j-cli` occurrences |
| `CLAUDE.md` | Build command |

### Test files requiring updates

| File | Change |
|---|---|
| `internal/shell/interactive_test.go:87-88` | Assert `"lom"` instead of `"neo4j-cli"` in version output |
| `internal/logger/logger_test.go:212-213` | Assert `"lom"` instead of `"neo4j-cli"` in log path |
| `internal/config/config_test.go:17` | Update comment about `~/.lom/config.json` |

### Env var mapping (old → new)

| Old | New |
|---|---|
| `NEO4J_CLI_AGENT` | `LOM_AGENT` |
| `NEO4J_CLI_RW` | `LOM_RW` |
| `NEO4J_CLI_REQUEST_ID` | `LOM_REQUEST_ID` |
| `CLI_LOG_LEVEL` | `LOM_LOG_LEVEL` |
| `CLI_LOG_FORMAT` | `LOM_LOG_FORMAT` |
| `CLI_LOG_OUTPUT` | `LOM_LOG_OUTPUT` |
| `CLI_LOG_FILE` | `LOM_LOG_FILE` |
| `CLI_SHELL_ENABLED` | `LOM_SHELL_ENABLED` |
| `CLI_SHELL_PROMPT` | `LOM_SHELL_PROMPT` |
| `CLI_SHELL_HISTORY_FILE` | `LOM_SHELL_HISTORY_FILE` |
| `CLI_NEO4J_URI` | `LOM_NEO4J_URI` |
| `CLI_NEO4J_USERNAME` | `LOM_NEO4J_USERNAME` |
| `CLI_NEO4J_PASSWORD` | `LOM_NEO4J_PASSWORD` |
| `CLI_NEO4J_DATABASE` | `LOM_NEO4J_DATABASE` |
| `CLI_AURA_CLIENT_ID` | `LOM_AURA_CLIENT_ID` |
| `CLI_AURA_CLIENT_SECRET` | `LOM_AURA_CLIENT_SECRET` |
| `CLI_AURA_TIMEOUT_SECONDS` | `LOM_AURA_TIMEOUT_SECONDS` |
| `CLI_AURA_INSTANCE_DEFAULTS_*` | `LOM_AURA_INSTANCE_DEFAULTS_*` |
| `CLI_CYPHER_SHELL_LIMIT` | `LOM_CYPHER_SHELL_LIMIT` |
| `CLI_CYPHER_EXEC_LIMIT` | `LOM_CYPHER_EXEC_LIMIT` |
| `CLI_CYPHER_OUTPUT_FORMAT` | `LOM_CYPHER_OUTPUT_FORMAT` |
| `CLI_TELEMETRY_METRICS` | `LOM_TELEMETRY_METRICS` |
| `CLI_TELEMETRY_MIXPANEL_TOKEN` | `LOM_TELEMETRY_MIXPANEL_TOKEN` |

## Acceptance Criteria

- [ ] `go build -o bin/lom ./cmd/lom` succeeds.
- [ ] `bin/lom --help` shows `lom` as the command name.
- [ ] `bin/lom version` shows `lom <version>`.
- [ ] `LOM_AGENT=true bin/lom` activates agent mode.
- [ ] `LOM_NEO4J_URI=bolt://host:7687 bin/lom cypher "RETURN 1"` is respected.
- [ ] Default config file resolves to `~/.lom/config.json`.
- [ ] Default log file resolves to `~/.lom/lom.log`.
- [ ] `go test ./...` passes with no failures.
- [ ] `golangci-lint run` reports zero issues.
- [ ] No occurrences of `neo4j-cli`, `NEO4J_CLI_`, or `CLI_` prefix remain in any user-facing string, path, or env var name.

## Out of Scope

- Backwards-compatible env var aliases (`NEO4J_CLI_*` / `CLI_*` continuing to work).
- Renaming the GitHub repository or Go module path.
- Migrating existing user config files from `~/.neo4j-cli/` to `~/.lom/`.

## Open Questions

None — all questions resolved during PRD Q&A.
