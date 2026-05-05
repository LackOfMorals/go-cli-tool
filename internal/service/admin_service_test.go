package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/cli/go-cli-tool/internal/service"
)

// ---- AdminService tests -------------------------------------------------

func TestAdminService_ShowUsers_RepoError(t *testing.T) {
	repoErr := errors.New("connection refused")
	repo := &mockGraphRepository{
		executeFunc: func(_ context.Context, _ string, _ map[string]interface{}) (interface{}, error) {
			return nil, repoErr
		},
	}
	svc := service.NewAdminService(repo)
	_, err := svc.ShowUsers(context.Background())
	if err == nil {
		t.Fatal("expected error from repo")
	}
	if !errors.Is(err, repoErr) {
		t.Errorf("expected wrapped repo error; got: %v", err)
	}
}

func TestAdminService_ShowDatabases_RepoError(t *testing.T) {
	repoErr := errors.New("timeout")
	repo := &mockGraphRepository{
		executeFunc: func(_ context.Context, _ string, _ map[string]interface{}) (interface{}, error) {
			return nil, repoErr
		},
	}
	svc := service.NewAdminService(repo)
	_, err := svc.ShowDatabases(context.Background())
	if err == nil {
		t.Fatal("expected error from repo")
	}
	if !errors.Is(err, repoErr) {
		t.Errorf("expected wrapped repo error; got: %v", err)
	}
}

// TestAdminService_ShowUsers_PlaceholderResult verifies that the stub
// repository result (a plain string) returns ErrResultNotParseable rather
// than silently returning hardcoded fake data.
func TestAdminService_ShowUsers_PlaceholderResult(t *testing.T) {
	repo := &mockGraphRepository{
		executeFunc: func(_ context.Context, _ string, _ map[string]interface{}) (interface{}, error) {
			return "result placeholder", nil // mimics the stub repository
		},
	}
	svc := service.NewAdminService(repo)
	_, err := svc.ShowUsers(context.Background())
	if err == nil {
		t.Fatal("expected ErrResultNotParseable, got nil")
	}
	if !errors.Is(err, service.ErrResultNotParseable) {
		t.Errorf("expected ErrResultNotParseable in error chain; got: %v", err)
	}
}

// TestAdminService_ShowDatabases_PlaceholderResult mirrors the ShowUsers test.
func TestAdminService_ShowDatabases_PlaceholderResult(t *testing.T) {
	repo := &mockGraphRepository{
		executeFunc: func(_ context.Context, _ string, _ map[string]interface{}) (interface{}, error) {
			return "result placeholder", nil
		},
	}
	svc := service.NewAdminService(repo)
	_, err := svc.ShowDatabases(context.Background())
	if err == nil {
		t.Fatal("expected ErrResultNotParseable, got nil")
	}
	if !errors.Is(err, service.ErrResultNotParseable) {
		t.Errorf("expected ErrResultNotParseable in error chain; got: %v", err)
	}
}

// TestAdminService_ShowUsers_QueriesCorrectCypher verifies the Cypher sent
// to the repository.
func TestAdminService_ShowUsers_QueriesCorrectCypher(t *testing.T) {
	var gotCypher string
	repo := &mockGraphRepository{
		executeFunc: func(_ context.Context, cypher string, _ map[string]interface{}) (interface{}, error) {
			gotCypher = cypher
			return "placeholder", nil
		},
	}
	svc := service.NewAdminService(repo)
	_, _ = svc.ShowUsers(context.Background())

	if gotCypher != "SHOW USERS" {
		t.Errorf("cypher: got %q, want %q", gotCypher, "SHOW USERS")
	}
}

// TestAdminService_ShowDatabases_QueriesCorrectCypher mirrors the ShowUsers test.
func TestAdminService_ShowDatabases_QueriesCorrectCypher(t *testing.T) {
	var gotCypher string
	repo := &mockGraphRepository{
		executeFunc: func(_ context.Context, cypher string, _ map[string]interface{}) (interface{}, error) {
			gotCypher = cypher
			return "placeholder", nil
		},
	}
	svc := service.NewAdminService(repo)
	_, _ = svc.ShowDatabases(context.Background())

	if gotCypher != "SHOW DATABASES" {
		t.Errorf("cypher: got %q, want %q", gotCypher, "SHOW DATABASES")
	}
}

// TestAdminService_ContextPropagated verifies the context flows through to
// the repository for both admin operations.
func TestAdminService_ContextPropagated(t *testing.T) {
	type key struct{}
	ctx := context.WithValue(context.Background(), key{}, "sentinel")

	for _, name := range []string{"ShowUsers", "ShowDatabases"} {
		t.Run(name, func(t *testing.T) {
			var gotCtx context.Context
			repo := &mockGraphRepository{
				executeFunc: func(c context.Context, _ string, _ map[string]interface{}) (interface{}, error) {
					gotCtx = c
					return "placeholder", nil
				},
			}
			svc := service.NewAdminService(repo)
			switch name {
			case "ShowUsers":
				_, _ = svc.ShowUsers(ctx)
			case "ShowDatabases":
				_, _ = svc.ShowDatabases(ctx)
			}
			if gotCtx == nil || gotCtx.Value(key{}) != "sentinel" {
				t.Errorf("context not propagated to repository in %s", name)
			}
		})
	}
}
