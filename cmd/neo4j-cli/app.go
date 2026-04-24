package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"

	"github.com/cli/go-cli-tool/internal/analytics"
	"github.com/cli/go-cli-tool/internal/commands"
	"github.com/cli/go-cli-tool/internal/config"
	"github.com/cli/go-cli-tool/internal/logger"
	"github.com/cli/go-cli-tool/internal/presentation"
	"github.com/cli/go-cli-tool/internal/repository"
	"github.com/cli/go-cli-tool/internal/service"
	"github.com/cli/go-cli-tool/internal/shell"
	"github.com/cli/go-cli-tool/internal/tool"
	"github.com/cli/go-cli-tool/internal/tools"
	"github.com/spf13/cobra"
)

// Version is injected at build time:
//
//	go build -ldflags "-X 'main.Version=1.2.3'"
var Version = "development"

const (
	mixPanelEndpoint = "https://api.mixpanel.com"
	mixPanelToken    = "4bfb2414ab973c741b6f067bf06d5575" // #nosec G101
)

// ---- CLI flag vars -------------------------------------------------------

var (
	configPath         string
	logLevel           string
	logFormat          string
	logOutput          string
	logFile            string
	shellMode          bool
	execTool           string
	execArgs           []string
	noMetrics          bool
	neo4jURI           string
	neo4jUsername      string
	neo4jDatabase      string
	auraClientID       string
	auraTimeoutSeconds int
	outputFormat       string
)

// ---- App ----------------------------------------------------------------

type App struct {
	cfg          *config.Config
	log          logger.Service
	logCloser    io.Closer // non-nil when logging to a file; closed last in close()
	analytic     analytics.Service
	presentation *presentation.PresentationService
	registry     *tools.ToolRegistry
	repo         repository.GraphRepository // held so close() can release driver resources

	cypherSvc service.CypherService
	cloudSvc  service.CloudService
	adminSvc  service.AdminService
}

func run() int {
	if err := buildRootCommand().Execute(); err != nil {
		return 1
	}
	return 0
}

func buildRootCommand() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:           "neo4j-cli",
		Short:         "A CLI for Neo4j",
		Long:          `neo4j-cli is a CLI for Neo4j and can be used with commands, or if no commands are given, as a shell.  `,
		RunE:          runCLI,
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	pf := rootCmd.PersistentFlags()
	pf.StringVar(&configPath, "config-file", "", "Path to a JSON configuration file")
	pf.StringVar(&logLevel, "log-level", "", "Log level: debug, info, warn, error")
	pf.StringVar(&logFormat, "log-format", "", "Log format: text, json")
	pf.StringVar(&logOutput, "log-output", "", "Log destination: stderr (default), stdout, file")
	pf.StringVar(&logFile, "log-file", "", "Log file path when --log-output=file (default: ~/.neo4j-cli/neo4j-cli.log)")
	pf.BoolVar(&noMetrics, "no-metrics", false, "Disable sending usage metrics to Neo4j (overrides config file and CLI_TELEMETRY_METRICS env var)")
	pf.BoolVarP(&shellMode, "shell", "s", false, "Start interactive shell (default when no --exec)")
	pf.StringVar(&neo4jURI, "neo4j-uri", "", "Neo4j bolt URI (e.g. bolt://localhost:7687)")
	pf.StringVar(&neo4jUsername, "neo4j-username", "", "Neo4j username")
	pf.StringVar(&neo4jDatabase, "neo4j-database", "", "Neo4j database name")
	pf.StringVar(&auraClientID, "aura-client-id", "", "Neo4j Aura API client ID")
	pf.IntVar(&auraTimeoutSeconds, "aura-timeout", 0, "Aura API request timeout in seconds")
	pf.StringVar(&outputFormat, "format", "", "Output format: table, json, pretty-json, graph")

	rootCmd.AddCommand(buildCloudCommand())
	rootCmd.AddCommand(buildCypherCommand())
	rootCmd.AddCommand(buildAdminCommand())
	rootCmd.AddCommand(buildConfigCommand())

	return rootCmd
}

