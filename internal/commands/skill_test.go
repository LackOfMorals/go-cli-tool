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
	service_mocks "github.com/cli/go-cli-tool/internal/service/mocks"
	"github.com/cli/go-cli-tool/internal/tool"
	"go.uber.org/mock/gomock"
)

// ---- helpers ------------------------------------------------------------

func skillCtx(t *testing.T) dispatch.Context {
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

// ---- skill install ------------------------------------------------------

func TestSkillCategory_Install_NoPositional_FansOutToAllDetected(t *testing.T) {
	ctrl := gomock.NewController(t)
	mock := service_mocks.NewMockSkillService(ctrl)
	// Empty agentName signals "all detected agents".
	mock.EXPECT().Install(gomock.Any(), "").Return([]service.InstallResult{
		{Agent: "claude-code", Path: "/tmp/.claude/skills/neo4j-cli/SKILL.md"},
		{Agent: "cursor", Path: "/tmp/.cursor/rules/neo4j-cli/SKILL.md"},
	}, nil)

	cat := commands.BuildSkillCategory(mock)
	ctx := skillCtx(t)

	result, err := cat.Dispatch([]string{"install"}, ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(result.Items))
	}
	if result.Items[0]["agent"] != "claude-code" {
		t.Errorf("expected claude-code as first agent, got %v", result.Items[0]["agent"])
	}
	if !strings.HasSuffix(result.Items[1]["path"].(string), "SKILL.md") {
		t.Errorf("expected path to end with SKILL.md, got %v", result.Items[1]["path"])
	}

	out := humanOut(t, result, ctx)
	for _, want := range []string{"claude-code", "cursor", "Agent", "Path"} {
		if !strings.Contains(out, want) {
			t.Errorf("table output should contain %q:\n%s", want, out)
		}
	}
}

func TestSkillCategory_Install_WithPositional_TargetsSingleAgent(t *testing.T) {
	ctrl := gomock.NewController(t)
	mock := service_mocks.NewMockSkillService(ctrl)
	mock.EXPECT().Install(gomock.Any(), "cursor").Return([]service.InstallResult{
		{Agent: "cursor", Path: "/tmp/.cursor/rules/neo4j-cli/SKILL.md"},
	}, nil)

	cat := commands.BuildSkillCategory(mock)
	result, err := cat.Dispatch([]string{"install", "cursor"}, skillCtx(t))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(result.Items))
	}
	if result.Items[0]["agent"] != "cursor" {
		t.Errorf("expected cursor, got %v", result.Items[0]["agent"])
	}
}

func TestSkillCategory_Install_PropagatesError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mock := service_mocks.NewMockSkillService(ctrl)
	svcErr := errors.New("no agents detected")
	mock.EXPECT().Install(gomock.Any(), "").Return(nil, svcErr)

	cat := commands.BuildSkillCategory(mock)
	_, err := cat.Dispatch([]string{"install"}, skillCtx(t))
	if err == nil {
		t.Fatal("expected error to be propagated")
	}
	if !errors.Is(err, svcErr) {
		t.Errorf("expected service error in chain, got: %v", err)
	}
}

// ---- skill remove -------------------------------------------------------

func TestSkillCategory_Remove_NoPositional_FansOut(t *testing.T) {
	ctrl := gomock.NewController(t)
	mock := service_mocks.NewMockSkillService(ctrl)
	mock.EXPECT().Remove(gomock.Any(), "").Return([]service.RemoveResult{
		{Agent: "claude-code"},
		{Agent: "cursor"},
	}, nil)

	cat := commands.BuildSkillCategory(mock)
	ctx := skillCtx(t)

	result, err := cat.Dispatch([]string{"remove"}, ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(result.Items))
	}
	if result.Items[0]["agent"] != "claude-code" {
		t.Errorf("expected claude-code, got %v", result.Items[0]["agent"])
	}
	out := humanOut(t, result, ctx)
	if !strings.Contains(out, "claude-code") || !strings.Contains(out, "cursor") {
		t.Errorf("table output should list both agents:\n%s", out)
	}
}

