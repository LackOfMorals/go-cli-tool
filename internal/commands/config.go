package commands

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/cli/go-cli-tool/internal/config"
	"github.com/cli/go-cli-tool/internal/dispatch"
	"github.com/cli/go-cli-tool/internal/presentation"
)

// ---- Config item registry -----------------------------------------------

// configItemDef describes one user-facing configuration key.
type configItemDef struct {
	Key         string
	Section     string
	Description string
	Secret      bool
	Default     string
	Get         func(cfg config.Config) string
	Set         func(cfg *config.Config, val string) error // nil means read-only
}

var configRegistry = []configItemDef{

	// ---- Logging ---------------------------------------------------------

	{
		Key: "log.level", Section: "Logging", Default: "info",
		Description: "Log verbosity: debug, info, warn, error",
		Get:         func(c config.Config) string { return c.LogLevel },
		Set: func(c *config.Config, v string) error {
			switch v {
			case "debug", "info", "warn", "error":
				c.LogLevel = v
				return nil
			}
			return fmt.Errorf("must be one of: debug, info, warn, error")
		},
	},
	{
		Key: "log.format", Section: "Logging", Default: "text",
		Description: "Log format: text, json",
		Get:         func(c config.Config) string { return c.LogFormat },
		Set: func(c *config.Config, v string) error {
			if v != "text" && v != "json" {
				return fmt.Errorf("must be one of: text, json")
			}
			c.LogFormat = v
			return nil
		},
	},
	{
		Key: "log.output", Section: "Logging", Default: "stderr",
		Description: "Log destination: stderr, stdout, file",
		Get:         func(c config.Config) string { return c.LogOutput },
		Set: func(c *config.Config, v string) error {
			if v != "stderr" && v != "stdout" && v != "file" {
				return fmt.Errorf("must be one of: stderr, stdout, file")
			}
			c.LogOutput = v
			return nil
		},
	},
	{
		Key: "log.file", Section: "Logging", Default: "",
		Description: "Log file path (used when log.output=file)",
		Get:         func(c config.Config) string { return c.LogFile },
		Set:         func(c *config.Config, v string) error { c.LogFile = v; return nil },
	},

	// ---- Shell -----------------------------------------------------------

	{
		Key: "shell.prompt", Section: "Shell", Default: "neo4j> ",
		Description: "Interactive shell prompt string",
		Get:         func(c config.Config) string { return c.Shell.Prompt },
		Set: func(c *config.Config, v string) error {
			if v == "" {
				return fmt.Errorf("prompt cannot be empty")
			}
			c.Shell.Prompt = v
			return nil
		},
	},
	{
		Key: "shell.history_file", Section: "Shell", Default: ".neo4j_history",
		Description: "Path to the shell command history file",
		Get:         func(c config.Config) string { return c.Shell.HistoryFile },
		Set:         func(c *config.Config, v string) error { c.Shell.HistoryFile = v; return nil },
	},

	// ---- Telemetry -------------------------------------------------------

	{
		Key: "telemetry.metrics", Section: "Telemetry", Default: "true",
		Description: "Send anonymous usage metrics to Neo4j: true, false",
		Get: func(c config.Config) string {
			if c.Telemetry.Metrics {
				return "true"
			}
			return "false"
		},
		Set: func(c *config.Config, v string) error {
			b, err := strconv.ParseBool(v)
			if err != nil {
				return fmt.Errorf("must be true or false")
			}
			c.Telemetry.Metrics = b
			return nil
		},
	},

	// ---- Neo4j -----------------------------------------------------------

	{
		Key: "neo4j.uri", Section: "Neo4j", Default: "bolt://localhost:7687",
		Description: "Bolt connection URI",
		Get:         func(c config.Config) string { return c.Neo4j.URI },
		Set:         func(c *config.Config, v string) error { c.Neo4j.URI = v; return nil },
	},
	{
		Key: "neo4j.username", Section: "Neo4j", Default: "neo4j",
		Description: "Neo4j username",
		Get:         func(c config.Config) string { return c.Neo4j.Username },
		Set:         func(c *config.Config, v string) error { c.Neo4j.Username = v; return nil },
	},
	{
		Key: "neo4j.password", Section: "Neo4j", Default: "", Secret: true,
		Description: "Neo4j password",
		Get:         func(c config.Config) string { return c.Neo4j.Password },
		Set:         func(c *config.Config, v string) error { c.Neo4j.Password = v; return nil },
	},
	{
		Key: "neo4j.database", Section: "Neo4j", Default: "neo4j",
		Description: "Default Neo4j database name",
		Get:         func(c config.Config) string { return c.Neo4j.Database },
		Set:         func(c *config.Config, v string) error { c.Neo4j.Database = v; return nil },
	},

	// ---- Aura ------------------------------------------------------------

	{
		Key: "aura.client_id", Section: "Aura", Default: "",
		Description: "Aura API client ID",
		Get:         func(c config.Config) string { return c.Aura.ClientID },
		Set:         func(c *config.Config, v string) error { c.Aura.ClientID = v; return nil },
	},
	{
		Key: "aura.client_secret", Section: "Aura", Default: "", Secret: true,
		Description: "Aura API client secret",
		Get:         func(c config.Config) string { return c.Aura.ClientSecret },
		Set:         func(c *config.Config, v string) error { c.Aura.ClientSecret = v; return nil },
	},
	{
		Key: "aura.timeout_seconds", Section: "Aura", Default: "30",
		Description: "Aura API request timeout in seconds",
		Get:         func(c config.Config) string { return strconv.Itoa(c.Aura.TimeoutSeconds) },
		Set: func(c *config.Config, v string) error {
			n, err := strconv.Atoi(v)
			if err != nil || n < 1 {
				return fmt.Errorf("must be a positive integer")
			}
			c.Aura.TimeoutSeconds = n
			return nil
		},
	},

	// ---- Aura instance defaults ------------------------------------------

	{
		Key: "aura.instance_defaults.tenant_id", Section: "Aura Instance Defaults", Default: "",
		Description: "Default tenant ID for new Aura instances",
		Get:         func(c config.Config) string { return c.Aura.InstanceDefaults.TenantID },
		Set:         func(c *config.Config, v string) error { c.Aura.InstanceDefaults.TenantID = v; return nil },
	},
	{
		Key: "aura.instance_defaults.cloud_provider", Section: "Aura Instance Defaults", Default: "gcp",
		Description: "Default cloud provider for new instances: aws, gcp, azure",
		Get:         func(c config.Config) string { return c.Aura.InstanceDefaults.CloudProvider },
		Set: func(c *config.Config, v string) error {
			if v != "aws" && v != "gcp" && v != "azure" {
				return fmt.Errorf("must be one of: aws, gcp, azure")
			}
			c.Aura.InstanceDefaults.CloudProvider = v
			return nil
		},
	},
	{
		Key: "aura.instance_defaults.region", Section: "Aura Instance Defaults", Default: "europe-west1",
		Description: "Default region for new Aura instances",
		Get:         func(c config.Config) string { return c.Aura.InstanceDefaults.Region },
		Set:         func(c *config.Config, v string) error { c.Aura.InstanceDefaults.Region = v; return nil },
	},
	{
		Key: "aura.instance_defaults.type", Section: "Aura Instance Defaults", Default: "enterprise-db",
		Description: "Default instance type, e.g. enterprise-db",
		Get:         func(c config.Config) string { return c.Aura.InstanceDefaults.Type },
		Set:         func(c *config.Config, v string) error { c.Aura.InstanceDefaults.Type = v; return nil },
	},
	{
		Key: "aura.instance_defaults.version", Section: "Aura Instance Defaults", Default: "5",
		Description: "Default Neo4j version for new instances",
		Get:         func(c config.Config) string { return c.Aura.InstanceDefaults.Version },
		Set:         func(c *config.Config, v string) error { c.Aura.InstanceDefaults.Version = v; return nil },
	},
	{
		Key: "aura.instance_defaults.memory", Section: "Aura Instance Defaults", Default: "8GB",
		Description: "Default instance memory, e.g. 8GB",
		Get:         func(c config.Config) string { return c.Aura.InstanceDefaults.Memory },
		Set:         func(c *config.Config, v string) error { c.Aura.InstanceDefaults.Memory = v; return nil },
	},

	// ---- Cypher ----------------------------------------------------------

	{
		Key: "cypher.shell_limit", Section: "Cypher", Default: "25",
		Description: "Default LIMIT injected into queries in interactive shell mode",
		Get:         func(c config.Config) string { return strconv.Itoa(c.Cypher.ShellLimit) },
		Set: func(c *config.Config, v string) error {
			n, err := strconv.Atoi(v)
			if err != nil || n < 1 {
				return fmt.Errorf("must be a positive integer")
			}
			c.Cypher.ShellLimit = n
			return nil
		},
	},
	{
		Key: "cypher.exec_limit", Section: "Cypher", Default: "100",
		Description: "Default LIMIT injected into queries in non-interactive (--exec) mode",
		Get:         func(c config.Config) string { return strconv.Itoa(c.Cypher.ExecLimit) },
		Set: func(c *config.Config, v string) error {
			n, err := strconv.Atoi(v)
			if err != nil || n < 1 {
				return fmt.Errorf("must be a positive integer")
			}
			c.Cypher.ExecLimit = n
			return nil
		},
	},
	{
		Key: "cypher.output_format", Section: "Cypher", Default: "table",
		Description: "Default output format: table, json, pretty-json, graph",
		Get:         func(c config.Config) string { return c.Cypher.OutputFormat },
		Set: func(c *config.Config, v string) error {
			switch v {
			case "table", "json", "pretty-json", "graph":
				c.Cypher.OutputFormat = v
				return nil
			}
			return fmt.Errorf("must be one of: table, json, pretty-json, graph")
		},
	},
}

