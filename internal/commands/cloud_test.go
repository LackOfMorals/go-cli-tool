package commands_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/cli/go-cli-tool/internal/commands"
	"github.com/cli/go-cli-tool/internal/config"
	"github.com/cli/go-cli-tool/internal/dispatch"
	"github.com/cli/go-cli-tool/internal/logger"
	"github.com/cli/go-cli-tool/internal/presentation"
	"github.com/cli/go-cli-tool/internal/service"
	"github.com/cli/go-cli-tool/internal/tool"
)

// ---- mock cloud service -------------------------------------------------

type mockCloudService struct {
	instances service.InstancesService
	projects  service.ProjectsService
}

func (m *mockCloudService) Instances() service.InstancesService { return m.instances }
func (m *mockCloudService) Projects() service.ProjectsService   { return m.projects }

// ---- mock instances service ---------------------------------------------

type mockInstancesService struct {
	listResult   []service.Instance
	listErr      error
	getResult    *service.Instance
	getErr       error
	createResult *service.CreatedInstance
	createErr    error
	updateResult *service.Instance
	updateErr    error
	pauseErr     error
	resumeErr    error
	deleteErr    error
}

func (m *mockInstancesService) List(_ context.Context) ([]service.Instance, error) {
	return m.listResult, m.listErr
}
func (m *mockInstancesService) Get(_ context.Context, _ string) (*service.Instance, error) {
	return m.getResult, m.getErr
}
func (m *mockInstancesService) Create(_ context.Context, _ *service.CreateInstanceParams) (*service.CreatedInstance, error) {
	return m.createResult, m.createErr
}
func (m *mockInstancesService) Update(_ context.Context, _ string, _ *service.UpdateInstanceParams) (*service.Instance, error) {
	return m.updateResult, m.updateErr
}
func (m *mockInstancesService) Pause(_ context.Context, _ string) error  { return m.pauseErr }
func (m *mockInstancesService) Resume(_ context.Context, _ string) error { return m.resumeErr }
func (m *mockInstancesService) Delete(_ context.Context, _ string) error { return m.deleteErr }

// ---- mock projects service ----------------------------------------------

type mockProjectsService struct {
	listResult []service.Project
	listErr    error
	getResult  *service.Project
	getErr     error
}

func (m *mockProjectsService) List(_ context.Context) ([]service.Project, error) {
	return m.listResult, m.listErr
}
func (m *mockProjectsService) Get(_ context.Context, _ string) (*service.Project, error) {
	return m.getResult, m.getErr
}

// ---- helpers ------------------------------------------------------------

func cloudCtx(t *testing.T) dispatch.Context {
	t.Helper()
	log := logger.NewLoggerService(logger.FormatText, logger.LevelError)
	pres, err := presentation.NewPresentationService(presentation.OutputFormatTable, log)
	if err != nil {
		t.Fatalf("NewPresentationService: %v", err)
	}
	return dispatch.Context{
		Context:   context.Background(),
		Config:    config.Config{},
		IO:        &mockIO{},
		Presenter: pres,
	}
}

// mockIO satisfies tool.IOHandler for tests.
type mockIO struct {
	readLines []string
	written   []string
}

func (m *mockIO) Read() (string, error) {
	if len(m.readLines) == 0 {
		return "", nil
	}
	line := m.readLines[0]
	m.readLines = m.readLines[1:]
	return line, nil
}
func (m *mockIO) Write(format string, args ...interface{}) {
	m.written = append(m.written, fmt.Sprintf(format, args...))
}
func (m *mockIO) WriteError(_ error) {}
func (m *mockIO) WriteLine(line string) {
	m.written = append(m.written, line)
}
func (m *mockIO) WriteJSON(_ interface{}) error { return nil }

// ---- instances list -----------------------------------------------------

func TestCloudCategory_InstancesList_Empty(t *testing.T) {
	svc := &mockCloudService{instances: &mockInstancesService{}}
	cat := commands.BuildCloudCategory(svc)

	result, err := cat.Dispatch([]string{"instances", "list"}, cloudCtx(t))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Items == nil {
		t.Error("expected non-nil Items for empty list")
	}
	if len(result.Items) != 0 {
		t.Errorf("expected 0 items, got %d", len(result.Items))
	}
}

