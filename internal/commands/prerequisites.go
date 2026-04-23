package commands

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/cli/go-cli-tool/internal/config"
	"github.com/cli/go-cli-tool/internal/tool"
)

// Neo4jPrerequisite returns a prerequisite check that verifies the Neo4j
// connection is minimally configured (URI and username are both present).
// Wire it into categories that require a live database connection:
//
//	commands.BuildCypherCategory(svc).SetPrerequisite(commands.Neo4jPrerequisite(&cfg.Neo4j))
//	commands.BuildAdminCategory(svc).SetPrerequisite(commands.Neo4jPrerequisite(&cfg.Neo4j))
func Neo4jPrerequisite(cfg *config.Neo4jConfig) func() error {
	return func() error {
		if cfg.URI == "" {
			return fmt.Errorf("%w: Neo4j connection not configured\n"+
				"  Set neo4j.uri in your config file or use --neo4j-uri",
				tool.ErrPrerequisite)
		}
		if cfg.Username == "" {
			return fmt.Errorf("%w: Neo4j username not configured\n"+
				"  Set neo4j.username in your config file or use --neo4j-username",
				tool.ErrPrerequisite)
		}
		return nil
	}
}

// AuraPrerequisite returns a prerequisite check that verifies the Aura API
// credentials (client ID and client secret) are both configured.
// This is the non-interactive version, suitable for testing.
// In the shell, use InteractiveAuraPrerequisite instead.
//
//	commands.BuildCloudCategory(svc).SetPrerequisite(commands.AuraPrerequisite(&cfg.Aura))
func AuraPrerequisite(cfg *config.AuraConfig) func() error {
	return func() error {
		if cfg.ClientID == "" {
			return fmt.Errorf("%w: Aura API client ID not configured\n"+
				"  Set aura.client_id in your config file or use --aura-client-id",
				tool.ErrPrerequisite)
		}
		if cfg.ClientSecret == "" {
			return fmt.Errorf("%w: Aura API client secret not configured\n"+
				"  Set aura.client_secret in your config file",
				tool.ErrPrerequisite)
		}
		return nil
	}
}

// InteractiveAuraPrerequisite returns a prerequisite check that prompts the
// user for Aura API credentials when they are not yet configured, then saves
// them to the config file for future sessions.
//
//   - aura points to the live AuraConfig inside full so the cloud service sees
//     the updated credentials on the very next call.
//   - full is the complete Config used for serialisation.
//   - cfgPath is the file to write; if empty, DefaultConfigFilePath() is used.
//
// Use this in the shell app instead of AuraPrerequisite:
//
//	commands.BuildCloudCategory(svc).
//	    SetPrerequisite(commands.InteractiveAuraPrerequisite(&cfg.Aura, &cfg, configPath))
func InteractiveAuraPrerequisite(aura *config.AuraConfig, full *config.Config, cfgPath string) func() error {
	return func() error {
		if aura.ClientID != "" && aura.ClientSecret != "" {
			return nil
		}

		fmt.Fprintln(os.Stderr, "\nNeo4j Aura API credentials are required.")
		fmt.Fprintln(os.Stderr, "Tip: set CLI_AURA_CLIENT_ID / CLI_AURA_CLIENT_SECRET to skip this prompt.")
		fmt.Fprintln(os.Stderr, "")

		fmt.Fprint(os.Stderr, "Aura Client ID: ")
		id, err := readConsoleLine()
		if err != nil || strings.TrimSpace(id) == "" {
			return fmt.Errorf("%w: Aura client ID is required", tool.ErrPrerequisite)
		}

		fmt.Fprint(os.Stderr, "Aura Client Secret: ")
		secret, err := readConsoleLine()
		if err != nil || strings.TrimSpace(secret) == "" {
			return fmt.Errorf("%w: Aura client secret is required", tool.ErrPrerequisite)
		}

		aura.ClientID = strings.TrimSpace(id)
		aura.ClientSecret = strings.TrimSpace(secret)
		// Propagate back so the full config is in sync when SaveConfiguration
		// serialises it.
		full.Aura = *aura

		path := cfgPath
		if path == "" {
			path = config.DefaultConfigFilePath()
		}

		svc := config.NewConfigService(config.Overrides{ConfigFile: path})
		if saveErr := svc.SaveConfiguration(*full); saveErr != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not save credentials to %s: %v\n", path, saveErr)
			fmt.Fprintln(os.Stderr, "Credentials accepted for this session only.")
		} else {
			fmt.Fprintf(os.Stderr, "✓ Credentials saved to %s\n\n", path)
		}

		return nil
	}
}

// readConsoleLine reads one line from stdin, trimming the newline. It is used
// only for interactive credential prompts where no readline instance is active.
func readConsoleLine() (string, error) {
	r := bufio.NewReader(os.Stdin)
	line, err := r.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	return strings.TrimRight(line, "\r\n"), nil
}

// InteractiveNeo4jPrerequisite returns a prerequisite that prompts for Neo4j
// connection details when they are missing, then persists them to the config
// file so subsequent sessions do not prompt again.
//
// The trigger is an empty password — that is the only credential with no
// sensible default. All four fields (URI, username, password, database) are
// prompted, with the current or default value shown in brackets so the user
// can just press Enter to accept it.
func InteractiveNeo4jPrerequisite(neo4j *config.Neo4jConfig, fullCfg *config.Config, cfgPath string) func() error {
	return func() error {
		// Password is the only field that must be explicitly provided by the
		// user. If it is already set, assume the connection is fully configured.
		if neo4j.Password != "" {
			return nil
		}

		fmt.Fprintln(os.Stderr, "\nNeo4j connection details are required.")
		fmt.Fprintln(os.Stderr, "Tip: set CLI_NEO4J_URI, CLI_NEO4J_USERNAME, CLI_NEO4J_PASSWORD, CLI_NEO4J_DATABASE to skip this prompt.")
		fmt.Fprintln(os.Stderr)

		// promptField prints "Label [default]: " and returns whatever the user
		// typed, falling back to dflt on blank input.
		promptField := func(label, dflt string) string {
			fmt.Fprintf(os.Stderr, "%s [%s]: ", label, dflt)
			input, _ := readConsoleLine()
			input = strings.TrimSpace(input)
			if input == "" {
				return dflt
			}
			return input
		}

		uriDefault := neo4j.URI
		if uriDefault == "" {
			uriDefault = "bolt://localhost:7687"
		}
		neo4j.URI = promptField("URI", uriDefault)

		usernameDefault := neo4j.Username
		if usernameDefault == "" {
			usernameDefault = "neo4j"
		}
		neo4j.Username = promptField("Username", usernameDefault)

		fmt.Fprint(os.Stderr, "Password: ")
		pass, _ := readConsoleLine()
		neo4j.Password = strings.TrimSpace(pass)

		databaseDefault := neo4j.Database
		if databaseDefault == "" {
			databaseDefault = "neo4j"
		}
		neo4j.Database = promptField("Database", databaseDefault)

		fullCfg.Neo4j = *neo4j

		path := cfgPath
		if path == "" {
			path = config.DefaultConfigFilePath()
		}
		svc := config.NewConfigService(config.Overrides{ConfigFile: path})
		if saveErr := svc.SaveConfiguration(*fullCfg); saveErr != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not save connection details: %v\n", saveErr)
			fmt.Fprintln(os.Stderr, "Connection details accepted for this session only.")
		} else {
			fmt.Fprintf(os.Stderr, "\u2713 Connection details saved to %s\n\n", path)
		}

		return nil
	}
}
