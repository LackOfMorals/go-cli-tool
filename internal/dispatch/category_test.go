package dispatch_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/cli/go-cli-tool/internal/dispatch"
	"github.com/cli/go-cli-tool/internal/tool"
)

// ---- helpers ------------------------------------------------------------

func newTestCategory(name string) *dispatch.Category {
	return dispatch.NewCategory(name, name+" description")
}

func blankCtx() dispatch.Context {
	return dispatch.Context{Context: context.Background()}
}

// ---- NewCategory --------------------------------------------------------

func TestNewCategory(t *testing.T) {
	c := dispatch.NewCategory("test", "a description")
	if c.Name != "test" {
		t.Errorf("Name: got %q, want %q", c.Name, "test")
	}
	if c.Description != "a description" {
		t.Errorf("Description: got %q, want %q", c.Description, "a description")
	}
}

// ---- Dispatch: no args --------------------------------------------------

func TestDispatch_NoArgs_NoDirectHandler_ReturnsHelp(t *testing.T) {
	c := newTestCategory("mycat")
	result, err := c.Dispatch(nil, blankCtx())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result.Message, "mycat") {
		t.Errorf("expected help to mention category name, got: %s", result.Message)
	}
}

func TestDispatch_NoArgs_WithDirectHandler_ReturnsUsageError(t *testing.T) {
	c := newTestCategory("cypher").
		SetDirectHandler(func(_ []string, _ dispatch.Context) (dispatch.CommandResult, error) {
			return dispatch.MessageResult("direct"), nil
		})
	_, err := c.Dispatch(nil, blankCtx())
	if err == nil {
		t.Fatal("expected usage error when calling direct handler with no args")
	}
	if !strings.Contains(err.Error(), "usage") {
		t.Errorf("expected usage error, got: %v", err)
	}
}

// ---- Dispatch: named command --------------------------------------------

func TestDispatch_NamedCommand(t *testing.T) {
	called := false
	c := newTestCategory("admin").
		AddCommand(&dispatch.Command{
			Name:    "show-users",
			Handler: func(_ []string, _ dispatch.Context) (dispatch.CommandResult, error) { called = true; return dispatch.MessageResult("users"), nil },
		})

	result, err := c.Dispatch([]string{"show-users"}, blankCtx())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("command handler was not called")
	}
	if result.Message != "users" {
		t.Errorf("got %q, want %q", result.Message, "users")
	}
}

func TestDispatch_NamedCommand_PassesRestArgs(t *testing.T) {
	var gotArgs []string
	c := newTestCategory("root").
		AddCommand(&dispatch.Command{
			Name: "get",
			Handler: func(args []string, _ dispatch.Context) (dispatch.CommandResult, error) {
				gotArgs = args
				return dispatch.CommandResult{}, nil
			},
		})

	_, _ = c.Dispatch([]string{"get", "id-123"}, blankCtx())
	if len(gotArgs) != 1 || gotArgs[0] != "id-123" {
		t.Errorf("got args %v, want [id-123]", gotArgs)
	}
}

// ---- Dispatch: direct handler -------------------------------------------

func TestDispatch_DirectHandler_CalledWhenNoCommandMatches(t *testing.T) {
	var gotArgs []string
	c := newTestCategory("cypher").
		SetDirectHandler(func(args []string, _ dispatch.Context) (dispatch.CommandResult, error) {
			gotArgs = args
			return dispatch.MessageResult("ok"), nil
		})

	result, err := c.Dispatch([]string{"MATCH", "(n)", "RETURN", "n"}, blankCtx())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Message != "ok" {
		t.Errorf("got %q, want %q", result.Message, "ok")
	}
	if len(gotArgs) != 4 {
		t.Errorf("expected 4 args forwarded to direct handler, got %v", gotArgs)
	}
}

// ---- Dispatch: subcategory ----------------------------------------------

func TestDispatch_Subcategory(t *testing.T) {
	called := false
	sub := newTestCategory("instances").
		AddCommand(&dispatch.Command{
			Name:    "list",
			Handler: func(_ []string, _ dispatch.Context) (dispatch.CommandResult, error) { called = true; return dispatch.MessageResult("list"), nil },
		})

	c := newTestCategory("cloud").AddSubcategory(sub)

	result, err := c.Dispatch([]string{"instances", "list"}, blankCtx())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("sub-category command handler not called")
	}
	if result.Message != "list" {
		t.Errorf("got %q, want %q", result.Message, "list")
	}
}

func TestDispatch_Subcategory_NoArgs_ReturnsSubHelp(t *testing.T) {
	sub := dispatch.NewCategory("instances", "Manage instances")
	c := newTestCategory("cloud").AddSubcategory(sub)

	result, err := c.Dispatch([]string{"instances"}, blankCtx())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result.Message, "instances") {
		t.Errorf("expected sub-category help, got: %s", result.Message)
	}
}

