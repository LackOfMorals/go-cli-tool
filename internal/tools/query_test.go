package tools_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/cli/go-cli-tool/internal/service"
	"github.com/cli/go-cli-tool/internal/tool"
	"github.com/cli/go-cli-tool/internal/tools"
)

// ---- mockCypherService --------------------------------------------------

type mockCypherService struct {
	result  service.QueryResult
	err     error
	lastCtx context.Context
	lastQry string
}

func (m *mockCypherService) Execute(ctx context.Context, query string, _ map[string]interface{}) (service.QueryResult, error) {
	m.lastCtx = ctx
	m.lastQry = query
	return m.result, m.err
}

// singleCol builds a one-column QueryResult for simple test fixtures.
func singleCol(val string) service.QueryResult {
	return service.QueryResult{
		Columns: []string{"result"},
		Rows:    []service.QueryRow{{"result": val}},
	}
}

// ---- QueryTool tests ----------------------------------------------------

func TestQueryTool_NoArgs_ReturnsError(t *testing.T) {
	svc := &mockCypherService{}
	qt := tools.NewQueryTool(svc)

	ctx := tool.NewContext()
	_, err := qt.Execute(*ctx)
	if err == nil {
		t.Fatal("expected error when no args provided")
	}
}

func TestQueryTool_ExecutesQuery(t *testing.T) {
	svc := &mockCypherService{result: singleCol("node1")}
	qt := tools.NewQueryTool(svc)

	ctx := tool.NewContext().WithArgs([]string{"MATCH (n) RETURN n"})
	result, err := qt.Execute(*ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Error("expected successful result")
	}
	if !strings.Contains(result.Output, "node1") {
		t.Errorf("output should contain 'node1'; got: %q", result.Output)
	}
}

func TestQueryTool_ServiceError_Propagated(t *testing.T) {
	svcErr := errors.New("query failed")
	svc := &mockCypherService{err: svcErr}
	qt := tools.NewQueryTool(svc)

	ctx := tool.NewContext().WithArgs([]string{"MATCH (n) RETURN n"})
	_, err := qt.Execute(*ctx)
	if err == nil {
		t.Fatal("expected service error to be propagated")
	}
	if !errors.Is(err, svcErr) {
		t.Errorf("expected service error in chain; got: %v", err)
	}
}

func TestQueryTool_ContextPropagated(t *testing.T) {
	type key struct{}
	cmdCtx := context.WithValue(context.Background(), key{}, "sentinel")

	svc := &mockCypherService{result: singleCol("ok")}
	qt := tools.NewQueryTool(svc)

	ctx := tool.NewContext().
		WithContext(cmdCtx).
		WithArgs([]string{"MATCH (n) RETURN n"})

	_, err := qt.Execute(*ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if svc.lastCtx == nil || svc.lastCtx.Value(key{}) != "sentinel" {
		t.Error("context not propagated to cypher service")
	}
}

func TestQueryTool_OnlyFirstArgUsedAsQuery(t *testing.T) {
	svc := &mockCypherService{result: singleCol("ok")}
	qt := tools.NewQueryTool(svc)

	ctx := tool.NewContext().WithArgs([]string{"MATCH (n) RETURN n", "extra"})
	_, _ = qt.Execute(*ctx)

	if svc.lastQry != "MATCH (n) RETURN n LIMIT 100" {
		t.Errorf("service received %q, expected first arg with LIMIT injected", svc.lastQry)
	}
}

func TestQueryTool_Name(t *testing.T) {
	qt := tools.NewQueryTool(&mockCypherService{})
	if qt.Name() != "query" {
		t.Errorf("Name: got %q, want %q", qt.Name(), "query")
	}
}
