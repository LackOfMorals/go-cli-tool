package config_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/cli/go-cli-tool/internal/config"
)

// ---- helpers ------------------------------------------------------------

func loadDefaults(t *testing.T) config.Config {
	t.Helper()
	// Point HOME at a temp dir so LoadConfiguration does not pick up
	// a real ~/.neo4j-cli/config.json that might be present on this machine.
	t.Setenv("HOME", t.TempDir())
	cfg, err := config.NewConfigService(config.Overrides{}).LoadConfiguration()
	if err != nil {
		t.Fatalf("LoadConfiguration: %v", err)
	}
	return cfg
}

func writeTempConfig(t *testing.T, data map[string]any) string {
	t.Helper()
	b, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, b, 0600); err != nil {
		t.Fatalf("write config file: %v", err)
	}
	return path
}

// ---- Defaults -----------------------------------------------------------

func TestDefaults_LogLevel(t *testing.T) {
	cfg := loadDefaults(t)
	if cfg.LogLevel != "info" {
		t.Errorf("LogLevel default: got %q, want %q", cfg.LogLevel, "info")
	}
}

func TestDefaults_LogFormat(t *testing.T) {
	cfg := loadDefaults(t)
	if cfg.LogFormat != "text" {
		t.Errorf("LogFormat default: got %q, want %q", cfg.LogFormat, "text")
	}
}

func TestDefaults_Neo4jURI(t *testing.T) {
	cfg := loadDefaults(t)
	if cfg.Neo4j.URI != "bolt://localhost:7687" {
		t.Errorf("Neo4j.URI default: got %q, want %q", cfg.Neo4j.URI, "bolt://localhost:7687")
	}
}

func TestDefaults_Neo4jUsername(t *testing.T) {
	cfg := loadDefaults(t)
	if cfg.Neo4j.Username != "neo4j" {
		t.Errorf("Neo4j.Username default: got %q, want %q", cfg.Neo4j.Username, "neo4j")
	}
}

func TestDefaults_Neo4jDatabase(t *testing.T) {
	cfg := loadDefaults(t)
	if cfg.Neo4j.Database != "neo4j" {
		t.Errorf("Neo4j.Database default: got %q, want %q", cfg.Neo4j.Database, "neo4j")
	}
}

func TestDefaults_ShellEnabled(t *testing.T) {
	cfg := loadDefaults(t)
	if !cfg.Shell.Enabled {
		t.Error("Shell.Enabled should default to true")
	}
}

func TestDefaults_MetricsEnabled(t *testing.T) {
	cfg := loadDefaults(t)
	if !cfg.Telemetry.Metrics {
		t.Error("Telemetry.Metrics should default to true")
	}
}

func TestDefaults_AuraTimeout(t *testing.T) {
	cfg := loadDefaults(t)
	if cfg.Aura.TimeoutSeconds != 30 {
		t.Errorf("Aura.TimeoutSeconds default: got %d, want 30", cfg.Aura.TimeoutSeconds)
	}
}

func TestDefaults_ToolsMapNotNil(t *testing.T) {
	cfg := loadDefaults(t)
	if cfg.Tools == nil {
		t.Error("Tools map should be initialised, not nil")
	}
}

// ---- Explicit overrides -------------------------------------------------

func TestOverride_LogLevel(t *testing.T) {
	cfg, err := config.NewConfigService(config.Overrides{LogLevel: "debug"}).LoadConfiguration()
	if err != nil {
		t.Fatalf("LoadConfiguration: %v", err)
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("got %q, want %q", cfg.LogLevel, "debug")
	}
}

func TestOverride_LogFormat(t *testing.T) {
	cfg, err := config.NewConfigService(config.Overrides{LogFormat: "json"}).LoadConfiguration()
	if err != nil {
		t.Fatalf("LoadConfiguration: %v", err)
	}
	if cfg.LogFormat != "json" {
		t.Errorf("got %q, want %q", cfg.LogFormat, "json")
	}
}