// runCategory returns a cobra RunE that creates the app, finds the named
// shell category, and dispatches the remaining CLI args through it directly —
// no interactive shell required.
func runCategory(name string) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		app, err := newApp(cmd, args)
		if err != nil {
			return fmt.Errorf("startup: %w", err)
		}
		defer app.close()
		return app.dispatchCategory(name, args)
	}
}

// dispatchCategory routes args through the named shell category and prints
// the result. With no args it prints the category's help text. Credential
// prerequisites (Aura, Neo4j) are triggered automatically by Dispatch.
func (a *App) dispatchCategory(name string, args []string) error {
	cats := a.buildCategories()
	cat, ok := cats[name]
	if !ok {
		return fmt.Errorf("unknown category %q", name)
	}

	if len(args) == 0 {
		fmt.Println(cat.Help())
		return nil
	}

	if outputFormat != "" {
		f := presentation.OutputFormat(outputFormat)
		if !f.IsValid() {
			return fmt.Errorf("invalid format %q: must be one of table, json, pretty-json, graph", outputFormat)
		}
		if err := a.presentation.SetFormat(f); err != nil {
			return fmt.Errorf("set format: %w", err)
		}
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	shellCtx := shell.ShellContext{
		Context:   ctx,
		Config:    *a.cfg,
		Logger:    a.log,
		Telemetry: a.analytic,
		Presenter: a.presentation,
		Registry:  a.registry,
		IO:        tool.NewDefaultIOHandler(),
	}

	result, err := cat.Dispatch(args, shellCtx)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return fmt.Errorf("interrupted")
		}
		return err
	}
	if result != "" {
		fmt.Println(result)
	}
	return nil
}

func buildCloudCommand() *cobra.Command {
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
		RunE:          runCategory("cloud"),
		SilenceUsage:  true,
		SilenceErrors: true,
	}
}

func buildCypherCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "cypher [query]",
		Short: "Execute a Cypher query against a Neo4j database",
		Long: `Execute a Cypher query against the connected Neo4j database.

Supply the query directly on the command line. Without a query, an
interactive prompt is shown.

Query flags (parsed inline, not by the shell):
  --param key=value    Add a query parameter (repeatable; auto-typed)
  --format table|json|pretty-json|graph
                       Override output format for this query
  --limit N            Override the auto-injected row limit (default 25)`,
		Example: `  neo4j-cli cypher "MATCH (n:Person) RETURN n.name, n.age;"
  neo4j-cli cypher --param name=Alice "MATCH (n:Person {name:\$name}) RETURN n;"
  neo4j-cli cypher --format json "MATCH (n) RETURN n;"`,
		RunE:               runCategory("cypher"),
		DisableFlagParsing: true, // let parseCypherFlags handle --param/--format/--limit
		SilenceUsage:       true,
		SilenceErrors:      true,
	}
}

func buildAdminCommand() *cobra.Command {
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
		RunE:          runCategory("admin"),
		SilenceUsage:  true,
		SilenceErrors: true,
	}
}

func buildConfigCommand() *cobra.Command {
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
		RunE:          runCategory("config"),
		SilenceUsage:  true,
		SilenceErrors: true,
	}
}

// overridesFromCmd extracts only the flags the user explicitly set, so a
// flag default can never silently clobber a value from the config file.
func overridesFromCmd(cmd *cobra.Command) config.Overrides {
	o := config.Overrides{}
	f := cmd.Flags()

	if f.Changed("config-file") {
		o.ConfigFile, _ = f.GetString("config-file")
	}
	if f.Changed("log-level") {
		o.LogLevel, _ = f.GetString("log-level")
	}
	if f.Changed("log-format") {
		o.LogFormat, _ = f.GetString("log-format")
	}
	if f.Changed("log-output") {
		o.LogOutput, _ = f.GetString("log-output")
	}
	if f.Changed("log-file") {
		o.LogFile, _ = f.GetString("log-file")
	}
	if f.Changed("no-metrics") {
		v := !noMetrics // --no-metrics disables; --no-metrics=false re-enables
		o.MetricsEnabled = &v
	}
	if f.Changed("shell") {
		v, _ := f.GetBool("shell")
		o.ShellEnabled = &v
	}
	if f.Changed("neo4j-uri") {
		o.Neo4jURI, _ = f.GetString("neo4j-uri")
	}
	if f.Changed("neo4j-username") {
		o.Neo4jUsername, _ = f.GetString("neo4j-username")
	}
	if f.Changed("neo4j-database") {
		o.Neo4jDatabase, _ = f.GetString("neo4j-database")
	}
	if f.Changed("aura-client-id") {
		o.AuraClientID, _ = f.GetString("aura-client-id")
	}
	if f.Changed("aura-timeout") {
		v, _ := f.GetInt("aura-timeout")
		o.AuraTimeout = &v
	}
	return o
}