func TestSkillCategory_Remove_WithPositional_TargetsSingleAgent(t *testing.T) {
	ctrl := gomock.NewController(t)
	mock := service_mocks.NewMockSkillService(ctrl)
	mock.EXPECT().Remove(gomock.Any(), "cursor").Return([]service.RemoveResult{
		{Agent: "cursor"},
	}, nil)

	cat := commands.BuildSkillCategory(mock)
	result, err := cat.Dispatch([]string{"remove", "cursor"}, skillCtx(t))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Items) != 1 || result.Items[0]["agent"] != "cursor" {
		t.Errorf("expected single cursor result, got %v", result.Items)
	}
}

func TestSkillCategory_Remove_PropagatesError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mock := service_mocks.NewMockSkillService(ctrl)
	svcErr := errors.New("permission denied")
	mock.EXPECT().Remove(gomock.Any(), "cursor").Return(nil, svcErr)

	cat := commands.BuildSkillCategory(mock)
	_, err := cat.Dispatch([]string{"remove", "cursor"}, skillCtx(t))
	if err == nil {
		t.Fatal("expected error to be propagated")
	}
	if !errors.Is(err, svcErr) {
		t.Errorf("expected service error in chain, got: %v", err)
	}
}

// ---- skill list ---------------------------------------------------------

func TestSkillCategory_List_RendersAllAgents(t *testing.T) {
	ctrl := gomock.NewController(t)
	mock := service_mocks.NewMockSkillService(ctrl)
	mock.EXPECT().List(gomock.Any()).Return([]service.AgentStatus{
		{Name: "claude-code", DisplayName: "Claude Code", Detected: true, Installed: true},
		{Name: "cursor", DisplayName: "Cursor", Detected: true, Installed: false},
		{Name: "windsurf", DisplayName: "Windsurf", Detected: false, Installed: false},
	}, nil)

	cat := commands.BuildSkillCategory(mock)
	ctx := skillCtx(t)

	result, err := cat.Dispatch([]string{"list"}, ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(result.Items))
	}
	// JSON shape: native booleans, snake_case keys.
	if result.Items[0]["installed"] != true {
		t.Errorf("expected installed=true (bool) for claude-code, got %v (%T)",
			result.Items[0]["installed"], result.Items[0]["installed"])
	}
	if result.Items[0]["display_name"] != "Claude Code" {
		t.Errorf("expected display_name key in JSON items, got %v", result.Items[0])
	}

	// Table rendering uses yes/no for booleans.
	out := humanOut(t, result, ctx)
	for _, want := range []string{"claude-code", "cursor", "windsurf", "yes", "no", "Detected", "Installed"} {
		if !strings.Contains(out, want) {
			t.Errorf("table output should contain %q:\n%s", want, out)
		}
	}
}

func TestSkillCategory_List_AliasLs(t *testing.T) {
	ctrl := gomock.NewController(t)
	mock := service_mocks.NewMockSkillService(ctrl)
	mock.EXPECT().List(gomock.Any()).Return([]service.AgentStatus{
		{Name: "claude-code", DisplayName: "Claude Code", Detected: true, Installed: true},
	}, nil)

	cat := commands.BuildSkillCategory(mock)
	result, err := cat.Dispatch([]string{"ls"}, skillCtx(t))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Items) != 1 {
		t.Fatalf("expected 1 item via ls alias, got %d", len(result.Items))
	}
}

func TestSkillCategory_List_PropagatesError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mock := service_mocks.NewMockSkillService(ctrl)
	svcErr := errors.New("filesystem error")
	mock.EXPECT().List(gomock.Any()).Return(nil, svcErr)

	cat := commands.BuildSkillCategory(mock)
	_, err := cat.Dispatch([]string{"list"}, skillCtx(t))
	if err == nil {
		t.Fatal("expected error to be propagated")
	}
	if !errors.Is(err, svcErr) {
		t.Errorf("expected service error in chain, got: %v", err)
	}
}

// ---- table vs json output -----------------------------------------------

