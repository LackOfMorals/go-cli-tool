package shell_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/cli/go-cli-tool/internal/config"
	"github.com/cli/go-cli-tool/internal/logger"
	"github.com/cli/go-cli-tool/internal/shell"
	"github.com/cli/go-cli-tool/internal/tool"
)

// ---- helpers ------------------------------------------------------------

// newShell builds a minimal InteractiveShell wired with a real logger and
// a basic config, ready for Execute() tests without a live terminal.
func newShell(t *testing.T) *shell.InteractiveShell {
	t.Helper()
	s := shell.NewInteractiveShell()
	s.SetLogger(logger.NewLoggerService(logger.FormatText, logger.LevelError))
	s.SetConfig(config.Config{
		LogLevel:  "info",
		LogFormat: "text",
		Shell: config.ShellConfig{
			Prompt:      "neo4j> ",
			HistoryFile: ".neo4j_history",
		},
		Neo4j: config.Neo4jConfig{
			URI:      "bolt://localhost:7687",
			Username: "neo4j",
			Database: "neo4j",
		},
		Aura: config.AuraConfig{
			TimeoutSeconds: 30,
		},
		Telemetry: config.TelemetryConfig{Metrics: true},
	})
	s.SetVersion("1.2.3")
	return s
}

// ---- Execute: empty / whitespace ----------------------------------------

func TestExecute_Empty(t *testing.T) {
	s := newShell(t)
	out, err := s.Execute("")
	if err != nil || out != "" {
		t.Errorf("Execute(\"\") = (%q, %v), want (\"\", nil)", out, err)
	}
}

func TestExecute_Whitespace(t *testing.T) {
	s := newShell(t)
	out, err := s.Execute("   ")
	if err != nil || out != "" {
		t.Errorf("Execute(\"   \") = (%q, %v), want (\"\", nil)", out, err)
	}
}

// ---- Execute: unknown command -------------------------------------------

func TestExecute_UnknownCommand(t *testing.T) {
	s := newShell(t)
	_, err := s.Execute("nonexistent")
	if err == nil {
		t.Fatal("expected error for unknown command")
	}
	if !strings.Contains(err.Error(), "nonexistent") {
		t.Errorf("error should name the command, got: %v", err)
	}
}

// ---- Builtins: version --------------------------------------------------

func TestExecute_Version(t *testing.T) {
	s := newShell(t)
	out, err := s.Execute("version")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "1.2.3") {
		t.Errorf("expected version in output, got: %q", out)
	}
	if !strings.Contains(out, "nctl") {
		t.Errorf("expected 'nctl' in output, got: %q", out)
	}
}

// ---- Builtins: exit / quit ----------------------------------------------

func TestExecute_Exit(t *testing.T) {
	s := newShell(t)
	out, err := s.Execute("exit")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "Goodbye") {
		t.Errorf("expected goodbye message, got: %q", out)
	}
	if s.IsRunning() {
		t.Error("shell should not be running after exit")
	}
}

func TestExecute_Quit(t *testing.T) {
	s := newShell(t)
	_, err := s.Execute("quit")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.IsRunning() {
		t.Error("shell should not be running after quit")
	}
}

// ---- Builtins: help -----------------------------------------------------

func TestExecute_Help_NoArgs(t *testing.T) {
	s := newShell(t)
	out, err := s.Execute("help")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, want := range []string{"exit", "help", "config", "version"} {
		if !strings.Contains(out, want) {
			t.Errorf("help output should mention %q, got:\n%s", want, out)
		}
	}
}

func TestExecute_Help_Category(t *testing.T) {
	s := newShell(t)
	cat := shell.NewCategory("mycat", "My category description")
	s.SetCategories(map[string]*shell.Category{"mycat": cat})

	out, err := s.Execute("help mycat")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "My category description") {
		t.Errorf("expected category description in help, got: %s", out)
	}
}

func TestExecute_Help_UnknownCategory(t *testing.T) {
	s := newShell(t)
	_, err := s.Execute("help nonexistent")
	if err == nil {
		t.Fatal("expected error for unknown category")
	}
}

// ---- Builtins: config ---------------------------------------------------

func TestExecute_Config_NoConfigLoaded(t *testing.T) {
	s := shell.NewInteractiveShell()
	out, err := s.Execute("config")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "no configuration loaded") {
		t.Errorf("expected 'no configuration loaded', got: %q", out)
	}
}

func TestExecute_Config_ShowsSections(t *testing.T) {
	s := newShell(t)
	out, err := s.Execute("config")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, section := range []string{"Logging", "Shell", "Neo4j", "Aura", "Telemetry"} {
		if !strings.Contains(out, section) {
			t.Errorf("config output should contain section %q, got:\n%s", section, out)
		}
	}
}

