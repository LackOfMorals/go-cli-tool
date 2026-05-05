package shell_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/cli/go-cli-tool/internal/dispatch"
	"github.com/cli/go-cli-tool/internal/presentation"
	"github.com/cli/go-cli-tool/internal/shell"
)

// ---- stub presenter --------------------------------------------------------

// stubPresenter is a minimal presentation.Service whose Format / FormatAs
// methods record the data they receive and return a fixed string.
type stubPresenter struct {
	lastData       interface{}
	lastFormat     presentation.OutputFormat
	returnString   string
	returnErr      error
}

func (s *stubPresenter) Format(data interface{}) (string, error) {
	s.lastData = data
	s.lastFormat = ""
	if s.returnErr != nil {
		return "", s.returnErr
	}
	return s.returnString, nil
}

func (s *stubPresenter) FormatAs(data interface{}, format presentation.OutputFormat) (string, error) {
	s.lastData = data
	s.lastFormat = format
	if s.returnErr != nil {
		return "", s.returnErr
	}
	return s.returnString, nil
}

func (s *stubPresenter) RegisterFormatter(_ presentation.OutputFormat, _ presentation.OutputFormatter) error {
	return nil
}

func (s *stubPresenter) SetFormat(_ presentation.OutputFormat) error {
	return nil
}

// ---- helpers ---------------------------------------------------------------

// identityCtxFor is a ctxFor function that builds a bare dispatch.Context
// carrying only the context.Context from the shell.Context.
func identityCtxFor(sc shell.Context) dispatch.Context {
	return dispatch.Context{Context: sc.Context}
}

// shellCtxWith returns a minimal shell.Context optionally wired with a presenter.
func shellCtxWith(p presentation.Service) shell.Context {
	return shell.Context{
		Context:   context.Background(),
		Presenter: p,
	}
}

// ---- BridgeCategory: structure ---------------------------------------------

func TestBridgeCategory_NameAndDescription(t *testing.T) {
	src := dispatch.NewCategory("mycat", "my description")
	bridged := shell.BridgeCategory(src, identityCtxFor)

	if bridged.Name != "mycat" {
		t.Errorf("Name: got %q, want %q", bridged.Name, "mycat")
	}
	if bridged.Description != "my description" {
		t.Errorf("Description: got %q, want %q", bridged.Description, "my description")
	}
}

func TestBridgeCategory_RecursiveSubcategory(t *testing.T) {
	sub := dispatch.NewCategory("instances", "Manage instances").
		AddCommand(&dispatch.Command{
			Name: "list",
			Handler: func(_ []string, _ dispatch.Context) (dispatch.CommandResult, error) {
				return dispatch.MessageResult("listed"), nil
			},
		})
	src := dispatch.NewCategory("cloud", "Cloud ops").AddSubcategory(sub)

	bridged := shell.BridgeCategory(src, identityCtxFor)

	instCat := bridged.Subcat("instances")
	if instCat == nil {
		t.Fatal("expected bridged sub-category 'instances', got nil")
	}
	if instCat.Name != "instances" {
		t.Errorf("sub-category Name: got %q, want %q", instCat.Name, "instances")
	}

	// The bridged sub-category command should be callable.
	out, err := instCat.Dispatch([]string{"list"}, shellCtxWith(nil))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != "listed" {
		t.Errorf("got %q, want %q", out, "listed")
	}
}

// ---- BridgeCategory: MessageResult -----------------------------------------

func TestBridgeCategory_MessageResult(t *testing.T) {
	src := dispatch.NewCategory("test", "test category").
		AddCommand(&dispatch.Command{
			Name: "greet",
			Handler: func(_ []string, _ dispatch.Context) (dispatch.CommandResult, error) {
				return dispatch.MessageResult("hello world"), nil
			},
		})

	bridged := shell.BridgeCategory(src, identityCtxFor)
	out, err := bridged.Dispatch([]string{"greet"}, shellCtxWith(nil))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != "hello world" {
		t.Errorf("got %q, want %q", out, "hello world")
	}
}

// ---- BridgeCategory: Presentation result -----------------------------------

func TestBridgeCategory_PresentationResult_CallsFormat(t *testing.T) {
	tableData := presentation.NewTableData(
		[]string{"id", "name"},
		[][]interface{}{{"1", "alice"}},
	)

	src := dispatch.NewCategory("test", "test").
		AddCommand(&dispatch.Command{
			Name: "list",
			Handler: func(_ []string, _ dispatch.Context) (dispatch.CommandResult, error) {
				return dispatch.ListResult(tableData, nil), nil
			},
		})

	p := &stubPresenter{returnString: "formatted table"}
	bridged := shell.BridgeCategory(src, identityCtxFor)
	out, err := bridged.Dispatch([]string{"list"}, shellCtxWith(p))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != "formatted table" {
		t.Errorf("got %q, want %q", out, "formatted table")
	}
	if p.lastData != tableData {
		t.Error("presenter.Format was not called with the expected data")
	}
}

func TestBridgeCategory_PresentationResult_WithFormatOverride(t *testing.T) {
	tableData := presentation.NewTableData([]string{"x"}, [][]interface{}{{"y"}})

	src := dispatch.NewCategory("test", "test").
		AddCommand(&dispatch.Command{
			Name: "run",
			Handler: func(_ []string, _ dispatch.Context) (dispatch.CommandResult, error) {
				return dispatch.CommandResult{
					Presentation:   tableData,
					FormatOverride: "json",
				}, nil
			},
		})

	p := &stubPresenter{returnString: `{"x":"y"}`}
	bridged := shell.BridgeCategory(src, identityCtxFor)
	out, err := bridged.Dispatch([]string{"run"}, shellCtxWith(p))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != `{"x":"y"}` {
		t.Errorf("got %q, want %q", out, `{"x":"y"}`)
	}
	if p.lastFormat != "json" {
		t.Errorf("FormatAs called with format %q, want %q", p.lastFormat, "json")
	}
}

