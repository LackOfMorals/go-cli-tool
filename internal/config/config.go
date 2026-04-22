package config

import (
	"fmt"
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
	Shell     ShellConfig           `mapstructure:"shell"      json:"shell"`
	Telemetry TelemetryConfig       `mapstructure:"telemetry"  json:"telemetry"`
	Neo4j     Neo4jConfig           `mapstructure:"neo4j"      json:"neo4j"`
	Aura      AuraConfig            `mapstructure:"aura"       json:"aura"`
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
type AuraConfig struct {
	ClientID       string `mapstructure:"client_id"       json:"client_id"`
	ClientSecret   string `mapstructure:"client_secret"   json:"client_secret"`
	TimeoutSeconds int    `mapstructure:"timeout_seconds" json:"timeout_seconds"`
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

// ConfigService loads and merges configuration from defaults, a file, and
// explicit overrides. It no longer depends on Cobra — flag values are
// extracted by the caller and passed in as an Overrides struct.
type ConfigService struct {
	overrides Overrides
	loader    configLoader
}

// NewConfigService returns a Service that resolves configuration with the
// following precedence (highest → lowest):
//
//	explicit Overrides → env vars (CLI_ prefix) → config file → defaults
func NewConfigService(overrides Overrides) Service {
	return &ConfigService{
		overrides: overrides,
		loader:    newViperConfigLoader(),
	}
}

// LoadConfiguration resolves the final Config. Env vars always take
// precedence over file values because Viper's internal priority chain puts
// env vars above config-file keys regardless of load order.
func (c *ConfigService) LoadConfiguration() (Config, error) {
	// Load the file first (if specified). Env vars will still win on unmarshal
	// because AutomaticEnv is set on the underlying Viper instance.
	if c.overrides.ConfigFile != "" {
		if err := c.loader.readFile(c.overrides.ConfigFile); err != nil {
			return Config{}, fmt.Errorf("read config file %q: %w", c.overrides.ConfigFile, err)
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
func (c *ConfigService) applyOverrides(cfg *Config) {
	o := c.overrides
	if o.LogLevel != "" {
		cfg.LogLevel = o.LogLevel
	}
	if o.LogFormat != "" {
		cfg.LogFormat = o.LogFormat
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
func (c *ConfigService) SaveConfiguration(cfg Config) error {
	if c.overrides.ConfigFile == "" {
		return fmt.Errorf("no config file path specified (use --config-file)")
	}
	return c.loader.save(c.overrides.ConfigFile, cfg)
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
	l.v.Set("tools", cfg.Tools)

	l.v.SetConfigFile(path)
	if err := l.v.WriteConfigAs(path); err != nil {
		return fmt.Errorf("write config file %q: %w", path, err)
	}
	return nil
}