func TestCloudCategory_InstancesList_FormatsTable(t *testing.T) {
	ctx := cloudCtx(t)
	svc := &mockCloudService{
		instances: &mockInstancesService{
			listResult: []service.Instance{
				{ID: "abc-123", Name: "my-db", TenantID: "my-project", CloudProvider: "GCP"},
			},
		},
	}
	cat := commands.BuildCloudCategory(svc)

	result, err := cat.Dispatch([]string{"instances", "list"}, ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(result.Items))
	}
	if result.Items[0]["id"] != "abc-123" {
		t.Errorf("expected id=abc-123, got %v", result.Items[0]["id"])
	}
	if result.Items[0]["name"] != "my-db" {
		t.Errorf("expected name=my-db, got %v", result.Items[0]["name"])
	}
	// Human table rendering
	out, err := ctx.Presenter.Format(result.Presentation)
	if err != nil {
		t.Fatalf("Format error: %v", err)
	}
	for _, want := range []string{"abc-123", "my-db", "my-project", "GCP"} {
		if !strings.Contains(out, want) {
			t.Errorf("human output should contain %q:\n%s", want, out)
		}
	}
}

func TestCloudCategory_InstancesList_Error(t *testing.T) {
	svcErr := errors.New("api unavailable")
	svc := &mockCloudService{instances: &mockInstancesService{listErr: svcErr}}
	cat := commands.BuildCloudCategory(svc)

	_, err := cat.Dispatch([]string{"instances", "list"}, cloudCtx(t))
	if !errors.Is(err, svcErr) {
		t.Errorf("expected service error in chain, got: %v", err)
	}
}

// ---- instances get ------------------------------------------------------

func TestCloudCategory_InstancesGet_NoID_ReturnsError(t *testing.T) {
	svc := &mockCloudService{instances: &mockInstancesService{}}
	cat := commands.BuildCloudCategory(svc)

	_, err := cat.Dispatch([]string{"instances", "get"}, cloudCtx(t))
	if err == nil {
		t.Fatal("expected error when ID is missing")
	}
}