// ---- Lookup helpers -----------------------------------------------------

func findConfigItem(key string) (*configItemDef, bool) {
	for i := range configRegistry {
		if configRegistry[i].Key == key {
			return &configRegistry[i], true
		}
	}
	return nil, false
}

func configKeyHint() string {
	var b strings.Builder
	for _, item := range configRegistry {
		fmt.Fprintf(&b, "  %s\n", item.Key)
	}
	return strings.TrimRight(b.String(), "\n")
}

func displayValue(item *configItemDef, cfg config.Config) string {
	v := item.Get(cfg)
	if item.Secret {
		if v == "" {
			return "(not set)"
		}
		return "(set)"
	}
	if v == "" {
		return "(not set)"
	}
	return v
}

func saveConfig(cfg *config.Config, cfgPath string) error {
	path := cfgPath
	if path == "" {
		path = config.DefaultConfigFilePath()
	}
	svc := config.NewConfigService(config.Overrides{ConfigFile: path})
	return svc.SaveConfiguration(*cfg)
}

// ---- Category builder ---------------------------------------------------

// BuildConfigCategory returns the config top-level category.
func BuildConfigCategory(cfg *config.Config, cfgPath string) *dispatch.Category {
	return dispatch.NewCategory("config", "Manage CLI configuration").
		AddCommand(configListCmd(cfg)).
		AddCommand(configSetCmd(cfg, cfgPath)).
		AddCommand(configDeleteCmd(cfg, cfgPath)).
		AddCommand(configResetCmd(cfg, cfgPath))
}

