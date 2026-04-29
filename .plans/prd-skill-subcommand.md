# PRD: `skill` subcommand — install agent skills for neo4j-cli

## Overview

Add a top-level `skill` category to neo4j-cli that installs a SKILL.md document into the user's local AI agent directories (Claude Code, Cursor, Codex, etc). Agents read these skills and learn how to invoke neo4j-cli correctly — particularly with `--agent` mode. Pattern adapted from [oskarhane/homebrew-neo4j-query/src/skill.rs](https://github.com/oskarhane/homebrew-neo4j-query/blob/main/src/skill.rs), but simplified: we copy directly from the embedded binary bytes into each agent's skills dir, with no canonical/cache dir.

## Goals

- One command (`neo4j-cli skill install`) drops a working skill into every detected agent.
- SKILL.md content is generated at build time from the cobra command tree, so it stays in sync as commands evolve.
- Reserve a hand-written "gotchas" section for guidance the command tree can't express (e.g. "always pass `--agent` when calling from another agent").
- Detect, install, and remove for the same agent set as the Rust reference.
- Conform to existing repo conventions: service interface, prerequisite-free category, `MutationMode` declared, JSON output under `--agent`.

## Non-Goals

- Hosting / distributing skills outside this binary (no registry, no remote pull).
- Per-agent skill customisation (one canonical SKILL.md, identical across agents).
- Symlinks — always copy. Re-run install after upgrade.
- Windows-specific install paths beyond what the agent set already implies.
- Auto-install on first run / bundled with `neo4j-cli config set` etc.
- Authoring the prose of the gotchas section (placeholder + one stub gotcha; real content iterated separately).

## Requirements

### Functional Requirements

