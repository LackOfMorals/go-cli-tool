package core

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/spf13/viper"
)

// ConfigLoader defines the interface for loading configuration
type ConfigLoader interface {
	Load(path string) (Config, error)
	Save(path string, config Config) error
	LoadFromEnv() Config
}

// Config represents the application configuration
type Config struct {
	LogLevel  string                `mapstructure:"log_level"`
	LogFormat string                `mapstructure:"log_format"`
	Shell     ShellConfig           `mapstructure:"shell"`
	Telemetry TelemetryConfig       `mapstructure:"telemetry"`
	Tools     map[string]ToolConfig `mapstructure:"tools"`
}

// TelemetryConfig represents telemetry-specific configuration
type TelemetryConfig struct {
	MixpanelToken string `mapstructure:"mixpanel_token"`
}

// ShellConfig represents shell-specific configuration
type ShellConfig struct {
	Prompt      string `mapstructure:"prompt"`
	HistoryFile string `mapstructure:"history_file"`
}

// ToolConfig represents individual tool configuration
type ToolConfig struct {
	Enabled bool                   `mapstructure:"enabled"`
	Params  map[string]interface{} `mapstructure:"params"`
}

// JSONConfigLoader implements ConfigLoader for JSON files
type JSONConfigLoader struct {
	mu sync.Mutex
}

// NewJSONConfigLoader creates a new JSON config loader
func NewJSONConfigLoader() *JSONConfigLoader {
	return &JSONConfigLoader{}
}

// Load reads configuration from a JSON file
func (l *JSONConfigLoader) Load(path string) (Config, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	var config Config

	// Read the file
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("failed to read config file: %w", err)
	}

	// Parse JSON
	if err := json.Unmarshal(data, &config); err != nil {
		return Config{}, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Apply environment variable overrides
	config = l.applyEnvOverrides(config)

	// Set defaults
	l.setDefaults(&config)

	return config, nil
}

// Save writes configuration to a JSON file
func (l *JSONConfigLoader) Save(path string, config Config) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// LoadFromEnv loads configuration values from environment variables
func (l *JSONConfigLoader) LoadFromEnv() Config {
	config := Config{
		LogLevel:  getEnvOrDefault("CLI_LOG_LEVEL", "info"),
		LogFormat: getEnvOrDefault("CLI_LOG_FORMAT", "text"),
		Shell: ShellConfig{
			Prompt:      getEnvOrDefault("CLI_SHELL_PROMPT", "cli> "),
			HistoryFile: getEnvOrDefault("CLI_SHELL_HISTORY", ".cli_history"),
		},
		Tools: make(map[string]ToolConfig),
	}

	return config
}

// applyEnvOverrides applies environment variable overrides to config
func (l *JSONConfigLoader) applyEnvOverrides(config Config) Config {
	// Override log level from environment
	if envLevel := os.Getenv("CLI_LOG_LEVEL"); envLevel != "" {
		config.LogLevel = envLevel
	}

	// Override log format from environment
	if envFormat := os.Getenv("CLI_LOG_FORMAT"); envFormat != "" {
		config.LogFormat = envFormat
	}

	// Override shell prompt from environment
	if envPrompt := os.Getenv("CLI_SHELL_PROMPT"); envPrompt != "" {
		config.Shell.Prompt = envPrompt
	}

	// Override shell history file from environment
	if envHistory := os.Getenv("CLI_SHELL_HISTORY"); envHistory != "" {
		config.Shell.HistoryFile = envHistory
	}

	return config
}

// setDefaults sets default values for configuration
func (l *JSONConfigLoader) setDefaults(config *Config) {
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
	if config.Tools == nil {
		config.Tools = make(map[string]ToolConfig)
	}
}

// ViperConfigLoader implements ConfigLoader using Viper
type ViperConfigLoader struct {
	viper *viper.Viper
}

// NewViperConfigLoader creates a new Viper-based config loader
func NewViperConfigLoader() *ViperConfigLoader {
	v := viper.New()

	// Set defaults
	v.SetDefault("log_level", "info")
	v.SetDefault("log_format", "text")
	v.SetDefault("shell.prompt", "cli> ")
	v.SetDefault("shell.history_file", ".cli_history")

	return &ViperConfigLoader{viper: v}
}

// Load reads configuration using Viper
func (l *ViperConfigLoader) Load(path string) (Config, error) {
	l.viper.SetConfigFile(path)

	if err := l.viper.ReadInConfig(); err != nil {
		return Config{}, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := l.viper.Unmarshal(&config); err != nil {
		return Config{}, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return config, nil
}

// Save writes configuration using Viper
func (l *ViperConfigLoader) Save(path string, config Config) error {
	l.viper.SetConfigFile(path)

	if err := l.viper.Unmarshal(&config); err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := l.viper.WriteConfigAs(path); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// LoadFromEnv loads configuration from environment variables using Viper
func (l *ViperConfigLoader) LoadFromEnv() Config {
	// Viper automatically reads environment variables with CLI_ prefix
	l.viper.SetEnvPrefix("CLI")
	l.viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	l.viper.AutomaticEnv()

	config := Config{
		LogLevel:  l.viper.GetString("log_level"),
		LogFormat: l.viper.GetString("log_format"),
		Shell: ShellConfig{
			Prompt:      l.viper.GetString("shell.prompt"),
			HistoryFile: l.viper.GetString("shell.history_file"),
		},
		Tools: make(map[string]ToolConfig),
	}

	return config
}

// getEnvOrDefault returns environment variable or default value
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// MergeConfigs merges multiple configurations with priority
func MergeConfigs(base Config, override Config) Config {
	if override.LogLevel != "" {
		base.LogLevel = override.LogLevel
	}
	if override.LogFormat != "" {
		base.LogFormat = override.LogFormat
	}
	if override.Shell.Prompt != "" {
		base.Shell.Prompt = override.Shell.Prompt
	}
	if override.Shell.HistoryFile != "" {
		base.Shell.HistoryFile = override.Shell.HistoryFile
	}

	// Merge tools config
	if base.Tools == nil {
		base.Tools = make(map[string]ToolConfig)
	}
	for name, toolConfig := range override.Tools {
		base.Tools[name] = toolConfig
	}

	return base
}
