package main

import (
	"fmt"

	"github.com/cli/go-cli-tool/internal/analytics"
	"github.com/cli/go-cli-tool/internal/cli"
	"github.com/cli/go-cli-tool/internal/commands"
	"github.com/cli/go-cli-tool/internal/config"
	"github.com/cli/go-cli-tool/internal/logger"
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
	configPath string
	logLevel   string
	logFormat  string
	shellMode  bool
	execTool   string
	execArgs   []string
	metrics    bool

	// Neo4j connection (non-secret values only — password comes from env/file).
	neo4jURI      string
	neo4jUsername string
	neo4jDatabase string

	// Aura API (non-secret values only — client secret comes from env/file).
	auraClientID      string
	auraTimeoutSeconds int
)

// ---- App ----------------------------------------------------------------

type App struct {
	cfg      *config.Config
	log      logger.Service
	analytic analytics.Service
	neo4jCLI *cli.Neo4jCLI
	registry *tools.ToolRegistry

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

	// ---- General flags --------------------------------------------------
	pf := rootCmd.PersistentFlags()
	pf.StringVar(&configPath, "config-file", "", "Path to a JSON configuration file")
	pf.StringVar(&logLevel, "log-level", "", "Log level: debug, info, warn, error")
	pf.StringVar(&logFormat, "log-format", "", "Log format: text, json")
	pf.BoolVar(&metrics, "metrics", true, "Send usage metrics to Neo4j")
	pf.BoolVarP(&shellMode, "shell", "s", false, "Start interactive shell (default when no --exec)")

	// ---- Neo4j connection flags -----------------------------------------
	// Secrets (password) are intentionally excluded: passing a password on
	// the command line exposes it in shell history and ps output. Use
	// CLI_NEO4J_PASSWORD or the config file instead.
	pf.StringVar(&neo4jURI, "neo4j-uri", "", "Neo4j bolt URI (e.g. bolt://localhost:7687)")
	pf.StringVar(&neo4jUsername, "neo4j-username", "", "Neo4j username")
	pf.StringVar(&neo4jDatabase, "neo4j-database", "", "Neo4j database name (default: neo4j)")

	// ---- Aura API flags -------------------------------------------------
	// Client secret is excluded for the same reason as the password above.
	// Use CLI_AURA_CLIENT_SECRET or the config file.
	pf.StringVar(&auraClientID, "aura-client-id", "", "Neo4j Aura API client ID")
	pf.IntVar(&auraTimeoutSeconds, "aura-timeout", 0, "Aura API request timeout in seconds (default: 30)")

	// ---- Exec flags (root-local) ----------------------------------------
	rootCmd.Flags().StringVar(&execTool, "exec", "", "Execute a named tool directly and exit")
	rootCmd.Flags().StringSliceVar(&execArgs, "args", []string{}, "Arguments passed to the --exec tool")

	return rootCmd
}

func runCLI(cmd *cobra.Command, args []string) error {
	app, err := newApp(cmd, args)
	if err != nil {
		return fmt.Errorf("startup: %w", err)
	}
	defer app.close()
	return app.dispatch()
}

func newApp(cmd *cobra.Command, args []string) (*App, error) {
	// 1. Config — flags > env vars > config file > defaults.
	cfgSvc := config.NewConfigService(cmd, args)
	cfg, err := cfgSvc.LoadConfiguration()
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	// 2. Logger
	log := logger.NewLoggerService(
		logger.ParseLogFormat(cfg.LogFormat),
		logger.ParseLogLevel(cfg.LogLevel),
	)

	// 3. Analytics
	an := analytics.NewAnalytics(mixPanelToken, mixPanelEndpoint, cfg.Neo4j.URI)
	if !cfg.Telemetry.Metrics {
		an.Disable()
	}

	// 4. CLI facade
	neo4jCLI, err := cli.NewCLI(&cfg, log, an)
	if err != nil {
		return nil, fmt.Errorf("init CLI facade: %w", err)
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

	// 7. Tool registry
	registry := buildRegistry(&cfg, neo4jCLI, log, repo)

	return &App{
		cfg:       &cfg,
		log:       log,
		analytic:  an,
		neo4jCLI:  neo4jCLI,
		registry:  registry,
		cypherSvc: cypherSvc,
		cloudSvc:  cloudSvc,
		adminSvc:  adminSvc,
	}, nil
}

func buildRegistry(cfg *config.Config, neo4jCLI *cli.Neo4jCLI, log logger.Service, repo repository.GraphRepository) *tools.ToolRegistry {
	graphSvc := service.NewGraphService(repo)
	registry := tools.NewToolRegistry()

	for _, t := range []tool.Tool{
		tools.NewEchoTool(),
		tools.NewHelpTool(registry),
		tools.NewQueryTool(graphSvc),
	} {
		registerTool(registry, t, cfg, log)
	}

	return registry
}

func registerTool(r *tools.ToolRegistry, t tool.Tool, cfg *config.Config, log logger.Service) {
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
		"cypher": commands.BuildCypherCategory(a.cypherSvc),
		"cloud":  commands.BuildCloudCategory(a.cloudSvc),
		"admin":  commands.BuildAdminCategory(a.adminSvc),
	}
}

func (a *App) dispatch() error {
	a.log.Info("starting neo4j-cli", logger.Field{Key: "version", Value: Version})

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

	toolCtx := tool.NewContext().
		WithArgs(execArgs).
		WithLogger(a.log).
		WithIO(tool.NewDefaultIOHandler()).
		WithPresenter(a.neo4jCLI.Presentation)

	result, err := t.Execute(*toolCtx)
	if err != nil {
		return fmt.Errorf("execute %q: %w", execTool, err)
	}
	if !result.Success {
		if result.Error != nil {
			return result.Error
		}
		return fmt.Errorf("tool %q reported failure", execTool)
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
	s.SetTelemetry(a.neo4jCLI.Telemetry)
	s.SetPresenter(a.neo4jCLI.Presentation)
	s.SetCategories(a.buildCategories())

	a.log.Info("starting interactive shell")
	return s.Start()
}

func (a *App) close() {
	// Future: close Neo4j driver session, flush analytics queue.
}
