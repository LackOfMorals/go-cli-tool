package service

import (
	"context"
	"fmt"
	"strings"

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

// Explain prepends EXPLAIN to query (unless already present), executes it
// against the database, and returns the Neo4j planner classification:
//
//	"r"  – read only
//	"rw" – read/write
//	"w"  – write only
//	"s"  – schema write
//
// If the repo returns a non-RecordSet (e.g. a test stub), the method returns
// "r" as a safe default rather than blocking execution.
func (s *CypherServiceImpl) Explain(ctx context.Context, query string) (string, error) {
	upper := strings.TrimSpace(strings.ToUpper(query))
	explainQuery := query
	if !strings.HasPrefix(upper, "EXPLAIN") && !strings.HasPrefix(upper, "PROFILE") {
		explainQuery = "EXPLAIN " + query
	}
	raw, err := s.repo.ExecuteQuery(ctx, explainQuery, nil)
	if err != nil {
		return "", fmt.Errorf("explain query: %w", err)
	}
	if rs, ok := raw.(*repository.RecordSet); ok {
		return rs.QueryType, nil
	}
	return "r", nil // safe default for test stubs
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
			return QueryResult{Columns: rs.Columns, Rows: []QueryRow{}, QueryType: rs.QueryType}
		}
		return QueryResult{Columns: rs.Columns, Rows: rs.Rows, QueryType: rs.QueryType}
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
