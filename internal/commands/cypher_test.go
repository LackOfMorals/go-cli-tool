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
	result     service.QueryResult
	err        error
	lastCtx    context.Context
	lastQuery  string
	lastParams map[string]interface{}
}

func (m *mockCypherService) Execute(ctx context.Context, query string, params map[string]interface{}) (service.QueryResult, error) {
	m.lastCtx = ctx
	m.lastQuery = query
	m.lastParams = params
	return m.result, m.err
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

	out, err := cat.Dispatch([]string{"MATCH", "(n)", "RETURN", "n.name,", "n.age"}, cypherCtx(t))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
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

	out, err := cat.Dispatch([]string{"MATCH", "(n)", "RETURN", "n"}, ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
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

	out, err := cat.Dispatch([]string{"--format", "graph", "MATCH", "(n)", "RETURN", "n"}, ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Graph format uses "○" bullet points; table uses "│"
	if strings.Contains(out, "│") && !strings.Contains(out, "○") {
		t.Errorf("expected graph format output, got table output:\n%s", out)
	}
}

func TestCypherCategory_NoResults_ReturnsMessage(t *testing.T) {
	svc := &mockCypherService{result: service.QueryResult{}}
	cat := commands.BuildCypherCategory(svc)

	out, err := cat.Dispatch([]string{"MATCH", "(n)", "RETURN", "n"}, cypherCtx(t))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
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
