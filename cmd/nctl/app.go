package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/spf13/cobra"

	"github.com/cli/go-cli-tool/internal/analytics"
	"github.com/cli/go-cli-tool/internal/commands"
	"github.com/cli/go-cli-tool/internal/config"
	"github.com/cli/go-cli-tool/internal/dispatch"
	"github.com/cli/go-cli-tool/internal/logger"
	"github.com/cli/go-cli-tool/internal/presentation"
	"github.com/cli/go-cli-tool/internal/repository"
	"github.com/cli/go-cli-tool/internal/service"
	"github.com/cli/go-cli-tool/internal/shell"
	"github.com/cli/go-cli-tool/internal/tool"
	"github.com/cli/go-cli-tool/internal/tools"
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
	noMetrics          bool
	neo4jURI           string
	neo4jUsername      string
	neo4jDatabase      string
	auraClientID       string
	auraTimeoutSeconds int
	outputFormat       string

	// Shell-mode flags
	shellFlag bool // --shell (explicit; may be true or false)

	// Agent-mode flags
	agentMode   bool   // --agent / NCTL_AGENT=true
	allowWrites bool   // --rw / NCTL_RW=true
	requestID   string // --request-id / NCTL_REQUEST_ID
	timeoutStr  string // --timeout (e.g. "30s")
)

// ---- App ----------------------------------------------------------------

type App struct {
	cfg          *config.Config
	log          logger.Service
	logCloser    io.Closer // non-nil when logging to a file; closed last in close()
	analytic     analytics.Service
	presentation presentation.Service
	registry     *tools.ToolRegistry
	repo         repository.GraphRepository // held so close() can release driver resources

	cypherSvc service.CypherService
	cloudSvc  service.CloudService
	adminSvc  service.AdminService
}

func run() int {
	// Pre-scan raw os.Args for agent-mode flags before cobra runs.
	// This ensures that errors cobra raises itself (e.g. unknown subcommand,
	// flag parse failure) are still formatted as JSON envelopes when --agent
	// is present anywhere on the command line.
	for _, arg := range os.Args[1:] {
		switch arg {
		case "--agent":
			agentMode = true
		case "--rw":
			allowWrites = true
		}
	}

	// Honour env vars too; either source winning is correct.
	if os.Getenv("NCTL_AGENT") == "true" {
		agentMode = true
	}
	if os.Getenv("NCTL_RW") == "true" {
		allowWrites = true
	}
	if v := os.Getenv("NCTL_REQUEST_ID"); v != "" {
		requestID = v
	}

	cmd := buildRootCommand()
	if err := cmd.Execute(); err != nil {
		if agentMode {
			printAgentError(err, currentRequestID())
		} else {
			fmt.Fprintf(os.Stderr, "Error: %s\n\n", err)
			var names []string
			for _, sub := range cmd.Commands() {
				if !sub.Hidden {
					names = append(names, sub.Name())
				}
			}
			sort.Strings(names)
			fmt.Fprintf(os.Stderr, "Valid commands: %s\n", strings.Join(names, ", "))
			fmt.Fprintf(os.Stderr, "Run 'nctl --help' for usage.\n")
		}
		return 1
	}
	return 0
}

// currentRequestID returns the active request ID, generating one if the user
// did not supply --request-id or NCTL_REQUEST_ID.
func currentRequestID() string {
	if requestID != "" {
		return requestID
	}
	return uuid.New().String()
}

// printAgentError writes a structured JSON error envelope to stdout so agents
// reading stdout (not stderr) get machine-readable failure information.
func printAgentError(err error, reqID string) {
	code := "EXECUTION_ERROR"
	message := err.Error()

	var ae *tool.AgentError
	switch {
	case errors.As(err, &ae):
		code = ae.Code
		message = ae.Message
	case strings.Contains(message, "unknown command"):
		code = "UNKNOWN_COMMAND"
	case strings.Contains(message, "unknown flag") || strings.Contains(message, "unknown shorthand flag"):
		code = "UNKNOWN_FLAG"
	}

	envelope := map[string]interface{}{
		"status": "error",
		"error": map[string]string{
			"code":    code,
			"message": message,
		},
		"request_id":     reqID,
		"schema_version": "1",
	}
	b, _ := json.Marshal(envelope)
	fmt.Println(string(b))
}

