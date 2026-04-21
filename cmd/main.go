package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/cli/go-cli-tool/internal/core"
	"github.com/cli/go-cli-tool/internal/repository"
	"github.com/cli/go-cli-tool/internal/service"
	"github.com/cli/go-cli-tool/internal/shell"
	"github.com/cli/go-cli-tool/internal/tool"
	"github.com/cli/go-cli-tool/internal/tools"
	"github.com/spf13/cobra"
)

// noopTelemetry is a TelemetryService that does nothing
type noopTelemetry struct{}

func (t *noopTelemetry) Track(ctx context.Context, eventName string, props map[string]any) error { return nil }
func (t *noopTelemetry) TrackStartup(ctx context.Context) error                                 { return nil }
func (t *noopTelemetry) TrackShutdown(ctx context.Context) error                                { return nil }
func (t *noopTelemetry) TrackToolUsed(ctx context.Context, toolName string, args []string) error {
	return nil
}
func (t *noopTelemetry) TrackToolSuccess(ctx context.Context, toolName string, dur float64) error {
	return nil
}
func (t *noopTelemetry) TrackToolError(ctx context.Context, toolName string, err error) error {
	return nil
}

var (
	configPath string
	logLevel   string
	logFormat  string
	shellMode  bool
	execTool   string
	execArgs   []string
)

func main() {
	// Initialize root command
	rootCmd := &cobra.Command{
		Use:   "go-cli-tool",
		Short: "A modular, extensible CLI framework",
		Long:  `go-cli-tool is a Go-based CLI framework with shell interface, logging, and configuration management.`,
		Run:   runRoot,
	}

	// Add flags
	rootCmd.PersistentFlags().StringVar(&configPath, "config", "", "Path to configuration file")
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "", "Log level (debug, info, warn, error)")
	rootCmd.PersistentFlags().StringVar(&logFormat, "log-format", "", "Log format (text, json)")
	rootCmd.PersistentFlags().BoolVarP(&shellMode, "shell", "s", false, "Start interactive shell")
	rootCmd.PersistentFlags().StringVar(&execTool, "exec", "", "Execute a tool directly")
	rootCmd.Flags().StringSliceVar(&execArgs, "args", []string{}, "Arguments for tool execution")

	// Execute
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func runRoot(cmd *cobra.Command, args []string) {
	// Load configuration
	config := loadConfiguration()

	// Create logger
	logger := createLogger(config)

	// Initialize Telemetry
	var telemetry service.TelemetryService
	if config.Telemetry.MixpanelToken != "" {
		telemetry = service.NewMixpanelService(config.Telemetry.MixpanelToken, logger)
	} else {
		// No-op telemetry if token is missing
		telemetry = &noopTelemetry{}
	}

	ctx := context.Background()
	telemetry.TrackStartup(ctx)
	defer telemetry.TrackShutdown(ctx)

	logger.Info("Starting go-cli-tool", core.Field{Key: "version", Value: "1.0.0"})

	// Initialize Repository
	repo := repository.NewNeo4jRepository("bolt://localhost:7687", "neo4j", "password")
	defer repo.Close()

	// Initialize Service
	graphService := service.NewGraphService(repo)

	// Initialize Presentation Service
	presentation := service.NewPresentationService(service.OutputFormat(config.LogFormat), logger)

	// Create tool registry
	registry := tools.NewToolRegistry()

	// Register built-in tools
	registerTools(registry, config, logger, graphService)

	logger.Info("Registered tools", core.Field{Key: "count", Value: registry.Count()})

	// Execute tool directly if specified
	if execTool != "" {
		executeToolDirect(registry, logger, telemetry, presentation)
		return
	}

	// Start shell by default
	startShell(registry, config, logger, telemetry, presentation)
}

// loadConfiguration loads configuration from file and environment
func loadConfiguration() core.Config {
	// Start with default config
	configLoader := core.NewJSONConfigLoader()

	// Load from config file if specified
	if configPath != "" {
		if config, err := configLoader.Load(configPath); err == nil {
			// Override with command-line flags
			if logLevel != "" {
				config.LogLevel = logLevel
			}
			if logFormat != "" {
				config.LogFormat = logFormat
			}
			return config
		}
	}

	// Load from environment
	config := configLoader.LoadFromEnv()

	// Override with command-line flags
	if logLevel != "" {
		config.LogLevel = logLevel
	}
	if logFormat != "" {
		config.LogFormat = logFormat
	}

	// Set defaults if not loaded
	if config.LogLevel == "" {
		config.LogLevel = "info"
	}
	if config.LogFormat == "" {
		config.LogFormat = "text"
	}
	if config.Shell.Prompt == "" {
		config.Shell.Prompt = "cli> "
	}
	if config.Shell.HistoryFile == "" {
		config.Shell.HistoryFile = ".cli_history"
	}

	return config
}

