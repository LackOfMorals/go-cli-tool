package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/cli/go-cli-tool/internal/repository"
)

// ErrResultNotParseable is returned by the admin service when the repository
// result cannot be decoded into typed records. This is expected until
// neo4j-go-driver/v5 is wired into Neo4jRepository; once the driver lands
// the parse* functions below will use neo4j.Record field accessors instead.
var ErrResultNotParseable = errors.New(
	"result parsing not implemented: wire neo4j-go-driver/v5 into the repository",
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
	users, err := parseUsersResult(result)
	if err != nil {
		return nil, fmt.Errorf("show users: %w", err)
	}
	return users, nil
}

func (s *AdminServiceImpl) ShowDatabases(ctx context.Context) ([]Database, error) {
	result, err := s.repo.ExecuteQuery(ctx, "SHOW DATABASES", nil)
	if err != nil {
		return nil, fmt.Errorf("show databases: %w", err)
	}
	dbs, err := parseDatabasesResult(result)
	if err != nil {
		return nil, fmt.Errorf("show databases: %w", err)
	}
	return dbs, nil
}

// parseUsersResult converts a raw repository result into a []User slice.
//
// TODO: when neo4j-go-driver/v5 is wired in, type-assert result to
// []neo4j.Record and iterate over each record using:
//
//	username, _ := record.Get("user")
//	roles, _ := record.Get("roles")
func parseUsersResult(result interface{}) ([]User, error) {
	// The placeholder repository returns a plain string; any string result
	// means the real driver is not yet connected.
	if _, ok := result.(string); ok {
		return nil, ErrResultNotParseable
	}
	return nil, fmt.Errorf("unrecognised result type %T: %w", result, ErrResultNotParseable)
}

// parseDatabasesResult converts a raw repository result into a []Database slice.
//
// TODO: when neo4j-go-driver/v5 is wired in, type-assert result to
// []neo4j.Record and iterate over each record using:
//
//	name, _ := record.Get("name")
//	status, _ := record.Get("currentStatus")
func parseDatabasesResult(result interface{}) ([]Database, error) {
	if _, ok := result.(string); ok {
		return nil, ErrResultNotParseable
	}
	return nil, fmt.Errorf("unrecognised result type %T: %w", result, ErrResultNotParseable)
}
