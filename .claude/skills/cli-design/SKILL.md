---
name: cli-design
description: >
  Design command-line interface (CLI) UX: arguments, flags, subcommands, help
  text, output formats, error messages, exit codes, interactive prompts,
  config/env precedence, and safe/dry-run behavior. Use this skill whenever
  the user is designing, reviewing, or building a CLI tool — including when
  they ask about argument parsing, flag naming, --help layout, stdin/stdout
  conventions, shell completion, or how to handle destructive operations safely.
  Also trigger for questions like "how should my CLI handle errors", "what exit
  codes should I use", "should this be a flag or a subcommand", or any request
  to audit an existing CLI's UX.
---

# CLI Design Skill

Reference: [Command Line Interface Guidelines (clig.dev)](https://clig.dev/)

---

## Core Principles (apply to every decision)

1. **Human-first** — if humans will use it interactively, design for humans. Machines can adapt via flags.
2. **Composable** — stdout for data, stderr for messages. Zero on success, non-zero on failure. Pipeable by default.
3. **Consistent** — follow conventions users already know from `git`, `curl`, `docker` etc.
4. **Say just enough** — don't be silent (looks broken), don't be noisy (buries what matters).
5. **Conversational** — suggest corrections, confirm scary actions, explain what to do next.

---

## Arguments & Flags

### When to use what

| Construct | Use for |
|---|---|
| Positional argument | Required input that has a clear ordering (e.g. `cp SRC DST`) |
| Short flag `-x` | Frequently used options worth a single keystroke |
| Long flag `--name` | Everything else; always provide the long form |
| Subcommand | Distinct operations sharing a binary (e.g. `git commit`, `git push`) |
| Environment variable | Secrets, per-environment defaults, CI/CD configuration |
| Config file | Persistent user preferences; defaults that survive sessions |

### Flag naming conventions

- Prefer `--output-format` over `--outputFormat` (kebab-case)
- Boolean flags that default to **false** (e.g. `--verbose`, `--dry-run`, `--force`) need no negation — don't generate `--no-dry-run` or `--no-force`
- Boolean flags that default to **true** need a `--no-X` escape hatch (e.g. `--no-pager`, `--no-color`, `--no-verify`) so users can opt out
- Destructive actions require explicit opt-in flags (`--force`, `--yes`, `--confirm`)
- `-h` / `--help` — never overload; always reserved for help
- `-v` / `--verbose` — near-universal convention; use it. Use `-V` (uppercase) for `--version` to avoid collision
- `-V` / `--version` — print `<name> <semver>` to **stdout** and exit 0; never exit non-zero
- `-q` / `--quiet` — suppress all non-error output
- `-n` / `--dry-run` — preview without side effects
- `-o` / `--output` — **output file path** (e.g. `-o result.json`). Do not reuse `-o` for format selection
- `--format` / `--output-format` — output format selector (`text|json|yaml`); keep separate from `-o`
- `-f` is **contested**: means `--file` in `kubectl`/`docker run`, means `--force` in `rm`/`git clean`. If your tool needs both, assign `-f` to whichever is more frequent and leave the other long-form only. Never silently assign `-f` to both.
- `--force` — override safety checks; long-form only is fine if `-f` is taken

### Argument validation

- Validate early, fail fast, and say exactly what was wrong
- If you can guess intent, suggest the fix: `unknown flag --versbose, did you mean --verbose?`
- Accept both `-f value` and `-f=value`; both `--flag value` and `--flag=value`

---

## Subcommands

Use subcommands when your tool has multiple distinct operations (think `git`, `kubectl`, `docker`).

### Structure

```
myapp [global flags] <subcommand> [subcommand flags] [args]
```

- Global flags come before the subcommand
- Each subcommand can have its own flags and help
- Provide `help` as a subcommand: `myapp help deploy`
- Also support `myapp deploy --help`

### Naming

- Use verbs for operations: `create`, `delete`, `list`, `get`, `apply`, `run`
- Avoid abbreviations unless they are universal (`ls` is fine; `dloy` for deploy is not)
- Pick **one** ordering and apply it consistently across the whole tool:
  - **Verb-first** (`create instance`, `delete snapshot`) — aligns with how users think about actions; easier to discover
  - **Noun-first** (`instance create`, `instance delete`) — groups related subcommands together in help output; better for tools with many resource types (e.g. `kubectl`)
- Don't mix orderings — `myapp create instance` and `myapp snapshot delete` in the same tool is confusing

---

## Help Text

### Layout (follow this order)

The canonical order for `--help` output is: description → USAGE → EXAMPLES → OPTIONS. The "lead with examples" principle applies to *documentation pages and tutorials*, not to the `--help` flag layout — the usage line must come first so users understand the syntax before reading examples.

```
Short one-line description of what this does.

USAGE:
  myapp [flags] <required-arg>

EXAMPLES:
  myapp deploy --env production app.yaml
  myapp list --format json

OPTIONS:
  -h, --help             Show this help
  -v, --verbose          Show detailed output
      --dry-run          Preview changes without applying them
      --format string    Output format: text|json|yaml (default: text)
  -o, --output string    Write output to file instead of stdout

  Use 'myapp <subcommand> --help' for subcommand details.
  Full docs: https://example.com/docs
```

### Rules

- Show help on `-h`, `--help`, and `myapp help`
- Show *concise* help (usage + 1-2 examples) when run with no arguments **only if your tool cannot do anything useful without arguments**. If no-args has a sensible default (`git status`, `ls`, `myapp version`), do that instead — don't break it with a help screen
- Do not show help when stdin is being piped in; the tool should process it
- Lead with examples — users read examples before descriptions
- Bold headings (use ANSI or your library's formatter)
- Strip ANSI escapes when piped (detect TTY)
- Include a link to web docs and a support/issue URL
- Surface the 5–10 most common flags first; put obscure ones at the bottom

---

## Output

### stdout vs stderr

| Goes to stdout | Goes to stderr |
|---|---|
| Primary data output | Log messages, progress, warnings |
| Machine-readable results | Error messages |
| File content being transformed | Status/confirmation messages |

**Exception**: if `--format json` is set, the JSON goes to stdout even if it represents an error response — the caller is responsible for checking the payload.

### Output formats

Always support plain text (default). Add `--format json` for scripting. Consider `--format yaml`, `--format table`.

When stdout is a TTY: use color, progress bars, spinners, aligned columns.  
When piped: emit plain, parseable lines. Detect with `isatty(stdout)`.

```
--format text    Human-readable default; may include color/formatting when TTY
--format json    Machine-readable; always valid JSON; no ANSI escapes ever
--format yaml    Optional; mirrors JSON structure
--plain          Strips color and decoration from text output without changing
                 the format — use this alongside --format text when the user
                 wants grep/awk-safe output but NOT a structured format like JSON.
                 Do NOT treat --plain as a synonym for --format text; they are
                 different: --format text may still use color when TTY.
```

### Color control

Color must respect three signals, checked in this order:

1. `--color auto|always|never` flag (explicit user choice wins)
2. `NO_COLOR` environment variable — if set (to any value), disable all color output regardless of TTY. This is a [published standard](https://no-color.org/) respected by most modern CLIs.
3. TTY detection — enable color only when stdout is a TTY

Never emit ANSI escape codes when `--format json` or `--format yaml` is active, regardless of TTY state.

### Progress & verbosity

- Long-running ops (>2s) should show a spinner or progress bar
- Use `--verbose` / `-v` to expose internal steps
- Use `--quiet` / `-q` to suppress all non-error output
- Never print nothing for a successful destructive action — confirm what happened

---

## Error Messages

### Format

```
Error: <what went wrong>
  <why it happened>       ← optional; include when it adds clarity
  <what to do next>       ← optional; include when you can give actionable advice
```

Parts 2 and 3 are optional. A focused accurate error beats a padded one. For unexpected internal errors where you don't know the cause, omit the "why" and "next step" rather than writing something vague like "check the documentation". Use `--verbose` to expose the underlying error detail instead.

Example (user error — all three parts appropriate):
```
Error: cannot connect to database
  Connection refused at localhost:5432
  Check that the database is running: myapp db status
```

Example (internal error — concise is better):
```
Error: snapshot export failed
  Run with --verbose for details or open an issue at https://example.com/issues
```

### Rules

- Print errors to **stderr**
- Be specific — "authentication failed" > "request failed" > "error"
- Distinguish user errors (bad input) from system errors (network, permissions)
- Don't print stack traces to end users — use `--verbose` or a log file for that
- Suggest the next action wherever possible
- Typo/near-miss: `unknown command "stauts", did you mean "status"?`

---

## Exit Codes

| Code | Meaning |
|---|---|
| `0` | Success |
| `1` | General / unspecified error |
| `2` | Misuse of CLI (bad arguments, unknown flags) — a POSIX/bash convention; many tools use `1` instead. Adopt `2` for argument errors if your ecosystem supports it, but document it either way |
| `3–125` | Application-defined; document them |
| `126` | Command found but not executable (reserved by shell) |
| `127` | Command not found (reserved by shell) |
| `130` | Interrupted by Ctrl-C (SIGINT) |

Map your most important failure modes to distinct codes so scripts can branch on them. Document exit codes in `--help` or man pages.

---

## Interactive Prompts

Use prompts for:
- Missing required input that can't be inferred
- Confirming destructive actions
- Wizard-style onboarding

### Rules

- `--yes` / `-y` — confirms a specific destructive action ("yes, delete those resources"). Does not suppress all prompts globally.
- `--non-interactive` — signals the tool is running in a script. Skips all prompts; fails with a clear error if required input is missing rather than hanging. Use this in CI pipelines.
- Always provide both flags for scripting environments. `--yes` alone is not enough — it won't help if there are prompts for genuinely missing required inputs.
- Detect non-TTY stdin automatically as a hint to behave as if `--non-interactive` is set, but still fail clearly if input is missing
- Default values in square brackets: `Overwrite file? [y/N]`
- Uppercase letter = default: `[y/N]` means default is No; `[Y/n]` means default is Yes
- For destructive actions: default to No, require explicit `y`

```
# Good prompt pattern
? Delete 47 resources from production? This cannot be undone. [y/N]: 
```

---

## Config & Environment Precedence

Apply in this order (highest wins):

```
1. CLI flags          (--output json)
2. Environment vars   (MYAPP_OUTPUT=json)
3. Project config     (.myapp.yaml or myapp.toml in working dir)
4. User config        (~/.config/myapp/config.yaml)
5. System config      (/etc/myapp/config.yaml)
6. Compiled defaults
```

### Environment variables

- Prefix all vars: `MYAPP_TOKEN`, `MYAPP_LOG_LEVEL`
- Document every env var in `--help` or a dedicated `myapp env` subcommand
- Secrets (tokens, passwords) — support env var *and* file path (`MYAPP_TOKEN_FILE`)
- Never log or echo secrets; redact in `--verbose` output

### Config files

- Use a well-known format (YAML, TOML, JSON) — don't invent one
- Print the config file path in `--verbose` output so users know which one loaded
- Support `--config /path/to/file` to override location
- Provide `myapp config [get|set|list]` subcommands for editing

---

## Safe / Dry-Run Behavior

### Destructive operation checklist

1. **`--dry-run` / `-n`**: print exactly what *would* happen without doing it. Output should look identical to a real run except prefixed with `[dry-run]`. **If the dry-run encounters an error (invalid config, auth failure, conflict), exit non-zero and report it exactly as a real run would** — a clean dry-run should mean the real run has a good chance of succeeding.
2. **`--yes` / `-y`**: confirms a specific destructive action and skips its confirmation prompt. Does not suppress unrelated prompts for missing required inputs — use `--non-interactive` for that.
3. **`--non-interactive`**: suppress all prompts; fail with a clear error if required input is missing. Use in CI.
4. **`--force`**: override safety checks (e.g. delete non-empty bucket). Long-form only is fine if `-f` is taken. Never default to true.
5. **Confirmation prompt**: for ops deleting/overwriting data, always prompt unless `--yes` provided
6. **Describe before acting**: print a summary of planned changes before executing
7. **Rollback/undo guidance**: on failure mid-operation, explain what was done and what was not

### Example dry-run output

```
$ myapp deploy --dry-run
[dry-run] Would create: service/api (3 replicas)
[dry-run] Would update: configmap/settings
[dry-run] Would NOT affect: database (no schema changes)

Run without --dry-run to apply these changes.
```

---

## stdin / Piped Input

If your tool processes data (transforms, filters, queries), it should be composable via pipes.

- Accept `-` as a filename argument to mean stdin: `myapp process -` or `cat file.json | myapp process -`
- If a file argument is omitted and stdin is not a TTY, read from stdin automatically (the `cat` convention)
- If stdin is an interactive terminal and no file is given, either show help and exit, or print a prompt to stderr so the user knows the tool is waiting for input — never hang silently
- Detect with `isatty(stdin)` — do not prompt or show interactive UI when stdin is a pipe
- Write processed output to stdout so the result can be piped further

```bash
# All three should be equivalent for a well-behaved tool:
myapp process input.json
cat input.json | myapp process -
myapp process - < input.json
```

---

## Go + Cobra/Viper

> For detailed patterns and a full working template, see `references/cobra-viper.md`.

### Project layout

One file per subcommand under `cmd/`. `main.go` calls `cmd.Execute()` and nothing else.

### Always set these on rootCmd

```go
var rootCmd = &cobra.Command{
    SilenceErrors: true, // prevents cobra printing errors twice when RunE returns one
    SilenceUsage:  true, // prevents usage being dumped on every RunE error
    PersistentPreRunE: initRuntime,
}
```

Without `SilenceErrors`/`SilenceUsage`, cobra will print errors and usage text automatically, conflicting with your own formatted error output.

### Use `RunE`, never `Run`

`RunE` returns an error; `Run` does not. Always use `RunE` so errors propagate cleanly to `Execute()` and you can drive exit codes from them. Never call `os.Exit` inside a `RunE` function — return an error instead.

### The Cobra ↔ Viper integration contract

Three rules that must all hold together:

1. **In `init()`**: call `viper.BindPFlag("key", cmd.Flags().Lookup("flag-name"))` for every flag you want viper to manage
2. **In `PersistentPreRunE` on rootCmd**: set `viper.SetEnvPrefix`, call `viper.AutomaticEnv()`, and `viper.ReadInConfig()`. Also set `viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))` so `--my-flag` maps to `MYAPP_MY_FLAG`
3. **In `RunE`**: always read values via `viper.GetString("key")`, never from the flag variable — the flag variable is **not** updated by viper's precedence chain (env vars and config file values won't be visible through it)

### `PersistentPreRunE` inheritance trap

If a subcommand declares its own `PersistentPreRunE`, Cobra will **not** call the parent's. You must call `initRuntime(cmd, args)` explicitly from the subcommand's hook. Failing to do this means auth/config loading silently skips for that subcommand.

### Testability

Use `cmd.OutOrStdout()` and `cmd.ErrOrStderr()` instead of `os.Stdout`/`os.Stderr` everywhere. In tests, inject buffers with `rootCmd.SetOut(buf)`. Use a fresh `viper.New()` instance per test — the global singleton bleeds state between tests.

---

## Shell Completion

Provide shell completion for all major shells (bash, zsh, fish). Most argument-parsing libraries generate these automatically.

Subcommand: `myapp completion bash >> ~/.bashrc`

---

## AI Agent CLI Design

Some CLIs are designed primarily or exclusively for consumption by AI agents (LLMs, autonomous pipelines, CI orchestrators) rather than human users. These require a different set of defaults. **A CLI that serves both audiences needs to handle them explicitly** — the same output format cannot be optimal for both.

### Detecting agent mode

Provide an explicit signal rather than inferring from TTY state alone (a CI environment is not an agent):

```
--agent              Enable agent-optimised output and behaviour
MYAPP_AGENT=true     Environment variable equivalent (set by the orchestrator)
```

When agent mode is active, all other output-related defaults change as described below.

### Output: structured by default

In agent mode, **JSON must be the default format** — the agent cannot parse decorative text reliably.

- Every successful response is a valid JSON object on stdout. Never mix prose into stdout.
- Errors are also JSON, on stdout (not stderr), so the agent reads one stream:

```json
// Success
{"status": "ok", "data": { ... }, "request_id": "a1b2c3"}

// Error
{"status": "error", "error": {"code": "AUTH_FAILED", "message": "token expired"}, "request_id": "a1b2c3"}
```

- Use a stable `status` field (`"ok"` / `"error"`) as the primary branch point so agents don't need to parse error messages.
- Provide a machine-readable `code` string on errors (e.g. `"NOT_FOUND"`, `"RATE_LIMITED"`) — exit codes alone are insufficient for agents that parse output rather than branching on shell `$?`.

### No interactive prompts, ever

Agent mode must never block waiting for input. If required input is missing:

- Do not prompt
- Emit a structured JSON error to stdout
- Exit non-zero immediately

Agents cannot type. A hung process is an agent deadlock.

### Suppress all decorative output

In agent mode disable unconditionally:

- ANSI color and bold/italic escapes
- Spinners and progress bars (they corrupt stdout JSON)
- Pagination (never pipe through a pager)
- "Using config file: ..." and similar informational stderr lines (use `--verbose` explicitly if the orchestrator wants them)

Stderr should be silent in agent mode unless something genuinely unexpected has happened that the orchestrator should log.

### Operation and correlation IDs

Include a `request_id` in every response (generated per invocation). Accept a `--request-id` / `MYAPP_REQUEST_ID` input so orchestrators can inject their own trace ID for end-to-end correlation across multi-step pipelines.

### Stable output schemas

An agent's parser is brittle — field renames or structural changes break it silently. Treat your JSON output schema as a versioned API contract:

- Document the schema for every command's output
- Add fields freely; never remove or rename existing fields without a major version bump
- Include a `schema_version` field in output so agents can detect and handle changes
- Provide `myapp <subcommand> --output json-schema` to emit the JSON Schema for that command's output, enabling agents to validate responses and understand structure without reading docs

### JSONL for streaming / long-running operations

For commands that produce multiple results or run asynchronously, emit newline-delimited JSON (JSONL) — one complete JSON object per line. Agents can `readline()` and process incrementally without waiting for the command to finish:

```json lines
{"type": "progress", "step": 1, "total": 5, "message": "Creating network"}
{"type": "progress", "step": 2, "total": 5, "message": "Provisioning instance"}
{"type": "result",   "status": "ok", "data": {"id": "inst-xyz"}}
```

Always emit the terminal `result` object last. The agent looks for `"type": "result"` to know the command completed.

### Idempotency

Operations that modify state should be idempotent where possible (applying the same command twice produces the same result, not an error). Agents retry on transient failure — a non-idempotent command creates duplicates or inconsistent state on retry.

### `--timeout` flag

Agents operate within bounded time windows. Provide a `--timeout duration` flag (e.g. `--timeout 30s`) and respect it. Exit with a specific exit code (e.g. `124`) and structured error on timeout so the orchestrator can distinguish a timeout from a logic failure.

### Discoverability

Agents exploring a CLI for the first time need machine-readable discovery:

- `myapp --help --format json` (or `myapp commands --format json`) should emit a JSON listing of all available subcommands with their descriptions and flag schemas
- This allows an agent to introspect the tool and construct valid invocations without reading documentation

---

## Common Anti-Patterns to Avoid

- **Silent success on destructive action** — always confirm what was done
- **Mutable default args** — if the default behaviour destroys data, require an explicit flag
- **Positional args for optional things** — use flags for anything optional
- **Cryptic exit codes** — document non-zero codes
- **Printing to stdout AND stderr mixed** in machine-readable mode
- **No `--dry-run`** for ops that modify remote state
- **Env vars without a `--` flag equivalent** — always pair them
- **Prompting in a pipe** — detect TTY; never hang waiting for stdin in a script
- **`--no-X` on flags that default to false** — don't generate `--no-force` or `--no-dry-run`; only provide `--no-X` when the default is true
- **`-o` used for both file path and format selector** — keep `--output`/`-o` for file path; use `--format` for format selection
- **`-f` assigned to two things** — if your tool needs both `--file` and `--force`, assign `-f` to one only and leave the other long-form
- **Ignoring `NO_COLOR`** — honour it unconditionally; don't only rely on TTY detection
- **Using `--yes` to suppress all prompts in CI** — use `--non-interactive` for that; `--yes` only confirms specific actions
- **Dry-run that ignores errors** — a dry-run that exits 0 despite encountering auth/config errors is misleading; fail as a real run would
- *(Agent CLIs)* **Mixing prose into stdout** — in agent mode stdout is a data channel; all human-readable status text goes to stderr or is suppressed
- *(Agent CLIs)* **Sending errors to stderr only** — agents may only read stdout; structured errors must appear on stdout in agent mode
- *(Agent CLIs)* **Blocking on stdin in agent mode** — agents cannot type; a missing required input must fail immediately with a structured error
- *(Agent CLIs)* **Unstable field names** — renaming or removing JSON output fields without a version bump breaks every agent silently
- *(Agent CLIs)* **Reading flag variable instead of `viper.GetString()`** — in Cobra+Viper, flag variables are not updated by the viper precedence chain; always use viper getters in `RunE`
- *(Agent CLIs)* **Global `viper` singleton in tests** — use `viper.New()` per test to prevent state leaking between test cases

---

## Reference Files

See `references/` for extended guidance — load only the file relevant to the current task:

- `references/cobra-viper.md` — Full working Go patterns: root command template, flag binding, `PersistentPreRunE` inheritance, error/exit code wiring, testability setup
- `references/flag-naming.md` — Extended flag naming patterns and conventions
- `references/exit-code-registry.md` — Conventional exit code registry by domain
