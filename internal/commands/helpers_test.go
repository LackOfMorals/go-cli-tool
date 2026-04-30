package commands_test

import (
	"testing"

	"github.com/cli/go-cli-tool/internal/dispatch"
)

// humanOut renders a CommandResult to a human-readable string for test
// assertions. It uses the Presenter when a Presentation payload is present,
// and falls back to Message for action-only results.
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
