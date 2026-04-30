package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

// ---- Overrides ----------------------------------------------------------

// Overrides carries values that were explicitly set by the caller (typically
// from CLI flags). Only non-zero / non-nil fields are applied, so a flag
// default can never silently clobber a value from the config file.
//
// Secrets (Neo4j password, Aura client secret) are intentionally omitted:
// they must come from the config file or environment variables, never from
// CLI flags that would appear in shell history or `ps` output.
type Overrides struct {
	ConfigFile  string // non-empty → load and overlay this file
	LogLevel    string // non-empty → override
	LogFormat   string // non-empty → override
	LogOutput   string // non-empty → override (stderr | stdout | file)
	LogFile     string // non-empty → override log file path
	ShellEnabled   *bool // non-nil → override
	MetricsEnabled *bool // non-nil → override

	// Neo4j connection (non-secret fields only)
	Neo4jURI      string
	Neo4jUsername string
	Neo4jDatabase string

	// Aura API (non-secret fields only)
	AuraClientID string
	AuraTimeout  *int // non-nil → override timeout_seconds
}

// ---- Config types -------------------------------------------------------

type Config struct {
	LogLevel  string                `mapstructure:"log_level"  json:"log_level"`
	LogFormat string                `mapstructure:"log_format" json:"log_format"`
	LogOutput string                `mapstructure:"log_output" json:"log_output"`
	LogFile   string                `mapstructure:"log_file"   json:"log_file"`
	Shell     ShellConfig           `mapstructure:"shell"      json:"shell"`
	Telemetry TelemetryConfig       `mapstructure:"telemetry"  json:"telemetry"`
	Neo4j     Neo4jConfig           `mapstructure:"neo4j"      json:"neo4j"`
	Aura      AuraConfig            `mapstructure:"aura"       json:"aura"`
	Cypher    CypherConfig          `mapstructure:"cypher"     json:"cypher"`
	Tools     map[string]ToolConfig `mapstructure:"tools"      json:"tools"`
}

// Neo4jConfig holds connection details for a Neo4j database instance.
//
// Env vars (CLI_ prefix):
//
//	CLI_NEO4J_URI, CLI_NEO4J_USERNAME, CLI_NEO4J_PASSWORD, CLI_NEO4J_DATABASE
type Neo4jConfig struct {
	URI      string `mapstructure:"uri"      json:"uri"`
	Username string `mapstructure:"username" json:"username"`
	Password string `mapstructure:"password" json:"password"`
	Database string `mapstructure:"database" json:"database"`
}

// AuraConfig holds credentials and options for the Neo4j Aura management API.
//
// Env vars (CLI_ prefix):
//
//	CLI_AURA_CLIENT_ID, CLI_AURA_CLIENT_SECRET, CLI_AURA_TIMEOUT_SECONDS
//	CLI_AURA_INSTANCE_DEFAULTS_TENANT_ID, CLI_AURA_INSTANCE_DEFAULTS_CLOUD_PROVIDER, ...
type AuraConfig struct {
	ClientID         string               `mapstructure:"client_id"         json:"client_id"`
	ClientSecret     string               `mapstructure:"client_secret"     json:"client_secret"`
	TimeoutSeconds   int                  `mapstructure:"timeout_seconds"   json:"timeout_seconds"`
	InstanceDefaults AuraInstanceDefaults `mapstructure:"instance_defaults" json:"instance_defaults"`
}

// AuraInstanceDefaults holds defaults for new instance creation.
// These are merged with per-command arguments, with explicit args taking precedence.
//
// Env vars (CLI_ prefix, e.g. CLI_AURA_INSTANCE_DEFAULTS_TENANT_ID):
type AuraInstanceDefaults struct {
	TenantID      string `mapstructure:"tenant_id"       json:"tenant_id"`
	CloudProvider string `mapstructure:"cloud_provider"  json:"cloud_provider"`
	Region        string `mapstructure:"region"          json:"region"`
	Type          string `mapstructure:"type"            json:"type"`
	Version       string `mapstructure:"version"         json:"version"`
	Memory        string `mapstructure:"memory"          json:"memory"`
}

// DefaultConfigFilePath returns the default path for the CLI configuration
// file: ~/.neo4j-cli/config.json. Falls back to a local file on error.
func DefaultConfigFilePath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".neo4j-cli-config.json"
	}
	return filepath.Join(home, ".neo4j-cli", "config.json")
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

