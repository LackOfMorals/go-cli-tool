package config

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// ---- Config types -------------------------------------------------------

type Config struct {
	LogLevel  string                `mapstructure:"log_level"       json:"log_level"`
	LogFormat string                `mapstructure:"log_format"      json:"log_format"`
	Shell     ShellConfig           `mapstructure:"shell"           json:"shell"`
	Telemetry TelemetryConfig       `mapstructure:"telemetry"       json:"telemetry"`
	Tools     map[string]ToolConfig `mapstructure:"tools"           json:"tools"`
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
//	flags > env vars > config file > defaults
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

// applyFlagOverrides only overrides fields when the user explicitly set the
// flag on the command line. This is why --metrics (default true) won't
// silently clobber a `"metrics": false` in a config file.
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
}

// SaveConfiguration writes the current config back to the file given by
// --config-file. Errors if no path is set.
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

	// Defaults
	v.SetDefault("log_level", "info")
	v.SetDefault("log_format", "text")
	v.SetDefault("shell.enabled", false)
	v.SetDefault("shell.prompt", "cli> ")
	v.SetDefault("shell.history_file", ".cli_history")
	v.SetDefault("telemetry.metrics", true)
	v.SetDefault("telemetry.mixpanel_token", "")

	// Env: CLI_LOG_LEVEL, CLI_SHELL_PROMPT, CLI_TELEMETRY_METRICS, ...
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
	// No file: defaults + env are already wired into the viper instance.
	cfg, err := l.unmarshal()
	if err != nil {
		// With only defaults + env in play this shouldn't fail; fall back empty.
		return Config{Tools: map[string]ToolConfig{}}
	}
	return cfg
}

func (l *viperConfigLoader) save(path string, cfg Config) error {
	l.v.Set("log_level", cfg.LogLevel)
	l.v.Set("log_format", cfg.LogFormat)
	l.v.Set("shell.enabled", cfg.Shell.Enabled)
	l.v.Set("shell.prompt", cfg.Shell.Prompt)
	l.v.Set("shell.history_file", cfg.Shell.HistoryFile)
	l.v.Set("telemetry.metrics", cfg.Telemetry.Metrics)
	l.v.Set("telemetry.mixpanel_token", cfg.Telemetry.MixpanelToken)
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
