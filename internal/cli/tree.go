// Package cli builds the cobra command tree for neo4j-cli. The tree builder
// is split out of cmd/neo4j-cli so that other tools (e.g. cmd/gen-skill) can
// walk the same command surface without wiring real services.
//
// BuildCobraTree is the single source of truth for command Use, Short, Long,
// Example, and persistent flags. The actual service wiring and RunE handlers
// live in cmd/neo4j-cli; they are injected via Options.
package cli

import (
	"github.com/spf13/cobra"
)

// Flags holds every persistent flag exposed by the root command. The app
// binds its own *Flags so cobra mutates these fields during parse; the
// generator can pass nil to use a throwaway *Flags instance.
type Flags struct {
	LogLevel           string
	LogFormat          string
	LogOutput          string
	LogFile            string
	NoMetrics          bool
	Neo4jURI           string
	Neo4jUsername      string
	Neo4jDatabase      string
	AuraClientID       string
	AuraTimeoutSeconds int
	OutputFormat       string

	// Agent-mode flags
	AgentMode   bool
	AllowWrites bool
	RequestID   string
	TimeoutStr  string
}

// RunFactory builds a cobra RunE for a given category name (e.g. "cypher",
// "cloud"). The generator passes nil so RunE is never wired and no service
// constructor runs during tree construction.
type RunFactory func(name string) func(cmd *cobra.Command, args []string) error

// Options configures BuildCobraTree.
type Options struct {
	// Flags receives persistent flag bindings. May be nil — BuildCobraTree
	// allocates a throwaway Flags instance when used purely for tree walking.
	Flags *Flags

	// RunFactory wires RunE for each subcommand. May be nil for generator use.
	RunFactory RunFactory
}

