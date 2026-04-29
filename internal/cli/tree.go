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
	ConfigPath         string
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

	bindPersistentFlags(rootCmd, flags)

	rootCmd.AddCommand(buildCloudCommand(opts.RunFactory))
	rootCmd.AddCommand(buildCypherCommand(opts.RunFactory))
	rootCmd.AddCommand(buildAdminCommand(opts.RunFactory))
	rootCmd.AddCommand(buildConfigCommand(opts.RunFactory))

	return rootCmd
}

// bindPersistentFlags wires every persistent flag to the corresponding field
// on flags. Defaults and usage strings live here so help output is identical
// whether called from the app or the generator.
func bindPersistentFlags(rootCmd *cobra.Command, flags *Flags) {
	pf := rootCmd.PersistentFlags()
	pf.StringVar(&flags.ConfigPath, "config-file", "", "Path to a JSON configuration file")
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

func buildAdminCommand(rf RunFactory) *cobra.Command {
	return &cobra.Command{
		Use:   "admin",
		Short: "Administrative operations against a Neo4j database",
		Long: `Perform administrative operations against the connected Neo4j database.

Available commands:
  show-users       List all database users and their roles
  show-databases   List all databases and their status

Use --format to control output (table, json, pretty-json, graph).`,
		Example: `  neo4j-cli admin show-users
  neo4j-cli admin show-users --format json
  neo4j-cli admin show-databases`,
		RunE:          runEFor(rf, "admin"),
		SilenceUsage:  true,
		SilenceErrors: true,
	}
}

func buildConfigCommand(rf RunFactory) *cobra.Command {
	return &cobra.Command{
		Use:   "config",
		Short: "Manage CLI configuration",
		Long: `Manage CLI configuration. Changes made with 'set' and 'delete' are
persisted to the config file immediately.

Available commands:
  list             Show all keys, their current values, and descriptions
  set <key> <val>  Set a configuration value
  delete <key>     Reset a key to its default
  reset            Wipe the config file and restore all defaults

Use --format to control output for 'config list'.`,
		Example: `  neo4j-cli config list
  neo4j-cli config list --format json
  neo4j-cli config set neo4j.uri bolt://myhost:7687
  neo4j-cli config set cypher.output_format json
  neo4j-cli config delete neo4j.password
  neo4j-cli config reset`,
		RunE:          runEFor(rf, "config"),
		SilenceUsage:  true,
		SilenceErrors: true,
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
