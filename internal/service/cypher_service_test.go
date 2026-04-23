package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/cli/go-cli-tool/internal/service"
)

// ---- mockGraphRepository ------------------------------------------------

type mockGraphRepository struct {
	executeFunc func(ctx context.Context, cypher string, params map[string]interface{}) (interface{}, error)
}

func (m *mockGraphRepository) ExecuteQuery(ctx context.Context, cypher string, params map[string]interface{}) (interface{}, error) {
	return m.executeFunc(ctx, cypher, params)
}

func (m *mockGraphRepository) Close() error { return nil }

// ---- CypherService tests ------------------------------------------------

func TestCypherService_EmptyQuery_ReturnsError(t *testing.T) {
	repo := &mockGraphRepository{
		executeFunc: func(_ context.Context, _ string, _ map[string]interface{}) (interface{}, error) {
			t.Error("repo should not be called for empty query")
			return nil, nil
		},
	}
	svc := service.NewCypherService(repo)
	_, err := svc.Execute(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty query")
	}
}

func TestCypherService_ExecutesCypher(t *testing.T) {
	const query = "MATCH (n) RETURN n LIMIT 1"
	var gotQuery string

	repo := &mockGraphRepository{
		executeFunc: func(_ context.Context, cypher string, _ map[string]interface{}) (interface{}, error) {
			gotQuery = cypher
			return "row1", nil
		},
	}
	svc := service.NewCypherService(repo)
	out, err := svc.Execute(context.Background(), query)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotQuery != query {
		t.Errorf("repo received %q, want %q", gotQuery, query)
	}
	if out == "" {
		t.Error("expected non-empty output")
	}
}

func TestCypherService_RepoError_Propagated(t *testing.T) {
	repoErr := errors.New("connection refused")
	repo := &mockGraphRepository{
		executeFunc: func(_ context.Context, _ string, _ map[string]interface{}) (interface{}, error) {
			return nil, repoErr
		},
	}
	svc := service.NewCypherService(repo)
	_, err := svc.Execute(context.Background(), "MATCH (n) RETURN n")
	if err == nil {
		t.Fatal("expected error from repo")
	}
	if !errors.Is(err, repoErr) {
		t.Errorf("error chain should contain repo error; got: %v", err)
	}
}

func TestCypherService_ContextPropagated(t *testing.T) {
	type key struct{}
	ctx := context.WithValue(context.Background(), key{}, "sentinel")
	var gotCtx context.Context

	repo := &mockGraphRepository{
		executeFunc: func(c context.Context, _ string, _ map[string]interface{}) (interface{}, error) {
			gotCtx = c
			return "ok", nil
		},
	}
	svc := service.NewCypherService(repo)
	_, err := svc.Execute(ctx, "MATCH (n) RETURN n")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotCtx == nil || gotCtx.Value(key{}) != "sentinel" {
		t.Error("context was not propagated to repository")
	}
}

func TestCypherService_ResultFormattedAsString(t *testing.T) {
	repo := &mockGraphRepository{
		executeFunc: func(_ context.Context, _ string, _ map[string]interface{}) (interface{}, error) {
			return 42, nil // non-string result
		},
	}
	svc := service.NewCypherService(repo)
	out, err := svc.Execute(context.Background(), "RETURN 42")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != "42" {
		t.Errorf("got %q, want %q", out, "42")
	}
}
