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
go build -o bin/neo4j-cli ./cmd/neo4j-cli
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
