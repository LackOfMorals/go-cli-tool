package config_test

import (
	"testing"

	"github.com/cli/go-cli-tool/internal/config"
)

// ---- helpers ------------------------------------------------------------

func loadDefaults(t *testing.T) config.Config {
	t.Helper()
	cfg, err := config.NewConfigService(config.Overrides{}).LoadConfiguration()
	if err != nil {
		t.Fatalf("LoadConfiguration: %v", err)
	}
	return cfg
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

func TestOverride_EmptyStringDoesNotClobberDefault(t *testing.T) {
	cfg, err := config.NewConfigService(config.Overrides{}).LoadConfiguration()
	if err != nil {
		t.Fatalf("LoadConfiguration: %v", err)
	}
	if cfg.Neo4j.URI == "" {
		t.Error("empty-string override should not clobber the default URI")
	}
}

// ---- Precedence ---------------------------------------------------------

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
