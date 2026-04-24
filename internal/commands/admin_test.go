package commands_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/cli/go-cli-tool/internal/commands"
	"github.com/cli/go-cli-tool/internal/logger"
	"github.com/cli/go-cli-tool/internal/presentation"
	"github.com/cli/go-cli-tool/internal/service"
	"github.com/cli/go-cli-tool/internal/shell"
)

// ---- mockAdminService ---------------------------------------------------

type mockAdminService struct {
	usersResult []service.User
	usersErr    error
	dbsResult   []service.Database
	dbsErr      error
}

func (m *mockAdminService) ShowUsers(ctx context.Context) ([]service.User, error) {
	return m.usersResult, m.usersErr
}

func (m *mockAdminService) ShowDatabases(ctx context.Context) ([]service.Database, error) {
	return m.dbsResult, m.dbsErr
}

// ---- helpers ------------------------------------------------------------

func adminCtx(t *testing.T) shell.ShellContext {
	t.Helper()
	log := logger.NewLoggerService(logger.FormatText, logger.LevelError)
	pres, err := presentation.NewPresentationService(presentation.OutputFormatTable, log)
	if err != nil {
		t.Fatalf("NewPresentationService: %v", err)
	}
	return shell.ShellContext{
		Context:   context.Background(),
		Presenter: pres,
	}
}

// ---- show-users ---------------------------------------------------------

func TestAdminCategory_ShowUsers_Empty(t *testing.T) {
	svc := &mockAdminService{}
	cat := commands.BuildAdminCategory(svc)

	out, err := cat.Dispatch([]string{"show-users"}, adminCtx(t))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "No users found") {
		t.Errorf("expected empty message, got: %q", out)
	}
}

func TestAdminCategory_ShowUsers_FormatsTable(t *testing.T) {
	svc := &mockAdminService{
		usersResult: []service.User{
			{Username: "alice", Roles: []string{"admin"}},
			{Username: "bob", Roles: []string{"reader", "writer"}},
		},
	}
	cat := commands.BuildAdminCategory(svc)

	out, err := cat.Dispatch([]string{"show-users"}, adminCtx(t))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, want := range []string{"alice", "admin", "bob", "reader", "writer", "Username", "Roles"} {
		if !strings.Contains(out, want) {
			t.Errorf("output should contain %q, got:\n%s", want, out)
		}
	}
}

func TestAdminCategory_ShowUsers_Error(t *testing.T) {
	svcErr := errors.New("permission denied")
	svc := &mockAdminService{usersErr: svcErr}
	cat := commands.BuildAdminCategory(svc)

	_, err := cat.Dispatch([]string{"show-users"}, adminCtx(t))
	if err == nil {
		t.Fatal("expected error to be propagated")
	}
	if !errors.Is(err, svcErr) {
		t.Errorf("expected service error in chain, got: %v", err)
	}
}

// ---- show-databases -----------------------------------------------------

func TestAdminCategory_ShowDatabases_Empty(t *testing.T) {
	svc := &mockAdminService{}
	cat := commands.BuildAdminCategory(svc)

	out, err := cat.Dispatch([]string{"show-databases"}, adminCtx(t))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "No databases found") {
		t.Errorf("expected empty message, got: %q", out)
	}
}

func TestAdminCategory_ShowDatabases_FormatsTable(t *testing.T) {
	svc := &mockAdminService{
		dbsResult: []service.Database{
			{Name: "neo4j", Status: "online"},
			{Name: "system", Status: "online"},
		},
	}
	cat := commands.BuildAdminCategory(svc)

	out, err := cat.Dispatch([]string{"show-databases"}, adminCtx(t))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, want := range []string{"neo4j", "system", "online", "Name", "Status"} {
		if !strings.Contains(out, want) {
			t.Errorf("output should contain %q, got:\n%s", want, out)
		}
	}
}

func TestAdminCategory_ShowDatabases_Error(t *testing.T) {
	svcErr := errors.New("db unavailable")
	svc := &mockAdminService{dbsErr: svcErr}
	cat := commands.BuildAdminCategory(svc)

	_, err := cat.Dispatch([]string{"show-databases"}, adminCtx(t))
	if err == nil {
		t.Fatal("expected error to be propagated")
	}
	if !errors.Is(err, svcErr) {
		t.Errorf("expected service error in chain, got: %v", err)
	}
}

// ---- unknown command ----------------------------------------------------

func TestAdminCategory_UnknownCommand(t *testing.T) {
	svc := &mockAdminService{}
	cat := commands.BuildAdminCategory(svc)

	_, err := cat.Dispatch([]string{"nonexistent"}, adminCtx(t))
	if err == nil {
		t.Fatal("expected error for unknown admin command")
	}
}

// ---- no args → help -----------------------------------------------------

func TestAdminCategory_NoArgs_ReturnsHelp(t *testing.T) {
	svc := &mockAdminService{}
	cat := commands.BuildAdminCategory(svc)

	out, err := cat.Dispatch(nil, adminCtx(t))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "admin") {
		t.Errorf("expected help output, got: %q", out)
	}
}

// ---- context propagation ------------------------------------------------

func TestAdminCategory_ShowUsers_ContextPropagated(t *testing.T) {
	type key struct{}
	ctx := context.WithValue(context.Background(), key{}, "sentinel")
	sc := adminCtx(t)
	sc.Context = ctx

	var gotCtx context.Context
	svc := &mockAdminService{
		usersResult: []service.User{{Username: "u"}},
	}
	// Override via a wrapper that captures the context.
	cat := commands.BuildAdminCategory(&contextCapturingAdminSvc{
		inner:   svc,
		capture: func(c context.Context) { gotCtx = c },
	})

	_, err := cat.Dispatch([]string{"show-users"}, sc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotCtx == nil || gotCtx.Value(key{}) != "sentinel" {
		t.Error("context not propagated to admin service")
	}
}

// contextCapturingAdminSvc wraps an AdminService and captures the context.
type contextCapturingAdminSvc struct {
	inner   service.AdminService
	capture func(context.Context)
}

func (s *contextCapturingAdminSvc) ShowUsers(ctx context.Context) ([]service.User, error) {
	s.capture(ctx)
	return s.inner.ShowUsers(ctx)
}

func (s *contextCapturingAdminSvc) ShowDatabases(ctx context.Context) ([]service.Database, error) {
	s.capture(ctx)
	return s.inner.ShowDatabases(ctx)
}