// ---- Dispatch: unknown command ------------------------------------------

func TestDispatch_UnknownCommand_ReturnsError(t *testing.T) {
	c := newTestCategory("admin")
	_, err := c.Dispatch([]string{"nonexistent"}, blankCtx())
	if err == nil {
		t.Fatal("expected error for unknown command")
	}
	if !strings.Contains(err.Error(), "nonexistent") {
		t.Errorf("error should mention the unknown command, got: %v", err)
	}
}

// ---- Find ---------------------------------------------------------------

func TestFind_Self(t *testing.T) {
	c := newTestCategory("root")
	result := c.Find(nil)
	if result != c {
		t.Error("Find(nil) should return self")
	}
}

func TestFind_ExistingSubcat(t *testing.T) {
	sub := newTestCategory("instances")
	c := newTestCategory("cloud").AddSubcategory(sub)

	result := c.Find([]string{"instances"})
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.Name != "instances" {
		t.Errorf("got %q, want %q", result.Name, "instances")
	}
}

func TestFind_Nonexistent(t *testing.T) {
	c := newTestCategory("cloud")
	result := c.Find([]string{"nonexistent"})
	if result != nil {
		t.Errorf("expected nil for nonexistent path, got %v", result)
	}
}

// ---- Help ---------------------------------------------------------------

func TestHelp_ContainsNameAndDescription(t *testing.T) {
	c := dispatch.NewCategory("admin", "Admin operations")
	help := c.Help()
	if !strings.Contains(help, "admin") {
		t.Errorf("help should contain category name, got: %s", help)
	}
	if !strings.Contains(help, "Admin operations") {
		t.Errorf("help should contain description, got: %s", help)
	}
}

func TestHelp_ListsCommands(t *testing.T) {
	c := newTestCategory("admin").
		AddCommand(&dispatch.Command{Name: "show-users", Usage: "show-users", Description: "List users"}).
		AddCommand(&dispatch.Command{Name: "show-databases", Usage: "show-databases", Description: "List databases"})

	help := c.Help()
	if !strings.Contains(help, "show-users") {
		t.Errorf("help should list show-users, got: %s", help)
	}
	if !strings.Contains(help, "show-databases") {
		t.Errorf("help should list show-databases, got: %s", help)
	}
}

func TestHelp_ListsSubcategories(t *testing.T) {
	sub := dispatch.NewCategory("instances", "Manage instances")
	c := newTestCategory("cloud").AddSubcategory(sub)

	help := c.Help()
	if !strings.Contains(help, "instances") {
		t.Errorf("help should list sub-category, got: %s", help)
	}
}

// ---- Prerequisites -----------------------------------------------------

func TestPrerequisite_BlocksDispatch(t *testing.T) {
	c := newTestCategory("admin").
		AddCommand(&dispatch.Command{
			Name:    "show-users",
			Handler: func(_ []string, _ dispatch.Context) (dispatch.CommandResult, error) { return dispatch.MessageResult("ok"), nil },
		}).
		SetPrerequisite(func() error {
			return fmt.Errorf("database not configured")
		})

	_, err := c.Dispatch([]string{"show-users"}, blankCtx())
	if err == nil {
		t.Fatal("expected prerequisite error")
	}
	if !strings.Contains(err.Error(), "database not configured") {
		t.Errorf("expected prerequisite message in error, got: %v", err)
	}
}

func TestPrerequisite_PassesWhenMet(t *testing.T) {
	called := false
	c := newTestCategory("admin").
		AddCommand(&dispatch.Command{
			Name:    "show-users",
			Handler: func(_ []string, _ dispatch.Context) (dispatch.CommandResult, error) { called = true; return dispatch.MessageResult("ok"), nil },
		}).
		SetPrerequisite(func() error { return nil })

	_, err := c.Dispatch([]string{"show-users"}, blankCtx())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("handler should be called when prerequisite passes")
	}
}

func TestPrerequisite_NoArgsAlwaysShowsHelp(t *testing.T) {
	c := newTestCategory("admin").
		SetPrerequisite(func() error {
			return fmt.Errorf("no database")
		})

	result, err := c.Dispatch(nil, blankCtx())
	if err != nil {
		t.Fatalf("bare category name should show help, not error: %v", err)
	}
	if !strings.Contains(result.Message, "admin") {
		t.Errorf("expected help output, got: %s", result.Message)
	}
}