// ---- config list --------------------------------------------------------

func configListCmd(cfg *config.Config) *dispatch.Command {
	return &dispatch.Command{
		Name:        "list",
		Aliases:     []string{"ls"},
		Usage:       "list",
		Description: "Show all configuration keys, their current values, and descriptions",
		Handler: func(args []string, ctx dispatch.Context) (string, error) {
			cols := []string{"Key", "Value", "Description"}
			rows := make([][]interface{}, 0, len(configRegistry))
			for i := range configRegistry {
				item := &configRegistry[i]
				rows = append(rows, []interface{}{
					item.Key,
					displayValue(item, *cfg),
					item.Description,
				})
			}
			return ctx.Presenter.Format(presentation.NewTableData(cols, rows))
		},
	}
}

// ---- config set ---------------------------------------------------------

func configSetCmd(cfg *config.Config, cfgPath string) *dispatch.Command {
	return &dispatch.Command{
		Name:        "set",
		Usage:       "set <key> <value>",
		Description: "Set a configuration value and persist it to the config file",
		Handler: func(args []string, ctx dispatch.Context) (string, error) {
			if len(args) < 2 {
				return "", fmt.Errorf(
					"usage: config set <key> <value>\n\nAvailable keys (run 'config list' for descriptions):\n%s",
					configKeyHint(),
				)
			}

			key := args[0]
			val := strings.Join(args[1:], " ")

			item, ok := findConfigItem(key)
			if !ok {
				return "", fmt.Errorf(
					"unknown key %q\n\nAvailable keys (run 'config list' for descriptions):\n%s",
					key, configKeyHint(),
				)
			}
			if item.Set == nil {
				return "", fmt.Errorf("key %q is read-only", key)
			}
			if err := item.Set(cfg, val); err != nil {
				return "", fmt.Errorf("%s: %w", key, err)
			}
			if err := saveConfig(cfg, cfgPath); err != nil {
				return "", fmt.Errorf("save config: %w", err)
			}

			display := val
			if item.Secret {
				display = "(set)"
			}
			return fmt.Sprintf("✓ %s = %s.", key, display), nil
		},
	}
}

