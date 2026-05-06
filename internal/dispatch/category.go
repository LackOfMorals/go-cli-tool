package dispatch

import (
	"fmt"
	"sort"
	"strings"

	"github.com/cli/go-cli-tool/internal/tool"
)

// Command is a leaf action within a category or sub-category.
type Command struct {
	Name        string
	Aliases     []string // optional short-form names, e.g. ["ls"] for "list"
	Description string
	Usage       string // short usage hint shown in help, e.g. "pause <id>"
	Handler     CommandHandler

	// MutationMode declares whether this command modifies remote state.
	// ModeRead (default) executes unconditionally.
	// ModeWrite is blocked in agent mode without --rw before Handler is called.
	// ModeConditional requires a runtime check (e.g. EXPLAIN for Cypher).
	MutationMode tool.MutationMode
}

// Category groups related commands under a top-level keyword.
//
// # Nesting
//
// Categories can be nested one level deep:
//
//	lom cloud instances list
//	lom cloud instances pause <id>
//
// A category may also carry a DirectHandler for cases where the entire
// remaining input should be forwarded verbatim:
//
//	lom cypher "MATCH (n) RETURN n LIMIT 5"
//
// # Prerequisites
//
// Categories that require an external dependency (a database connection,
// API credentials, etc.) should declare it via SetPrerequisite. The check
// runs on every non-empty dispatch so the caller gets a clear, actionable
// error message before the underlying service call fails. Invoking the
// category with no arguments (to show help) always succeeds even when the
// prerequisite would fail.
//
// # Thread safety
//
// The builder methods (AddCommand, AddSubcategory, SetDirectHandler,
// SetPrerequisite) are NOT safe for concurrent use. They are intended to be
// called once during application startup before any goroutine calls Dispatch,
// Help, or Find. Dispatch, Help, Find, SubcategoryNames, CommandNames, and
// Subcat are safe to call concurrently once the category tree is fully built.
type Category struct {
	Name        string
	Description string

	directHandler        CommandHandler
	directHandlerEmptyOK bool         // if true, directHandler is called even with no args
	prerequisite         func() error // optional; checked before every non-help dispatch
	subcats              map[string]*Category
	commands             map[string]*Command
}

// NewCategory creates an empty category ready for commands and sub-categories.
func NewCategory(name, description string) *Category {
	return &Category{
		Name:        name,
		Description: description,
		subcats:     make(map[string]*Category),
		commands:    make(map[string]*Command),
	}
}

// ---- Builder methods (call only during startup) -------------------------

// SetDirectHandler installs a catch-all handler for free-form input that does
// not match any sub-category or named command. Returns the receiver for
// chaining.
func (c *Category) SetDirectHandler(h CommandHandler) *Category {
	c.directHandler = h
	return c
}

// AllowEmptyDirectHandler permits the direct handler to be invoked with no
// arguments. The prerequisite (if any) is still fired first. Use this for
// categories where the handler can prompt for input when nothing is provided
// on the command line (e.g. the cypher category).
func (c *Category) AllowEmptyDirectHandler() *Category {
	c.directHandlerEmptyOK = true
	return c
}

// SetPrerequisite installs a dependency check that runs before every
// non-help dispatch on this category. If fn returns an error the command is
// not executed and the error is returned to the caller directly.
//
// Returns the receiver for chaining.
func (c *Category) SetPrerequisite(fn func() error) *Category {
	c.prerequisite = fn
	return c
}

// AddCommand registers a named command. Returns the receiver for chaining.
// All aliases defined in cmd.Aliases are registered in the same map so that
// Dispatch resolves them without any extra logic.
func (c *Category) AddCommand(cmd *Command) *Category {
	c.commands[cmd.Name] = cmd
	for _, alias := range cmd.Aliases {
		c.commands[alias] = cmd
	}
	return c
}

// AddSubcategory registers a nested category. Returns the receiver for chaining.
func (c *Category) AddSubcategory(sub *Category) *Category {
	c.subcats[sub.Name] = sub
	return c
}

// ---- Read-only accessors (safe for concurrent use after startup) --------

