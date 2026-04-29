# AGENTS.md

Project-specific notes for AI agents working on this repo.

## Feedback Instructions

### TEST COMMANDS

- `go test ./...` — run all tests
- `go test ./internal/<pkg>/...` — run a single package

### BUILD COMMANDS

- `go build ./...` — verify everything compiles

### LINT COMMANDS

- `go vet ./...` — stdlib vet (always available)
- `golangci-lint run` — full lint per `.golangci.yaml` (may not be installed locally; CI runs it)

### FORMAT COMMANDS

- `gofmt -w <file-or-dir>` — format
- `gofmt -l <file-or-dir>` — check (empty output = clean)

## Architecture

- `cmd/neo4j-cli/` — main entry (wires services into cobra)
- `cmd/gen-skill/` — build-time SKILL.md generator (planned, task-004)
- `internal/cli/` — shared cobra tree builder (Use/Short/Long/Flags/Example), reused by app + generator
- `internal/skill/` — agent catalog + embedded SKILL.md
- `internal/service/` — service interfaces + impls (Cypher, Cloud, Admin, Skill planned)
- `internal/commands/` — cobra-agnostic dispatch.Category builders per service
- `internal/dispatch/` — agent-mode JSON envelopes, MutationMode enforcement
- Mocks: generated via `go.uber.org/mock` mockgen (existing pattern under `internal/*/mocks/`)

## Conventions

- Module path: `github.com/cli/go-cli-tool`
- Go 1.24
- Service interfaces live in `internal/service/interfaces.go`
- Result types are exported on the interface package; JSON tags expected for agent-mode envelopes
- MutationMode is declared on the `dispatch.Command`, not checked inside handlers
- Errors from handlers wrap with `fmt.Errorf("...: %w", err)`
- Output formats: `--format table|json` (consistent with `config list`)

## Gotchas

- `golangci-lint` may not be installed locally — `go vet` + `gofmt -l` is the minimum local check.
- Tests should never write to real `$HOME`; use `t.TempDir()` + `t.Setenv("HOME", tmp)`.
- The repo's Rust reference for the skill subcommand is at `/Users/oskarhane/Development/neo4j-query/src/skill.rs` (see PRD).
- `neo4j-cli` binary in repo root is a build artifact — do not stage it in commits.
