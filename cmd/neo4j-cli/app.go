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
		Use:   "neo4j-cli",
		Short: "A CLI for Neo4j",
		Long: `neo4j-cli is a CLI for Neo4j.

Runs as an interactive shell by default. Supply --exec to run a single
named tool and exit immediately.`,
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

	rootCmd.Flags().StringVar(&execTool, "exec", "", "Execute a named tool directly and exit")
	rootCmd.Flags().StringSliceVar(&execArgs, "args", []string{}, "Arguments passed to the --exec tool")

	return rootCmd
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

	// 4. Presentation service
	format := presentation.OutputFormat(cfg.LogFormat)
	if !format.IsValid() {
		format = presentation.OutputFormatText
	}
	pres, err := presentation.NewPresentationService(format, log)
	if err != nil {
		return nil, fmt.Errorf("init presentation: %w", err)
	}

	// 5. Graph repository — shared by cypher and admin services.
	repo := repository.NewNeo4jRepository(
		cfg.Neo4j.URI,
		cfg.Neo4j.Username,
		cfg.Neo4j.Password,
		cfg.Neo4j.Database,
	)

	// 6. Domain services
	cypherSvc := service.NewCypherService(repo)
	cloudSvc := service.NewCloudService(cfg.Aura)
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
		"cypher": commands.BuildCypherCategory(a.cypherSvc).
			SetPrerequisite(commands.Neo4jPrerequisite(&a.cfg.Neo4j)),
		"cloud": commands.BuildCloudCategory(a.cloudSvc).
			SetPrerequisite(commands.AuraPrerequisite(&a.cfg.Aura)),
		"admin": commands.BuildAdminCategory(a.adminSvc).
			SetPrerequisite(commands.Neo4jPrerequisite(&a.cfg.Neo4j)),
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
