package service

import "context"

// ---- Cypher -------------------------------------------------------------

// CypherService executes Cypher queries against a Neo4j database.
type CypherService interface {
	// Execute runs an arbitrary Cypher query and returns the result as a
	// formatted string. Read-only enforcement is the caller's responsibility.
	Execute(ctx context.Context, query string) (string, error)
}

// ---- Cloud --------------------------------------------------------------

// CloudService is the top-level entry point for Aura cloud operations.
// It exposes sub-services per resource type, mirroring the shell hierarchy.
type CloudService interface {
	Instances() InstancesService
	Projects() ProjectsService
}

// Instance represents a Neo4j Aura DB instance.
type Instance struct {
	ID     string
	Name   string
	Status string
	Region string
	Tier   string
}

// InstancesService manages Aura DB instances.
type InstancesService interface {
	List(ctx context.Context) ([]Instance, error)
	Get(ctx context.Context, id string) (*Instance, error)
	Pause(ctx context.Context, id string) error
	Resume(ctx context.Context, id string) error
	Delete(ctx context.Context, id string) error
}

// Project represents an Aura project (tenant).
type Project struct {
	ID   string
	Name string
}

// ProjectsService manages Aura projects.
type ProjectsService interface {
	List(ctx context.Context) ([]Project, error)
	Get(ctx context.Context, id string) (*Project, error)
}

// ---- Admin --------------------------------------------------------------

// User represents a database user.
type User struct {
	Username string
	Roles    []string
}

// Database represents a Neo4j database within an instance.
type Database struct {
	Name   string
	Status string
}

// AdminService performs administrative operations against a Neo4j instance.
type AdminService interface {
	ShowUsers(ctx context.Context) ([]User, error)
	ShowDatabases(ctx context.Context) ([]Database, error)
}
