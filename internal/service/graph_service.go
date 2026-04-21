package service

import (
	"context"
	"github.com/cli/go-cli-tool/internal/repository"
)

// GraphService handles business logic for graph operations
type GraphService struct {
	repo repository.GraphRepository
}

func NewGraphService(repo repository.GraphRepository) *GraphService {
	return &GraphService{repo: repo}
}

func (s *GraphService) RunQuery(ctx context.Context, cypher string) (interface{}, error) {
	// Business logic/validation goes here
	return s.repo.ExecuteQuery(ctx, cypher, nil)
}
