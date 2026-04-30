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
	"github.com/cli/go-cli-tool/internal/cli"
	"github.com/cli/go-cli-tool/internal/commands"
	"github.com/cli/go-cli-tool/internal/config"
	"github.com/cli/go-cli-tool/internal/dispatch"
	"github.com/cli/go-cli-tool/internal/logger"
	"github.com/cli/go-cli-tool/internal/presentation"
	"github.com/cli/go-cli-tool/internal/repository"
	"github.com/cli/go-cli-tool/internal/service"
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

// flags holds every persistent flag bound by the cobra tree. The cli package
// owns the struct so the tree builder is reusable from cmd/gen-skill.
var flags = &cli.Flags{}

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
	skillSvc  service.SkillService
}

func run() int {
	// Pre-scan raw os.Args for agent-mode flags before cobra runs.
	// This ensures that errors cobra raises itself (e.g. unknown subcommand,
	// flag parse failure) are still formatted as JSON envelopes when --agent
	// is present anywhere on the command line.
	for _, arg := range os.Args[1:] {
		switch arg {
		case "--agent":
			flags.AgentMode = true
		case "--rw":
			flags.AllowWrites = true
		}
	}

	// Honour env vars too; either source winning is correct.
	if os.Getenv("NEO4J_CLI_AGENT") == "true" {
		flags.AgentMode = true
	}
	if os.Getenv("NEO4J_CLI_RW") == "true" {
		flags.AllowWrites = true
	}
	if v := os.Getenv("NEO4J_CLI_REQUEST_ID"); v != "" {
		flags.RequestID = v
	}

	cmd := buildRootCommand()
	if err := cmd.Execute(); err != nil {
		if flags.AgentMode {
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
			fmt.Fprintf(os.Stderr, "Run 'neo4j-cli --help' for usage.\n")
		}
		return 1
	}
	return 0
}

// currentRequestID returns the active request ID, generating one if the user
// did not supply --request-id or NEO4J_CLI_REQUEST_ID.
func currentRequestID() string {
	if flags.RequestID != "" {
		return flags.RequestID
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

// buildRootCommand assembles the cobra tree and wires RunE handlers via the
// shared internal/cli builder so cmd/gen-skill walks an identical tree.
func buildRootCommand() *cobra.Command {
	return cli.BuildCobraTree(cli.Options{
		Flags:      flags,
		RunFactory: runCategory,
	})
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

// isJSONMode reports whether the current invocation should produce JSON envelope
// output. True when --format json/pretty-json is set, or when --agent is active
// with no explicit format override.
func isJSONMode() bool {
	switch flags.OutputFormat {
	case "json", "pretty-json":
		return true
	}
	return flags.AgentMode && flags.OutputFormat == ""
}

// resolveHumanFormat returns the output format to use for human-mode rendering,
// preferring a per-result override, then the --format flag, then a default.
func resolveHumanFormat(override presentation.OutputFormat) presentation.OutputFormat {
	if override != "" {
		return override
	}
	if f := presentation.OutputFormat(flags.OutputFormat); f.IsValid() {
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
	if flags.OutputFormat == "pretty-json" {
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
			flags.AgentMode = true
		case "--rw":
			flags.AllowWrites = true
		}
	}

	cats := a.buildCategories()
	cat, ok := cats[name]
	if !ok {
		return fmt.Errorf("unknown category %q", name)
	}

	// Validate --format early so we surface bad values before network calls.
	if flags.OutputFormat != "" {
		f := presentation.OutputFormat(flags.OutputFormat)
		if !f.IsValid() {
			return fmt.Errorf("invalid format %q: must be one of table, json, pretty-json, graph", flags.OutputFormat)
		}
	}

	// Build the command context with optional timeout.
	baseCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	var ctx context.Context
	if flags.TimeoutStr != "" {
		d, err := time.ParseDuration(flags.TimeoutStr)
		if err != nil {
			return fmt.Errorf("invalid --timeout %q: %w", flags.TimeoutStr, err)
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
		AgentMode:   flags.AgentMode,
		AllowWrites: flags.AllowWrites,
		RequestID:   reqID,
	}

	result, err := cat.Dispatch(args, shellCtx)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return fmt.Errorf("interrupted")
		}
		if errors.Is(err, context.DeadlineExceeded) {
			if flags.AgentMode {
				return tool.NewAgentError("TIMEOUT",
					fmt.Sprintf("command timed out after %s", flags.TimeoutStr))
			}
			return fmt.Errorf("command timed out after %s", flags.TimeoutStr)
		}
		return err
	}

	if isJSONMode() {
		fmt.Println(buildJSONEnvelope(result, reqID))
		return nil
	}
	return a.printHumanResult(result)
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
		v := !flags.NoMetrics // --no-metrics disables; --no-metrics=false re-enables
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
	skillSvc := service.NewSkillService()

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
		skillSvc:     skillSvc,
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
	if flags.AgentMode {
		neo4jPrereq = commands.Neo4jPrerequisite(&a.cfg.Neo4j)
		auraPrereq = commands.AuraPrerequisite(&a.cfg.Aura)
	} else {
		neo4jPrereq = commands.InteractiveNeo4jPrerequisite(&a.cfg.Neo4j, a.cfg, flags.ConfigPath)
		auraPrereq = commands.InteractiveAuraPrerequisite(&a.cfg.Aura, a.cfg, flags.ConfigPath)
	}

	return map[string]*dispatch.Category{
		"cypher": commands.BuildCypherCategory(a.cypherSvc).
			SetPrerequisite(neo4jPrereq),
		"cloud": commands.BuildCloudCategory(a.cloudSvc).
			SetPrerequisite(auraPrereq),
		"admin": commands.BuildAdminCategory(a.adminSvc).
			SetPrerequisite(neo4jPrereq),
		"config": commands.BuildConfigCategory(a.cfg, flags.ConfigPath),
		"skill":  commands.BuildSkillCategory(a.skillSvc),
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