func TestCloudCategory_InstancesGet_ShowsDetail(t *testing.T) {
	ctx := cloudCtx(t)
	svc := &mockCloudService{
		instances: &mockInstancesService{
			getResult: &service.Instance{
				ID: "xyz", Name: "prod-db", Status: "running",
				Region: "us-east-1", Tier: "enterprise-db", Memory: "16GB",
				CloudProvider: "aws", ConnectionURL: "bolt+s://xyz.databases.neo4j.io",
			},
		},
	}
	cat := commands.BuildCloudCategory(svc)

	result, err := cat.Dispatch([]string{"instances", "get", "xyz"}, ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Check JSON Item with snake_case keys
	if result.Item == nil {
		t.Fatal("expected Item to be set")
	}
	if result.Item["id"] != "xyz" {
		t.Errorf("expected id=xyz, got %v", result.Item["id"])
	}
	if result.Item["connection_url"] != "bolt+s://xyz.databases.neo4j.io" {
		t.Errorf("expected connection_url set, got %v", result.Item["connection_url"])
	}
	// Human rendering
	out, err := ctx.Presenter.Format(result.Presentation)
	if err != nil {
		t.Fatalf("Format error: %v", err)
	}
	for _, want := range []string{"xyz", "prod-db", "running", "16GB", "bolt+s://xyz.databases.neo4j.io"} {
		if !strings.Contains(out, want) {
			t.Errorf("human output should contain %q:\n%s", want, out)
		}
	}
}

// ---- instances create ---------------------------------------------------

func TestCloudCategory_InstancesCreate_MissingName_ReturnsError(t *testing.T) {
	svc := &mockCloudService{instances: &mockInstancesService{}}
	cat := commands.BuildCloudCategory(svc)
	_, err := cat.Dispatch([]string{"instances", "create"}, cloudCtx(t))
	if err == nil {
		t.Fatal("expected error when name is missing")
	}
}

func TestCloudCategory_InstancesCreate_MissingTenant_ReturnsError(t *testing.T) {
	svc := &mockCloudService{instances: &mockInstancesService{}}
	cat := commands.BuildCloudCategory(svc)
	_, err := cat.Dispatch([]string{"instances", "create", "name=my-db"}, cloudCtx(t))
	if err == nil {
		t.Fatal("expected error when tenant ID is missing")
	}
	if !strings.Contains(err.Error(), "tenant") {
		t.Errorf("error should mention tenant, got: %v", err)
	}
}

func TestCloudCategory_InstancesCreate_Success(t *testing.T) {
	created := &service.CreatedInstance{
		Instance: service.Instance{
			ID: "new-id", Name: "my-db", Status: "creating",
			ConnectionURL: "bolt+s://new.databases.neo4j.io",
			Username:      "neo4j",
		},
		Password: "s3cr3t!",
	}
	svc := &mockCloudService{
		instances: &mockInstancesService{createResult: created},
	}
	io := &mockIO{}
	ctx := cloudCtx(t)
	ctx.IO = io
	ctx.Config.Aura.InstanceDefaults = config.AuraInstanceDefaults{
		TenantID: "tenant-abc", CloudProvider: "gcp",
		Region: "europe-west1", Type: "enterprise-db", Version: "5", Memory: "8GB",
	}
	cat := commands.BuildCloudCategory(svc)

	result, err := cat.Dispatch([]string{"instances", "create", "name=my-db"}, ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// JSON Item should have snake_case keys including password
	if result.Item == nil {
		t.Fatal("expected Item to be set")
	}
	if result.Item["id"] != "new-id" {
		t.Errorf("expected id=new-id, got %v", result.Item["id"])
	}
	if result.Item["password"] != "s3cr3t!" {
		t.Errorf("expected password in Item, got %v", result.Item["password"])
	}

	// Save warning goes to IO, not to the structured result
	ioOut := strings.Join(io.written, "")
	if !strings.Contains(ioOut, "NOT be shown again") {
		t.Errorf("expected save warning in IO output, got: %q", ioOut)
	}
}

// ---- instances update ---------------------------------------------------

func TestCloudCategory_InstancesUpdate_NoArgs_ReturnsError(t *testing.T) {
	svc := &mockCloudService{instances: &mockInstancesService{}}
	cat := commands.BuildCloudCategory(svc)

	_, err := cat.Dispatch([]string{"instances", "update"}, cloudCtx(t))
	if err == nil {
		t.Fatal("expected error when no args")
	}
}

func TestCloudCategory_InstancesUpdate_NoFields_ReturnsError(t *testing.T) {
	svc := &mockCloudService{instances: &mockInstancesService{}}
	cat := commands.BuildCloudCategory(svc)

	_, err := cat.Dispatch([]string{"instances", "update", "some-id"}, cloudCtx(t))
	if err == nil {
		t.Fatal("expected error when no name or memory provided")
	}
}

func TestCloudCategory_InstancesUpdate_Success(t *testing.T) {
	updated := &service.Instance{ID: "abc", Name: "renamed-db", Memory: "16GB"}
	svc := &mockCloudService{
		instances: &mockInstancesService{updateResult: updated},
	}
	cat := commands.BuildCloudCategory(svc)

	result, err := cat.Dispatch([]string{"instances", "update", "abc", "name=renamed-db", "memory=16GB"}, cloudCtx(t))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Item == nil {
		t.Fatal("expected Item to be set")
	}
	if result.Item["name"] != "renamed-db" {
		t.Errorf("expected name=renamed-db, got %v", result.Item["name"])
	}
	if result.Item["memory"] != "16GB" {
		t.Errorf("expected memory=16GB, got %v", result.Item["memory"])
	}
}

// ---- instances delete ---------------------------------------------------

func TestCloudCategory_InstancesDelete_Confirmed(t *testing.T) {
	svc := &mockCloudService{instances: &mockInstancesService{}}
	cat := commands.BuildCloudCategory(svc)

	io := &mockIO{readLines: []string{"yes"}}
	ctx := cloudCtx(t)
	ctx.IO = io

	result, err := cat.Dispatch([]string{"instances", "delete", "del-id"}, ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Item == nil {
		t.Fatal("expected Item to be set")
	}
	if result.Item["status"] != "deleted" {
		t.Errorf("expected status=deleted, got %v", result.Item["status"])
	}
}

func TestCloudCategory_InstancesDelete_Cancelled(t *testing.T) {
	svc := &mockCloudService{instances: &mockInstancesService{}}
	cat := commands.BuildCloudCategory(svc)

	io := &mockIO{readLines: []string{"no"}}
	ctx := cloudCtx(t)
	ctx.IO = io

	result, err := cat.Dispatch([]string{"instances", "delete", "del-id"}, ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result.Message, "cancelled") {
		t.Errorf("expected cancellation message, got: %q", result.Message)
	}
}

func TestCloudCategory_InstancesDelete_AgentMode_NoPrompt(t *testing.T) {
	// In agent mode with --rw the delete handler should not prompt at all.
	svc := &mockCloudService{instances: &mockInstancesService{}}
	cat := commands.BuildCloudCategory(svc)

	io := &mockIO{} // no readLines queued — a prompt would return "", causing cancellation
	ctx := cloudCtx(t)
	ctx.IO = io
	ctx.AgentMode = true
	ctx.AllowWrites = true

	result, err := cat.Dispatch([]string{"instances", "delete", "del-id"}, ctx)
	if err != nil {
		t.Fatalf("unexpected error in agent+rw mode: %v", err)
	}
	if result.Item == nil {
		t.Fatal("expected Item to be set")
	}
	if result.Item["status"] != "deleted" {
		t.Errorf("expected status=deleted without prompt, got %v", result.Item["status"])
	}
	if len(io.written) > 0 {
		t.Errorf("no output should be written in agent mode; got: %v", io.written)
	}
}

func TestCloudCategory_InstancesDelete_AgentMode_BlockedWithoutRW(t *testing.T) {
	// In agent mode without --rw, the dispatcher must block before the handler.
	svc := &mockCloudService{instances: &mockInstancesService{}}
	cat := commands.BuildCloudCategory(svc)

	ctx := cloudCtx(t)
	ctx.AgentMode = true
	ctx.AllowWrites = false

	_, err := cat.Dispatch([]string{"instances", "delete", "del-id"}, ctx)
	if err == nil {
		t.Fatal("expected READ_ONLY error in agent mode without --rw")
	}
	var ae *tool.AgentError
	if !errors.As(err, &ae) {
		t.Errorf("expected AgentError; got: %T %v", err, err)
	}
}

// ---- projects list ------------------------------------------------------

func TestCloudCategory_ProjectsList_FormatsTable(t *testing.T) {
	svc := &mockCloudService{
		instances: &mockInstancesService{},
		projects: &mockProjectsService{
			listResult: []service.Project{
				{ID: "tenant-1", Name: "Production"},
				{ID: "tenant-2", Name: "Development"},
			},
		},
	}
	cat := commands.BuildCloudCategory(svc)
	ctx := cloudCtx(t)

	result, err := cat.Dispatch([]string{"projects", "list"}, ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(result.Items))
	}
	if result.Items[0]["id"] != "tenant-1" {
		t.Errorf("expected first item id=tenant-1, got %v", result.Items[0]["id"])
	}
	// Human rendering
	out, fmtErr := ctx.Presenter.Format(result.Presentation)
	if fmtErr != nil {
		t.Fatalf("Format error: %v", fmtErr)
	}
	for _, want := range []string{"tenant-1", "Production", "tenant-2", "Development"} {
		if !strings.Contains(out, want) {
			t.Errorf("output should contain %q:\n%s", want, out)
		}
	}
}

// ---- AuraPrerequisite blocks cloud dispatch -----------------------------

func TestCloudCategory_AuraPrerequisite_BlocksDispatch(t *testing.T) {
	svc := &mockCloudService{instances: &mockInstancesService{}}
	cat := commands.BuildCloudCategory(svc).
		SetPrerequisite(commands.AuraPrerequisite(&config.AuraConfig{})) // no creds

	_, err := cat.Dispatch([]string{"instances", "list"}, cloudCtx(t))
	if err == nil {
		t.Fatal("expected prerequisite error")
	}
	if !errors.Is(err, tool.ErrPrerequisite) {
		t.Errorf("expected tool.ErrPrerequisite, got: %v", err)
	}
}

func TestCloudCategory_AuraPrerequisite_AllowsHelpWithoutCreds(t *testing.T) {
	svc := &mockCloudService{instances: &mockInstancesService{}}
	cat := commands.BuildCloudCategory(svc).
		SetPrerequisite(commands.AuraPrerequisite(&config.AuraConfig{}))

	// Bare "cloud" → show help. Prerequisite must NOT fire.
	_, err := cat.Dispatch(nil, cloudCtx(t))
	if errors.Is(err, tool.ErrPrerequisite) {
		t.Error("prerequisite should not fire on bare category invocation")
	}
}