func runCLI(cmd *cobra.Command, args []string) error {
	app, err := newApp(cmd, args)
	if err != nil {
		return fmt.Errorf("startup: %w", err)
	}
	defer app.close()
	return app.dispatch()
}

func newApp(cmd *cobra.Command, _ []string) (*App, error) {
	// 1. Config — explicit overrides > env vars > file > defaults.
	cfg, err := config.NewConfigService(overridesFromCmd(cmd)).LoadConfiguration()
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	// 2. Logger — resolve output destination then create the service.
	// The logCloser is non-nil only when writing to a file; it is closed
	// last in App.close() so log messages from earlier cleanup steps are
	// still captured.
	var logCloser io.Closer
	logOut := logger.ParseLogOutput(cfg.LogOutput)
	var logWriter io.Writer
	if logOut == logger.OutputFile {
		f, err := logger.OpenLogFile(cfg.LogFile)
		if err != nil {
			return nil, fmt.Errorf("open log file: %w", err)
		}
		logWriter = f
		logCloser = f
	} else {
		logWriter = logger.WriterFor(logOut)
	}
	log := logger.NewLoggerServiceToWriter(
		logger.ParseLogFormat(cfg.LogFormat),
		logger.ParseLogLevel(cfg.LogLevel),
		logWriter,
	)

	// 3. Analytics
	an := analytics.NewAnalytics(mixPanelToken, mixPanelEndpoint, cfg.Neo4j.URI, Version, log)
	if !cfg.Telemetry.Metrics {
		an.Disable()
	}

	// 4. Presentation service — always start in table mode for a CLI.
	// Commands use FormatAs for per-query overrides; the session default
	// can be changed with `set cypher-format <format>`.
	pres, err := presentation.NewPresentationService(presentation.OutputFormatTable, log)
	if err != nil {
		return nil, fmt.Errorf("init presentation: %w", err)
	}

	// 5. Graph repository — shared by cypher and admin services.
	// Receives a pointer to cfg.Neo4j so that credentials provided
	// interactively by InteractiveNeo4jPrerequisite are visible on first use.
	repo := repository.NewNeo4jRepository(&cfg.Neo4j)

	// 6. Domain services
	cypherSvc := service.NewCypherService(repo)
	cloudSvc := service.NewCloudService(&cfg.Aura) // pointer so interactive prerequisite can populate credentials
	adminSvc := service.NewAdminService(repo)

	// 7. Tool registry — QueryTool reuses cypherSvc so there is one query path.
	registry := buildRegistry(&cfg, log, cypherSvc)

	return &App{
		cfg:          &cfg,
		log:          log,
		logCloser:    logCloser,
		analytic:     an,
		presentation: pres,
		registry:     registry,
		repo:         repo,
		cypherSvc:    cypherSvc,
		cloudSvc:     cloudSvc,
		adminSvc:     adminSvc,
	}, nil
}

// buildRegistry constructs the tool registry. QueryTool receives the same
// CypherService used by the 'cypher' shell category — one query path, no
// duplication.
func buildRegistry(cfg *config.Config, log logger.Service, cypherSvc service.CypherService) *tools.ToolRegistry {
	registry := tools.NewToolRegistry()

	for _, t := range []tool.Tool{
		tools.NewEchoTool(),
		tools.NewHelpTool(registry),
		tools.NewQueryTool(cypherSvc),
	} {
		registerTool(registry, t, cfg, log)
	}

	return registry
}