// createLogger creates a logger based on configuration
func createLogger(config core.Config) core.Logger {
	level := core.ParseLogLevel(config.LogLevel)
	format := core.ParseLogFormat(config.LogFormat)
	return core.NewLogger(format, level)
}

// registerTools registers all available tools
func registerTools(registry *tools.ToolRegistry, config core.Config, logger core.Logger, graphService *service.GraphService) {
	// Create and register echo tool
	echoTool := tools.NewEchoTool()
	if toolConfig, ok := config.Tools["echo"]; ok {
		if err := registry.RegisterWithConfig(echoTool, toolConfig); err != nil {
			logger.Warn("Failed to configure echo tool", core.Field{Key: "error", Value: err.Error()})
		}
	} else {
		registry.Register(echoTool)
	}

	// Create and register help tool
	helpTool := tools.NewHelpTool(registry)
	if toolConfig, ok := config.Tools["help"]; ok {
		if err := registry.RegisterWithConfig(helpTool, toolConfig); err != nil {
			logger.Warn("Failed to configure help tool", core.Field{Key: "error", Value: err.Error()})
		}
	} else {
		registry.Register(helpTool)
	}

	// Create and register query tool
	queryTool := tools.NewQueryTool(graphService)
	if toolConfig, ok := config.Tools["query"]; ok {
		if err := registry.RegisterWithConfig(queryTool, toolConfig); err != nil {
			logger.Warn("Failed to configure query tool", core.Field{Key: "error", Value: err.Error()})
		}
	} else {
		registry.Register(queryTool)
	}
}

// executeToolDirect executes a tool directly from command line
func executeToolDirect(registry *tools.ToolRegistry, logger core.Logger, telemetry service.TelemetryService, presenter *service.PresentationService) {
	// Get the tool
	toolInstance, err := registry.Get(execTool)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Track tool used
	ctx := context.Background()
	telemetry.TrackToolUsed(ctx, execTool, execArgs)
	start := time.Now()

	// Create execution context
	toolCtx := tool.NewContext().
		WithArgs(execArgs).
		WithLogger(logger).
		WithIO(tool.NewDefaultIOHandler()).
		WithPresenter(presenter)

	// Execute the tool
	result, err := toolInstance.Execute(*toolCtx)
	duration := time.Since(start).Seconds()

	if err != nil {
		telemetry.TrackToolError(ctx, execTool, err)
		fmt.Fprintf(os.Stderr, "Error executing tool: %v\n", err)
		os.Exit(1)
	}

	if !result.Success {
		if result.Error != nil {
			telemetry.TrackToolError(ctx, execTool, result.Error)
			fmt.Fprintf(os.Stderr, "Error: %v\n", result.Error)
		} else {
			telemetry.TrackToolError(ctx, execTool, fmt.Errorf("tool execution failed"))
			fmt.Fprintln(os.Stderr, "Tool execution failed")
		}
		os.Exit(1)
	}

	// Track success
	telemetry.TrackToolSuccess(ctx, execTool, duration)

	// Output result
	if result.Output != "" {
		fmt.Println(result.Output)
	}
}

// startShell starts the interactive shell
func startShell(registry *tools.ToolRegistry, config core.Config, logger core.Logger, telemetry service.TelemetryService, presenter *service.PresentationService) {
	// Create shell
	s := shell.NewInteractiveShell()
	s.SetLogger(logger)
	s.SetConfig(config)
	s.SetRegistry(registry)
	s.SetTelemetry(telemetry)
	s.SetPresenter(presenter)

	logger.Info("Starting interactive shell")

	// Start shell
	if err := s.Start(); err != nil {
		logger.Fatal("Failed to start shell", core.Field{Key: "error", Value: err.Error()})
	}

	// Wait for shell to stop
	for s.IsRunning() {
		// Keep alive
	}
}

// getEnvOrDefault returns environment variable or default value
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// parseArgs parses command line arguments into flags and positional args
func parseArgs(args []string) ([]string, map[string]string) {
	var positional []string
	flags := make(map[string]string)

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if strings.HasPrefix(arg, "--") {
			parts := strings.SplitN(arg[2:], "=", 2)
			if len(parts) == 2 {
				flags[parts[0]] = parts[1]
			} else if i+1 < len(args) && !strings.HasPrefix(args[i+1], "--") {
				flags[parts[0]] = args[i+1]
				i++
			}
		} else if strings.HasPrefix(arg, "-") {
			parts := strings.SplitN(arg[1:], "=", 2)
			if len(parts) == 2 {
				flags[parts[0]] = parts[1]
			} else if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				flags[parts[0]] = args[i+1]
				i++
			}
		} else {
			positional = append(positional, arg)
		}
	}

	return positional, flags
}
