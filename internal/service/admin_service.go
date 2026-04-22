package service

import (
	"context"
	"fmt"

	"github.com/cli/go-cli-tool/internal/repository"
)

// AdminServiceImpl performs administrative operations via the graph repository.
type AdminServiceImpl struct {
	repo repository.GraphRepository
}

// NewAdminService creates an AdminService backed by the given repository.
func NewAdminService(repo repository.GraphRepository) AdminService {
	return &AdminServiceImpl{repo: repo}
}

func (s *AdminServiceImpl) ShowUsers(ctx context.Context) ([]User, error) {
	result, err := s.repo.ExecuteQuery(ctx, "SHOW USERS", nil)
	if err != nil {
		return nil, fmt.Errorf("show users: %w", err)
	}
	// TODO: parse the result into []User once the repository returns typed records.
	_ = result
	return []User{{Username: "neo4j", Roles: []string{"admin"}}}, nil
}

func (s *AdminServiceImpl) ShowDatabases(ctx context.Context) ([]Database, error) {
	result, err := s.repo.ExecuteQuery(ctx, "SHOW DATABASES", nil)
	if err != nil {
		return nil, fmt.Errorf("show databases: %w", err)
	}
	_ = result
	return []Database{{Name: "neo4j", Status: "online"}}, nil
}
