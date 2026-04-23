package tools_test

import (
	"context"
	"errors"
	"testing"

	"github.com/cli/go-cli-tool/internal/tool"
	"github.com/cli/go-cli-tool/internal/tools"
)

// ---- mockCypherService --------------------------------------------------

type mockCypherService struct {
	result  string
	err     error
	lastCtx context.Context
	lastQry string
}

func (m *mockCypherService) Execute(ctx context.Context, query string) (string, error) {
	m.lastCtx = ctx
	m.lastQry = query
	return m.result, m.err
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
	svc := &mockCypherService{result: "node1"}
	qt := tools.NewQueryTool(svc)

	ctx := tool.NewContext().WithArgs([]string{"MATCH (n) RETURN n"})
	result, err := qt.Execute(*ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Error("expected successful result")
	}
	if result.Output != "node1" {
		t.Errorf("output: got %q, want %q", result.Output, "node1")
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

	svc := &mockCypherService{result: "ok"}
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
	svc := &mockCypherService{result: "ok"}
	qt := tools.NewQueryTool(svc)

	// QueryTool is designed for --exec mode where the full query is one arg.
	// Only the first arg should be sent to the service.
	ctx := tool.NewContext().WithArgs([]string{"MATCH (n) RETURN n", "extra"})
	_, _ = qt.Execute(*ctx)

	if svc.lastQry != "MATCH (n) RETURN n" {
		t.Errorf("service received %q, expected only the first arg", svc.lastQry)
	}
}

func TestQueryTool_Name(t *testing.T) {
	qt := tools.NewQueryTool(&mockCypherService{})
	if qt.Name() != "query" {
		t.Errorf("Name: got %q, want %q", qt.Name(), "query")
	}
}