func TestOverride_Neo4jURI(t *testing.T) {
	cfg, err := config.NewConfigService(config.Overrides{Neo4jURI: "bolt://override:7687"}).LoadConfiguration()
	if err != nil {
		t.Fatalf("LoadConfiguration: %v", err)
	}
	if cfg.Neo4j.URI != "bolt://override:7687" {
		t.Errorf("got %q, want %q", cfg.Neo4j.URI, "bolt://override:7687")
	}
}

func TestOverride_Neo4jUsername(t *testing.T) {
	cfg, err := config.NewConfigService(config.Overrides{Neo4jUsername: "alice"}).LoadConfiguration()
	if err != nil {
		t.Fatalf("LoadConfiguration: %v", err)
	}
	if cfg.Neo4j.Username != "alice" {
		t.Errorf("got %q, want %q", cfg.Neo4j.Username, "alice")
	}
}

func TestOverride_Neo4jDatabase(t *testing.T) {
	cfg, err := config.NewConfigService(config.Overrides{Neo4jDatabase: "mydb"}).LoadConfiguration()
	if err != nil {
		t.Fatalf("LoadConfiguration: %v", err)
	}
	if cfg.Neo4j.Database != "mydb" {
		t.Errorf("got %q, want %q", cfg.Neo4j.Database, "mydb")
	}
}

func TestOverride_MetricsDisabled(t *testing.T) {
	f := false
	cfg, err := config.NewConfigService(config.Overrides{MetricsEnabled: &f}).LoadConfiguration()
	if err != nil {
		t.Fatalf("LoadConfiguration: %v", err)
	}
	if cfg.Telemetry.Metrics {
		t.Error("expected Metrics to be disabled via override")
	}
}

func TestOverride_AuraClientID(t *testing.T) {
	cfg, err := config.NewConfigService(config.Overrides{AuraClientID: "client-123"}).LoadConfiguration()
	if err != nil {
		t.Fatalf("LoadConfiguration: %v", err)
	}
	if cfg.Aura.ClientID != "client-123" {
		t.Errorf("got %q, want %q", cfg.Aura.ClientID, "client-123")
	}
}

func TestOverride_AuraTimeout(t *testing.T) {
	timeout := 60
	cfg, err := config.NewConfigService(config.Overrides{AuraTimeout: &timeout}).LoadConfiguration()
	if err != nil {
		t.Fatalf("LoadConfiguration: %v", err)
	}
	if cfg.Aura.TimeoutSeconds != 60 {
		t.Errorf("got %d, want 60", cfg.Aura.TimeoutSeconds)
	}
}

// ---- Zero-value overrides do not clobber defaults -----------------------

// A zero-value string override must not replace the default value.
func TestOverride_EmptyStringDoesNotClobberDefault(t *testing.T) {
	// Overrides.Neo4jURI is empty (zero value) — the default should win.
	cfg, err := config.NewConfigService(config.Overrides{}).LoadConfiguration()
	if err != nil {
		t.Fatalf("LoadConfiguration: %v", err)
	}
	if cfg.Neo4j.URI == "" {
		t.Error("empty-string override should not clobber the default URI")
	}
}

// ---- Config file loading ------------------------------------------------

func TestConfigFile_OverridesDefault(t *testing.T) {
	path := writeTempConfig(t, map[string]any{
		"log_level":  "debug",
		"log_format": "json",
		"neo4j": map[string]any{
			"uri": "bolt://filehost:7687",
		},
	})

	cfg, err := config.NewConfigService(config.Overrides{ConfigFile: path}).LoadConfiguration()
	if err != nil {
		t.Fatalf("LoadConfiguration: %v", err)
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("log_level from file: got %q, want %q", cfg.LogLevel, "debug")
	}
	if cfg.LogFormat != "json" {
		t.Errorf("log_format from file: got %q, want %q", cfg.LogFormat, "json")
	}
	if cfg.Neo4j.URI != "bolt://filehost:7687" {
		t.Errorf("neo4j.uri from file: got %q, want %q", cfg.Neo4j.URI, "bolt://filehost:7687")
	}
}

