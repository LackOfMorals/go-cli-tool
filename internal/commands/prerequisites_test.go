package commands_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/cli/go-cli-tool/internal/commands"
	"github.com/cli/go-cli-tool/internal/config"
	"github.com/cli/go-cli-tool/internal/tool"
)

// ---- Neo4jPrerequisite --------------------------------------------------

func TestNeo4jPrerequisite_PassesWhenConfigured(t *testing.T) {
	cfg := &config.Neo4jConfig{
		URI:      "bolt://localhost:7687",
		Username: "neo4j",
	}
	fn := commands.Neo4jPrerequisite(cfg)
	if err := fn(); err != nil {
		t.Errorf("expected no error when fully configured, got: %v", err)
	}
}

func TestNeo4jPrerequisite_FailsWhenURIMissing(t *testing.T) {
	cfg := &config.Neo4jConfig{Username: "neo4j"}
	fn := commands.Neo4jPrerequisite(cfg)
	err := fn()
	if err == nil {
		t.Fatal("expected error when URI is missing")
	}
	if !errors.Is(err, tool.ErrPrerequisite) {
		t.Errorf("expected tool.ErrPrerequisite in chain, got: %v", err)
	}
	if !strings.Contains(err.Error(), "neo4j.uri") {
		t.Errorf("error should mention neo4j.uri, got: %v", err)
	}
}

func TestNeo4jPrerequisite_FailsWhenUsernameMissing(t *testing.T) {
	cfg := &config.Neo4jConfig{URI: "bolt://localhost:7687"}
	fn := commands.Neo4jPrerequisite(cfg)
	err := fn()
	if err == nil {
		t.Fatal("expected error when username is missing")
	}
	if !errors.Is(err, tool.ErrPrerequisite) {
		t.Errorf("expected tool.ErrPrerequisite in chain, got: %v", err)
	}
	if !strings.Contains(err.Error(), "neo4j.username") {
		t.Errorf("error should mention neo4j.username, got: %v", err)
	}
}

// ---- AuraPrerequisite ---------------------------------------------------

func TestAuraPrerequisite_PassesWhenConfigured(t *testing.T) {
	cfg := &config.AuraConfig{
		ClientID:     "client-id",
		ClientSecret: "client-secret",
	}
	fn := commands.AuraPrerequisite(cfg)
	if err := fn(); err != nil {
		t.Errorf("expected no error when fully configured, got: %v", err)
	}
}

func TestAuraPrerequisite_FailsWhenClientIDMissing(t *testing.T) {
	cfg := &config.AuraConfig{ClientSecret: "secret"}
	fn := commands.AuraPrerequisite(cfg)
	err := fn()
	if err == nil {
		t.Fatal("expected error when client ID is missing")
	}
	if !errors.Is(err, tool.ErrPrerequisite) {
		t.Errorf("expected tool.ErrPrerequisite in chain, got: %v", err)
	}
	if !strings.Contains(err.Error(), "aura.client_id") {
		t.Errorf("error should mention aura.client_id, got: %v", err)
	}
}

func TestAuraPrerequisite_FailsWhenSecretMissing(t *testing.T) {
	cfg := &config.AuraConfig{ClientID: "id"}
	fn := commands.AuraPrerequisite(cfg)
	err := fn()
	if err == nil {
		t.Fatal("expected error when client secret is missing")
	}
	if !errors.Is(err, tool.ErrPrerequisite) {
		t.Errorf("expected tool.ErrPrerequisite in chain, got: %v", err)
	}
	if !strings.Contains(err.Error(), "aura.client_secret") {
		t.Errorf("error should mention aura.client_secret, got: %v", err)
	}
}

// ---- Integration: prerequisite wired into category ----------------------

func TestCypherCategory_WithPrerequisite_BlocksDispatch(t *testing.T) {
	svc := &mockCypherService{result: "ok"}
	cat := commands.BuildCypherCategory(svc).
		SetPrerequisite(commands.Neo4jPrerequisite(&config.Neo4jConfig{})) // empty = not configured

	_, err := cat.Dispatch([]string{"MATCH", "(n)", "RETURN", "n"}, cypherCtx(t))
	if err == nil {
		t.Fatal("expected prerequisite error")
	}
	if !errors.Is(err, tool.ErrPrerequisite) {
		t.Errorf("expected tool.ErrPrerequisite, got: %v", err)
	}
}

func TestCypherCategory_WithPrerequisite_AllowsHelpWithoutConnection(t *testing.T) {
	// Typing "cypher" alone should not trigger the prerequisite — it returns
	// a usage hint which requires no database connection.
	svc := &mockCypherService{}
	cat := commands.BuildCypherCategory(svc).
		SetPrerequisite(commands.Neo4jPrerequisite(&config.Neo4jConfig{}))

	_, err := cat.Dispatch(nil, cypherCtx(t))
	// Direct-handler categories return a usage *error* on no args, which is
	// expected — but it must NOT be an ErrPrerequisite.
	if errors.Is(err, tool.ErrPrerequisite) {
		t.Error("prerequisite should not fire on bare category invocation")
	}
}
