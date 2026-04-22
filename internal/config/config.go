package config

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// ---- Config types -------------------------------------------------------

type Config struct {
	LogLevel  string                `mapstructure:"log_level"  json:"log_level"`
	LogFormat string                `mapstructure:"log_format" json:"log_format"`
	Shell     ShellConfig           `mapstructure:"shell"      json:"shell"`
	Telemetry TelemetryConfig       `mapstructure:"telemetry"  json:"telemetry"`
	Neo4j     Neo4jConfig           `mapstructure:"neo4j"      json:"neo4j"`
	Aura      AuraConfig            `mapstructure:"aura"       json:"aura"`
	Tools     map[string]ToolConfig `mapstructure:"tools"      json:"tools"`
}

// Neo4jConfig holds connection details for a Neo4j database instance.
//
// Env vars (prefix CLI_):
//
//	CLI_NEO4J_URI       bolt://localhost:7687
//	CLI_NEO4J_USERNAME  neo4j
//	CLI_NEO4J_PASSWORD  (set via env or config file — never a CLI flag)
//	CLI_NEO4J_DATABASE  neo4j
type Neo4jConfig struct {
	URI      string `mapstructure:"uri"      json:"uri"`
	Username string `mapstructure:"username" json:"username"`
	Password string `mapstructure:"password" json:"password"`
	Database string `mapstructure:"database" json:"database"`
}

// AuraConfig holds credentials and options for the Neo4j Aura management API.
//
// Env vars (prefix CLI_):
//
//	CLI_AURA_CLIENT_ID       your-client-id
//	CLI_AURA_CLIENT_SECRET   (set via env or config file — never a CLI flag)
//	CLI_AURA_TIMEOUT_SECONDS 30
type AuraConfig struct {
	ClientID       string `mapstructure:"client_id"        json:"client_id"`
	ClientSecret   string `mapstructure:"client_secret"    json:"client_secret"`
	TimeoutSeconds int    `mapstructure:"timeout_seconds"  json:"timeout_seconds"`
}

type TelemetryConfig struct {
	MixpanelToken string `mapstructure:"mixpanel_token" json:"mixpanel_token"`
	Metrics       bool   `mapstructure:"metrics"        json:"metrics"`
}

type ShellConfig struct {
	Enabled     bool   `mapstructure:"enabled"      json:"enabled"`
	Prompt      string `mapstructure:"prompt"       json:"prompt"`
	HistoryFile string `mapstructure:"history_file" json:"history_file"`
}

type ToolConfig struct {
	Enabled bool                   `mapstructure:"enabled" json:"enabled"`
	Params  map[string]interface{} `mapstructure:"params"  json:"params"`
}

// ---- Service ------------------------------------------------------------

type ConfigService struct {
	cmd    *cobra.Command
	args   []string
	loader configLoader
}

// NewConfigService returns a Service backed by a Viper-based loader.
func NewConfigService(cmd *cobra.Command, args []string) Service {
	return &ConfigService{
		cmd:    cmd,
		args:   args,
		loader: newViperConfigLoader(),
	}
}

// LoadConfiguration resolves config with precedence:
//
//	CLI flags > env vars (CLI_ prefix) > config file > defaults
func (c *ConfigService) LoadConfiguration() (Config, error) {
	configPath, err := c.cmd.Flags().GetString("config-file")
	if err != nil {
		return Config{}, fmt.Errorf("get config-file flag: %w", err)
	}

	var cfg Config
	if configPath != "" {
		cfg, err = c.loader.load(configPath)
		if err != nil {
			return Config{}, err
		}
	} else {
		cfg = c.loader.loadFromEnv()
	}

	c.applyFlagOverrides(&cfg)
	return cfg, nil
}

// applyFlagOverrides applies only the flags the user explicitly set on the
// command line, so a flag default never silently clobbers a config file value.
func (c *ConfigService) applyFlagOverrides(cfg *Config) {
	flags := c.cmd.Flags()

	if flags.Changed("log-level") {
		cfg.LogLevel, _ = flags.GetString("log-level")
	}
	if flags.Changed("log-format") {
		cfg.LogFormat, _ = flags.GetString("log-format")
	}
	if flags.Changed("metrics") {
		cfg.Telemetry.Metrics, _ = flags.GetBool("metrics")
	}
	if flags.Changed("shell") {
		cfg.Shell.Enabled, _ = flags.GetBool("shell")
	}

	// Neo4j connection flags.
	if flags.Changed("neo4j-uri") {
		cfg.Neo4j.URI, _ = flags.GetString("neo4j-uri")
	}
	if flags.Changed("neo4j-username") {
		cfg.Neo4j.Username, _ = flags.GetString("neo4j-username")
	}
	if flags.Changed("neo4j-database") {
		cfg.Neo4j.Database, _ = flags.GetString("neo4j-database")
	}

	// Aura API flags.
	if flags.Changed("aura-client-id") {
		cfg.Aura.ClientID, _ = flags.GetString("aura-client-id")
	}
	if flags.Changed("aura-timeout") {
		cfg.Aura.TimeoutSeconds, _ = flags.GetInt("aura-timeout")
	}
}

