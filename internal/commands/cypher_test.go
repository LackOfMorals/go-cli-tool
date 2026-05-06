package commands_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/cli/go-cli-tool/internal/commands"
	"github.com/cli/go-cli-tool/internal/config"
	"github.com/cli/go-cli-tool/internal/logger"
	"github.com/cli/go-cli-tool/internal/presentation"
	"github.com/cli/go-cli-tool/internal/service"
	"github.com/cli/go-cli-tool/internal/dispatch"
)

// ---- mockCypherService --------------------------------------------------

type mockCypherService struct {
	result      service.QueryResult
	err         error
	explainType string // returned by Explain; defaults to "r"
	explainErr  error
	lastCtx     context.Context
	lastQuery   string
	lastParams  map[string]interface{}
}

func (m *mockCypherService) Execute(ctx context.Context, query string, params map[string]interface{}) (service.QueryResult, error) {
	m.lastCtx = ctx
	m.lastQuery = query
	m.lastParams = params
	return m.result, m.err
}

func (m *mockCypherService) Explain(_ context.Context, _ string) (string, error) {
	if m.explainErr != nil {
		return "", m.explainErr
	}
	if m.explainType != "" {
		return m.explainType, nil
	}
	return "r", nil // safe default
}

// ---- helpers ------------------------------------------------------------

func cypherCtx(t *testing.T) dispatch.Context {
	t.Helper()
	log := logger.NewLoggerService(logger.FormatText, logger.LevelError)
	pres, err := presentation.NewPresentationService(presentation.OutputFormatTable, log)
	if err != nil {
		t.Fatalf("NewPresentationService: %v", err)
	}
	return dispatch.Context{
		Context: context.Background(),
		Config: config.Config{
			Cypher: config.CypherConfig{
				ShellLimit:   25,
				OutputFormat: "table",
			},
		},
		IO:        &mockIO{},
		Presenter: pres,
	}
}

func singleRowResult(col, val string) service.QueryResult {
	return service.QueryResult{
		Columns: []string{col},
		Rows:    []service.QueryRow{{col: val}},
	}
}

func multiColResult(cols []string, rows []service.QueryRow) service.QueryResult {
	return service.QueryResult{Columns: cols, Rows: rows}
}

// ---- Basic dispatch -----------------------------------------------------

func TestCypherCategory_NoArgs_PromptsForStatement_EmptyInputReturnsError(t *testing.T) {
	// mockIO with no queued lines returns "" from Read(), which the prompt
	// treats as "no statement provided".
	svc := &mockCypherService{}
	cat := commands.BuildCypherCategory(svc)

	_, err := cat.Dispatch(nil, cypherCtx(t))
	if err == nil {
		t.Fatal("expected error when no cypher statement entered")
	}
	if !strings.Contains(err.Error(), "required") {
		t.Errorf("error should mention required; got: %v", err)
	}
}