func buildRootCommand() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "nctl",
		Short: "A CLI for Neo4j",
		Long: `nctl is a command-line tool for Neo4j.

Use a subcommand to interact with Neo4j databases and Aura cloud resources, or
run without arguments to start the interactive shell.`,
		RunE:          launchShell,
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	pf := rootCmd.PersistentFlags()
	pf.StringVar(&configPath, "config-file", "", "Path to a JSON configuration file")
	pf.StringVar(&logLevel, "log-level", "", "Log level: debug, info, warn, error")
	pf.StringVar(&logFormat, "log-format", "", "Log format: text, json")
	pf.StringVar(&logOutput, "log-output", "", "Log destination: stderr (default), stdout, file")
	pf.StringVar(&logFile, "log-file", "", "Log file path when --log-output=file (default: ~/.nctl/nctl.log)")
	pf.BoolVar(&noMetrics, "no-metrics", false, "Disable sending usage metrics to Neo4j (overrides config file and CLI_TELEMETRY_METRICS env var)")
	pf.StringVar(&neo4jURI, "neo4j-uri", "", "Neo4j bolt URI (e.g. bolt://localhost:7687)")
	pf.StringVar(&neo4jUsername, "neo4j-username", "", "Neo4j username")
	pf.StringVar(&neo4jDatabase, "neo4j-database", "", "Neo4j database name")
	pf.StringVar(&auraClientID, "aura-client-id", "", "Neo4j Aura API client ID")
	pf.IntVar(&auraTimeoutSeconds, "aura-timeout", 0, "Aura API request timeout in seconds")
	pf.StringVar(&outputFormat, "format", "", "Output format: table, json, pretty-json, graph, toon")

	// Shell-mode flag
	pf.BoolVar(&shellFlag, "shell", true, "Enable the interactive shell (env: CLI_SHELL_ENABLED=false to disable)")

	// Agent-mode flags
	pf.BoolVar(&agentMode, "agent", false, "Enable agent-optimised mode: JSON output, read-only by default, no interactive prompts, errors on stdout (env: NCTL_AGENT=true)")
	pf.BoolVar(&allowWrites, "rw", false, "Permit write/mutating operations in agent mode (env: NCTL_RW=true)")
	pf.StringVar(&requestID, "request-id", "", "Correlation ID included in every agent-mode JSON response (env: NCTL_REQUEST_ID)")
	pf.StringVar(&timeoutStr, "timeout", "", "Maximum time for a command to run, e.g. 30s or 2m (exit code 124 on timeout)")

	rootCmd.AddCommand(buildCloudCommand())
	rootCmd.AddCommand(buildCypherCommand())
	rootCmd.AddCommand(buildAdminCommand())
	rootCmd.AddCommand(buildConfigCommand())

	return rootCmd
}

// runCategory returns a cobra RunE that creates the app, finds the named
// category, and dispatches the remaining CLI args through it directly.
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

// launchShell is the RunE for the root command when no subcommand is provided.
// It starts the interactive REPL unless agent mode is active or the shell is
// explicitly disabled, in which case it falls back to cobra's help output.
func launchShell(cmd *cobra.Command, _ []string) error {
	// Agent mode: the root command should not interfere with subcommand dispatch.
	// Cobra calls RunE on the root when no subcommand matches — returning an
	// error here would produce a confusing error message, so print help instead.
	if agentMode {
		return cmd.Help()
	}

	app, err := newApp(cmd, nil)
	if err != nil {
		return fmt.Errorf("startup: %w", err)
	}
	defer app.close()

	// Shell disabled via config, env var, or --shell=false flag.
	if !app.cfg.Shell.Enabled {
		return cmd.Help()
	}

	return startShell(app)
}

// startShell wires all app services into a new InteractiveShell, bridges the
// dispatch categories, and runs the REPL until the user exits.
func startShell(app *App) error {
	s := shell.NewInteractiveShell()
	s.SetLogger(app.log)
	s.SetConfig(*app.cfg)
	s.SetTelemetry(app.analytic)
	s.SetPresenter(app.presentation)
	s.SetRegistry(app.registry)
	s.SetVersion(Version)

	// Build dispatch categories using non-interactive prerequisites so
	// missing credentials prompt the user inside the shell REPL rather
	// than blocking before it starts.
	cats := app.buildCategories()

	// Bridge each dispatch category into a shell category. The ctxFor
	// function converts the per-command shell.Context into the
	// dispatch.Context expected by the underlying handlers.
	shellCats := make(map[string]*shell.Category, len(cats))
	for name, cat := range cats {
		shellCats[name] = shell.BridgeCategory(cat, func(ctx shell.Context) dispatch.Context {
			return dispatch.Context{
				Context:     ctx.Context,
				Config:      ctx.Config,
				Logger:      ctx.Logger,
				Telemetry:   ctx.Telemetry,
				Presenter:   ctx.Presenter,
				Registry:    ctx.Registry,
				IO:          ctx.IO,
				AgentMode:   false,
				AllowWrites: true,
			}
		})
	}
	s.SetCategories(shellCats)

	return s.Start()
}

