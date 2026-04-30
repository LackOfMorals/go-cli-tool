package commands

import (
	"fmt"

	"github.com/cli/go-cli-tool/internal/config"
	"github.com/cli/go-cli-tool/internal/tool"
)

// Neo4jPrerequisite returns a prerequisite check that verifies the Neo4j
// connection is minimally configured (URI and username are both present).
// Wire it into categories that require a live database connection:
//
//	commands.BuildCypherCategory(svc).SetPrerequisite(commands.Neo4jPrerequisite(&cfg.Neo4j))
func Neo4jPrerequisite(cfg *config.Neo4jConfig) func() error {
	return func() error {
		if cfg.URI == "" {
			return fmt.Errorf("%w: Neo4j connection not configured\n"+
				"  Set neo4j.uri via --neo4j-uri flag or CLI_NEO4J_URI env var",
				tool.ErrPrerequisite)
		}
		if cfg.Username == "" {
			return fmt.Errorf("%w: Neo4j username not configured\n"+
				"  Set neo4j.username via --neo4j-username flag or CLI_NEO4J_USERNAME env var",
				tool.ErrPrerequisite)
		}
		return nil
	}
}

// AuraPrerequisite returns a prerequisite check that verifies the Aura API
// credentials (client ID and client secret) are both configured.
//
//	commands.BuildCloudCategory(svc).SetPrerequisite(commands.AuraPrerequisite(&cfg.Aura))
func AuraPrerequisite(cfg *config.AuraConfig) func() error {
	return func() error {
		if cfg.ClientID == "" {
			return fmt.Errorf("%w: Aura API client ID not configured\n"+
				"  Set aura.client_id via --aura-client-id flag or CLI_AURA_CLIENT_ID env var",
				tool.ErrPrerequisite)
		}
		if cfg.ClientSecret == "" {
			return fmt.Errorf("%w: Aura API client secret not configured\n"+
				"  Set aura.client_secret via CLI_AURA_CLIENT_SECRET env var",
				tool.ErrPrerequisite)
		}
		return nil
	}
}