// SubcategoryNames returns the names of all direct sub-categories, sorted.
func (c *Category) SubcategoryNames() []string {
	names := make([]string, 0, len(c.subcats))
	for k := range c.subcats {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}

// CommandNames returns the canonical name of every direct command, sorted.
// Alias keys registered by AddCommand are excluded; use AllCommandNames for
// cases where aliases should also be offered.
func (c *Category) CommandNames() []string {
	seen := make(map[string]bool, len(c.commands))
	var names []string
	for k, cmd := range c.commands {
		if k == cmd.Name && !seen[k] {
			names = append(names, k)
			seen[k] = true
		}
	}
	sort.Strings(names)
	return names
}

// AllCommandNames returns every registered key (canonical names + aliases),
// sorted.
func (c *Category) AllCommandNames() []string {
	names := make([]string, 0, len(c.commands))
	for k := range c.commands {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}

// Subcat returns the named sub-category, or nil if it does not exist.
func (c *Category) Subcat(name string) *Category {
	return c.subcats[name]
}

// ---- Read-only accessor for bridge adapters ----------------------------

// Commands returns a snapshot map of name → *Command for all registered keys
// (canonical names and aliases). The returned map is a copy and safe to read
// from other packages; mutations do not affect the category.
func (c *Category) Commands() map[string]*Command {
	out := make(map[string]*Command, len(c.commands))
	for k, v := range c.commands {
		out[k] = v
	}
	return out
}

// Prerequisite returns the prerequisite function installed on this category,
// or nil if none has been set.
func (c *Category) Prerequisite() func() error {
	return c.prerequisite
}

// ---- Dispatch / navigation ----------------------------------------------

// Dispatch routes args through the category tree and calls the matching
// handler. args should be the tokens that follow the category name on the
// command line.
//
// Resolution order:
//  1. No args → show help (or a usage hint if a DirectHandler is set)
//  2. args[0] matches a sub-category → delegate to sub.Dispatch(args[1:])
//  3. args[0] matches a command → call cmd.Handler(args[1:])
//  4. DirectHandler is set → call it with the full args slice
//  5. Nothing matches → return a descriptive error
func (c *Category) Dispatch(args []string, ctx Context) (CommandResult, error) {
	if len(args) == 0 {
		switch {
		case c.directHandler != nil && c.directHandlerEmptyOK:
			if c.prerequisite != nil {
				if err := c.prerequisite(); err != nil {
					return CommandResult{}, err
				}
			}
			return c.directHandler(args, ctx)
		case c.directHandler != nil:
			return CommandResult{}, fmt.Errorf("usage: %s <query>", c.Name)
		default:
			return MessageResult(c.Help()), nil
		}
	}

	// Check dependency prerequisites before any real work. This is done after
	// the no-args guard so that calling a bare category name always shows help
	// even when the dependency is unavailable.
	if c.prerequisite != nil {
		if err := c.prerequisite(); err != nil {
			return CommandResult{}, err
		}
	}

	name := args[0]
	rest := args[1:]

	if sub, ok := c.subcats[name]; ok {
		return sub.Dispatch(rest, ctx)
	}

	if cmd, ok := c.commands[name]; ok {
		// Agent-mode read-only enforcement: block ModeWrite commands before the
		// handler is ever called. ModeConditional enforcement (EXPLAIN check)
		// is delegated to the handler itself (e.g. the cypher direct handler).
		if ctx.AgentMode && !ctx.AllowWrites && cmd.MutationMode == tool.ModeWrite {
			return CommandResult{}, tool.NewAgentError("READ_ONLY",
				fmt.Sprintf("%q is a write operation; re-run with --rw to permit mutations", cmd.Name))
		}
		return cmd.Handler(rest, ctx)
	}

	if c.directHandler != nil {
		return c.directHandler(args, ctx)
	}

	return CommandResult{}, fmt.Errorf("%s: unknown command %q — run 'lom %s --help' to see available commands",
		c.Name, name, c.Name)
}

// Find navigates the category tree along path and returns the Category at
// that position, or nil if the path does not exist. An empty path returns
// the receiver.
func (c *Category) Find(path []string) *Category {
	if len(path) == 0 {
		return c
	}
	if sub, ok := c.subcats[path[0]]; ok {
		return sub.Find(path[1:])
	}
	return nil
}

// Help returns a formatted summary of all sub-categories and commands,
// sorted alphabetically.
func (c *Category) Help() string {
	var b strings.Builder

	fmt.Fprintf(&b, "%s — %s\n", c.Name, c.Description)

	if len(c.subcats) > 0 {
		fmt.Fprintln(&b, "\nSub-categories:")
		for _, k := range sortedKeys(c.subcats) {
			sub := c.subcats[k]
			fmt.Fprintf(&b, "  %-18s %s\n", sub.Name, sub.Description)
		}
	}

	if len(c.commands) > 0 {
		fmt.Fprintln(&b, "\nCommands:")
		for _, k := range c.CommandNames() {
			cmd := c.commands[k]
			label := cmd.Name
			if cmd.Usage != "" {
				label = cmd.Usage
			}
			if len(cmd.Aliases) > 0 {
				label += " (" + strings.Join(cmd.Aliases, ", ") + ")"
			}
			fmt.Fprintf(&b, "  %-24s %s\n", label, cmd.Description)
		}
	}

	return strings.TrimRight(b.String(), "\n")
}

func sortedKeys(m map[string]*Category) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