func TestPrerequisite_DirectHandlerNoArgsShowsUsage(t *testing.T) {
	prereqCalled := false
	c := newTestCategory("cypher").
		SetDirectHandler(func(_ []string, _ dispatch.Context) (dispatch.CommandResult, error) {
			return dispatch.MessageResult("ok"), nil
		}).
		SetPrerequisite(func() error {
			prereqCalled = true
			return fmt.Errorf("no database")
		})

	_, err := c.Dispatch(nil, blankCtx())
	if err == nil {
		t.Fatal("expected usage error from direct handler")
	}
	if prereqCalled {
		t.Error("prerequisite should not be called when no args are provided")
	}
}

func TestPrerequisite_WrapsErrPrerequisite(t *testing.T) {
	c := newTestCategory("test").
		AddCommand(&dispatch.Command{
			Name:    "cmd",
			Handler: func(_ []string, _ dispatch.Context) (dispatch.CommandResult, error) { return dispatch.CommandResult{}, nil },
		}).
		SetPrerequisite(func() error {
			return fmt.Errorf("%w: something missing", tool.ErrPrerequisite)
		})

	_, err := c.Dispatch([]string{"cmd"}, blankCtx())
	if !errors.Is(err, tool.ErrPrerequisite) {
		t.Errorf("expected tool.ErrPrerequisite in error chain, got: %v", err)
	}
}

// ---- Aliases -----------------------------------------------------------

func TestDispatch_Alias(t *testing.T) {
	called := false
	c := newTestCategory("admin").
		AddCommand(&dispatch.Command{
			Name:    "show-users",
			Aliases: []string{"su", "users"},
			Handler: func(_ []string, _ dispatch.Context) (dispatch.CommandResult, error) {
				called = true
				return dispatch.MessageResult("users"), nil
			},
		})

	for _, input := range []string{"show-users", "su", "users"} {
		called = false
		_, err := c.Dispatch([]string{input}, blankCtx())
		if err != nil {
			t.Fatalf("Dispatch(%q): unexpected error: %v", input, err)
		}
		if !called {
			t.Errorf("Dispatch(%q): handler not called", input)
		}
	}
}

func TestCommandNames_ExcludesAliases(t *testing.T) {
	c := newTestCategory("root").
		AddCommand(&dispatch.Command{Name: "list", Aliases: []string{"ls"}}).
		AddCommand(&dispatch.Command{Name: "delete", Aliases: []string{"rm", "del"}})

	names := c.CommandNames()
	want := []string{"delete", "list"}
	if len(names) != len(want) {
		t.Fatalf("CommandNames() = %v, want %v", names, want)
	}
	for i, n := range names {
		if n != want[i] {
			t.Errorf("names[%d] = %q, want %q", i, n, want[i])
		}
	}
}

func TestAllCommandNames_IncludesAliases(t *testing.T) {
	c := newTestCategory("root").
		AddCommand(&dispatch.Command{Name: "list", Aliases: []string{"ls"}}).
		AddCommand(&dispatch.Command{Name: "delete", Aliases: []string{"rm"}})

	names := c.AllCommandNames()
	want := []string{"delete", "list", "ls", "rm"}
	if len(names) != len(want) {
		t.Fatalf("AllCommandNames() = %v, want %v", names, want)
	}
	for i, n := range names {
		if n != want[i] {
			t.Errorf("names[%d] = %q, want %q", i, n, want[i])
		}
	}
}

func TestHelp_ShowsAliases(t *testing.T) {
	c := newTestCategory("admin").
		AddCommand(&dispatch.Command{
			Name:        "show-users",
			Aliases:     []string{"su"},
			Description: "List all users",
		})

	help := c.Help()
	if !strings.Contains(help, "su") {
		t.Errorf("help should show alias 'su', got:\n%s", help)
	}
}

// ---- CommandNames / SubcategoryNames sorted ----------------------------

func TestSubcategoryNames_Sorted(t *testing.T) {
	c := newTestCategory("root").
		AddSubcategory(newTestCategory("zebra")).
		AddSubcategory(newTestCategory("alpha")).
		AddSubcategory(newTestCategory("middle"))

	names := c.SubcategoryNames()
	want := []string{"alpha", "middle", "zebra"}
	for i, n := range names {
		if n != want[i] {
			t.Errorf("names[%d] = %q, want %q", i, n, want[i])
		}
	}
}

func TestCommandNames_Sorted(t *testing.T) {
	c := newTestCategory("root").
		AddCommand(&dispatch.Command{Name: "zzz"}).
		AddCommand(&dispatch.Command{Name: "aaa"}).
		AddCommand(&dispatch.Command{Name: "mmm"})

	names := c.CommandNames()
	want := []string{"aaa", "mmm", "zzz"}
	for i, n := range names {
		if n != want[i] {
			t.Errorf("names[%d] = %q, want %q", i, n, want[i])
		}
	}
}

// ---- MutationMode enforcement ------------------------------------------