func TestExecute_Config_PasswordRedacted(t *testing.T) {
	s := shell.NewInteractiveShell()
	s.SetConfig(config.Config{
		Neo4j: config.Neo4jConfig{Password: "supersecret"},
	})
	out, err := s.Execute("config")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(out, "supersecret") {
		t.Error("config output must not reveal the password value")
	}
	if !strings.Contains(out, "(set)") {
		t.Errorf("config output should show '(set)' for a set password, got:\n%s", out)
	}
}

func TestExecute_Config_PasswordNotSet(t *testing.T) {
	s := shell.NewInteractiveShell()
	s.SetConfig(config.Config{})
	out, err := s.Execute("config")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "(not set)") {
		t.Errorf("config output should show '(not set)' for missing password, got:\n%s", out)
	}
}

func TestExecute_Config_TelemetryEnabled(t *testing.T) {
	s := shell.NewInteractiveShell()
	s.SetConfig(config.Config{Telemetry: config.TelemetryConfig{Metrics: true}})
	out, _ := s.Execute("config")
	if !strings.Contains(out, "enabled") {
		t.Errorf("expected 'enabled' for metrics on, got:\n%s", out)
	}
}

func TestExecute_Config_TelemetryDisabled(t *testing.T) {
	s := shell.NewInteractiveShell()
	s.SetConfig(config.Config{Telemetry: config.TelemetryConfig{Metrics: false}})
	out, _ := s.Execute("config")
	if !strings.Contains(out, "disabled") {
		t.Errorf("expected 'disabled' for metrics off, got:\n%s", out)
	}
}

// ---- Builtins: set ------------------------------------------------------

func TestExecute_Set_MissingArgs(t *testing.T) {
	s := newShell(t)
	_, err := s.Execute("set")
	if err == nil {
		t.Fatal("expected usage error for 'set' with no args")
	}
}

func TestExecute_Set_UnknownKey(t *testing.T) {
	s := newShell(t)
	_, err := s.Execute("set unknownkey value")
	if err == nil {
		t.Fatal("expected error for unknown config key")
	}
}

func TestExecute_Set_LogLevel(t *testing.T) {
	log := logger.NewLoggerService(logger.FormatText, logger.LevelInfo)
	s := newShell(t)
	s.SetLogger(log)

	out, err := s.Execute("set log-level debug")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "debug") {
		t.Errorf("expected confirmation, got: %q", out)
	}
}

// ---- Builtins: log-level ------------------------------------------------

func TestExecute_LogLevel_Get(t *testing.T) {
	s := newShell(t)
	out, err := s.Execute("log-level")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "info") {
		t.Errorf("expected current log level in output, got: %q", out)
	}
}

func TestExecute_LogLevel_Set(t *testing.T) {
	s := newShell(t)
	out, err := s.Execute("log-level debug")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "debug") {
		t.Errorf("expected confirmation, got: %q", out)
	}
}

// ---- Category dispatch --------------------------------------------------

func TestExecute_DispatchesToCategory(t *testing.T) {
	s := newShell(t)

	called := false
	cat := shell.NewCategory("mycat", "test").
		AddCommand(&shell.Command{
			Name: "run",
			Handler: func(_ []string, _ shell.Context) (string, error) {
				called = true
				return "ran", nil
			},
		})
	s.SetCategories(map[string]*shell.Category{"mycat": cat})

	out, err := s.Execute("mycat run")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("category command handler was not called")
	}
	if out != "ran" {
		t.Errorf("got %q, want %q", out, "ran")
	}
}

// ---- Custom registered handlers -----------------------------------------

func TestExecute_CustomHandler(t *testing.T) {
	s := newShell(t)
	s.RegisterCommand("greet", func(args []string, _ shell.Context) (string, error) {
		return "hello " + strings.Join(args, " "), nil
	})

	out, err := s.Execute("greet world")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != "hello world" {
		t.Errorf("got %q, want %q", out, "hello world")
	}
}

// ---- Tool registry dispatch ---------------------------------------------

// stubRegistry is a minimal Registry implementation for tests.
type stubRegistry struct {
	tools map[string]tool.Tool
}

func (r *stubRegistry) Get(name string) (tool.Tool, error) {
	if t, ok := r.tools[name]; ok {
		return t, nil
	}
	return nil, errors.New("not found")
}

func (r *stubRegistry) ListNames() []string {
	names := make([]string, 0, len(r.tools))
	for n := range r.tools {
		names = append(names, n)
	}
	return names
}

