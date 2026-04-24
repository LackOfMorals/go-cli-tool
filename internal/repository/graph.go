package repository

import "context"

// GraphRepository defines the interface for Neo4j operations.
type GraphRepository interface {
	ExecuteQuery(ctx context.Context, cypher string, params map[string]interface{}) (interface{}, error)
	Close() error
}

// RecordSet is the repository-layer representation of a query result.
// All driver-specific types (neo4j.Node, neo4j.Relationship, etc.) are
// converted to plain Go values before being placed here, so upstream layers
// (service, commands) never need to import the Neo4j driver directly.
type RecordSet struct {
	Columns []string
	Rows    []map[string]interface{}
}
