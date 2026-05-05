// Command gen-skill walks the cobra command tree exposed by internal/cli and
// emits a deterministic Markdown SKILL.md to internal/skill/skill.md.gen. The
// generated file is embedded into the binary via internal/skill/embed.go and
// shipped to agent runtimes by the `neo4j-cli skill install` command.
//
// The generator is invoked by `go generate ./...` (see the //go:generate
// directive in internal/skill/embed.go) and is also safe to run directly with
// `go run ./cmd/gen-skill` from the project root. It performs no service
// construction, no I/O against Neo4j or Aura, and must produce byte-identical
// output for unchanged inputs.
package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/cli/go-cli-tool/internal/cli"
)

const (
	skillDirRel       = "internal/skill"
	additionsFilename = "skill-additions.md"
	outputFilename    = "skill.md.gen"
)

// frontMatter is the YAML front-matter prepended to the generated SKILL.md so
// agent runtimes (Claude Code, Cursor, etc.) can index and trigger the skill.
// Keep the description trigger-rich: it is the field agents match against to
// decide whether to load the skill.
const frontMatter = `---
name: neo4j-cli
description: Run Cypher queries against Neo4j databases and manage Neo4j Aura cloud resources (instances, projects, tenants) from the command line. Use when the user asks to query a Neo4j graph, inspect the schema, list/create/pause/resume/delete an Aura instance, manage projects, or run admin operations like show-users or show-databases. Always invoke with ` + "`--agent`" + ` for JSON output; pair with ` + "`--rw`" + ` only when a write or mutation is explicitly intended.
---

`

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "gen-skill: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	root, err := projectRoot()
	if err != nil {
		return err
	}

	additionsPath := filepath.Join(root, skillDirRel, additionsFilename)
	additions, err := os.ReadFile(additionsPath)
	if err != nil {
		return fmt.Errorf("read %s: %w", additionsPath, err)
	}

	tree := cli.BuildCobraTree(cli.Options{})

	var buf bytes.Buffer
	buf.WriteString(frontMatter)
	renderRoot(&buf, tree)
	renderSubcommands(&buf, tree)
	renderGotchas(&buf, additions)

	outPath := filepath.Join(root, skillDirRel, outputFilename)
	if err := os.WriteFile(outPath, buf.Bytes(), 0o600); err != nil {
		return fmt.Errorf("write %s: %w", outPath, err)
	}
	return nil
}

// projectRoot resolves the repository root from the location of this source
// file. This keeps `go run ./cmd/gen-skill` (CWD: repo root) and
// `go generate ./...` (CWD: containing pkg) producing identical output.
func projectRoot() (string, error) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("runtime.Caller failed")
	}
	// file = <root>/cmd/gen-skill/main.go
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..")), nil
}

func renderRoot(buf *bytes.Buffer, root *cobra.Command) {
	fmt.Fprintf(buf, "# %s\n\n", root.Name())
	if s := strings.TrimSpace(root.Short); s != "" {
		fmt.Fprintf(buf, "%s\n\n", s)
	}
	if l := strings.TrimSpace(root.Long); l != "" {
		fmt.Fprintf(buf, "%s\n\n", l)
	}

	// Render persistent flags once at the root so subcommand sections stay
	// focused on subcommand-specific flags.
	if hasFlags(root.PersistentFlags()) {
		buf.WriteString("## Global Flags\n\n")
		writeFlagsTable(buf, root.PersistentFlags())
		buf.WriteString("\n")
	}
}

func renderSubcommands(buf *bytes.Buffer, root *cobra.Command) {
	subs := append([]*cobra.Command(nil), root.Commands()...)
	sort.Slice(subs, func(i, j int) bool { return subs[i].Name() < subs[j].Name() })

	for _, sub := range subs {
		if sub.Hidden || sub.Name() == "help" {
			continue
		}
		renderCommand(buf, sub)
	}
}

func renderCommand(buf *bytes.Buffer, cmd *cobra.Command) {
	fmt.Fprintf(buf, "## %s\n\n", cmd.Name())

	if u := strings.TrimSpace(cmd.UseLine()); u != "" {
		fmt.Fprintf(buf, "Usage: `%s`\n\n", u)
	}

	if s := strings.TrimSpace(cmd.Short); s != "" {
		fmt.Fprintf(buf, "%s\n\n", s)
	}
	if l := strings.TrimSpace(cmd.Long); l != "" && cmd.Long != cmd.Short {
		fmt.Fprintf(buf, "%s\n\n", l)
	}

	if hasFlags(cmd.Flags()) {
		buf.WriteString("### Flags\n\n")
		writeFlagsTable(buf, cmd.Flags())
		buf.WriteString("\n")
	}

	if e := strings.TrimSpace(cmd.Example); e != "" {
		buf.WriteString("### Examples\n\n")
		buf.WriteString("```\n")
		buf.WriteString(e)
		if !strings.HasSuffix(e, "\n") {
			buf.WriteString("\n")
		}
		buf.WriteString("```\n\n")
	}
}

func renderGotchas(buf *bytes.Buffer, additions []byte) {
	buf.WriteString("## Gotchas\n\n")
	body := strings.TrimRight(string(additions), "\n")
	if body == "" {
		return
	}
	buf.WriteString(body)
	buf.WriteString("\n")
}

// hasFlags reports whether the flag set defines any non-hidden flag local to
// the command (i.e. excludes inherited persistent flags so subcommand tables
// don't duplicate the Global Flags section).
func hasFlags(fs *pflag.FlagSet) bool {
	found := false
	fs.VisitAll(func(f *pflag.Flag) {
		if !f.Hidden {
			found = true
		}
	})
	return found
}

func writeFlagsTable(buf *bytes.Buffer, fs *pflag.FlagSet) {
	type row struct {
		name, shorthand, typ, def, usage string
	}
	var rows []row
	fs.VisitAll(func(f *pflag.Flag) {
		if f.Hidden {
			return
		}
		rows = append(rows, row{
			name:      f.Name,
			shorthand: f.Shorthand,
			typ:       f.Value.Type(),
			def:       f.DefValue,
			usage:     strings.ReplaceAll(f.Usage, "|", `\|`),
		})
	})
	sort.Slice(rows, func(i, j int) bool { return rows[i].name < rows[j].name })

	buf.WriteString("| Flag | Type | Default | Description |\n")
	buf.WriteString("|------|------|---------|-------------|\n")
	for _, r := range rows {
		flag := "--" + r.name
		if r.shorthand != "" {
			flag = "-" + r.shorthand + ", " + flag
		}
		def := r.def
		if def == "" {
			def = "-"
		}
		fmt.Fprintf(buf, "| `%s` | %s | %s | %s |\n", flag, r.typ, def, r.usage)
	}
}
