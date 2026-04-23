package service

import (
	"context"
	"fmt"

	"github.com/cli/go-cli-tool/internal/repository"
)

// CypherServiceImpl executes Cypher queries via the graph repository.
type CypherServiceImpl struct {
	repo repository.GraphRepository
}

// NewCypherService creates a CypherService backed by the given repository.
func NewCypherService(repo repository.GraphRepository) CypherService {
	return &CypherServiceImpl{repo: repo}
}

// Execute runs the given Cypher query and returns a formatted result string.
func (s *CypherServiceImpl) Execute(ctx context.Context, query string) (string, error) {
	if query == "" {
		return "", fmt.Errorf("query cannot be empty")
	}

	result, err := s.repo.ExecuteQuery(ctx, query, nil)
	if err != nil {
		return "", fmt.Errorf("query failed: %w", err)
	}

	return fmt.Sprintf("%v", result), nil
}
