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
	Usage       string // short usage hint, e.g. "pause <id>"
	Handler     CommandHandler
}

// Category groups related commands under a top-level keyword.
//
// Categories can be nested one level deep, enabling input like:
//
//	neo4j> cloud instances list
//	neo4j> cloud instances pause <id>
//
// A category may also carry a DirectHandler for cases where the entire
// remaining input should be forwarded verbatim to a single handler:
//
//	neo4j> cypher MATCH (n) RETURN n LIMIT 5
//
// When a DirectHandler is set and the first token does not match any
// sub-category or command, the full token slice is passed to it.
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

// SetDirectHandler installs a handler that receives all remaining args when
// the input doesn't match any sub-category or command name. Returns the
// receiver so calls can be chained.
func (c *Category) SetDirectHandler(h CommandHandler) *Category {
	c.directHandler = h
	return c
}

// AddCommand registers a named command under this category.
func (c *Category) AddCommand(cmd *Command) *Category {
	c.commands[cmd.Name] = cmd
	return c
}

// AddSubcategory registers a named sub-category under this category.
func (c *Category) AddSubcategory(sub *Category) *Category {
	c.subcats[sub.Name] = sub
	return c
}

// Dispatch routes args through the category tree and calls the matching
// handler. args should be the tokens that follow the category name on the
// command line.
//
// Resolution order:
//  1. No args → show help (or usage hint if a DirectHandler is set)
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

// Help returns a formatted summary of all sub-categories and commands.
func (c *Category) Help() string {
	var b strings.Builder

	fmt.Fprintf(&b, "%s — %s\n", c.Name, c.Description)

	if len(c.subcats) > 0 {
		fmt.Fprintln(&b, "\nSub-categories:")
		keys := sortedKeys(c.subcats)
		for _, k := range keys {
			sub := c.subcats[k]
			fmt.Fprintf(&b, "  %-18s %s\n", sub.Name, sub.Description)
		}
	}

	if len(c.commands) > 0 {
		fmt.Fprintln(&b, "\nCommands:")
		keys := sortedCmdKeys(c.commands)
		for _, k := range keys {
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