func TestCypherCategory_NoArgs_PromptsForStatement_ExecutesWhenProvided(t *testing.T) {
	svc := &mockCypherService{result: singleRowResult("n", "Alice")}
	cat := commands.BuildCypherCategory(svc)

	// Queue the statement in the IO mock; leave parameter line blank.
	ctx := cypherCtx(t)
	ctx.IO = &mockIO{readLines: []string{"MATCH (n) RETURN n", ""}}

	_, err := cat.Dispatch(nil, ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(svc.lastQuery, "MATCH (n) RETURN n") {
		t.Errorf("service not called with prompted statement; got: %q", svc.lastQuery)
	}
}

func TestCypherCategory_NoArgs_PromptsForStatementAndParams(t *testing.T) {
	svc := &mockCypherService{result: singleRowResult("n", "x")}
	cat := commands.BuildCypherCategory(svc)

	ctx := cypherCtx(t)
	ctx.IO = &mockIO{readLines: []string{
		"MATCH (n:Person {name:$name}) RETURN n;",
		"name=Alice age=30",
	}}

	_, err := cat.Dispatch(nil, ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if svc.lastParams["name"] != "Alice" {
		t.Errorf("expected param name=Alice; got: %v", svc.lastParams)
	}
	if svc.lastParams["age"] != int64(30) {
		t.Errorf("expected param age=30; got: %v", svc.lastParams["age"])
	}
}

func TestCypherCategory_ExecutesQuery(t *testing.T) {
	svc := &mockCypherService{result: singleRowResult("n", "Alice")}
	cat := commands.BuildCypherCategory(svc)

	_, err := cat.Dispatch([]string{"MATCH", "(n)", "RETURN", "n"}, cypherCtx(t))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// The query has RETURN so a LIMIT should be injected.
	if !strings.Contains(svc.lastQuery, "MATCH (n) RETURN n") {
		t.Errorf("query not forwarded correctly; got: %q", svc.lastQuery)
	}
	if !strings.Contains(svc.lastQuery, "LIMIT") {
		t.Errorf("LIMIT not injected; got: %q", svc.lastQuery)
	}
}

func TestCypherCategory_ServiceError_Propagated(t *testing.T) {
	svcErr := errors.New("connection refused")
	svc := &mockCypherService{err: svcErr}
	cat := commands.BuildCypherCategory(svc)

	_, err := cat.Dispatch([]string{"MATCH", "(n)", "RETURN", "n"}, cypherCtx(t))
	if !errors.Is(err, svcErr) {
		t.Errorf("expected wrapped service error; got: %v", err)
	}
}

func TestCypherCategory_ContextPropagated(t *testing.T) {
	type key struct{}
	ctx := context.WithValue(context.Background(), key{}, "sentinel")
	sc := cypherCtx(t)
	sc.Context = ctx

	svc := &mockCypherService{result: singleRowResult("x", "1")}
	cat := commands.BuildCypherCategory(svc)

	_, err := cat.Dispatch([]string{"MATCH", "(n)", "RETURN", "n"}, sc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if svc.lastCtx == nil || svc.lastCtx.Value(key{}) != "sentinel" {
		t.Error("context not propagated to service")
	}
}

// ---- LIMIT injection ----------------------------------------------------

func TestCypherCategory_InjectsShellLimit(t *testing.T) {
	svc := &mockCypherService{result: singleRowResult("n", "x")}
	cat := commands.BuildCypherCategory(svc)
	ctx := cypherCtx(t)
	ctx.Config.Cypher.ShellLimit = 42

	_, err := cat.Dispatch([]string{"MATCH", "(n)", "RETURN", "n"}, ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(svc.lastQuery, "LIMIT 42") {
		t.Errorf("expected LIMIT 42 in query; got: %q", svc.lastQuery)
	}
}

func TestCypherCategory_DoesNotInjectLimit_WhenAlreadyPresent(t *testing.T) {
	svc := &mockCypherService{result: singleRowResult("n", "x")}
	cat := commands.BuildCypherCategory(svc)

	_, err := cat.Dispatch([]string{"MATCH", "(n)", "RETURN", "n", "LIMIT", "10"}, cypherCtx(t))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should not duplicate the LIMIT.
	count := strings.Count(strings.ToUpper(svc.lastQuery), "LIMIT")
	if count != 1 {
		t.Errorf("expected exactly one LIMIT clause; got query: %q", svc.lastQuery)
	}
}

func TestCypherCategory_DoesNotInjectLimit_ForWriteQuery(t *testing.T) {
	svc := &mockCypherService{result: service.QueryResult{}}
	cat := commands.BuildCypherCategory(svc)

	_, _ = cat.Dispatch([]string{"CREATE", "(n:Person", "{name:'Alice'})"}, cypherCtx(t))
	if strings.Contains(strings.ToUpper(svc.lastQuery), "LIMIT") {
		t.Errorf("LIMIT should not be injected for write queries; got: %q", svc.lastQuery)
	}
}

func TestCypherCategory_LimitFlag_OverridesConfig(t *testing.T) {
	svc := &mockCypherService{result: singleRowResult("n", "x")}
	cat := commands.BuildCypherCategory(svc)

	_, err := cat.Dispatch([]string{"--limit", "7", "MATCH", "(n)", "RETURN", "n"}, cypherCtx(t))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(svc.lastQuery, "LIMIT 7") {
		t.Errorf("expected LIMIT 7 in query; got: %q", svc.lastQuery)
	}
}

// ---- Param parsing ------------------------------------------------------

func TestCypherCategory_ParamPassed_ToService(t *testing.T) {
	svc := &mockCypherService{result: singleRowResult("n", "Alice")}
	cat := commands.BuildCypherCategory(svc)

	_, err := cat.Dispatch(
		[]string{"--param", "name=Alice", "MATCH", "(n", "{name:$name})", "RETURN", "n"},
		cypherCtx(t),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if svc.lastParams == nil || svc.lastParams["name"] != "Alice" {
		t.Errorf("expected param name=Alice; got: %v", svc.lastParams)
	}
}

func TestCypherCategory_MultipleParams(t *testing.T) {
	svc := &mockCypherService{result: singleRowResult("n", "x")}
	cat := commands.BuildCypherCategory(svc)

	_, err := cat.Dispatch(
		[]string{"--param", "name=Bob", "--param", "age=30", "MATCH", "(n)", "RETURN", "n"},
		cypherCtx(t),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if svc.lastParams["name"] != "Bob" {
		t.Errorf("expected param name=Bob; got: %v", svc.lastParams)
	}
	// age=30 should be coerced to int64
	if svc.lastParams["age"] != int64(30) {
		t.Errorf("expected param age=30 (int64); got: %T %v", svc.lastParams["age"], svc.lastParams["age"])
	}
}

// ---- Output formatting --------------------------------------------------

func TestCypherCategory_TableFormat_ContainsColumns(t *testing.T) {
	svc := &mockCypherService{
		result: multiColResult(
			[]string{"name", "age"},
			[]service.QueryRow{
				{"name": "Alice", "age": 30},
				{"name": "Bob", "age": 25},
			},
		),
	}
	cat := commands.BuildCypherCategory(svc)
	ctx := cypherCtx(t)

	result, err := cat.Dispatch([]string{"MATCH", "(n)", "RETURN", "n.name,", "n.age"}, ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// JSON channel: Items should contain both rows
	if len(result.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(result.Items))
	}
	// Human channel: rendered table should contain columns and values
	out, fmtErr := ctx.Presenter.FormatAs(result.Presentation, result.FormatOverride)
	if fmtErr != nil {
		t.Fatalf("FormatAs error: %v", fmtErr)
	}
	for _, want := range []string{"name", "age", "Alice", "Bob", "30", "25"} {
		if !strings.Contains(out, want) {
			t.Errorf("table output should contain %q:\n%s", want, out)
		}
	}
}

func TestCypherCategory_GraphFormat_ContainsValues(t *testing.T) {
	svc := &mockCypherService{
		result: multiColResult(
			[]string{"name", "age"},
			[]service.QueryRow{{"name": "Alice", "age": 30}},
		),
	}
	cat := commands.BuildCypherCategory(svc)
	ctx := cypherCtx(t)
	ctx.Config.Cypher.OutputFormat = "graph"

	result, err := cat.Dispatch([]string{"MATCH", "(n)", "RETURN", "n"}, ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out, fmtErr := ctx.Presenter.FormatAs(result.Presentation, result.FormatOverride)
	if fmtErr != nil {
		t.Fatalf("FormatAs error: %v", fmtErr)
	}
	if !strings.Contains(out, "Alice") {
		t.Errorf("graph output should contain value Alice:\n%s", out)
	}
}

func TestCypherCategory_FormatFlag_OverridesConfig(t *testing.T) {
	svc := &mockCypherService{result: singleRowResult("name", "Charlie")}
	cat := commands.BuildCypherCategory(svc)
	ctx := cypherCtx(t)
	ctx.Config.Cypher.OutputFormat = "table" // config says table

	result, err := cat.Dispatch([]string{"--format", "graph", "MATCH", "(n)", "RETURN", "n"}, ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out, fmtErr := ctx.Presenter.FormatAs(result.Presentation, result.FormatOverride)
	if fmtErr != nil {
		t.Fatalf("FormatAs error: %v", fmtErr)
	}
	// Graph format uses "○" bullet points; table uses "│"
	if strings.Contains(out, "│") && !strings.Contains(out, "○") {
		t.Errorf("expected graph format output, got table output:\n%s", out)
	}
}

func TestCypherCategory_FormatFlag_TOON(t *testing.T) {
	svc := &mockCypherService{result: singleRowResult("name", "Charlie")}
	cat := commands.BuildCypherCategory(svc)
	ctx := cypherCtx(t)
	ctx.Config.Cypher.OutputFormat = "table" // config says table

	result, err := cat.Dispatch([]string{"--format", "toon", "MATCH", "(n)", "RETURN", "n"}, ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.FormatOverride != presentation.OutputFormatTOON {
		t.Errorf("expected FormatOverride=%q; got: %q",
			presentation.OutputFormatTOON, result.FormatOverride)
	}
	out, fmtErr := ctx.Presenter.FormatAs(result.Presentation, result.FormatOverride)
	if fmtErr != nil {
		t.Fatalf("FormatAs error: %v", fmtErr)
	}
	// TOON output for a single row should at least mention the column name and value.
	for _, want := range []string{"name", "Charlie"} {
		if !strings.Contains(out, want) {
			t.Errorf("toon output should contain %q:\n%s", want, out)
		}
	}
	// Distinguish from table output, which uses box-drawing characters.
	if strings.Contains(out, "│") {
		t.Errorf("expected toon output, got table output:\n%s", out)
	}
}

func TestCypherCategory_FormatFlag_TOON_CaseInsensitive(t *testing.T) {
	svc := &mockCypherService{result: singleRowResult("name", "Charlie")}
	cat := commands.BuildCypherCategory(svc)
	ctx := cypherCtx(t)

	result, err := cat.Dispatch([]string{"--format", "TOON", "MATCH", "(n)", "RETURN", "n"}, ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.FormatOverride != presentation.OutputFormatTOON {
		t.Errorf("expected FormatOverride=%q (lower-cased); got: %q",
			presentation.OutputFormatTOON, result.FormatOverride)
	}
}

func TestCypherCategory_NoResults_ReturnsMessage(t *testing.T) {
	svc := &mockCypherService{result: service.QueryResult{}}
	cat := commands.BuildCypherCategory(svc)
	ctx := cypherCtx(t)

	result, err := cat.Dispatch([]string{"MATCH", "(n)", "RETURN", "n"}, ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Items) != 0 {
		t.Errorf("expected 0 items for empty result, got %d", len(result.Items))
	}
	out, fmtErr := ctx.Presenter.FormatAs(result.Presentation, result.FormatOverride)
	if fmtErr != nil {
		t.Fatalf("FormatAs error: %v", fmtErr)
	}
	if !strings.Contains(out, "no results") {
		t.Errorf("expected no-results message; got: %q", out)
	}
}

func TestCypherCategory_MultiWordQuery_Joined(t *testing.T) {
	svc := &mockCypherService{result: singleRowResult("n", "ok")}
	cat := commands.BuildCypherCategory(svc)

	_, _ = cat.Dispatch([]string{"MATCH", "(n:Person)", "RETURN", "n", "LIMIT", "5"}, cypherCtx(t))

	if !strings.Contains(svc.lastQuery, "MATCH (n:Person) RETURN n LIMIT 5") {
		t.Errorf("query not joined correctly; got: %q", svc.lastQuery)
	}
}

// ---- Agent mode ---------------------------------------------------------

func agentCtx(t *testing.T, allowWrites bool) dispatch.Context {
	t.Helper()
	ctx := cypherCtx(t)
	ctx.AgentMode = true
	ctx.AllowWrites = allowWrites
	return ctx
}

func TestCypherCategory_AgentMode_NoQuery_ReturnsError(t *testing.T) {
	svc := &mockCypherService{}
	cat := commands.BuildCypherCategory(svc)

	_, err := cat.Dispatch(nil, agentCtx(t, false))
	if err == nil {
		t.Fatal("expected error when no query in agent mode")
	}
	if !strings.Contains(err.Error(), "agent mode") {
		t.Errorf("error should mention agent mode; got: %v", err)
	}
}

func TestCypherCategory_AgentMode_ReadQuery_Executes(t *testing.T) {
	svc := &mockCypherService{
		result:      singleRowResult("n", "Alice"),
		explainType: "r", // EXPLAIN reports read-only
	}
	cat := commands.BuildCypherCategory(svc)

	_, err := cat.Dispatch([]string{"MATCH", "(n)", "RETURN", "n"}, agentCtx(t, false))
	if err != nil {
		t.Fatalf("read query should succeed in agent mode without --rw: %v", err)
	}
}

func TestCypherCategory_AgentMode_WriteQuery_Blocked(t *testing.T) {
	svc := &mockCypherService{
		explainType: "rw", // EXPLAIN reports read/write
	}
	cat := commands.BuildCypherCategory(svc)

	_, err := cat.Dispatch([]string{"CREATE", "(n:Person)", "RETURN", "n"}, agentCtx(t, false))
	if err == nil {
		t.Fatal("expected WRITE_BLOCKED error in agent mode without --rw")
	}
	if !strings.Contains(err.Error(), "rw") && !strings.Contains(err.Error(), "write") {
		t.Errorf("error should mention write blocked; got: %v", err)
	}
}

func TestCypherCategory_AgentMode_WriteQuery_AllowedWith_RW(t *testing.T) {
	svc := &mockCypherService{
		result:      service.QueryResult{},
		explainType: "rw",
	}
	cat := commands.BuildCypherCategory(svc)

	// With --rw the EXPLAIN check is skipped and Execute is called directly.
	_, err := cat.Dispatch([]string{"CREATE", "(n:Person)", "RETURN", "n"}, agentCtx(t, true))
	if err != nil {
		t.Fatalf("write query should succeed in agent mode with --rw: %v", err)
	}
}

func TestCypherCategory_AgentMode_ExplainQuery_RunsDirectly(t *testing.T) {
	// Queries that already start with EXPLAIN should run without a pre-check.
	svc := &mockCypherService{result: singleRowResult("plan", "ok")}
	cat := commands.BuildCypherCategory(svc)

	_, err := cat.Dispatch([]string{"EXPLAIN", "MATCH", "(n)", "RETURN", "n"}, agentCtx(t, false))
	if err != nil {
		t.Fatalf("EXPLAIN query should run directly in agent mode: %v", err)
	}
	// Verify Execute was called with the EXPLAIN prefix intact.
	if !strings.HasPrefix(svc.lastQuery, "EXPLAIN") {
		t.Errorf("expected EXPLAIN query to be forwarded as-is; got: %q", svc.lastQuery)
	}
}
