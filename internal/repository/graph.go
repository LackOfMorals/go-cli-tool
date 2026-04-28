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
//
// QueryType carries the Neo4j planner classification for the statement:
//   - "r"  read only
//   - "rw" read/write
//   - "w"  write only
//   - "s"  schema write
//
// It is populated from the result summary and is used by the cypher service
// to enforce agent-mode read-only protection via EXPLAIN pre-checks.
type RecordSet struct {
	Columns   []string
	Rows      []map[string]interface{}
	QueryType string // Neo4j query classification: "r", "rw", "w", "s"
}
