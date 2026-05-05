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

// parseUsersResult converts a *repository.RecordSet from SHOW USERS into []User.
// SHOW USERS returns columns: user, roles, passwordChangeRequired, suspended, home.
func parseUsersResult(result interface{}) ([]User, error) {
	rs, ok := result.(*repository.RecordSet)
	if !ok {
		return nil, fmt.Errorf("unexpected result type %T", result)
	}
	users := make([]User, 0, len(rs.Rows))
	for _, row := range rs.Rows {
		u := User{}
		if v, ok := row["user"]; ok {
			u.Username, _ = v.(string)
		}
		if v, ok := row["roles"]; ok {
			switch rv := v.(type) {
			case []interface{}:
				for _, r := range rv {
					if s, ok := r.(string); ok {
						u.Roles = append(u.Roles, s)
					}
				}
			case []string:
				u.Roles = rv
			}
		}
		users = append(users, u)
	}
	return users, nil
}

// parseDatabasesResult converts a *repository.RecordSet from SHOW DATABASES into []Database.
// SHOW DATABASES returns many columns; we extract name and currentStatus.
func parseDatabasesResult(result interface{}) ([]Database, error) {
	rs, ok := result.(*repository.RecordSet)
	if !ok {
		return nil, fmt.Errorf("unexpected result type %T", result)
	}
	dbs := make([]Database, 0, len(rs.Rows))
	for _, row := range rs.Rows {
		db := Database{}
		if v, ok := row["name"]; ok {
			db.Name, _ = v.(string)
		}
		if v, ok := row["currentStatus"]; ok {
			db.Status, _ = v.(string)
		}
		dbs = append(dbs, db)
	}
	return dbs, nil
}