// ---- config delete ------------------------------------------------------

func configDeleteCmd(cfg *config.Config, cfgPath string) *dispatch.Command {
	return &dispatch.Command{
		Name:        "delete",
		Aliases:     []string{"del", "rm"},
		Usage:       "delete <key>",
		Description: "Reset a configuration key to its default value (requires confirmation)",
		Handler: func(args []string, ctx dispatch.Context) (string, error) {
			if len(args) == 0 {
				return "", fmt.Errorf(
					"usage: config delete <key>\n\nAvailable keys (run 'config list' for descriptions):\n%s",
					configKeyHint(),
				)
			}

			key := args[0]
			item, ok := findConfigItem(key)
			if !ok {
				return "", fmt.Errorf(
					"unknown key %q\n\nAvailable keys (run 'config list' for descriptions):\n%s",
					key, configKeyHint(),
				)
			}
			if item.Set == nil {
				return "", fmt.Errorf("key %q is read-only", key)
			}

			current := displayValue(item, *cfg)
			defaultDisplay := item.Default
			if item.Default == "" {
				defaultDisplay = "(not set)"
			}

			ctx.IO.Write(
				"Reset %q from %s to default %q? Type 'yes' to confirm: ",
				key, current, defaultDisplay,
			)
			confirm, err := ctx.IO.Read()
			if err != nil {
				return "", fmt.Errorf("read confirmation: %w", err)
			}
			if strings.TrimSpace(confirm) != "yes" {
				return "Reset cancelled.", nil
			}

			if err := item.Set(cfg, item.Default); err != nil {
				return "", fmt.Errorf("reset %s: %w", key, err)
			}
			if err := saveConfig(cfg, cfgPath); err != nil {
				return "", fmt.Errorf("save config: %w", err)
			}

			return fmt.Sprintf("✓ %s reset to default (%s).", key, defaultDisplay), nil
		},
	}
}

// ---- config reset -------------------------------------------------------

func configResetCmd(cfg *config.Config, cfgPath string) *dispatch.Command {
	return &dispatch.Command{
		Name:        "reset",
		Usage:       "reset",
		Description: "Delete the config file and restore all defaults (requires confirmation)",
		Handler: func(args []string, ctx dispatch.Context) (string, error) {
			path := cfgPath
			if path == "" {
				path = config.DefaultConfigFilePath()
			}

			ctx.IO.Write(
				"This will delete %s and restore all defaults.\nType 'yes' to confirm: ",
				path,
			)
			confirm, err := ctx.IO.Read()
			if err != nil {
				return "", fmt.Errorf("read confirmation: %w", err)
			}
			if strings.TrimSpace(confirm) != "yes" {
				return "Reset cancelled.", nil
			}

			if removeErr := os.Remove(path); removeErr != nil && !os.IsNotExist(removeErr) {
				return "", fmt.Errorf("delete config file: %w", removeErr)
			}

			newCfg, loadErr := config.NewConfigService(config.Overrides{}).LoadConfiguration()
			if loadErr != nil {
				return "", fmt.Errorf("reload defaults: %w", loadErr)
			}
			*cfg = newCfg

			return fmt.Sprintf("✓ Configuration reset to defaults.\n  Config file deleted: %s", path), nil
		},
	}
}
