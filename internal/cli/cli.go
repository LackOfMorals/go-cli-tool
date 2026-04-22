package cli

import (
	"errors"
	"fmt"

	"github.com/cli/go-cli-tool/internal/analytics"
	"github.com/cli/go-cli-tool/internal/config"
	"github.com/cli/go-cli-tool/internal/logger"
	"github.com/cli/go-cli-tool/internal/presentation"
)

// Neo4jCLI is the top-level facade that aggregates every initialised service.
// Downstream components (shell, tool executor) receive this struct so they
// don't have to manage individual service lifecycles themselves.
type Neo4jCLI struct {
	Log          logger.Service
	Presentation *presentation.PresentationService
	Config       *config.Config
	Telemetry    analytics.Service
}

// NewCLI constructs a Neo4jCLI from an already-loaded Config, an initialised
// logger, and an initialised analytics service. Accepting them as parameters
// (rather than constructing them internally) keeps startup order explicit in
// app.go and makes the CLI facade easy to test.
func NewCLI(cfg *config.Config, log logger.Service, tel analytics.Service) (*Neo4jCLI, error) {
	if cfg == nil {
		return nil, errors.New("config is required")
	}
	if log == nil {
		return nil, errors.New("logger is required")
	}
	if tel == nil {
		return nil, errors.New("analytics service is required")
	}

	// Resolve the output format: default to text if the config value is empty
	// or not a recognised format.
	format := presentation.OutputFormat(cfg.LogFormat)
	if !format.IsValid() {
		format = presentation.OutputFormatText
	}

	pres, err := presentation.NewPresentationService(format, log)
	if err != nil {
		return nil, fmt.Errorf("init presentation service: %w", err)
	}

	return &Neo4jCLI{
		Log:          log,
		Config:       cfg,
		Telemetry:    tel,
		Presentation: pres,
	}, nil
}