// BuildCobraTree assembles the full neo4j-cli command tree. It performs no
// service construction and no I/O; it is safe to call from a generator with
// nil RunFactory and nil Flags.
func BuildCobraTree(opts Options) *cobra.Command {
	flags := opts.Flags
	if flags == nil {
		flags = &Flags{}
	}

	rootCmd := &cobra.Command{
		Use:   "neo4j-cli",
		Short: "A CLI for Neo4j",
		Long: `neo4j-cli is a command-line tool for Neo4j.

Use a subcommand to interact with Neo4j databases and Aura cloud resources.`,
		// No RunE: cobra prints help when called with no subcommand.
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	rootCmd.CompletionOptions.DisableDefaultCmd = true

	bindPersistentFlags(rootCmd, flags)

	rootCmd.AddCommand(buildCloudCommand(opts.RunFactory))
	rootCmd.AddCommand(buildCypherCommand(opts.RunFactory))
	rootCmd.AddCommand(buildSkillCommand(opts.RunFactory))

	return rootCmd
}

// bindPersistentFlags wires every persistent flag to the corresponding field
// on flags. Defaults and usage strings live here so help output is identical
// whether called from the app or the generator.
func bindPersistentFlags(rootCmd *cobra.Command, flags *Flags) {
	pf := rootCmd.PersistentFlags()
	pf.StringVar(&flags.LogLevel, "log-level", "", "Log level: debug, info, warn, error")
	pf.StringVar(&flags.LogFormat, "log-format", "", "Log format: text, json")
	pf.StringVar(&flags.LogOutput, "log-output", "", "Log destination: stderr (default), stdout, file")
	pf.StringVar(&flags.LogFile, "log-file", "", "Log file path when --log-output=file (default: ~/.neo4j-cli/neo4j-cli.log)")
	pf.BoolVar(&flags.NoMetrics, "no-metrics", false, "Disable sending usage metrics to Neo4j (overrides config file and CLI_TELEMETRY_METRICS env var)")
	pf.StringVar(&flags.Neo4jURI, "neo4j-uri", "", "Neo4j bolt URI (e.g. bolt://localhost:7687)")
	pf.StringVar(&flags.Neo4jUsername, "neo4j-username", "", "Neo4j username")
	pf.StringVar(&flags.Neo4jDatabase, "neo4j-database", "", "Neo4j database name")
	pf.StringVar(&flags.AuraClientID, "aura-client-id", "", "Neo4j Aura API client ID")
	pf.IntVar(&flags.AuraTimeoutSeconds, "aura-timeout", 0, "Aura API request timeout in seconds")
	pf.StringVar(&flags.OutputFormat, "format", "", "Output format: table, json, pretty-json, graph")

	// Agent-mode flags
	pf.BoolVar(&flags.AgentMode, "agent", false, "Enable agent-optimised mode: JSON output, read-only by default, no interactive prompts, errors on stdout (env: NEO4J_CLI_AGENT=true)")
	pf.BoolVar(&flags.AllowWrites, "rw", false, "Permit write/mutating operations in agent mode (env: NEO4J_CLI_RW=true)")
	pf.StringVar(&flags.RequestID, "request-id", "", "Correlation ID included in every agent-mode JSON response (env: NEO4J_CLI_REQUEST_ID)")
	pf.StringVar(&flags.TimeoutStr, "timeout", "", "Maximum time for a command to run, e.g. 30s or 2m (exit code 124 on timeout)")
}

func buildCloudCommand(rf RunFactory) *cobra.Command {
	return &cobra.Command{
		Use:   "cloud",
		Short: "Manage Neo4j Aura cloud resources",
		Long: `Manage Neo4j Aura cloud resources.

Available sub-commands:
  instances   Create, list, get, update, pause, resume, and delete Aura DB instances
  projects    List and inspect Aura projects (tenants)

Use --format to control output (table, json, pretty-json, graph).`,
		Example: `  neo4j-cli cloud instances list
  neo4j-cli cloud instances list --format json
  neo4j-cli cloud instances get <id>
  neo4j-cli cloud instances create name=my-db tenant=<tenant-id> cloud=aws region=us-east-1
  neo4j-cli cloud instances pause <id>
  neo4j-cli cloud projects list`,
		RunE:          runEFor(rf, "cloud"),
		SilenceUsage:  true,
		SilenceErrors: true,
	}
}

func buildCypherCommand(rf RunFactory) *cobra.Command {
	return &cobra.Command{
		Use:   "cypher [query]",
		Short: "Execute a Cypher query against a Neo4j database",
		Long: `Execute a Cypher query against the connected Neo4j database.

Supply the query directly on the command line.

Query flags (parsed inline, not by cobra):
  --param key=value    Add a query parameter (repeatable; auto-typed)
  --format table|json|pretty-json|graph
                       Override output format for this query
  --limit N            Override the auto-injected row limit (default 25)`,
		Example: `  neo4j-cli cypher "MATCH (n:Person) RETURN n.name, n.age;"
  neo4j-cli cypher --param name=Alice "MATCH (n:Person {name:\$name}) RETURN n;"
  neo4j-cli cypher --format json "MATCH (n) RETURN n;"`,
		RunE:               runEFor(rf, "cypher"),
		DisableFlagParsing: true, // let parseCypherFlags handle --param/--format/--limit
		SilenceUsage:       true,
		SilenceErrors:      true,
	}
}

// runEFor returns the RunE for a given category name. When rf is nil (generator
// mode), RunE is left nil so cobra prints help instead of executing.
func runEFor(rf RunFactory, name string) func(*cobra.Command, []string) error {
	if rf == nil {
		return nil
	}
	return rf(name)
}

// runEForLeaf returns a RunE that dispatches through the named category with
// the leaf's own name prepended to args. This is used by skill's install /
// remove / list cobra subcommands so each leaf can have its own help/flags
// while still flowing through the shared dispatch tree.
func runEForLeaf(rf RunFactory, category, leaf string) func(*cobra.Command, []string) error {
	if rf == nil {
		return nil
	}
	inner := rf(category)
	if inner == nil {
		return nil
	}
	return func(cmd *cobra.Command, args []string) error {
		prefixed := append([]string{leaf}, args...)
		return inner(cmd, prefixed)
	}
}

func buildSkillCommand(rf RunFactory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "skill",
		Short: "Manage the embedded neo4j-cli SKILL.md across supported AI agents",
		Long: `Manage the embedded neo4j-cli SKILL.md for supported AI agents.

The neo4j-cli binary embeds a generated SKILL.md describing every command,
flag, and gotcha. The skill subcommand writes that file into each agent's
skills directory so the agent can load it on demand.

Available commands:
  install [agent]  Install SKILL.md into one agent (or every detected agent)
  remove  [agent]  Remove SKILL.md from one agent (or every agent that has it)
  list             Show every supported agent with detected/installed status

No Neo4j or Aura connection is required — install/remove/list operate purely
on the local filesystem.`,
		Example: `  neo4j-cli skill list
  neo4j-cli skill install
  neo4j-cli skill install claude-code
  neo4j-cli skill remove claude-code
  neo4j-cli skill remove`,
		RunE:          runEFor(rf, "skill"),
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "install [agent]",
		Short: "Install SKILL.md into one agent (or every detected agent when omitted)",
		Long: `Install the embedded SKILL.md into a supported AI agent's skills directory.

When called with no agent argument, install targets every agent detected on
the local machine. Pass an agent name (e.g. claude-code, cursor, copilot) to
install for a specific agent even if it is not currently detected.

The destination is <agent.skills_dir>/neo4j-cli/SKILL.md. Any existing file,
directory, or symlink at that path is removed before the new file is written,
so re-running install is idempotent.

Run 'neo4j-cli skill list' to see every supported agent and the canonical
agent names accepted here.`,
		Example: `  neo4j-cli skill install
  neo4j-cli skill install claude-code
  neo4j-cli skill install cursor`,
		Args:          cobra.MaximumNArgs(1),
		RunE:          runEForLeaf(rf, "skill", "install"),
		SilenceUsage:  true,
		SilenceErrors: true,
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "remove [agent]",
		Short: "Remove SKILL.md from one agent (or every agent that has it when omitted)",
		Long: `Remove the embedded SKILL.md from a supported AI agent's skills directory.

When called with no agent argument, remove targets every agent that currently
has the skill installed. Pass an agent name to remove for a specific agent.
Missing targets are silently ignored, so remove is safe to re-run.

Run 'neo4j-cli skill list' to see which agents currently have SKILL.md
installed.`,
		Example: `  neo4j-cli skill remove
  neo4j-cli skill remove claude-code`,
		Args:          cobra.MaximumNArgs(1),
		RunE:          runEForLeaf(rf, "skill", "remove"),
		SilenceUsage:  true,
		SilenceErrors: true,
	})

	cmd.AddCommand(&cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "Show every supported agent with its detected/installed status",
		Long: `List every supported AI agent with two booleans per row:

  Detected   the agent's config directory exists on this machine
  Installed  neo4j-cli's SKILL.md is currently installed for the agent

Use --format=json for machine-readable output suitable for agent-mode
consumers.`,
		Example: `  neo4j-cli skill list
  neo4j-cli skill list --format json`,
		Args:          cobra.NoArgs,
		RunE:          runEForLeaf(rf, "skill", "list"),
		SilenceUsage:  true,
		SilenceErrors: true,
	})

	return cmd
}
