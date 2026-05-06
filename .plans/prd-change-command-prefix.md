# PRD: Change Command Prefix (env-var docs + shell prompt)

## Overview

Finish the `neo4j-cli` → `lom` rename in two places it didn't reach:

1. **Env-var prefix.** At runtime, viper's env prefix is already `LOM_` (see [internal/config/config.go:319](internal/config/config.go#L319)) — `LOM_LOG_LEVEL`, `LOM_NEO4J_PASSWORD`, etc. are what the binary actually reads. But the README, several flag-help strings, multiple user-facing error messages, and AGENTS.md line 51 still document `CLI_*`. A user copying examples from the README today will set env vars that are silently ignored. This PRD updates docs and user-facing strings to match the runtime, with no behaviour change to env-var binding.
2. **Interactive shell prompt.** The default prompt is still hardcoded to `"neo4j> "` in three places. Change it to `"lom> "` (matching the trailing-space shape) so the prompt reflects the new binary name.

This is purely a finish-the-rename PRD — no new functionality, no behaviour change beyond the prompt string and the user-facing tips/errors/help that currently advertise the wrong env-var names.

## Goals

- Eliminate every active `CLI_*` mention in code, README, and AGENTS.md so users only see the env-var names that actually work.
- Default the interactive shell prompt to `"lom> "`.
- Land a single `Changed` changie fragment that records both renames as a "rename completion" entry.
- Leave config-file behaviour, env-var binding, and runtime semantics unchanged.

## Non-Goals

- Adding any backwards-compat shim that accepts both `CLI_*` and `LOM_*`. The runtime already only accepts `LOM_*`; this PRD does not add a startup warning, deprecation log, or env-var bridge.
- Rewriting historical CHANGELOG entries that mention `CLI_SHELL_ENABLED`, `neo4j-cli`, or `NEO4J_CLI_*`. Those document what shipped at the time and stay as-is.
- Migrating any user's saved `shell.prompt: "neo4j> "` config value. Defaults change in code; existing user-config values are their explicit choice and must be respected.
- Renaming any other prefixes (`NEO4J_*`, package paths, etc.) — those are out of scope.

## Requirements

### Functional Requirements