func TestConfigFile_NotFound_ReturnsError(t *testing.T) {
	_, err := config.NewConfigService(config.Overrides{
		ConfigFile: "/nonexistent/path/config.json",
	}).LoadConfiguration()
	if err == nil {
		t.Fatal("expected error for missing config file")
	}
}

// ---- Precedence: override > env var > file > default -------------------

func TestPrecedence_OverrideWinsOverFile(t *testing.T) {
	path := writeTempConfig(t, map[string]any{
		"neo4j": map[string]any{"uri": "bolt://filehost:7687"},
	})

	cfg, err := config.NewConfigService(config.Overrides{
		ConfigFile: path,
		Neo4jURI:   "bolt://override:7687",
	}).LoadConfiguration()
	if err != nil {
		t.Fatalf("LoadConfiguration: %v", err)
	}
	if cfg.Neo4j.URI != "bolt://override:7687" {
		t.Errorf("override should win over file: got %q", cfg.Neo4j.URI)
	}
}

func TestPrecedence_EnvVarOverridesFile(t *testing.T) {
	path := writeTempConfig(t, map[string]any{
		"neo4j": map[string]any{"uri": "bolt://filehost:7687"},
	})

	t.Setenv("CLI_NEO4J_URI", "bolt://envhost:7687")

	cfg, err := config.NewConfigService(config.Overrides{ConfigFile: path}).LoadConfiguration()
	if err != nil {
		t.Fatalf("LoadConfiguration: %v", err)
	}
	if cfg.Neo4j.URI != "bolt://envhost:7687" {
		t.Errorf("env var should win over file: got %q", cfg.Neo4j.URI)
	}
}

func TestPrecedence_EnvVarOverridesDefault(t *testing.T) {
	t.Setenv("CLI_NEO4J_DATABASE", "envdb")

	cfg, err := config.NewConfigService(config.Overrides{}).LoadConfiguration()
	if err != nil {
		t.Fatalf("LoadConfiguration: %v", err)
	}
	if cfg.Neo4j.Database != "envdb" {
		t.Errorf("env var should override default: got %q", cfg.Neo4j.Database)
	}
}

// ---- SaveConfiguration --------------------------------------------------

func TestSaveConfiguration_NoPath_ReturnsError(t *testing.T) {
	svc := config.NewConfigService(config.Overrides{})
	err := svc.SaveConfiguration(config.Config{})
	if err == nil {
		t.Fatal("expected error when no config file path is set")
	}
}

func TestSaveConfiguration_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.json")

	svc := config.NewConfigService(config.Overrides{ConfigFile: path})

	original := config.Config{
		LogLevel:  "warn",
		LogFormat: "json",
		Neo4j: config.Neo4jConfig{
			URI:      "bolt://saved:7687",
			Username: "saveduser",
			Database: "saveddb",
		},
		Aura: config.AuraConfig{TimeoutSeconds: 45},
	}

	if err := svc.SaveConfiguration(original); err != nil {
		t.Fatalf("SaveConfiguration: %v", err)
	}

	// Reload and verify the written values are preserved.
	loaded, err := config.NewConfigService(config.Overrides{ConfigFile: path}).LoadConfiguration()
	if err != nil {
		t.Fatalf("reload after save: %v", err)
	}
	if loaded.LogLevel != "warn" {
		t.Errorf("LogLevel: got %q, want warn", loaded.LogLevel)
	}
	if loaded.Neo4j.URI != "bolt://saved:7687" {
		t.Errorf("Neo4j.URI: got %q, want bolt://saved:7687", loaded.Neo4j.URI)
	}
	if loaded.Neo4j.Username != "saveduser" {
		t.Errorf("Neo4j.Username: got %q, want saveduser", loaded.Neo4j.Username)
	}
	if loaded.Aura.TimeoutSeconds != 45 {
		t.Errorf("Aura.TimeoutSeconds: got %d, want 45", loaded.Aura.TimeoutSeconds)
	}
}
