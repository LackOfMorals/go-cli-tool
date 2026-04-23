package shell_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/cli/go-cli-tool/internal/shell"
	"github.com/cli/go-cli-tool/internal/tool"
)

// ---- helpers ------------------------------------------------------------

func newTestCategory(name string) *shell.Category {
	return shell.NewCategory(name, name+" description")
}

func blankCtx() shell.ShellContext {
	return shell.ShellContext{Context: context.Background()}
}

// ---- NewCategory --------------------------------------------------------

func TestNewCategory(t *testing.T) {
	c := shell.NewCategory("test", "a description")
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
	out, err := c.Dispatch(nil, blankCtx())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "mycat") {
		t.Errorf("expected help to mention category name, got: %s", out)
	}
}

func TestDispatch_NoArgs_WithDirectHandler_ReturnsUsageError(t *testing.T) {
	c := newTestCategory("cypher").
		SetDirectHandler(func(args []string, ctx shell.ShellContext) (string, error) {
			return "direct", nil
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
		AddCommand(&shell.Command{
			Name:    "show-users",
			Handler: func(args []string, ctx shell.ShellContext) (string, error) { called = true; return "users", nil },
		})

	out, err := c.Dispatch([]string{"show-users"}, blankCtx())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("command handler was not called")
	}
	if out != "users" {
		t.Errorf("got %q, want %q", out, "users")
	}
}

func TestDispatch_NamedCommand_PassesRestArgs(t *testing.T) {
	var gotArgs []string
	c := newTestCategory("root").
		AddCommand(&shell.Command{
			Name: "get",
			Handler: func(args []string, ctx shell.ShellContext) (string, error) {
				gotArgs = args
				return "", nil
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
		SetDirectHandler(func(args []string, ctx shell.ShellContext) (string, error) {
			gotArgs = args
			return "ok", nil
		})

	out, err := c.Dispatch([]string{"MATCH", "(n)", "RETURN", "n"}, blankCtx())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != "ok" {
		t.Errorf("got %q, want %q", out, "ok")
	}
	if len(gotArgs) != 4 {
		t.Errorf("expected 4 args forwarded to direct handler, got %v", gotArgs)
	}
}

// ---- Dispatch: subcategory ----------------------------------------------

func TestDispatch_Subcategory(t *testing.T) {
	called := false
	sub := newTestCategory("instances").
		AddCommand(&shell.Command{
			Name:    "list",
			Handler: func(args []string, ctx shell.ShellContext) (string, error) { called = true; return "list", nil },
		})

	c := newTestCategory("cloud").AddSubcategory(sub)

	out, err := c.Dispatch([]string{"instances", "list"}, blankCtx())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("sub-category command handler not called")
	}
	if out != "list" {
		t.Errorf("got %q, want %q", out, "list")
	}
}

func TestDispatch_Subcategory_NoArgs_ReturnsSubHelp(t *testing.T) {
	sub := shell.NewCategory("instances", "Manage instances")
	c := newTestCategory("cloud").AddSubcategory(sub)

	out, err := c.Dispatch([]string{"instances"}, blankCtx())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "instances") {
		t.Errorf("expected sub-category help, got: %s", out)
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
	c := shell.NewCategory("admin", "Admin operations")
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
		AddCommand(&shell.Command{Name: "show-users", Usage: "show-users", Description: "List users"}).
		AddCommand(&shell.Command{Name: "show-databases", Usage: "show-databases", Description: "List databases"})

	help := c.Help()
	if !strings.Contains(help, "show-users") {
		t.Errorf("help should list show-users, got: %s", help)
	}
	if !strings.Contains(help, "show-databases") {
		t.Errorf("help should list show-databases, got: %s", help)
	}
}

func TestHelp_ListsSubcategories(t *testing.T) {
	sub := shell.NewCategory("instances", "Manage instances")
	c := newTestCategory("cloud").AddSubcategory(sub)

	help := c.Help()
	if !strings.Contains(help, "instances") {
		t.Errorf("help should list sub-category, got: %s", help)
	}
}

// ---- Prerequisites -----------------------------------------------------

func TestPrerequisite_BlocksDispatch(t *testing.T) {
	c := newTestCategory("admin").
		AddCommand(&shell.Command{
			Name:    "show-users",
			Handler: func(args []string, ctx shell.ShellContext) (string, error) { return "ok", nil },
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
		AddCommand(&shell.Command{
			Name:    "show-users",
			Handler: func(args []string, ctx shell.ShellContext) (string, error) { called = true; return "ok", nil },
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
	// Typing a bare category name (e.g. "admin") should show help even when
	// the prerequisite would fail — the user may just want to see what commands
	// are available before configuring the connection.
	c := newTestCategory("admin").
		SetPrerequisite(func() error {
			return fmt.Errorf("no database")
		})

	out, err := c.Dispatch(nil, blankCtx())
	if err != nil {
		t.Fatalf("bare category name should show help, not error: %v", err)
	}
	if !strings.Contains(out, "admin") {
		t.Errorf("expected help output, got: %s", out)
	}
}

func TestPrerequisite_DirectHandlerNoArgsShowsUsage(t *testing.T) {
	// Direct-handler categories (like cypher) return a usage error on no args,
	// not the help text — but they still should NOT run the prerequisite.
	prereqCalled := false
	c := newTestCategory("cypher").
		SetDirectHandler(func(args []string, ctx shell.ShellContext) (string, error) {
			return "ok", nil
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
	// Verify that the error returned by a prerequisite survives the dispatch
	// chain intact so callers can use errors.Is(err, tool.ErrPrerequisite).
	c := newTestCategory("test").
		AddCommand(&shell.Command{
			Name:    "cmd",
			Handler: func(args []string, ctx shell.ShellContext) (string, error) { return "", nil },
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
		AddCommand(&shell.Command{
			Name:    "show-users",
			Aliases: []string{"su", "users"},
			Handler: func(args []string, ctx shell.ShellContext) (string, error) {
				called = true
				return "users", nil
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
		AddCommand(&shell.Command{Name: "list", Aliases: []string{"ls"}}).
		AddCommand(&shell.Command{Name: "delete", Aliases: []string{"rm", "del"}})

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
		AddCommand(&shell.Command{Name: "list", Aliases: []string{"ls"}}).
		AddCommand(&shell.Command{Name: "delete", Aliases: []string{"rm"}})

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
		AddCommand(&shell.Command{
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
		AddCommand(&shell.Command{Name: "zzz"}).
		AddCommand(&shell.Command{Name: "aaa"}).
		AddCommand(&shell.Command{Name: "mmm"})

	names := c.CommandNames()
	want := []string{"aaa", "mmm", "zzz"}
	for i, n := range names {
		if n != want[i] {
			t.Errorf("names[%d] = %q, want %q", i, n, want[i])
		}
	}
}

// ---- Context propagation ------------------------------------------------

func TestDispatch_ContextPropagated(t *testing.T) {
	type key struct{}
	ctx := context.WithValue(context.Background(), key{}, "sentinel")
	sc := shell.ShellContext{Context: ctx}

	var gotCtx context.Context
	c := newTestCategory("root").
		AddCommand(&shell.Command{
			Name: "check",
			Handler: func(args []string, ctx shell.ShellContext) (string, error) {
				gotCtx = ctx.Context
				return "", nil
			},
		})

	_, _ = c.Dispatch([]string{"check"}, sc)
	if gotCtx == nil {
		t.Fatal("context not received by handler")
	}
	if gotCtx.Value(key{}) != "sentinel" {
		t.Error("context value not propagated to handler")
	}
}
