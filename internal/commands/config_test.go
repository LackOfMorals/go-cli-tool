package commands_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cli/go-cli-tool/internal/commands"
	"github.com/cli/go-cli-tool/internal/config"
	"github.com/cli/go-cli-tool/internal/dispatch"
	"github.com/cli/go-cli-tool/internal/logger"
	"github.com/cli/go-cli-tool/internal/presentation"
)

func configCtx(t *testing.T) dispatch.Context {
	t.Helper()
	log := logger.NewLoggerService(logger.FormatText, logger.LevelError)
	pres, err := presentation.NewPresentationService(presentation.OutputFormatTable, log)
	if err != nil {
		t.Fatalf("NewPresentationService: %v", err)
	}
	return dispatch.Context{
		Context:   context.Background(),
		Config:    config.Config{},
		IO:        &mockIO{},
		Presenter: pres,
	}
}

// TestConfigCategory_SetCypherOutputFormat_TOON verifies that "toon" is an
// accepted value for cypher.output_format and round-trips through the
// loader after being persisted to disk.
func TestConfigCategory_SetCypherOutputFormat_TOON(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.json")
	cfg := &config.Config{}

	cat := commands.BuildConfigCategory(cfg, cfgPath)

	if _, err := cat.Dispatch([]string{"set", "cypher.output_format", "toon"}, configCtx(t)); err != nil {
		t.Fatalf("set cypher.output_format=toon: %v", err)
	}
	if got := cfg.Cypher.OutputFormat; got != "toon" {
		t.Errorf("in-memory config: got %q, want %q", got, "toon")
	}

	// Round-trip through the loader: read the persisted file and confirm
	// the value survives unmarshal + validation.
	loaded, err := config.NewConfigService(config.Overrides{ConfigFile: cfgPath}).LoadConfiguration()
	if err != nil {
		t.Fatalf("load configuration: %v", err)
	}
	if loaded.Cypher.OutputFormat != "toon" {
		t.Errorf("loaded config: got %q, want %q", loaded.Cypher.OutputFormat, "toon")
	}
}

// TestConfigCategory_SetCypherOutputFormat_Invalid confirms that the
// existing rejection path still fires for unknown formats.
func TestConfigCategory_SetCypherOutputFormat_Invalid(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.json")
	cfg := &config.Config{}

	cat := commands.BuildConfigCategory(cfg, cfgPath)

	_, err := cat.Dispatch([]string{"set", "cypher.output_format", "bogus"}, configCtx(t))
	if err == nil {
		t.Fatal("expected error for invalid output format value")
	}
	if !strings.Contains(err.Error(), "table, json, pretty-json, graph, toon") {
		t.Errorf("error message should list valid formats including toon, got: %v", err)
	}
}

// TestConfigCategory_ListShowsTOONInDescription verifies that the user-
// facing description for cypher.output_format advertises toon.
func TestConfigCategory_ListShowsTOONInDescription(t *testing.T) {
	cfg := &config.Config{}
	cat := commands.BuildConfigCategory(cfg, "")

	result, err := cat.Dispatch([]string{"list"}, configCtx(t))
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	var found bool
	for _, item := range result.Items {
		if item["key"] != "cypher.output_format" {
			continue
		}
		desc, _ := item["description"].(string)
		if !strings.Contains(desc, "toon") {
			t.Errorf("cypher.output_format description should list toon: %q", desc)
		}
		found = true
		break
	}
	if !found {
		t.Fatal("cypher.output_format key not present in config list")
	}
}