// isJSONMode reports whether the current invocation should produce JSON envelope
// output. True when --format json/pretty-json is set, or when --agent is active
// with no explicit format override.
func isJSONMode() bool {
	switch outputFormat {
	case "json", "pretty-json":
		return true
	}
	return agentMode && outputFormat == ""
}

// resolveDefaultPresentationFormat picks the default OutputFormat to install
// on the presentation service at startup. Precedence:
//
//  1. cypher.output_format from config, when set to a recognised value
//  2. OutputFormatTOON when running in agent mode (machine-friendly default)
//  3. OutputFormatTable (today's human-mode default)
//
// Per-call overrides (--format flag, FormatAs) bypass this default entirely.
func resolveDefaultPresentationFormat(cfg *config.Config, agent bool) presentation.OutputFormat {
	if cfg != nil {
		if f := presentation.OutputFormat(cfg.Cypher.OutputFormat); f.IsValid() {
			return f
		}
	}
	if agent {
		return presentation.OutputFormatTOON
	}
	return presentation.OutputFormatTable
}

// resolveHumanFormat returns the output format to use for human-mode rendering,
// preferring a per-result override, then the --format flag, then a default.
func resolveHumanFormat(override presentation.OutputFormat) presentation.OutputFormat {
	if override != "" {
		return override
	}
	if f := presentation.OutputFormat(outputFormat); f.IsValid() {
		return f
	}
	return presentation.OutputFormatTable
}

// buildJSONEnvelope assembles the standard JSON response envelope.
func buildJSONEnvelope(result dispatch.CommandResult, reqID string) string {
	var data interface{}
	switch {
	case result.Item != nil:
		data = result.Item
	case result.Items != nil:
		data = result.Items
	case result.Message != "":
		data = map[string]interface{}{"message": result.Message}
	}

	envelope := map[string]interface{}{
		"status":         "ok",
		"data":           data,
		"request_id":     reqID,
		"schema_version": "1",
	}
	var b []byte
	if outputFormat == "pretty-json" {
		b, _ = json.MarshalIndent(envelope, "", "  ")
	} else {
		b, _ = json.Marshal(envelope)
	}
	return string(b)
}

// printHumanResult renders result for a human reader using the presentation
// service. Falls back to Message when Presentation is nil.
func (a *App) printHumanResult(result dispatch.CommandResult) error {
	if result.Presentation != nil {
		format := resolveHumanFormat(result.FormatOverride)
		out, err := a.presentation.FormatAs(result.Presentation, format)
		if err != nil {
			return fmt.Errorf("format result: %w", err)
		}
		if out != "" {
			fmt.Println(out)
		}
		return nil
	}
	if result.Message != "" {
		fmt.Println(result.Message)
	}
	return nil
}

// dispatchCategory routes args through the named category, formats the
// CommandResult, and writes it to stdout. Errors in agent mode are rendered
// as JSON envelopes by run(); dispatchCategory returns them unwrapped.
func (a *App) dispatchCategory(name string, args []string) error {
	// For subcommands with DisableFlagParsing (e.g. cypher), cobra skips
	// persistent flag parsing, so --agent and --rw arrive unprocessed in args.
	// Pre-scan here so that isJSONMode(), shellCtx, and buildCategories() all
	// see the correct agent/rw state before dispatch runs.
	for _, arg := range args {
		switch arg {
		case "--agent":
			agentMode = true
		case "--rw":
			allowWrites = true
		}
	}

	cats := a.buildCategories()
	cat, ok := cats[name]
	if !ok {
		return fmt.Errorf("unknown category %q", name)
	}

	// Validate --format early so we surface bad values before network calls.
	if outputFormat != "" {
		f := presentation.OutputFormat(outputFormat)
		if !f.IsValid() {
			return fmt.Errorf("invalid format %q: must be one of table, json, pretty-json, graph, toon", outputFormat)
		}
	}

	// Build the command context with optional timeout.
	baseCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	var ctx context.Context
	if timeoutStr != "" {
		d, err := time.ParseDuration(timeoutStr)
		if err != nil {
			return fmt.Errorf("invalid --timeout %q: %w", timeoutStr, err)
		}
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(baseCtx, d)
		defer cancel()
	} else {
		ctx = baseCtx
	}

	reqID := currentRequestID()

	shellCtx := dispatch.Context{
		Context:     ctx,
		Config:      *a.cfg,
		Logger:      a.log,
		Telemetry:   a.analytic,
		Presenter:   a.presentation,
		Registry:    a.registry,
		IO:          tool.NewDefaultIOHandler(),
		AgentMode:   agentMode,
		AllowWrites: allowWrites,
		RequestID:   reqID,
	}

	result, err := cat.Dispatch(args, shellCtx)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return fmt.Errorf("interrupted")
		}
		if errors.Is(err, context.DeadlineExceeded) {
			if agentMode {
				return tool.NewAgentError("TIMEOUT",
					fmt.Sprintf("command timed out after %s", timeoutStr))
			}
			return fmt.Errorf("command timed out after %s", timeoutStr)
		}
		return err
	}

	if isJSONMode() {
		fmt.Println(buildJSONEnvelope(result, reqID))
		return nil
	}
	return a.printHumanResult(result)
}

func buildCloudCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "cloud",
		Short: "Manage Neo4j Aura cloud resources",
		Long: `Manage Neo4j Aura cloud resources.

Available sub-commands:
  instances   Create, list, get, update, pause, resume, and delete Aura DB instances
  projects    List and inspect Aura projects (tenants)

Use --format to control output (table, json, pretty-json, graph, toon).`,
		Example: `  nctl cloud instances list
  nctl cloud instances list --format json
  nctl cloud instances get <id>
  nctl cloud instances create name=my-db tenant=<tenant-id> cloud=aws region=us-east-1
  nctl cloud instances pause <id>
  nctl cloud projects list`,
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

Supply the query directly on the command line.

Query flags (parsed inline, not by cobra):
  --param key=value    Add a query parameter (repeatable; auto-typed)
  --format table|json|pretty-json|graph|toon
                       Override output format for this query
  --limit N            Override the auto-injected row limit (default 25)`,
		Example: `  nctl cypher "MATCH (n:Person) RETURN n.name, n.age;"
  nctl cypher --param name=Alice "MATCH (n:Person {name:\$name}) RETURN n;"
  nctl cypher --format json "MATCH (n) RETURN n;"`,
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

Use --format to control output (table, json, pretty-json, graph, toon).`,
		Example: `  nctl admin show-users
  nctl admin show-users --format json
  nctl admin show-databases`,
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
		Example: `  nctl config list
  nctl config list --format json
  nctl config set neo4j.uri bolt://myhost:7687
  nctl config set cypher.output_format json
  nctl config delete neo4j.password
  nctl config reset`,
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
	if f.Changed("shell") {
		v := shellFlag
		o.ShellEnabled = &v
	}
	if f.Changed("no-metrics") {
		v := !noMetrics // --no-metrics disables; --no-metrics=false re-enables
		o.MetricsEnabled = &v
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

	// 3. Analytics — prefer a token from config/env (CLI_TELEMETRY_MIXPANEL_TOKEN)
	// so the hardcoded default can be overridden without a rebuild.
	token := mixPanelToken
	if cfg.Telemetry.MixpanelToken != "" {
		token = cfg.Telemetry.MixpanelToken
	}
	an := analytics.NewAnalytics(token, mixPanelEndpoint, cfg.Neo4j.URI, Version, log)
	if !cfg.Telemetry.Metrics {
		an.Disable()
	}

	// 4. Presentation service — pick a default that respects agent mode and
	// any explicit cypher.output_format the user has set in their config.
	// Commands use FormatAs for per-query overrides; the session default
	// can be changed with `set cypher-format <format>`.
	pres, err := presentation.NewPresentationService(resolveDefaultPresentationFormat(&cfg, agentMode), log)
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
// CypherService used by the 'cypher' command — one query path, no duplication.
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

func (a *App) buildCategories() map[string]*dispatch.Category {
	// In agent mode use non-interactive prerequisites so missing credentials
	// return a structured error immediately rather than blocking on stdin.
	var neo4jPrereq, auraPrereq func() error
	if agentMode {
		neo4jPrereq = commands.Neo4jPrerequisite(&a.cfg.Neo4j)
		auraPrereq = commands.AuraPrerequisite(&a.cfg.Aura)
	} else {
		neo4jPrereq = commands.InteractiveNeo4jPrerequisite(&a.cfg.Neo4j, a.cfg, configPath)
		auraPrereq = commands.InteractiveAuraPrerequisite(&a.cfg.Aura, a.cfg, configPath)
	}

	return map[string]*dispatch.Category{
		"cypher": commands.BuildCypherCategory(a.cypherSvc).
			SetPrerequisite(neo4jPrereq),
		"cloud": commands.BuildCloudCategory(a.cloudSvc).
			SetPrerequisite(auraPrereq),
		"admin": commands.BuildAdminCategory(a.adminSvc).
			SetPrerequisite(neo4jPrereq),
		"config": commands.BuildConfigCategory(a.cfg, configPath),
	}
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