// SaveConfiguration writes the current config back to the --config-file path.
func (c *ConfigService) SaveConfiguration(cfg Config) error {
	configPath, err := c.cmd.Flags().GetString("config-file")
	if err != nil {
		return fmt.Errorf("get config-file flag: %w", err)
	}
	if configPath == "" {
		return fmt.Errorf("no config file path specified (--config-file)")
	}
	return c.loader.save(configPath, cfg)
}

// ---- Viper-backed loader ------------------------------------------------

type viperConfigLoader struct {
	v *viper.Viper
}

func newViperConfigLoader() *viperConfigLoader {
	v := viper.New()

	// General defaults
	v.SetDefault("log_level", "info")
	v.SetDefault("log_format", "text")
	v.SetDefault("shell.enabled", false)
	v.SetDefault("shell.prompt", "neo4j> ")
	v.SetDefault("shell.history_file", ".neo4j_history")
	v.SetDefault("telemetry.metrics", true)
	v.SetDefault("telemetry.mixpanel_token", "")

	// Neo4j connection defaults
	v.SetDefault("neo4j.uri", "bolt://localhost:7687")
	v.SetDefault("neo4j.username", "neo4j")
	v.SetDefault("neo4j.password", "")
	v.SetDefault("neo4j.database", "neo4j")

	// Aura API defaults
	v.SetDefault("aura.client_id", "")
	v.SetDefault("aura.client_secret", "")
	v.SetDefault("aura.timeout_seconds", 30)

	// Env vars: CLI_ prefix, dots become underscores.
	// Examples: CLI_NEO4J_URI, CLI_NEO4J_PASSWORD,
	//           CLI_AURA_CLIENT_ID, CLI_AURA_CLIENT_SECRET
	v.SetEnvPrefix("CLI")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	return &viperConfigLoader{v: v}
}

func (l *viperConfigLoader) load(path string) (Config, error) {
	l.v.SetConfigFile(path)
	if err := l.v.ReadInConfig(); err != nil {
		return Config{}, fmt.Errorf("read config file %q: %w", path, err)
	}
	return l.unmarshal()
}

func (l *viperConfigLoader) loadFromEnv() Config {
	cfg, err := l.unmarshal()
	if err != nil {
		return Config{Tools: map[string]ToolConfig{}}
	}
	return cfg
}

func (l *viperConfigLoader) save(path string, cfg Config) error {
	// General
	l.v.Set("log_level", cfg.LogLevel)
	l.v.Set("log_format", cfg.LogFormat)
	l.v.Set("shell.enabled", cfg.Shell.Enabled)
	l.v.Set("shell.prompt", cfg.Shell.Prompt)
	l.v.Set("shell.history_file", cfg.Shell.HistoryFile)
	l.v.Set("telemetry.metrics", cfg.Telemetry.Metrics)
	l.v.Set("telemetry.mixpanel_token", cfg.Telemetry.MixpanelToken)

	// Neo4j — password intentionally included; the user controls this file.
	l.v.Set("neo4j.uri", cfg.Neo4j.URI)
	l.v.Set("neo4j.username", cfg.Neo4j.Username)
	l.v.Set("neo4j.password", cfg.Neo4j.Password)
	l.v.Set("neo4j.database", cfg.Neo4j.Database)

	// Aura — client_secret intentionally included; the user controls this file.
	l.v.Set("aura.client_id", cfg.Aura.ClientID)
	l.v.Set("aura.client_secret", cfg.Aura.ClientSecret)
	l.v.Set("aura.timeout_seconds", cfg.Aura.TimeoutSeconds)

	l.v.Set("tools", cfg.Tools)

	l.v.SetConfigFile(path)
	if err := l.v.WriteConfigAs(path); err != nil {
		return fmt.Errorf("write config file %q: %w", path, err)
	}
	return nil
}

func (l *viperConfigLoader) unmarshal() (Config, error) {
	var cfg Config
	if err := l.v.Unmarshal(&cfg); err != nil {
		return Config{}, fmt.Errorf("unmarshal config: %w", err)
	}
	if cfg.Tools == nil {
		cfg.Tools = make(map[string]ToolConfig)
	}
	return cfg, nil
}
