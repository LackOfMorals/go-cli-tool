package commands_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/cli/go-cli-tool/internal/commands"
	"github.com/cli/go-cli-tool/internal/dispatch"
	"github.com/cli/go-cli-tool/internal/logger"
	"github.com/cli/go-cli-tool/internal/presentation"
	"github.com/cli/go-cli-tool/internal/service"
)

// ---- mockAdminService ---------------------------------------------------

type mockAdminService struct {
	usersResult []service.User
	usersErr    error
	dbsResult   []service.Database
	dbsErr      error
}

func (m *mockAdminService) ShowUsers(_ context.Context) ([]service.User, error) {
	return m.usersResult, m.usersErr
}

func (m *mockAdminService) ShowDatabases(_ context.Context) ([]service.Database, error) {
	return m.dbsResult, m.dbsErr
}

// ---- helpers ------------------------------------------------------------

func adminCtx(t *testing.T) dispatch.Context {
	t.Helper()
	log := logger.NewLoggerService(logger.FormatText, logger.LevelError)
	pres, err := presentation.NewPresentationService(presentation.OutputFormatTable, log)
	if err != nil {
		t.Fatalf("NewPresentationService: %v", err)
	}
	return dispatch.Context{
		Context:   context.Background(),
		Presenter: pres,
	}
}

// humanOut renders a CommandResult to a string for test assertions.
func humanOut(t *testing.T, result dispatch.CommandResult, ctx dispatch.Context) string {
	t.Helper()
	if result.Presentation != nil && ctx.Presenter != nil {
		out, err := ctx.Presenter.Format(result.Presentation)
		if err != nil {
			t.Fatalf("Format error: %v", err)
		}
		return out
	}
	return result.Message
}

// ---- show-users ---------------------------------------------------------

func TestAdminCategory_ShowUsers_Empty(t *testing.T) {
	svc := &mockAdminService{}
	cat := commands.BuildAdminCategory(svc)

	result, err := cat.Dispatch([]string{"show-users"}, adminCtx(t))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Empty list → Items is non-nil empty slice, Presentation shows "(no results)"
	if result.Items == nil {
		t.Error("expected Items to be non-nil empty slice for empty list")
	}
	if len(result.Items) != 0 {
		t.Errorf("expected 0 items, got %d", len(result.Items))
	}
}

func TestAdminCategory_ShowUsers_FormatsTable(t *testing.T) {
	ctx := adminCtx(t)
	svc := &mockAdminService{
		usersResult: []service.User{
			{Username: "alice", Roles: []string{"admin"}},
			{Username: "bob", Roles: []string{"reader", "writer"}},
		},
	}
	cat := commands.BuildAdminCategory(svc)

	result, err := cat.Dispatch([]string{"show-users"}, ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check structured Items for JSON output.
	if len(result.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(result.Items))
	}
	if result.Items[0]["username"] != "alice" {
		t.Errorf("expected alice in Items[0], got %v", result.Items[0])
	}
	roles, ok := result.Items[0]["roles"].([]string)
	if !ok || len(roles) == 0 || roles[0] != "admin" {
		t.Errorf("expected roles=[admin] in Items[0], got %v", result.Items[0]["roles"])
	}

	// Check human rendering.
	out := humanOut(t, result, ctx)
	for _, want := range []string{"alice", "admin", "bob", "reader", "writer", "Username", "Roles"} {
		if !strings.Contains(out, want) {
			t.Errorf("human output should contain %q, got:\n%s", want, out)
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

	result, err := cat.Dispatch([]string{"show-databases"}, adminCtx(t))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Items == nil {
		t.Error("expected Items to be non-nil empty slice for empty list")
	}
	if len(result.Items) != 0 {
		t.Errorf("expected 0 items, got %d", len(result.Items))
	}
}

func TestAdminCategory_ShowDatabases_FormatsTable(t *testing.T) {
	ctx := adminCtx(t)
	svc := &mockAdminService{
		dbsResult: []service.Database{
			{Name: "neo4j", Status: "online"},
			{Name: "system", Status: "online"},
		},
	}
	cat := commands.BuildAdminCategory(svc)

	result, err := cat.Dispatch([]string{"show-databases"}, ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(result.Items))
	}
	if result.Items[0]["name"] != "neo4j" {
		t.Errorf("expected neo4j in Items[0], got %v", result.Items[0])
	}
	if result.Items[0]["status"] != "online" {
		t.Errorf("expected online status, got %v", result.Items[0]["status"])
	}

	out := humanOut(t, result, ctx)
	for _, want := range []string{"neo4j", "system", "online", "Name", "Status"} {
		if !strings.Contains(out, want) {
			t.Errorf("human output should contain %q, got:\n%s", want, out)
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

	result, err := cat.Dispatch(nil, adminCtx(t))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result.Message, "admin") {
		t.Errorf("expected help output, got: %q", result.Message)
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
