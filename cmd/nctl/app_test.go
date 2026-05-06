package main

import (
	"testing"

	"github.com/cli/go-cli-tool/internal/config"
	"github.com/cli/go-cli-tool/internal/presentation"
)

// TestResolveDefaultPresentationFormat_AgentNoConfig: agent mode with no
// cypher.output_format set picks TOON.
func TestResolveDefaultPresentationFormat_AgentNoConfig(t *testing.T) {
	cfg := &config.Config{}
	cfg.Cypher.OutputFormat = ""

	got := resolveDefaultPresentationFormat(cfg, true)
	if got != presentation.OutputFormatTOON {
		t.Errorf("agent + no config: got %q, want %q", got, presentation.OutputFormatTOON)
	}
}

// TestResolveDefaultPresentationFormat_AgentConfigSet: agent mode with an
// explicit cypher.output_format honours the config value over the agent
// TOON default.
func TestResolveDefaultPresentationFormat_AgentConfigSet(t *testing.T) {
	cases := []presentation.OutputFormat{
		presentation.OutputFormatJSON,
		presentation.OutputFormatPrettyJSON,
		presentation.OutputFormatGraph,
		presentation.OutputFormatTable,
		presentation.OutputFormatText,
		presentation.OutputFormatTOON, // explicit toon also wins, trivially
	}
	for _, want := range cases {
		t.Run(string(want), func(t *testing.T) {
			cfg := &config.Config{}
			cfg.Cypher.OutputFormat = string(want)

			got := resolveDefaultPresentationFormat(cfg, true)
			if got != want {
				t.Errorf("agent + config %q: got %q, want %q", want, got, want)
			}
		})
	}
}

// TestResolveDefaultPresentationFormat_NonAgentNoConfig: outside agent mode
// with no cypher.output_format the default stays as today (table).
func TestResolveDefaultPresentationFormat_NonAgentNoConfig(t *testing.T) {
	cfg := &config.Config{}
	cfg.Cypher.OutputFormat = ""

	got := resolveDefaultPresentationFormat(cfg, false)
	if got != presentation.OutputFormatTable {
		t.Errorf("non-agent + no config: got %q, want %q", got, presentation.OutputFormatTable)
	}
}

// TestResolveDefaultPresentationFormat_NonAgentConfigSet: outside agent mode
// an explicit cypher.output_format wins over the table default.
func TestResolveDefaultPresentationFormat_NonAgentConfigSet(t *testing.T) {
	cfg := &config.Config{}
	cfg.Cypher.OutputFormat = string(presentation.OutputFormatJSON)

	got := resolveDefaultPresentationFormat(cfg, false)
	if got != presentation.OutputFormatJSON {
		t.Errorf("non-agent + config json: got %q, want %q", got, presentation.OutputFormatJSON)
	}
}

// TestResolveDefaultPresentationFormat_InvalidConfigValue: an unrecognised
// cypher.output_format value falls through to the agent/non-agent default
// rather than propagating an invalid format to NewPresentationService (which
// would error out at startup).
func TestResolveDefaultPresentationFormat_InvalidConfigValue(t *testing.T) {
	cfg := &config.Config{}
	cfg.Cypher.OutputFormat = "xml-but-not-supported"

	if got := resolveDefaultPresentationFormat(cfg, true); got != presentation.OutputFormatTOON {
		t.Errorf("agent + invalid config: got %q, want %q", got, presentation.OutputFormatTOON)
	}
	if got := resolveDefaultPresentationFormat(cfg, false); got != presentation.OutputFormatTable {
		t.Errorf("non-agent + invalid config: got %q, want %q", got, presentation.OutputFormatTable)
	}
}

// TestResolveDefaultPresentationFormat_NilConfig: defensive — a nil config
// should not panic; the agent/non-agent default applies.
func TestResolveDefaultPresentationFormat_NilConfig(t *testing.T) {
	if got := resolveDefaultPresentationFormat(nil, true); got != presentation.OutputFormatTOON {
		t.Errorf("agent + nil cfg: got %q, want %q", got, presentation.OutputFormatTOON)
	}
	if got := resolveDefaultPresentationFormat(nil, false); got != presentation.OutputFormatTable {
		t.Errorf("non-agent + nil cfg: got %q, want %q", got, presentation.OutputFormatTable)
	}
}
