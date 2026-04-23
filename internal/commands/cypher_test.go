package commands_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/cli/go-cli-tool/internal/commands"
	"github.com/cli/go-cli-tool/internal/shell"
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

// ---- helpers ------------------------------------------------------------

func cypherCtx(t *testing.T) shell.ShellContext {
	t.Helper()
	return shell.ShellContext{Context: context.Background()}
}

// ---- BuildCypherCategory tests ------------------------------------------

func TestCypherCategory_NoArgs_ReturnsUsageError(t *testing.T) {
	svc := &mockCypherService{}
	cat := commands.BuildCypherCategory(svc)

	_, err := cat.Dispatch(nil, cypherCtx(t))
	if err == nil {
		t.Fatal("expected usage error when no args provided")
	}
	if !strings.Contains(err.Error(), "usage") {
		t.Errorf("error should mention usage, got: %v", err)
	}
}

func TestCypherCategory_ExecutesQuery(t *testing.T) {
	svc := &mockCypherService{result: "row1, row2"}
	cat := commands.BuildCypherCategory(svc)

	out, err := cat.Dispatch([]string{"MATCH", "(n)", "RETURN", "n"}, cypherCtx(t))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if svc.lastQry != "MATCH (n) RETURN n" {
		t.Errorf("query sent to service: got %q, want %q", svc.lastQry, "MATCH (n) RETURN n")
	}
	if out != "row1, row2" {
		t.Errorf("output: got %q, want %q", out, "row1, row2")
	}
}

func TestCypherCategory_ServiceError_Propagated(t *testing.T) {
	svcErr := errors.New("connection refused")
	svc := &mockCypherService{err: svcErr}
	cat := commands.BuildCypherCategory(svc)

	_, err := cat.Dispatch([]string{"MATCH", "(n)", "RETURN", "n"}, cypherCtx(t))
	if err == nil {
		t.Fatal("expected service error to be propagated")
	}
	if !errors.Is(err, svcErr) {
		t.Errorf("expected wrapped service error; got: %v", err)
	}
}

func TestCypherCategory_ContextPropagated(t *testing.T) {
	type key struct{}
	ctx := context.WithValue(context.Background(), key{}, "sentinel")
	sc := shell.ShellContext{Context: ctx}

	svc := &mockCypherService{result: "ok"}
	cat := commands.BuildCypherCategory(svc)

	_, err := cat.Dispatch([]string{"MATCH", "(n)", "RETURN", "n"}, sc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if svc.lastCtx == nil || svc.lastCtx.Value(key{}) != "sentinel" {
		t.Error("context not propagated to service")
	}
}

func TestCypherCategory_MultiWordQuery_Joined(t *testing.T) {
	svc := &mockCypherService{result: "ok"}
	cat := commands.BuildCypherCategory(svc)

	_, _ = cat.Dispatch([]string{"MATCH", "(n:Person)", "RETURN", "n", "LIMIT", "5"}, cypherCtx(t))

	want := "MATCH (n:Person) RETURN n LIMIT 5"
	if svc.lastQry != want {
		t.Errorf("joined query: got %q, want %q", svc.lastQry, want)
	}
}
