package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/cli/go-cli-tool/internal/repository"
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

// TestAdminService_ShowUsers_UnexpectedType verifies that a non-RecordSet
// result causes an error rather than silently returning empty data.
func TestAdminService_ShowUsers_UnexpectedType(t *testing.T) {
	repo := &mockGraphRepository{
		executeFunc: func(_ context.Context, _ string, _ map[string]interface{}) (interface{}, error) {
			return "unexpected string result", nil
		},
	}
	svc := service.NewAdminService(repo)
	_, err := svc.ShowUsers(context.Background())
	if err == nil {
		t.Fatal("expected error for unexpected result type, got nil")
	}
}

// TestAdminService_ShowUsers_ParsesRecordSet verifies correct parsing of a
// *repository.RecordSet returned by the real Neo4j repository.
func TestAdminService_ShowUsers_ParsesRecordSet(t *testing.T) {
	rs := &repository.RecordSet{
		Columns: []string{"user", "roles"},
		Rows: []map[string]interface{}{
			{"user": "alice", "roles": []interface{}{"reader", "editor"}},
			{"user": "bob", "roles": []interface{}{}},
		},
	}
	repo := &mockGraphRepository{
		executeFunc: func(_ context.Context, _ string, _ map[string]interface{}) (interface{}, error) {
			return rs, nil
		},
	}
	svc := service.NewAdminService(repo)
	users, err := svc.ShowUsers(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(users) != 2 {
		t.Fatalf("got %d users, want 2", len(users))
	}
	if users[0].Username != "alice" {
		t.Errorf("users[0].Username = %q, want %q", users[0].Username, "alice")
	}
	if len(users[0].Roles) != 2 || users[0].Roles[0] != "reader" {
		t.Errorf("users[0].Roles = %v, want [reader editor]", users[0].Roles)
	}
	if users[1].Username != "bob" {
		t.Errorf("users[1].Username = %q, want %q", users[1].Username, "bob")
	}
}

// TestAdminService_ShowDatabases_UnexpectedType mirrors the ShowUsers test.
func TestAdminService_ShowDatabases_UnexpectedType(t *testing.T) {
	repo := &mockGraphRepository{
		executeFunc: func(_ context.Context, _ string, _ map[string]interface{}) (interface{}, error) {
			return "unexpected string result", nil
		},
	}
	svc := service.NewAdminService(repo)
	_, err := svc.ShowDatabases(context.Background())
	if err == nil {
		t.Fatal("expected error for unexpected result type, got nil")
	}
}

// TestAdminService_ShowDatabases_ParsesRecordSet verifies correct parsing of a
// *repository.RecordSet returned by the real Neo4j repository.
func TestAdminService_ShowDatabases_ParsesRecordSet(t *testing.T) {
	rs := &repository.RecordSet{
		Columns: []string{"name", "currentStatus"},
		Rows: []map[string]interface{}{
			{"name": "neo4j", "currentStatus": "online"},
			{"name": "system", "currentStatus": "online"},
		},
	}
	repo := &mockGraphRepository{
		executeFunc: func(_ context.Context, _ string, _ map[string]interface{}) (interface{}, error) {
			return rs, nil
		},
	}
	svc := service.NewAdminService(repo)
	dbs, err := svc.ShowDatabases(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(dbs) != 2 {
		t.Fatalf("got %d databases, want 2", len(dbs))
	}
	if dbs[0].Name != "neo4j" || dbs[0].Status != "online" {
		t.Errorf("dbs[0] = {%q, %q}, want {neo4j, online}", dbs[0].Name, dbs[0].Status)
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
