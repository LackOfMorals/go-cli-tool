# PRD: Add TOON to Output Formats

## Overview

Add [TOON](https://github.com/toon-format/toon) (Token-Oriented Object Notation) as a first-class output format in nctl's presentation layer, on equal footing with the existing `text`, `json`, `pretty-json`, `table`, and `graph` formats. TOON is significantly more token-efficient than JSON, making it well-suited as the default output format when nctl is invoked from an LLM/agent context.

## Goals

- Let users opt into TOON via `--format toon` and the `cypher.output_format` config key, the same way they already select any other format.
- Make TOON the default output format in agent mode so LLM-driven sessions consume meaningfully fewer tokens.
- Render Neo4j-specific shapes (nodes, relationships) cleanly in TOON — without exposing internal sentinel keys (`_labels`, `_type`, `_id`).
- Land the change with no behavioural impact on the five existing formats.

## Non-Goals

What is explicitly out of scope.

- Implementing a custom TOON encoder. We will use `github.com/toon-format/toon-go`.
- Parsing TOON input. nctl only emits formatted output; it never reads TOON.
- Surfacing TOON-encoder tuning knobs (indent style, key ordering, etc.) as user-configurable settings.
- Forcing TOON in agent mode. The agent-mode default is soft — `--format` and explicit config still win.

## Requirements

### Functional Requirements

- REQ-F-001: Add a new `OutputFormatTOON OutputFormat = "toon"` constant in `internal/presentation/presentation.go` and include it in `OutputFormat.IsValid()`.
- REQ-F-002: Implement a `TOONFormatter` in the presentation package that satisfies `OutputFormatter` and uses `github.com/toon-format/toon-go` to encode the input.
- REQ-F-003: Register `TOONFormatter` inside `NewPresentationService` alongside the existing built-in formatters so it works for any command that goes through the presentation service.
- REQ-F-004: `TOONFormatter.Format` must handle the same input types the other formatters handle: `Tabular` (rows of cells), `*DetailData` (key/value blocks), and arbitrary scalars / strings.
- REQ-F-005: Before encoding, normalize Neo4j entity maps into a cleaner shape so the sentinel keys do not leak into the TOON output:
  - Nodes (maps with a `_labels` key) → `{labels: [...], properties: {...}}` where `properties` excludes any `_`-prefixed keys.
  - Relationships (maps with a `_type` key) → `{type: "...", properties: {...}}` where `properties` excludes any `_`-prefixed keys.
  - Other maps pass through unchanged.
- REQ-F-006: `cypher --format toon ...` must accept `toon` as a valid value, mirroring the existing handling of `table`, `graph`, `json`, `pretty-json`.
- REQ-F-007: `nctl config set cypher.output_format toon` must persist and take effect, mirroring the existing handling of other formats. Update any validation list / help text that enumerates allowed values.
- REQ-F-008: When nctl runs in agent mode and the user has not explicitly chosen a format (no `--format` flag, no `cypher.output_format` set in config), the default output format must be TOON. If either an explicit flag or a config value is present, that value wins.
- REQ-F-009: Update help text and any `--format` enumeration (e.g. cypher command help, config help) to list `toon` as a valid value.
- REQ-F-010: Regenerate `internal/skill/skill.md.gen` (and any other generated artefacts that enumerate output formats) after the wiring lands. CI must remain green.
- REQ-F-011: Add a `changie new` fragment describing the change.

### Non-Functional Requirements

- REQ-NF-001: `TOONFormatter` must be safe for concurrent use, matching the existing formatters' contract documented on `presentation.Service`.
- REQ-NF-002: Adding TOON must not change the output of any existing format. Existing presentation tests must continue to pass without modification.
- REQ-NF-003: Unit tests must cover `TOONFormatter` for: `TableData` (multi-row, empty), `DetailData`, scalar / string input, node entity maps, relationship entity maps, and pass-through of plain maps. Place tests alongside `internal/presentation/presentation_test.go`.
- REQ-NF-004: Agent-mode default selection must be unit-tested (TOON applied when no override; flag wins; config wins).
- REQ-NF-005: No new top-level dependencies beyond `github.com/toon-format/toon-go`.

## Technical Considerations

- **Library integration**: `github.com/toon-format/toon-go` will be added to `go.mod`. Pin to the latest stable release at the time of implementation; verify the import path and public API surface during the first task.
- **Where the formatter lives**: a new file `internal/presentation/toon.go` (with tests in `presentation_test.go` or a sibling `toon_test.go`) keeps the layout consistent with the inline `JSONFormatter` / `GraphFormatter` style already used in `presentation.go`. Either location is acceptable; pick whichever keeps file sizes reasonable.
- **Entity normalization**: implement a small `normalizeForTOON(any) any` helper that recurses through maps and slices, rewriting node and relationship maps as described in REQ-F-005. This same helper is what `TOONFormatter` calls before delegating to `toon-go`.
- **Agent-mode default plumbing**: nctl already has a notion of agent mode (see `internal/dispatch` — the dispatcher enforces `MutationMode` automatically in agent mode). The default format is currently passed into `NewPresentationService` from the app's startup wiring (`app.go`). Choose the agent-mode-aware default at that startup site:
  - If agent mode is active **and** the user has not set `cypher.output_format`, pass `OutputFormatTOON` as the default.
  - Otherwise pass whatever default the app chooses today.
  - The `--format` flag is already a per-call override via `FormatOverride` / `FormatAs`; no change needed to that path.
- **Help and skill generation**: `cypher`'s help string in `internal/commands/cypher.go` currently lists `table|graph|json|pretty-json`. Update to include `toon`. After the change, run `go generate ./...` and commit the regenerated `internal/skill/skill.md.gen` per CLAUDE.md's instructions.
- **Config validation**: if `cypher.output_format` is validated against a known list anywhere in `internal/config`, add `"toon"` to that list.
- **Behaviour on unknown TOON inputs**: `toon-go` is expected to handle arbitrary Go values via reflection. If it returns an error for a particular shape, surface it as `fmt.Errorf("encode TOON: %w", err)` (matching the JSON formatter's style), and let the caller decide what to do.

## Acceptance Criteria

- [ ] `cypher MATCH (n:Person) RETURN n LIMIT 1 --format toon` emits valid TOON output with no `_labels` / `_type` / `_id` keys present.
- [ ] `cypher --format toon RETURN 1 AS n` works for scalar results.
- [ ] `nctl config set cypher.output_format toon` persists and is honoured by subsequent cypher invocations without `--format`.
- [ ] All commands that go through the presentation service (e.g. `get`, `list`-style commands using `DetailData` / `TableData`) accept and produce correct TOON output when the active format is `toon`.
- [ ] In agent mode with no `--format` and no `cypher.output_format` set, the default output format is TOON.
- [ ] In agent mode, `--format json` still emits JSON; an explicit `cypher.output_format = json` config still emits JSON.
- [ ] All existing `presentation` package tests still pass unchanged.
- [ ] New unit tests cover: `Tabular`, `DetailData`, scalar, node, relationship, plain map, agent-mode-default selection.
- [ ] `go generate ./...` produces no diff after the change is committed.
- [ ] `go build ./...`, `go test ./...`, and `golangci-lint run` all pass.
- [ ] A changie fragment is included in the PR.

## Out of Scope

- A custom TOON encoder (we use `github.com/toon-format/toon-go`).
- TOON input parsing.
- User-configurable TOON encoder options (indent, key ordering, etc.).
- Hard-forcing TOON in agent mode — the agent-mode default is overridable.
- Re-formatting the existing five formats' output to match TOON's structural choices.

## Open Questions

- Which exact version of `github.com/toon-format/toon-go` to pin? Resolve during the first implementation task by checking the latest stable tag.
- Does `toon-go` expose options that meaningfully change output (indentation, ordering)? If so, do we want any non-default tuning at the call site, or stick with library defaults? Treat as a tuning decision during implementation; defaults are acceptable for v1 unless they look wrong.
- How does nctl detect agent mode at the precise startup point where the default format is chosen — is the signal already available in the `App` struct, or does it need to be threaded through? Verify during task breakdown.
