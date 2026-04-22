package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/cli/go-cli-tool/internal/cli"
	"github.com/cli/go-cli-tool/internal/config"
	"github.com/cli/go-cli-tool/internal/logger"
	"github.com/cli/go-cli-tool/internal/service"
	"github.com/cli/go-cli-tool/internal/shell"
	"github.com/cli/go-cli-tool/internal/tool"
	"github.com/cli/go-cli-tool/internal/tools"
	"github.com/spf13/cobra"
)

var (
	configPath string
	logLevel   string
	logFormat  string
	shellMode  bool
	execTool   string
	execArgs   []string
	metrics    bool
)

// go build -C cmd/neo4j-mcp -o ../../bin/ -ldflags "-X 'main.Version=9999'"
var Version = "development"

const MixPanelEndpoint = "https://api.mixpanel.com"
const MixPanelToken = "4bfb2414ab973c741b6f067bf06d5575" // #nosec G101 -- MixPanel tokens are safe to be public

// This is very intentionally very small
// It starts up the CLI and does an os.exit(0) for normal exit or os.exit(1) on an error
func main() {
	// Initialize root command
	rootCmd := &cobra.Command{
		Use:   "neo4j-cli",
		Short: "A cli for neo4j",
		Long:  `neo4j-cli is a CLI for use with Neo4j.  It has a shell interface, logging, and is configure using a JSON file, env vars or parameters.`,
		Run:   runCLI,
	}

	// Add flags
	rootCmd.PersistentFlags().StringVar(&configPath, "config-file", "", "Path to configuration file")
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "", "Log level (debug, info, warn, error)")
	rootCmd.PersistentFlags().StringVar(&logFormat, "log-format", "", "Log format (text, json)")
	rootCmd.PersistentFlags().BoolVarP(&metrics, "metrics", "", true, "Send metrics to Neo4j")
	rootCmd.PersistentFlags().BoolVarP(&shellMode, "shell", "s", false, "Start interactive shell")
	rootCmd.PersistentFlags().StringVar(&execTool, "exec", "", "Execute a tool directly")
	rootCmd.Flags().StringSliceVar(&execArgs, "args", []string{}, "Arguments for tool execution")

	// Execute
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}

	os.Exit(0)

}

func runCLI(cmd *cobra.Command, args []string) {

	// Load the configuration in
	configService := config.NewConfigService(cmd, args)
	cfg, _ := configService.LoadConfiguration()

	// NewCLI is a constructor that creates a newCLI instance
	cli, err := cli.NewCLI(&cfg)
	if err != nil {
		// same
		os.Exit(1)
	}

	// We're starting up
	cli.Log.Info("Starting neo4j-cli", logger.Field{Key: "version", Value: Version})

	// Create tool registry
	registry := tools.NewToolRegistry()

	// Execute tool directly if specified
	if execTool != "" {
		executeToolDirect(registry, cli)
		return
	}

	// Start shell by default
	startShell(registry, cli)
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
