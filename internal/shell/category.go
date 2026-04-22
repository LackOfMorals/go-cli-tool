package shell

import (
	"fmt"
	"sort"
	"strings"
)

// Command is a leaf action within a category or sub-category.
type Command struct {
	Name        string
	Description string
	Usage       string // short usage hint shown in help, e.g. "pause <id>"
	Handler     CommandHandler
}

// Category groups related commands under a top-level keyword.
//
// # Nesting
//
// Categories can be nested one level deep, enabling input like:
//
//	neo4j> cloud instances list
//	neo4j> cloud instances pause <id>
//
// A category may also carry a DirectHandler for cases where the entire
// remaining input should be forwarded verbatim:
//
//	neo4j> cypher MATCH (n) RETURN n LIMIT 5
//
// # Thread safety
//
// The builder methods (AddCommand, AddSubcategory, SetDirectHandler) are NOT
// safe for concurrent use. They are intended to be called once during
// application startup before any goroutine calls Dispatch, Help, or Find.
// Dispatch, Help, Find, SubcategoryNames, CommandNames, and Subcat are safe
// to call concurrently once the category tree is fully built.
type Category struct {
	Name          string
	Description   string
	directHandler CommandHandler
	subcats       map[string]*Category
	commands      map[string]*Command
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

// AddCommand registers a named command. Returns the receiver for chaining.
func (c *Category) AddCommand(cmd *Command) *Category {
	c.commands[cmd.Name] = cmd
	return c
}

// AddSubcategory registers a nested category. Returns the receiver for
// chaining.
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

// CommandNames returns the names of all direct commands, sorted.
func (c *Category) CommandNames() []string {
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
func (c *Category) Dispatch(args []string, ctx ShellContext) (string, error) {
	if len(args) == 0 {
		if c.directHandler != nil {
			return "", fmt.Errorf("usage: %s <query>", c.Name)
		}
		return c.Help(), nil
	}

	name := args[0]
	rest := args[1:]

	if sub, ok := c.subcats[name]; ok {
		return sub.Dispatch(rest, ctx)
	}

	if cmd, ok := c.commands[name]; ok {
		return cmd.Handler(rest, ctx)
	}

	if c.directHandler != nil {
		return c.directHandler(args, ctx)
	}

	return "", fmt.Errorf("%s: unknown command %q — type 'help %s' to see available commands",
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
		for _, k := range sortedCmdKeys(c.commands) {
			cmd := c.commands[k]
			label := cmd.Name
			if cmd.Usage != "" {
				label = cmd.Usage
			}
			fmt.Fprintf(&b, "  %-18s %s\n", label, cmd.Description)
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

func sortedCmdKeys(m map[string]*Command) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
