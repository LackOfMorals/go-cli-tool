package repository

import "context"

// GraphRepository defines the interface for Neo4j operations
type GraphRepository interface {
	ExecuteQuery(ctx context.Context, cypher string, params map[string]interface{}) (interface{}, error)
	Close() error
}
