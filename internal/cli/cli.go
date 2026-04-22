package cli

import (
	"errors"
	"fmt"

	"github.com/cli/go-cli-tool/internal/config"
	"github.com/cli/go-cli-tool/internal/logger"
	"github.com/cli/go-cli-tool/internal/telemetry"
)

// Represents the neo4jCLI instance
type Neo4jCLI struct {
	Log          logger.Service
	Presentation *presentation.Service
	Config       *config.Config
	Telemetry    *telemetry.TelemetryService
}

func NewCLI(cfg *config.Config) (*Neo4jCLI, error) {
	if cfg == nil {
		return nil, errors.New("config is required")
	}

	log := logger.NewLoggerService(
		logger.LogFormat(cfg.LogFormat),
		logger.LogLevel(cfg.LogLevel),
	)

	tel, err := telemetry.NewTelemetryService(cfg)
	if err != nil {
		return nil, fmt.Errorf("telemetry: %w", err)
	}

	pres := presentation.NewPresentationService( /* ... */ )

	return &Neo4jCLI{
		Log:          log,
		Config:       cfg,
		Telemetry:    tel,
		Presentation: pres,
	}, nil
}