func agentDispatchCtx(agentMode, allowWrites bool) dispatch.Context {
	return dispatch.Context{
		Context:     context.Background(),
		AgentMode:   agentMode,
		AllowWrites: allowWrites,
	}
}

func TestDispatch_ModeWrite_BlockedInAgentMode(t *testing.T) {
	called := false
	c := newTestCategory("cloud").
		AddCommand(&dispatch.Command{
			Name:         "delete",
			MutationMode: tool.ModeWrite,
			Handler: func(_ []string, _ dispatch.Context) (dispatch.CommandResult, error) {
				called = true
				return dispatch.MessageResult("deleted"), nil
			},
		})

	_, err := c.Dispatch([]string{"delete"}, agentDispatchCtx(true, false))
	if err == nil {
		t.Fatal("expected READ_ONLY error in agent mode without --rw")
	}
	if called {
		t.Error("handler must not be called when blocked")
	}
	var ae *tool.AgentError
	if !errors.As(err, &ae) {
		t.Errorf("expected AgentError; got: %T %v", err, err)
	}
	if ae != nil && ae.Code != "READ_ONLY" {
		t.Errorf("expected code READ_ONLY; got: %q", ae.Code)
	}
}

func TestDispatch_ModeWrite_AllowedWith_RW(t *testing.T) {
	called := false
	c := newTestCategory("cloud").
		AddCommand(&dispatch.Command{
			Name:         "delete",
			MutationMode: tool.ModeWrite,
			Handler: func(_ []string, _ dispatch.Context) (dispatch.CommandResult, error) {
				called = true
				return dispatch.MessageResult("deleted"), nil
			},
		})

	_, err := c.Dispatch([]string{"delete"}, agentDispatchCtx(true, true))
	if err != nil {
		t.Fatalf("unexpected error with --rw: %v", err)
	}
	if !called {
		t.Error("handler should be called when --rw is set")
	}
}

func TestDispatch_ModeWrite_AllowedOutsideAgentMode(t *testing.T) {
	called := false
	c := newTestCategory("cloud").
		AddCommand(&dispatch.Command{
			Name:         "delete",
			MutationMode: tool.ModeWrite,
			Handler: func(_ []string, _ dispatch.Context) (dispatch.CommandResult, error) {
				called = true
				return dispatch.MessageResult("deleted"), nil
			},
		})

	_, err := c.Dispatch([]string{"delete"}, blankCtx())
	if err != nil {
		t.Fatalf("unexpected error outside agent mode: %v", err)
	}
	if !called {
		t.Error("handler should be called outside agent mode regardless of MutationMode")
	}
}

func TestDispatch_ModeRead_NeverBlocked(t *testing.T) {
	called := false
	c := newTestCategory("cloud").
		AddCommand(&dispatch.Command{
			Name:         "list",
			MutationMode: tool.ModeRead,
			Handler: func(_ []string, _ dispatch.Context) (dispatch.CommandResult, error) {
				called = true
				return dispatch.MessageResult("list"), nil
			},
		})

	_, err := c.Dispatch([]string{"list"}, agentDispatchCtx(true, false))
	if err != nil {
		t.Fatalf("read command should never be blocked; got: %v", err)
	}
	if !called {
		t.Error("read command handler should always be called")
	}
}

func TestDispatch_ModeConditional_NotBlockedByDispatcher(t *testing.T) {
	called := false
	c := newTestCategory("cypher").
		AddCommand(&dispatch.Command{
			Name:         "query",
			MutationMode: tool.ModeConditional,
			Handler: func(_ []string, _ dispatch.Context) (dispatch.CommandResult, error) {
				called = true
				return dispatch.MessageResult("result"), nil
			},
		})

	_, err := c.Dispatch([]string{"query"}, agentDispatchCtx(true, false))
	if err != nil {
		t.Fatalf("dispatcher should not block ModeConditional; got: %v", err)
	}
	if !called {
		t.Error("ModeConditional handler must be called by dispatcher")
	}
}

// ---- Context propagation ------------------------------------------------

func TestDispatch_ContextPropagated(t *testing.T) {
	type key struct{}
	ctx := context.WithValue(context.Background(), key{}, "sentinel")
	dc := dispatch.Context{Context: ctx}

	var gotCtx context.Context
	c := newTestCategory("root").
		AddCommand(&dispatch.Command{
			Name: "check",
			Handler: func(_ []string, ctx dispatch.Context) (dispatch.CommandResult, error) {
				gotCtx = ctx.Context
				return dispatch.CommandResult{}, nil
			},
		})

	_, _ = c.Dispatch([]string{"check"}, dc)
	if gotCtx == nil {
		t.Fatal("context not received by handler")
	}
	if gotCtx.Value(key{}) != "sentinel" {
		t.Error("context value not propagated to handler")
	}
}
