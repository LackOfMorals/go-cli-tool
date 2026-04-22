package repository

import (
	"context"
	"fmt"
)

// Neo4jRepository implements GraphRepository against a live Neo4j instance.
// URI, credentials, and database name are resolved once at construction time.
type Neo4jRepository struct {
	uri      string
	username string
	password string
	database string
}

// NewNeo4jRepository creates a repository for the given connection details.
// database may be empty, in which case the server's default database is used.
func NewNeo4jRepository(uri, username, password, database string) *Neo4jRepository {
	return &Neo4jRepository{
		uri:      uri,
		username: username,
		password: password,
		database: database,
	}
}

// ExecuteQuery runs cypher against the configured database.
// TODO: replace the placeholder with a real neo4j-go-driver/v5 session.
func (r *Neo4jRepository) ExecuteQuery(ctx context.Context, cypher string, params map[string]interface{}) (interface{}, error) {
	if r.uri == "" {
		return nil, fmt.Errorf("no Neo4j URI configured — set CLI_NEO4J_URI or neo4j.uri in your config file")
	}
	// Placeholder until the driver is wired in.
	fmt.Printf("[neo4j %s/%s] %s\n", r.uri, r.database, cypher)
	return "result placeholder", nil
}

// Close releases any resources held by the repository.
func (r *Neo4jRepository) Close() error {
	return nil
}
