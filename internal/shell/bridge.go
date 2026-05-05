package shell

import (
	"github.com/cli/go-cli-tool/internal/dispatch"
)

// BridgeCategory wraps a dispatch.Category tree as a shell.Category tree so
// that dispatch commands can be invoked from the interactive shell without
// duplication of handler logic.
//
// ctxFor converts the shell.Context available at dispatch time into the
// dispatch.Context expected by the underlying handlers. The caller is
// responsible for building the appropriate dispatch.Context (agent mode flags,
// request ID, etc.).
//
// The bridged category tree is built once at startup; it is safe for
// concurrent read use after BridgeCategory returns.
func BridgeCategory(cat *dispatch.Category, ctxFor func(Context) dispatch.Context) *Category {
	bridged := NewCategory(cat.Name, cat.Description)

	// Forward the prerequisite so the shell honours the same readiness checks
	// as the CLI dispatcher (e.g. "is a Neo4j connection available?").
	if prereq := dispatchPrerequisite(cat); prereq != nil {
		bridged.SetPrerequisite(prereq)
	}

	// Recurse into sub-categories.
	for _, subName := range cat.SubcategoryNames() {
		sub := cat.Subcat(subName)
		if sub == nil {
			continue
		}
		bridged.AddSubcategory(BridgeCategory(sub, ctxFor))
	}

	// Bridge every named command.
	for _, cmdName := range cat.CommandNames() {
		dispCmd := cat.Commands()[cmdName]
		if dispCmd == nil {
			continue
		}

		dc := dispCmd // capture for the closure
		bridged.AddCommand(&Command{
			Name:        dc.Name,
			Aliases:     dc.Aliases,
			Description: dc.Description,
			Usage:       dc.Usage,
			Handler: func(args []string, shellCtx Context) (string, error) {
				result, err := dc.Handler(args, ctxFor(shellCtx))
				if err != nil {
					return "", err
				}
				return renderResult(result, shellCtx)
			},
		})
	}

	return bridged
}

// renderResult converts a dispatch.CommandResult to a plain string for the
// shell REPL.
//
// Rendering priority:
//  1. Presentation payload — delegate to the shell presenter's Format method.
//  2. Message fallback — return the Message string directly.
func renderResult(result dispatch.CommandResult, shellCtx Context) (string, error) {
	if result.Presentation != nil && shellCtx.Presenter != nil {
		if result.FormatOverride != "" {
			return shellCtx.Presenter.FormatAs(result.Presentation, result.FormatOverride)
		}
		return shellCtx.Presenter.Format(result.Presentation)
	}
	return result.Message, nil
}

// dispatchPrerequisite extracts the prerequisite function from a
// dispatch.Category using its exported accessor. Returns nil when none is set.
func dispatchPrerequisite(cat *dispatch.Category) func() error {
	return cat.Prerequisite()
}