func TestSkillCategory_List_JSONOutput(t *testing.T) {
	ctrl := gomock.NewController(t)
	mock := service_mocks.NewMockSkillService(ctrl)
	mock.EXPECT().List(gomock.Any()).Return([]service.AgentStatus{
		{Name: "claude-code", DisplayName: "Claude Code", Detected: true, Installed: false},
	}, nil)

	cat := commands.BuildSkillCategory(mock)
	ctx := skillCtx(t)

	result, err := cat.Dispatch([]string{"list"}, ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Format the same TableData via FormatAs(JSON) — verifies the
	// presentation channel can round-trip to JSON without losing data.
	out, fmtErr := ctx.Presenter.FormatAs(result.Presentation, presentation.OutputFormatJSON)
	if fmtErr != nil {
		t.Fatalf("FormatAs JSON error: %v", fmtErr)
	}
	for _, want := range []string{"claude-code", "Claude Code"} {
		if !strings.Contains(out, want) {
			t.Errorf("JSON output should contain %q:\n%s", want, out)
		}
	}
}

// ---- mutation-mode enforcement under agent mode -------------------------

func TestSkillCategory_Install_AgentMode_BlockedWithoutRW(t *testing.T) {
	// Install is ModeWrite — the dispatcher must block before the handler runs.
	ctrl := gomock.NewController(t)
	mock := service_mocks.NewMockSkillService(ctrl)
	// No EXPECT on Install — would fail the test if the handler were called.

	cat := commands.BuildSkillCategory(mock)
	ctx := skillCtx(t)
	ctx.AgentMode = true
	ctx.AllowWrites = false

	_, err := cat.Dispatch([]string{"install", "cursor"}, ctx)
	if err == nil {
		t.Fatal("expected READ_ONLY error in agent mode without --rw")
	}
	var ae *tool.AgentError
	if !errors.As(err, &ae) {
		t.Errorf("expected AgentError; got: %T %v", err, err)
	}
	if ae != nil && ae.Code != "READ_ONLY" {
		t.Errorf("expected READ_ONLY code, got %q", ae.Code)
	}
}

func TestSkillCategory_Remove_AgentMode_BlockedWithoutRW(t *testing.T) {
	ctrl := gomock.NewController(t)
	mock := service_mocks.NewMockSkillService(ctrl)
	// No EXPECT on Remove.

	cat := commands.BuildSkillCategory(mock)
	ctx := skillCtx(t)
	ctx.AgentMode = true
	ctx.AllowWrites = false

	_, err := cat.Dispatch([]string{"remove", "cursor"}, ctx)
	if err == nil {
		t.Fatal("expected READ_ONLY error in agent mode without --rw")
	}
	var ae *tool.AgentError
	if !errors.As(err, &ae) {
		t.Errorf("expected AgentError; got: %T %v", err, err)
	}
}

func TestSkillCategory_Install_AgentMode_AllowedWithRW(t *testing.T) {
	ctrl := gomock.NewController(t)
	mock := service_mocks.NewMockSkillService(ctrl)
	mock.EXPECT().Install(gomock.Any(), "cursor").Return([]service.InstallResult{
		{Agent: "cursor", Path: "/tmp/.cursor/rules/neo4j-cli/SKILL.md"},
	}, nil)

	cat := commands.BuildSkillCategory(mock)
	ctx := skillCtx(t)
	ctx.AgentMode = true
	ctx.AllowWrites = true

	result, err := cat.Dispatch([]string{"install", "cursor"}, ctx)
	if err != nil {
		t.Fatalf("unexpected error in agent+rw mode: %v", err)
	}
	if len(result.Items) != 1 || result.Items[0]["agent"] != "cursor" {
		t.Errorf("expected cursor result, got %v", result.Items)
	}
}

func TestSkillCategory_List_AgentMode_AllowedAsRead(t *testing.T) {
	// list is ModeRead — must succeed in agent mode without --rw.
	ctrl := gomock.NewController(t)
	mock := service_mocks.NewMockSkillService(ctrl)
	mock.EXPECT().List(gomock.Any()).Return([]service.AgentStatus{
		{Name: "claude-code", DisplayName: "Claude Code"},
	}, nil)

	cat := commands.BuildSkillCategory(mock)
	ctx := skillCtx(t)
	ctx.AgentMode = true
	ctx.AllowWrites = false

	result, err := cat.Dispatch([]string{"list"}, ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Items) != 1 {
		t.Errorf("expected 1 item, got %d", len(result.Items))
	}
}

// ---- no args → help -----------------------------------------------------

func TestSkillCategory_NoArgs_ReturnsHelp(t *testing.T) {
	ctrl := gomock.NewController(t)
	mock := service_mocks.NewMockSkillService(ctrl)
	// No EXPECT — bare category should not call the service.

	cat := commands.BuildSkillCategory(mock)
	result, err := cat.Dispatch(nil, skillCtx(t))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result.Message, "skill") {
		t.Errorf("expected help output, got: %q", result.Message)
	}
}