- **REQ-F-001**: New top-level subcommand `neo4j-cli skill` with three commands: `install [agent]`, `remove [agent]`, `list`. Agent is **positional**, optional. Matches repo convention (`cloud instances get <id>`, `config set <key> <value>`).
- **REQ-F-002**: `skill install [agent]` writes the embedded SKILL.md directly into each target agent's skills dir as `<skills_dir>/neo4j-cli/SKILL.md`. Existing target `<skills_dir>/neo4j-cli/` (file, dir, or symlink) is removed first.
- **REQ-F-003**: With no positional, install targets every detected agent (those whose `detect_dir` exists). If none detected, error with exit 1 and a message naming the supported agents.
- **REQ-F-004**: With `<agent>`, install targets only that agent. Unknown name → error, exit 1.
- **REQ-F-005**: `skill remove [agent]` removes `<skills_dir>/neo4j-cli/` from the target. With no positional, removes from every agent that currently has it. No canonical dir to clean (we don't write one).
- **REQ-F-006**: `skill list` prints a table: agent name, detected (yes/no), installed (yes/no). `--format json` returns a JSON array `[{name, display_name, detected, installed}, ...]`.
- **REQ-F-007**: Hardcoded agent list matches the Rust reference verbatim: `claude-code`, `cursor`, `windsurf`, `copilot`, `gemini-cli`, `cline`, `codex`, `pi`, `opencode`, `junie`. Each entry: `name`, `display_name`, `detect_dir`, `skills_dir`. Path templates support `~` and `$XDG_CONFIG_HOME` (default `~/.config`).
- **REQ-F-008**: SKILL.md is **generated at build time** from the cobra command tree (root + every subcommand: usage, short, long, flags, examples). Output is written to a tracked-but-generated file (`internal/skill/skill.md.gen`) and embedded into the binary via `//go:embed`. The generator imports the live cobra builders from `cmd/neo4j-cli/app.go`; this requires a small refactor so builders are tree-walkable without fully-wired services (services injected only for `RunE`, can be nil for doc generation).
- **REQ-F-009**: A hand-written `skill-additions.md` lives in-repo at `internal/skill/skill-additions.md` and is concatenated into the generated SKILL.md under a `## Gotchas` section. Ships with at least one stub: "Always pass `--agent` when invoking neo4j-cli from another agent — output becomes JSON and writes are blocked unless `--rw` is also passed."
- **REQ-F-010**: `MutationMode`: `install` and `remove` are `ModeWrite`; `list` is `ModeRead`. Under `--agent` without `--rw`, install/remove return `READ_ONLY` JSON error on stdout (handled by existing dispatcher).
- **REQ-F-011**: Under `--agent`, all three commands emit the standard JSON envelope (`status`, `data`/`error`, `request_id`, `schema_version`).
- **REQ-F-012**: `--help` integration (mostly automatic via cobra, but must be verified):
  - `neo4j-cli --help` lists `skill` under "Available Commands" with a one-line `Short`.
  - `neo4j-cli skill --help` shows `install`, `remove`, `list` as subcommands with their own `Short`s.
  - `neo4j-cli skill install --help`, `skill remove --help`, `skill list --help` each render usage, description (`Long`), flags, and at least one `Example`.
  - Tone and format match existing categories (`cloud`, `admin`, `cypher`, `config`).

### Non-Functional Requirements

- **REQ-NF-001**: No new runtime dependencies. Use stdlib `os`, `path/filepath`, `embed`, `text/tabwriter`.
- **REQ-NF-002**: Build-time SKILL.md generator is a separate Go program under `cmd/gen-skill/` invoked via `go generate ./...` and run by CI before `go build`. Generator must not depend on a running neo4j-cli binary; it imports the same cobra tree builders used by `cmd/neo4j-cli/app.go`.
- **REQ-NF-003**: Cross-platform paths (filepath.Join, no hardcoded `/`). Skip `expand_path` Windows quirks — accept that the hardcoded agent list is unix-shaped (`~/.foo`); document it.
- **REQ-NF-004**: Idempotent install: re-running `skill install` on an already-installed agent succeeds and overwrites cleanly.
- **REQ-NF-005**: Test coverage parity with existing commands — service unit tests with a fake filesystem (in-memory or `t.TempDir()`), command-layer tests using mocks per `mockgen` pattern.
- **REQ-NF-006**: Mutation classification enforced at the dispatch layer (declarative on the `Command` struct), not by checking `ctx.AgentMode` inside handlers.

## Technical Considerations

### Package layout

Follow repo conventions:

```
internal/
    skill/
        agents.go            // Agent struct, hardcoded AGENTS slice, expand path, find/detect helpers
        skill-additions.md   // hand-written gotchas, concatenated by generator
        skill.md.gen         // generated at build time, //go:embed-ed
        embed.go             // //go:embed skill.md.gen → SkillMD []byte
    service/
        interfaces.go        // + SkillService interface
        skill_service.go     // Install, Remove, List — takes a filesystem-ish interface
    commands/
        skill.go             // BuildSkillCategory(svc service.SkillService) *dispatch.Category
cmd/
    neo4j-cli/
        app.go               // wire skillSvc, register category, add cobra subcommand
    gen-skill/
        main.go              // generator: builds cobra tree, walks commands, emits skill.md.gen
```

### Service interface

```go
type SkillService interface {
    Install(ctx context.Context, agentName string) ([]InstallResult, error)
    Remove(ctx context.Context, agentName string) ([]RemoveResult, error)
    List(ctx context.Context) ([]AgentStatus, error)
}
```

`InstallResult{Agent, Path}`, `RemoveResult{Agent}`, `AgentStatus{Name, DisplayName, Detected, Installed}`.

Implementation takes a `Filesystem` interface (Create/Mkdir/Remove/Stat/Lstat/ReadDir/WriteFile) so service tests use an in-memory or temp-dir fake. Production impl wraps `os`.

### No prerequisite

`skill` does not connect to Neo4j or Aura → no `SetPrerequisite` call. Mirrors `config` category.

### Generator (`cmd/gen-skill/main.go`)

1. Refactor `cmd/neo4j-cli/app.go` so the cobra command-tree builder (`buildRootCommand` and friends) can run without fully-wired services — services injected only for `RunE` execution, can be nil for tree walking. Concretely: lift the cobra wiring into a function that takes service interfaces and tolerates nil for doc-generation purposes.
2. Generator constructs the tree with nil services, walks via `cmd.Commands()`, emits Markdown:
   - `# neo4j-cli` (root short + long)
   - For each subcommand: `## <name>`, usage, description, flags table, examples
3. Append: read `internal/skill/skill-additions.md`, prepend `## Gotchas\n\n`.
4. Write to `internal/skill/skill.md.gen`.

A `//go:generate go run ./cmd/gen-skill` directive lives in `internal/skill/embed.go`. CI step: `go generate ./... && git diff --exit-code internal/skill/skill.md.gen` to fail PRs that forget to regenerate.

### Mutation enforcement

Existing dispatcher already blocks `ModeWrite` commands under agent mode without `--rw`. Just declare modes correctly on each `dispatch.Command`. No handler-side `ctx.AgentMode` check beyond suppressing prompts (none expected for skill).

### Idempotency / safety

- Install removes target dir/symlink before write → safe on re-run.
- Remove tolerates "already absent" silently per agent.
- No canonical dir to clean up; embedded bytes are the only source.

## Acceptance Criteria

- [ ] `neo4j-cli --help` lists `skill` under "Available Commands" with a one-line description.
- [ ] `neo4j-cli skill --help` lists `install`, `remove`, `list` with descriptions matching the rest of the CLI's tone.
- [ ] `neo4j-cli skill install --help` (and `remove --help`, `list --help`) each show usage, `Long` description, flags, and at least one `Example`.
- [ ] `neo4j-cli skill list` prints a 3-column table; `--format json` returns a JSON array `[{name, display_name, detected, installed}, ...]`.
- [ ] `neo4j-cli skill install` with no agents detected → exit 1, stderr lists supported agents.
- [ ] `neo4j-cli skill install` with detected agents writes SKILL.md into each agent's skills dir; running it twice is a no-op (no error, files identical).
- [ ] `neo4j-cli skill install claude-code` works even if Claude is not detected; unknown name errors with exit 1.
- [ ] `neo4j-cli skill remove` removes from every agent that has it; subsequent run prints "skill was not installed for any agents".
- [ ] `neo4j-cli --agent skill install` returns `READ_ONLY` JSON error on stdout, exit non-zero. With `--rw`, succeeds and emits success JSON.
- [ ] `neo4j-cli --agent skill list` returns valid JSON envelope with the agent array as `data`.
- [ ] Generated `skill.md.gen` contains every cobra subcommand and ends with the gotchas section.
- [ ] `go generate ./... && git diff --exit-code` is clean on a fresh checkout (CI gate).
- [ ] Service has unit tests using a fake filesystem covering: install (no agent / one agent / all detected / overwrite), remove (with / without prior install), list.
- [ ] Command layer has tests using `mockgen`-generated `MockSkillService` per existing pattern.
- [ ] README updated: new `skill` row in the commands table, brief usage section.
- [ ] `changie new` fragment added (kind: `Added`).

## Out of Scope

- Authoring polished gotchas prose beyond the one stub.
- Per-agent skill variants.
- Symlink mode, skill versioning, remote skill registry.
- Auto-install hook on `config set` / first run.
- Windows-specific path handling beyond what stdlib `filepath` already provides.
- Adding new agents not in the Rust reference.

## Open Questions

None — all resolved:
- ~~Canonical dir~~ → dropped; embedded bytes copied directly to agent dirs.
- ~~Flag vs positional~~ → positional (`skill install [agent]`) per repo convention.
- ~~JSON shape~~ → bare JSON array.
- ~~Generator strategy~~ → live cobra builders + small refactor so tree-walk works without wired services.
- ~~Gotchas file~~ → single `internal/skill/skill-additions.md`.
