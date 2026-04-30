package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

// ---- Overrides ----------------------------------------------------------

// Overrides carries values that were explicitly set by the caller (typically
// from CLI flags). Only non-zero / non-nil fields are applied, so a flag
// default can never silently clobber a value from an environment variable.
//
// Secrets (Neo4j password, Aura client secret) are intentionally omitted:
// they must come from environment variables, never from CLI flags that would
// appear in shell history or `ps` output.
type Overrides struct {
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

// configServiceImpl resolves configuration from env vars and explicit
// overrides. Config file support is intentionally omitted in this version;
// all values come from environment variables (CLI_ prefix) or CLI flags.
type configServiceImpl struct {
	overrides Overrides
	loader    configLoader
}

// NewConfigService returns a Service that resolves configuration with the
// following precedence (highest → lowest):
//
//	explicit Overrides → env vars (CLI_ prefix) → defaults
func NewConfigService(overrides Overrides) Service {
	return &configServiceImpl{
		overrides: overrides,
		loader:    newViperConfigLoader(),
	}
}

// LoadConfiguration resolves the final Config. Env vars always take
// precedence over defaults because Viper's AutomaticEnv is active.
func (c *configServiceImpl) LoadConfiguration() (Config, error) {
	cfg, err := c.loader.unmarshal()
	if err != nil {
		return Config{}, err
	}
	c.applyOverrides(&cfg)
	return cfg, nil
}

// applyOverrides applies only the non-zero fields from Overrides, so a
// CLI flag always wins without a flag default silently overwriting an env var.
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