func TestBridgeCategory_PresentationResult_NilPresenter_FallsBackToMessage(t *testing.T) {
	tableData := presentation.NewTableData([]string{"x"}, [][]interface{}{{"y"}})

	src := dispatch.NewCategory("test", "test").
		AddCommand(&dispatch.Command{
			Name: "run",
			Handler: func(_ []string, _ dispatch.Context) (dispatch.CommandResult, error) {
				return dispatch.CommandResult{
					Presentation: tableData,
					Message:      "fallback",
				}, nil
			},
		})

	// No presenter wired.
	bridged := shell.BridgeCategory(src, identityCtxFor)
	out, err := bridged.Dispatch([]string{"run"}, shellCtxWith(nil))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != "fallback" {
		t.Errorf("got %q, want %q", out, "fallback")
	}
}

// ---- BridgeCategory: handler error -----------------------------------------

func TestBridgeCategory_HandlerError_Propagates(t *testing.T) {
	wantErr := fmt.Errorf("service unavailable")

	src := dispatch.NewCategory("test", "test").
		AddCommand(&dispatch.Command{
			Name: "fail",
			Handler: func(_ []string, _ dispatch.Context) (dispatch.CommandResult, error) {
				return dispatch.CommandResult{}, wantErr
			},
		})

	bridged := shell.BridgeCategory(src, identityCtxFor)
	_, err := bridged.Dispatch([]string{"fail"}, shellCtxWith(nil))
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, wantErr) {
		t.Errorf("got error %v, want %v in chain", err, wantErr)
	}
}

// ---- BridgeCategory: prerequisite ------------------------------------------

func TestBridgeCategory_PrerequisiteFailure_BlocksHandler(t *testing.T) {
	handlerCalled := false
	prereqErr := fmt.Errorf("no connection")

	src := dispatch.NewCategory("test", "test").
		AddCommand(&dispatch.Command{
			Name: "action",
			Handler: func(_ []string, _ dispatch.Context) (dispatch.CommandResult, error) {
				handlerCalled = true
				return dispatch.MessageResult("ok"), nil
			},
		}).
		SetPrerequisite(func() error {
			return prereqErr
		})

	bridged := shell.BridgeCategory(src, identityCtxFor)
	_, err := bridged.Dispatch([]string{"action"}, shellCtxWith(nil))
	if err == nil {
		t.Fatal("expected prerequisite error, got nil")
	}
	if !errors.Is(err, prereqErr) {
		t.Errorf("got error %v, want %v in chain", err, prereqErr)
	}
	if handlerCalled {
		t.Error("handler was called despite failing prerequisite")
	}
}

func TestBridgeCategory_PrerequisitePasses_HandlerCalled(t *testing.T) {
	handlerCalled := false

	src := dispatch.NewCategory("test", "test").
		AddCommand(&dispatch.Command{
			Name: "action",
			Handler: func(_ []string, _ dispatch.Context) (dispatch.CommandResult, error) {
				handlerCalled = true
				return dispatch.MessageResult("done"), nil
			},
		}).
		SetPrerequisite(func() error { return nil })

	bridged := shell.BridgeCategory(src, identityCtxFor)
	out, err := bridged.Dispatch([]string{"action"}, shellCtxWith(nil))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !handlerCalled {
		t.Error("handler was not called when prerequisite passed")
	}
	if out != "done" {
		t.Errorf("got %q, want %q", out, "done")
	}
}

func TestBridgeCategory_NoPrerequisite_HandlerCalled(t *testing.T) {
	handlerCalled := false

	src := dispatch.NewCategory("test", "test").
		AddCommand(&dispatch.Command{
			Name: "action",
			Handler: func(_ []string, _ dispatch.Context) (dispatch.CommandResult, error) {
				handlerCalled = true
				return dispatch.MessageResult("done"), nil
			},
		})
	// No prerequisite set.

	bridged := shell.BridgeCategory(src, identityCtxFor)
	_, err := bridged.Dispatch([]string{"action"}, shellCtxWith(nil))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !handlerCalled {
		t.Error("handler was not called")
	}
}

// ---- BridgeCategory: ctxFor propagation ------------------------------------

func TestBridgeCategory_CtxForReceivesShellContext(t *testing.T) {
	type key struct{}
	val := "sentinel"

	var capturedDispCtx dispatch.Context
	ctxFor := func(sc shell.Context) dispatch.Context {
		return dispatch.Context{Context: sc.Context}
	}

	src := dispatch.NewCategory("test", "test").
		AddCommand(&dispatch.Command{
			Name: "check",
			Handler: func(_ []string, dctx dispatch.Context) (dispatch.CommandResult, error) {
				capturedDispCtx = dctx
				return dispatch.MessageResult("ok"), nil
			},
		})

	shellCtx := shell.Context{
		Context: context.WithValue(context.Background(), key{}, val),
	}

	bridged := shell.BridgeCategory(src, ctxFor)
	_, err := bridged.Dispatch([]string{"check"}, shellCtx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedDispCtx.Context == nil {
		t.Fatal("dispatch.Context.Context was nil")
	}
	if capturedDispCtx.Context.Value(key{}) != val {
		t.Error("shell context value was not forwarded to dispatch context via ctxFor")
	}
}