func registerTool(r *tools.ToolRegistry, t tool.Tool, cfg *config.Config, log logger.Service) {
	// If the tool has an explicit config entry and is marked disabled, skip it entirely.
	if toolCfg, ok := cfg.Tools[t.Name()]; ok && !toolCfg.Enabled {
		log.Debug("skipping disabled tool", logger.Field{Key: "tool", Value: t.Name()})
		return
	}

	var err error
	if toolCfg, ok := cfg.Tools[t.Name()]; ok {
		err = r.RegisterWithConfig(t, toolCfg)
	} else {
		err = r.Register(t)
	}
	if err != nil {
		log.Warn("failed to register tool",
			logger.Field{Key: "tool", Value: t.Name()},
			logger.Field{Key: "error", Value: err.Error()},
		)
	}
}

func (a *App) buildCategories() map[string]*shell.Category {
	return map[string]*shell.Category{
		// InteractiveNeo4jPrerequisite prompts for URI/username/password on
		// first use and saves them so subsequent sessions skip the prompt.
		"cypher": commands.BuildCypherCategory(a.cypherSvc).
			SetPrerequisite(commands.InteractiveNeo4jPrerequisite(&a.cfg.Neo4j, a.cfg, configPath)),
		"cloud": commands.BuildCloudCategory(a.cloudSvc).
			SetPrerequisite(commands.InteractiveAuraPrerequisite(&a.cfg.Aura, a.cfg, configPath)),
		"admin": commands.BuildAdminCategory(a.adminSvc).
			SetPrerequisite(commands.InteractiveNeo4jPrerequisite(&a.cfg.Neo4j, a.cfg, configPath)),
		"config": commands.BuildConfigCategory(a.cfg, configPath),
	}
}

func (a *App) dispatch() error {
	a.log.Debug("starting neo4j-cli", logger.Field{Key: "version", Value: Version})

	if execTool != "" {
		return a.executeDirect()
	}
	return a.startShell()
}

func (a *App) executeDirect() error {
	t, err := a.registry.Get(execTool)
	if err != nil {
		return fmt.Errorf("tool %q not found", execTool)
	}

	// Cancel the tool if the user presses Ctrl+C.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	toolCtx := tool.NewContext().
		WithContext(ctx).
		WithArgs(execArgs).
		WithLogger(a.log).
		WithIO(tool.NewDefaultIOHandler()).
		WithPresenter(a.presentation)

	result, err := t.Execute(*toolCtx)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return fmt.Errorf("interrupted")
		}
		return fmt.Errorf("execute %q: %w", execTool, err)
	}
	if !result.Success {
		return fmt.Errorf("tool %q reported failure: %s", execTool, result.Output)
	}
	if result.Output != "" {
		fmt.Println(result.Output)
	}
	return nil
}

func (a *App) startShell() error {
	s := shell.NewInteractiveShell()
	s.SetLogger(a.log)
	s.SetConfig(*a.cfg)
	s.SetRegistry(a.registry)
	s.SetTelemetry(a.analytic)
	s.SetPresenter(a.presentation)
	s.SetCategories(a.buildCategories())
	s.SetVersion(Version)

	a.log.Debug("starting interactive shell")
	return s.Start()
}

func (a *App) close() {
	// Close the repository first so any in-flight driver connections are
	// released before the analytics flush blocks on network I/O.
	if a.repo != nil {
		if err := a.repo.Close(); err != nil {
			a.log.Error("failed to close repository",
				logger.Field{Key: "error", Value: err.Error()},
			)
		}
	}
	// Flush pending analytics events so none are dropped on shutdown.
	if a.analytic != nil {
		a.analytic.Flush()
	}
	// Close the log file last so that all messages from the steps above
	// are written before the file handle is released.
	if a.logCloser != nil {
		_ = a.logCloser.Close()
	}
}