// CypherConfig controls query execution and output behaviour.
//
// Env vars (CLI_ prefix):
//
//	CLI_CYPHER_SHELL_LIMIT, CLI_CYPHER_EXEC_LIMIT, CLI_CYPHER_OUTPUT_FORMAT
type CypherConfig struct {
	// ShellLimit is the default LIMIT injected in shell (interactive) mode.
	ShellLimit int `mapstructure:"shell_limit" json:"shell_limit"`
	// ExecLimit is the default LIMIT injected in non-interactive (--exec) mode.
	ExecLimit int `mapstructure:"exec_limit" json:"exec_limit"`
	// OutputFormat controls result rendering: "table" (default) or "graph".
	OutputFormat string `mapstructure:"output_format" json:"output_format"`
}

type ToolConfig struct {
	Enabled bool                   `mapstructure:"enabled" json:"enabled"`
	Params  map[string]interface{} `mapstructure:"params"  json:"params"`
}

// ---- Service ------------------------------------------------------------

// configServiceImpl loads and merges configuration from defaults, a file, and
// explicit overrides. It no longer depends on Cobra — flag values are
// extracted by the caller and passed in as an Overrides struct.
type configServiceImpl struct {
	overrides Overrides
	loader    configLoader
}

// NewConfigService returns a Service that resolves configuration with the
// following precedence (highest → lowest):
//
//	explicit Overrides → env vars (CLI_ prefix) → config file → defaults
func NewConfigService(overrides Overrides) Service {
	return &configServiceImpl{
		overrides: overrides,
		loader:    newViperConfigLoader(),
	}
}

// LoadConfiguration resolves the final Config. Env vars always take
// precedence over file values because Viper's internal priority chain puts
// env vars above config-file keys regardless of load order.
//
// File loading precedence:
//  1. Explicit --config-file path (hard error if file is missing)
//  2. DefaultConfigFilePath() — ~/.neo4j-cli/config.json (silently skipped if absent)
func (c *configServiceImpl) LoadConfiguration() (Config, error) {
	if c.overrides.ConfigFile != "" {
		// Explicit path: treat a missing file as an error.
		if err := c.loader.readFile(c.overrides.ConfigFile); err != nil {
			return Config{}, fmt.Errorf("read config file %q: %w", c.overrides.ConfigFile, err)
		}
	} else {
		// Auto-load the default path; silently ignore "file not found" so the
		// first run (before any credentials have been saved) works without error.
		defPath := DefaultConfigFilePath()
		if err := c.loader.readFile(defPath); err != nil && !os.IsNotExist(err) && !errors.Is(err, os.ErrNotExist) {
			// File exists but is unreadable / malformed — surface the error.
			return Config{}, fmt.Errorf("read config file %q: %w", defPath, err)
		}
	}

	cfg, err := c.loader.unmarshal()
	if err != nil {
		return Config{}, err
	}

	c.applyOverrides(&cfg)
	return cfg, nil
}

// applyOverrides applies only the non-zero fields from Overrides, so an
// explicit CLI flag always wins without a flag default silently overwriting
// a file value.
func (c *configServiceImpl) applyOverrides(cfg *Config) {
	o := c.overrides
	if o.LogLevel != "" {
		cfg.LogLevel = o.LogLevel
	}
	if o.LogFormat != "" {
		cfg.LogFormat = o.LogFormat
	}
	if o.LogOutput != "" {
		cfg.LogOutput = o.LogOutput
	}
	if o.LogFile != "" {
		cfg.LogFile = o.LogFile
	}
	if o.MetricsEnabled != nil {
		cfg.Telemetry.Metrics = *o.MetricsEnabled
	}
	if o.ShellEnabled != nil {
		cfg.Shell.Enabled = *o.ShellEnabled
	}
	if o.Neo4jURI != "" {
		cfg.Neo4j.URI = o.Neo4jURI
	}
	if o.Neo4jUsername != "" {
		cfg.Neo4j.Username = o.Neo4jUsername
	}
	if o.Neo4jDatabase != "" {
		cfg.Neo4j.Database = o.Neo4jDatabase
	}
	if o.AuraClientID != "" {
		cfg.Aura.ClientID = o.AuraClientID
	}
	if o.AuraTimeout != nil {
		cfg.Aura.TimeoutSeconds = *o.AuraTimeout
	}
}

// SaveConfiguration writes cfg back to the file specified in Overrides.
// The caller is responsible for supplying a path; use DefaultConfigFilePath()
// when none is available (e.g. in InteractiveAuraPrerequisite).
func (c *configServiceImpl) SaveConfiguration(cfg Config) error {
	if c.overrides.ConfigFile == "" {
		return fmt.Errorf("no config file path specified (use --config-file)")
	}
	// Ensure the parent directory exists so the write never fails on a
	// freshly provisioned machine.
	path := c.overrides.ConfigFile
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}
	return c.loader.save(path, cfg)
}