// echoTool is a test tool that echoes its first argument.
type echoTool struct{ *tool.BaseTool }

func newEchoTool() *echoTool {
	return &echoTool{tool.NewBaseTool("echo", "echo", "1.0.0")}
}

func (e *echoTool) Execute(ctx tool.Context) (tool.Result, error) {
	if len(ctx.Args) == 0 {
		return tool.SuccessResult(""), nil
	}
	return tool.SuccessResult(ctx.Args[0]), nil
}

func TestExecute_DispatchesToToolRegistry(t *testing.T) {
	s := newShell(t)
	s.SetRegistry(&stubRegistry{
		tools: map[string]tool.Tool{"echo": newEchoTool()},
	})

	out, err := s.Execute("echo hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != "hello" {
		t.Errorf("got %q, want %q", out, "hello")
	}
}

// ---- Context propagation ------------------------------------------------

// contextCaptureTool records the context it was called with.
type contextCaptureTool struct {
	*tool.BaseTool
	capturedCtx context.Context
}

func (e *contextCaptureTool) Execute(ctx tool.Context) (tool.Result, error) {
	e.capturedCtx = ctx.Context
	return tool.SuccessResult("ok"), nil
}

func TestExecute_ContextPropagatedToTool(t *testing.T) {
	s := newShell(t)
	ct := &contextCaptureTool{BaseTool: tool.NewBaseTool("capture", "capture", "1.0.0")}
	s.SetRegistry(&stubRegistry{tools: map[string]tool.Tool{"capture": ct}})

	_, err := s.Execute("capture")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ct.capturedCtx == nil {
		t.Fatal("context was not propagated to tool")
	}
}

// ---- Validate called before Execute ------------------------------------

// validatingTool records whether Validate was called and can be configured
// to fail validation so we can assert the flow stops before Execute.
type validatingTool struct {
	*tool.BaseTool
	validateErr  error
	validateCalled bool
	executeCalled  bool
}

func newValidatingTool(name string, validateErr error) *validatingTool {
	return &validatingTool{
		BaseTool:    tool.NewBaseTool(name, name, "1.0.0"),
		validateErr: validateErr,
	}
}

func (v *validatingTool) Validate(_ tool.Context) error {
	v.validateCalled = true
	return v.validateErr
}

func (v *validatingTool) Execute(_ tool.Context) (tool.Result, error) {
	v.executeCalled = true
	return tool.SuccessResult("executed"), nil
}

func TestExecute_ValidateCalledBeforeExecute(t *testing.T) {
	s := newShell(t)
	vt := newValidatingTool("validator", nil)
	s.SetRegistry(&stubRegistry{tools: map[string]tool.Tool{"validator": vt}})

	_, err := s.Execute("validator")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !vt.validateCalled {
		t.Error("Validate should be called before Execute")
	}
	if !vt.executeCalled {
		t.Error("Execute should be called when Validate passes")
	}
}

func TestExecute_ValidateFailurePreventsExecute(t *testing.T) {
	s := newShell(t)
	vt := newValidatingTool("blocker", fmt.Errorf("not ready"))
	s.SetRegistry(&stubRegistry{tools: map[string]tool.Tool{"blocker": vt}})

	_, err := s.Execute("blocker")
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "not ready") {
		t.Errorf("expected validation message in error, got: %v", err)
	}
	if vt.executeCalled {
		t.Error("Execute must not be called when Validate fails")
	}
}

// ---- Priority: builtins shadow categories and handlers -----------------

func TestExecute_BuiltinShadowsCategory(t *testing.T) {
	s := newShell(t)
	// "help" is a builtin — a category named "help" should never be reached.
	reached := false
	cat := shell.NewCategory("help", "shadow").
		SetDirectHandler(func(_ []string, _ shell.Context) (string, error) {
			reached = true
			return "", nil
		})
	s.SetCategories(map[string]*shell.Category{"help": cat})

	_, err := s.Execute("help")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reached {
		t.Error("builtin 'help' should shadow same-named category")
	}
}

// ---- IsBuiltinCommand ---------------------------------------------------

func TestIsBuiltinCommand(t *testing.T) {
	builtins := []string{"exit", "quit", "help", "config", "set", "log-level", "clear", "version"}
	for _, name := range builtins {
		if !shell.IsBuiltinCommand(name) {
			t.Errorf("%q should be a builtin command", name)
		}
	}
	for _, name := range []string{"cypher", "cloud", "admin", "query", "unknown"} {
		if shell.IsBuiltinCommand(name) {
			t.Errorf("%q should NOT be a builtin command", name)
		}
	}
}
