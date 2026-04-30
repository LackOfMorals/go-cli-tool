package service

import "context"

//go:generate mockgen -destination=mocks/mock_skill_service.go -package=service_mocks -typed github.com/cli/go-cli-tool/internal/service SkillService

// ---- Cypher -------------------------------------------------------------

// QueryRow is a single result row: column name → value.
type QueryRow = map[string]interface{}

// QueryResult holds the structured output of a Cypher query.
type QueryResult struct {
	Columns []string
	Rows    []QueryRow
	// QueryType is the Neo4j planner classification populated when the query
	// was preceded by EXPLAIN: "r" (read), "rw" (read/write), "w" (write
	// only), "s" (schema write). Empty string when not determined.
	QueryType string
}

// CypherService executes Cypher queries against a Neo4j database.
type CypherService interface {
	// Execute runs query with optional parameters and returns a structured
	// result. Callers are responsible for formatting the result for display.
	Execute(ctx context.Context, query string, params map[string]interface{}) (QueryResult, error)

	// Explain prepends EXPLAIN to query (unless already present), executes it,
	// and returns the Neo4j query-type string: "r", "rw", "w", or "s".
	// Use this in agent mode to determine whether a statement is safe to run
	// without --rw before sending it for real execution.
	Explain(ctx context.Context, query string) (string, error)
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
	ID            string
	Name          string
	Status        string
	Region        string
	Tier          string // instance type (e.g. "enterprise-db")
	CloudProvider string
	TenantID      string
	ConnectionURL string
	Username      string
	Memory        string
}

// CreatedInstance wraps Instance to carry the one-time password returned at
// creation. The Password field is only populated by Create and is never
// returned again by the API — callers must record it immediately.
type CreatedInstance struct {
	Instance
	Password string
}

// CreateInstanceParams holds the fields required to create a new Aura instance.
// Values not explicitly provided by the caller should be pre-filled from
// config.AuraInstanceDefaults before passing to the service.
type CreateInstanceParams struct {
	Name          string
	TenantID      string
	CloudProvider string
	Region        string
	Type          string // instance type, e.g. "enterprise-db"
	Version       string // Neo4j version, e.g. "5"
	Memory        string // e.g. "8GB"
}

// UpdateInstanceParams holds the mutable fields that can be changed on an
// existing Aura instance. Empty strings are ignored by the service layer.
type UpdateInstanceParams struct {
	Name   string // rename the instance
	Memory string // resize memory, e.g. "16GB"
}

// InstancesService manages Aura DB instances.
type InstancesService interface {
	List(ctx context.Context) ([]Instance, error)
	Get(ctx context.Context, id string) (*Instance, error)
	Create(ctx context.Context, params *CreateInstanceParams) (*CreatedInstance, error)
	Update(ctx context.Context, id string, params *UpdateInstanceParams) (*Instance, error)
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

// ---- Skill --------------------------------------------------------------

// InstallResult describes a single successful agent install.
type InstallResult struct {
	Agent string `json:"agent"`
	Path  string `json:"path"`
}

// RemoveResult describes a single successful agent removal.
type RemoveResult struct {
	Agent string `json:"agent"`
}

// AgentStatus is the per-agent row returned by SkillService.List.
type AgentStatus struct {
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
	Detected    bool   `json:"detected"`
	Installed   bool   `json:"installed"`
}

// SkillService installs, removes, and reports the status of the embedded
// SKILL.md across the supported AI agents. An empty agentName fans out to
// every detected agent (Install) or every agent that currently has the skill
// installed (Remove).
type SkillService interface {
	Install(ctx context.Context, agentName string) ([]InstallResult, error)
	Remove(ctx context.Context, agentName string) ([]RemoveResult, error)
	List(ctx context.Context) ([]AgentStatus, error)
}
