package service

import (
	"context"
	"fmt"

	"github.com/cli/go-cli-tool/internal/repository"
)

// CypherServiceImpl executes Cypher queries via the graph repository.
type CypherServiceImpl struct {
	repo interface {
		ExecuteQuery(ctx context.Context, cypher string, params map[string]interface{}) (interface{}, error)
	}
}

// NewCypherService creates a CypherService backed by the given repository.
func NewCypherService(repo interface {
	ExecuteQuery(ctx context.Context, cypher string, params map[string]interface{}) (interface{}, error)
}) CypherService {
	return &CypherServiceImpl{repo: repo}
}

// Execute runs the given Cypher query with optional parameters and returns a
// structured QueryResult suitable for table or graph rendering.
func (s *CypherServiceImpl) Execute(ctx context.Context, query string, params map[string]interface{}) (QueryResult, error) {
	if query == "" {
		return QueryResult{}, fmt.Errorf("query cannot be empty")
	}

	raw, err := s.repo.ExecuteQuery(ctx, query, params)
	if err != nil {
		return QueryResult{}, fmt.Errorf("query failed: %w", err)
	}

	return wrapRawResult(raw), nil
}

// wrapRawResult converts the value returned by the repository into a
// QueryResult.
//
// Priority:
//  1. *repository.RecordSet — the real driver path; columns and rows are
//     already in plain-Go form after convertValue in the repository.
//  2. QueryResult — already converted (future-proofing / test helpers).
//  3. Everything else — wrapped into a single-column "result" row for
//     backward compatibility with tests that return primitive stubs.
func wrapRawResult(v interface{}) QueryResult {
	if v == nil {
		return QueryResult{Columns: []string{}, Rows: []QueryRow{}}
	}

	if rs, ok := v.(*repository.RecordSet); ok {
		if rs == nil || len(rs.Rows) == 0 {
			return QueryResult{Columns: rs.Columns, Rows: []QueryRow{}}
		}
		return QueryResult{Columns: rs.Columns, Rows: rs.Rows}
	}

	if qr, ok := v.(QueryResult); ok {
		return qr
	}

	// Fallback: used by tests that return plain strings / ints as stubs.
	return QueryResult{
		Columns: []string{"result"},
		Rows:    []QueryRow{{"result": fmt.Sprintf("%v", v)}},
	}
}
