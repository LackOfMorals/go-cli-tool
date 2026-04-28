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
	_, err := svc.Execute(context.Background(), "", nil)
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
	result, err := svc.Execute(context.Background(), query, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotQuery != query {
		t.Errorf("repo received %q, want %q", gotQuery, query)
	}
	if len(result.Rows) == 0 {
		t.Error("expected at least one result row")
	}
}

func TestCypherService_PassesParams(t *testing.T) {
	want := map[string]interface{}{"name": "Alice"}
	var gotParams map[string]interface{}

	repo := &mockGraphRepository{
		executeFunc: func(_ context.Context, _ string, params map[string]interface{}) (interface{}, error) {
			gotParams = params
			return "ok", nil
		},
	}
	svc := service.NewCypherService(repo)
	_, err := svc.Execute(context.Background(), "MATCH (n) RETURN n", want)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotParams["name"] != "Alice" {
		t.Errorf("params not passed to repo: got %v, want %v", gotParams, want)
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
	_, err := svc.Execute(context.Background(), "MATCH (n) RETURN n", nil)
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
	_, err := svc.Execute(ctx, "MATCH (n) RETURN n", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotCtx == nil || gotCtx.Value(key{}) != "sentinel" {
		t.Error("context was not propagated to repository")
	}
}

func TestCypherService_WrapsScalarResult(t *testing.T) {
	repo := &mockGraphRepository{
		executeFunc: func(_ context.Context, _ string, _ map[string]interface{}) (interface{}, error) {
			return 42, nil
		},
	}
	svc := service.NewCypherService(repo)
	result, err := svc.Execute(context.Background(), "RETURN 42", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Columns) == 0 {
		t.Fatal("expected at least one column")
	}
	if len(result.Rows) == 0 {
		t.Fatal("expected at least one row")
	}
	// The value "42" should appear somewhere in the first row.
	found := false
	for _, v := range result.Rows[0] {
		if v == "42" {
			found = true
		}
	}
	if !found {
		t.Errorf("result rows: %v — expected to find \"42\"", result.Rows)
	}
}

// ---- Explain tests ------------------------------------------------------

func TestCypherService_Explain_PrependsMissingKeyword(t *testing.T) {
	var gotQuery string
	repo := &mockGraphRepository{
		executeFunc: func(_ context.Context, q string, _ map[string]interface{}) (interface{}, error) {
			gotQuery = q
			return nil, nil
		},
	}
	svc := service.NewCypherService(repo)
	_, _ = svc.Explain(context.Background(), "MATCH (n) RETURN n")
	if gotQuery != "EXPLAIN MATCH (n) RETURN n" {
		t.Errorf("expected EXPLAIN prepended; got: %q", gotQuery)
	}
}

func TestCypherService_Explain_DoesNotDuplicate_WhenAlreadyPresent(t *testing.T) {
	var gotQuery string
	repo := &mockGraphRepository{
		executeFunc: func(_ context.Context, q string, _ map[string]interface{}) (interface{}, error) {
			gotQuery = q
			return nil, nil
		},
	}
	svc := service.NewCypherService(repo)
	_, _ = svc.Explain(context.Background(), "EXPLAIN MATCH (n) RETURN n")
	if gotQuery != "EXPLAIN MATCH (n) RETURN n" {
		t.Errorf("EXPLAIN should not be duplicated; got: %q", gotQuery)
	}
}

func TestCypherService_Explain_DoesNotPrepend_WhenProfilePresent(t *testing.T) {
	var gotQuery string
	repo := &mockGraphRepository{
		executeFunc: func(_ context.Context, q string, _ map[string]interface{}) (interface{}, error) {
			gotQuery = q
			return nil, nil
		},
	}
	svc := service.NewCypherService(repo)
	_, _ = svc.Explain(context.Background(), "PROFILE MATCH (n) RETURN n")
	if gotQuery != "PROFILE MATCH (n) RETURN n" {
		t.Errorf("PROFILE query should not be modified; got: %q", gotQuery)
	}
}

func TestCypherService_Explain_RepoError_Propagated(t *testing.T) {
	repoErr := errors.New("explain failed")
	repo := &mockGraphRepository{
		executeFunc: func(_ context.Context, _ string, _ map[string]interface{}) (interface{}, error) {
			return nil, repoErr
		},
	}
	svc := service.NewCypherService(repo)
	_, err := svc.Explain(context.Background(), "MATCH (n) RETURN n")
	if err == nil {
		t.Fatal("expected error from repo")
	}
	if !errors.Is(err, repoErr) {
		t.Errorf("expected repo error in chain; got: %v", err)
	}
}

func TestCypherService_Explain_DefaultsToRead_ForNonRecordSet(t *testing.T) {
	// When the repo returns a plain string (test stub), Explain should
	// default to "r" rather than blocking execution.
	repo := &mockGraphRepository{
		executeFunc: func(_ context.Context, _ string, _ map[string]interface{}) (interface{}, error) {
			return "stub", nil // not a *repository.RecordSet
		},
	}
	svc := service.NewCypherService(repo)
	qt, err := svc.Explain(context.Background(), "MATCH (n) RETURN n")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if qt != "r" {
		t.Errorf("expected default query type \"r\"; got: %q", qt)
	}
}