// ---- Viper-backed loader ------------------------------------------------

type viperConfigLoader struct {
	v *viper.Viper
}

func newViperConfigLoader() *viperConfigLoader {
	v := viper.New()

	// Defaults
	v.SetDefault("log_level", "info")
	v.SetDefault("log_format", "text")
	v.SetDefault("log_output", "stderr")
	v.SetDefault("log_file", "")
	v.SetDefault("shell.enabled", false)
	v.SetDefault("shell.prompt", "neo4j> ")
	v.SetDefault("shell.history_file", ".neo4j_history")
	v.SetDefault("telemetry.metrics", true)
	v.SetDefault("telemetry.mixpanel_token", "")

	v.SetDefault("neo4j.uri", "bolt://localhost:7687")
	v.SetDefault("neo4j.username", "neo4j")
	v.SetDefault("neo4j.password", "")
	v.SetDefault("neo4j.database", "neo4j")

	v.SetDefault("aura.client_id", "")
	v.SetDefault("aura.client_secret", "")
	v.SetDefault("aura.timeout_seconds", 30)
	v.SetDefault("aura.instance_defaults.tenant_id", "")
	v.SetDefault("aura.instance_defaults.cloud_provider", "gcp")
	v.SetDefault("aura.instance_defaults.region", "europe-west1")
	v.SetDefault("aura.instance_defaults.type", "enterprise-db")
	v.SetDefault("aura.instance_defaults.version", "5")
	v.SetDefault("aura.instance_defaults.memory", "8GB")

	v.SetDefault("cypher.shell_limit", 25)
	v.SetDefault("cypher.exec_limit", 100)
	v.SetDefault("cypher.output_format", "table")

	// Env vars: CLI_NEO4J_URI, CLI_AURA_CLIENT_SECRET, etc.
	// The replacer maps "." → "_" so "neo4j.uri" → CLI_NEO4J_URI.
	v.SetEnvPrefix("CLI")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	return &viperConfigLoader{v: v}
}

// readFile loads a config file into the Viper instance. Because AutomaticEnv
// is already active, env vars will still win over file values on the next
// unmarshal call.
func (l *viperConfigLoader) readFile(path string) error {
	l.v.SetConfigFile(path)
	return l.v.ReadInConfig()
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

func (l *viperConfigLoader) save(path string, cfg Config) error {
	l.v.Set("log_level", cfg.LogLevel)
	l.v.Set("log_format", cfg.LogFormat)
	l.v.Set("log_output", cfg.LogOutput)
	l.v.Set("log_file", cfg.LogFile)
	l.v.Set("shell.enabled", cfg.Shell.Enabled)
	l.v.Set("shell.prompt", cfg.Shell.Prompt)
	l.v.Set("shell.history_file", cfg.Shell.HistoryFile)
	l.v.Set("telemetry.metrics", cfg.Telemetry.Metrics)
	l.v.Set("telemetry.mixpanel_token", cfg.Telemetry.MixpanelToken)
	l.v.Set("neo4j.uri", cfg.Neo4j.URI)
	l.v.Set("neo4j.username", cfg.Neo4j.Username)
	l.v.Set("neo4j.password", cfg.Neo4j.Password)
	l.v.Set("neo4j.database", cfg.Neo4j.Database)
	l.v.Set("aura.client_id", cfg.Aura.ClientID)
	l.v.Set("aura.client_secret", cfg.Aura.ClientSecret)
	l.v.Set("aura.timeout_seconds", cfg.Aura.TimeoutSeconds)
	l.v.Set("aura.instance_defaults.tenant_id", cfg.Aura.InstanceDefaults.TenantID)
	l.v.Set("aura.instance_defaults.cloud_provider", cfg.Aura.InstanceDefaults.CloudProvider)
	l.v.Set("aura.instance_defaults.region", cfg.Aura.InstanceDefaults.Region)
	l.v.Set("aura.instance_defaults.type", cfg.Aura.InstanceDefaults.Type)
	l.v.Set("aura.instance_defaults.version", cfg.Aura.InstanceDefaults.Version)
	l.v.Set("aura.instance_defaults.memory", cfg.Aura.InstanceDefaults.Memory)
	l.v.Set("cypher.shell_limit", cfg.Cypher.ShellLimit)
	l.v.Set("cypher.exec_limit", cfg.Cypher.ExecLimit)
	l.v.Set("cypher.output_format", cfg.Cypher.OutputFormat)
	l.v.Set("tools", cfg.Tools)

	l.v.SetConfigFile(path)
	if err := l.v.WriteConfigAs(path); err != nil {
		return fmt.Errorf("write config file %q: %w", path, err)
	}
	return nil
}