- REQ-F-001: Replace every `CLI_*` env-var reference in active production code with the corresponding `LOM_*` form. Confirmed sites:
  - [cmd/lom/app.go:187](cmd/lom/app.go#L187) — `--no-metrics` flag help (`CLI_TELEMETRY_METRICS`)
  - [cmd/lom/app.go:196](cmd/lom/app.go#L196) — `--shell` flag help (`CLI_SHELL_ENABLED`)
  - [cmd/lom/app.go:635](cmd/lom/app.go#L635) — comment (`CLI_TELEMETRY_MIXPANEL_TOKEN`)
  - [internal/repository/neo4j_repo.go:43,46](internal/repository/neo4j_repo.go#L43) — connection error messages (`CLI_NEO4J_URI`, `CLI_NEO4J_USERNAME`)
  - [internal/commands/prerequisites.go:79,145](internal/commands/prerequisites.go#L79) — interactive tip lines (`CLI_AURA_*`, `CLI_NEO4J_*`)
  - [internal/service/cloud_service.go:52](internal/service/cloud_service.go#L52) — error message (`CLI_AURA_CLIENT_ID`/`SECRET`)
  - During implementation, re-grep the tree to catch any further occurrences.
- REQ-F-002: Update [README.md](README.md) so the env-var table (currently lines ~200–225) and surrounding prose document `LOM_*` exclusively. Update the security note that mentions `CLI_NEO4J_PASSWORD` / `CLI_AURA_CLIENT_SECRET`.
- REQ-F-003: Update [AGENTS.md](AGENTS.md) line 51 — the note about `shell.enabled` → `CLI_SHELL_ENABLED` is incorrect; the actual prefix is `lom`. Replace `CLI` with `lom` in that note. Verify the rest of AGENTS.md is consistent.
- REQ-F-004: Change the default shell prompt from `"neo4j> "` to `"lom> "` (preserving the trailing space) in all three source locations:
  - [internal/shell/interactive.go:54](internal/shell/interactive.go#L54) — runtime default
  - [internal/config/config.go:288](internal/config/config.go#L288) — viper default for `shell.prompt`
  - [internal/commands/config.go:79](internal/commands/config.go#L79) — config registry `Default`
- REQ-F-005: Update doc-comment examples that still show `neo4j>`:
  - [internal/commands/cypher.go:22-27](internal/commands/cypher.go#L22) — six example lines
  - [internal/shell/interactive.go:668,674,681](internal/shell/interactive.go#L668) — three example lines
  - [internal/shell/category.go:24,25,30](internal/shell/category.go#L24) — three example lines
- REQ-F-006: Update test fixtures that hardcode `"neo4j> "`:
  - [internal/shell/interactive_test.go:28](internal/shell/interactive_test.go#L28)
  - [internal/shell/parse_command_test.go:65-66](internal/shell/parse_command_test.go#L65) — both the input string and the expected token slice
- REQ-F-007: Add one `Changed` changie fragment in `.changes/unreleased/` covering both renames. Body should make clear that (a) `CLI_*` env vars are not read by the binary today and (b) the default shell prompt is now `lom>`. Match the prose style of existing fragments.

### Non-Functional Requirements

- REQ-NF-001: No production `*.go` file outside [/Users/jgiffard/Projects/lom/CHANGELOG.md](CHANGELOG.md) and the changie fragment that introduces this change should mention `CLI_` after the work lands. A grep is the acceptance check.
- REQ-NF-002: No production `*.go` file should hardcode `"neo4j>"` after the work lands. The literal `"neo4j> "` may continue to exist only in CHANGELOG.md (historical) and inside `.changes/` archives.
- REQ-NF-003: README, AGENTS.md, and source-code error messages must agree on the env-var prefix.
- REQ-NF-004: Existing tests pass without modification beyond the explicit fixture updates in REQ-F-006. No new test logic is required, only fixture string substitution.
- REQ-NF-005: Saved user configs (`shell.prompt: "neo4j> "` set explicitly by a user) continue to be honoured. The default change only affects users who haven't set the value.

## Technical Considerations

- **No env-var binding change.** [internal/config/config.go:319](internal/config/config.go#L319) (`v.SetEnvPrefix("lom")`) stays put. This PRD is docs/help-text/error-message cleanup *around* an already-correct runtime.
- **One greppable acceptance check per rename.** After the work lands, both of these greps must be empty (excluding `CHANGELOG.md`, `.changes/`, and `.plans/`):
  ```
  grep -rn "CLI_" --include='*.go' --include='*.md' --include='*.yaml' .
  grep -rn "neo4j>" --include='*.go' .
  ```
- **Doc comments matter.** Cobra surfaces them in `--help` and `lom help <cmd>`, so the doc-comment examples in `cypher.go`, `interactive.go`, and `category.go` reach users, not just godoc.
- **Test fixture caveat.** The parser test in [internal/shell/parse_command_test.go:65-66](internal/shell/parse_command_test.go#L65) is checking shlex-style tokenization of `set prompt "neo4j> "` — the literal string is incidental to what the test asserts. Update the input and expected slice in lockstep.
- **Single changie fragment.** Filename: `.changes/unreleased/changed-lom-prefix-and-prompt.yaml` (or similar). Matches the "one feature, one fragment" pattern already used in this repo.
- **Generated files.** Run `go generate ./...` after the change. Expected diff: none, since this PRD does not change the cobra command tree or any mocked interface.

## Acceptance Criteria

- [ ] `grep -rn "CLI_" --include='*.go' --include='*.md' --include='*.yaml' .` returns no matches outside `CHANGELOG.md` and `.changes/` archives.
- [ ] `grep -rn "neo4j>" --include='*.go' .` returns no matches.
- [ ] Default shell prompt on a fresh install (no user config file) is `lom> `.
- [ ] `lom set prompt "anything"` and a user-set `shell.prompt:` in the config file still work — the rename is default-only, not a forced override.
- [ ] `lom --help`, `lom cypher --help`, and `lom help shell` all show `LOM_*` env-var names and `lom>` examples.
- [ ] User-facing error messages from connection failure ([internal/repository/neo4j_repo.go](internal/repository/neo4j_repo.go)) and prerequisite tips ([internal/commands/prerequisites.go](internal/commands/prerequisites.go)) advertise `LOM_*`.
- [ ] AGENTS.md line 51 (and any other inline notes) match the actual `lom` prefix.
- [ ] One `Changed` changie fragment exists in `.changes/unreleased/`, parses as valid changie YAML, and describes both renames in one bullet.
- [ ] `go build ./...`, `go test ./...`, `golangci-lint run`, and `go generate ./...` (no diff) all pass.

## Out of Scope

- Backwards-compat for `CLI_*` env vars (no warning, no shim, no fallback).
- Rewriting historical CHANGELOG entries.
- Migrating users' explicit saved `shell.prompt` values.
- Any further renaming work (binary, package paths, other prefixes) — already done in earlier PRs.
- Multi-fragment changelog entries.

## Open Questions

- Are there `CLI_*` mentions in additional places I haven't grepped (e.g. shell completion scripts under [config/](config/), `manifest.json`, the `goreleaser.yaml`)? Resolve during the first task: re-run the greps from the Technical Considerations section against the *whole tree*, not just `internal/` and `cmd/`.
