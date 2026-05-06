package repository

import (
	"context"
	"fmt"
	"sync"

	neo4j "github.com/neo4j/neo4j-go-driver/v6/neo4j"

	"github.com/cli/go-cli-tool/internal/config"
)

// Neo4jRepository implements GraphRepository against a live Neo4j instance.
//
// The driver is initialised lazily on the first query so that startup is fast
// and credentials collected interactively (via InteractiveNeo4jPrerequisite)
// are visible before the first connection attempt.
type Neo4jRepository struct {
	cfg    *config.Neo4jConfig
	mu     sync.Mutex
	driver neo4j.Driver // nil until first use
}

// NewNeo4jRepository creates a repository that reads connection config from
// the live pointer. cfg must point into the application's Config so that
// interactively supplied credentials are picked up on the first query.
func NewNeo4jRepository(cfg *config.Neo4jConfig) *Neo4jRepository {
	return &Neo4jRepository{cfg: cfg}
}

// ensureDriver returns the shared driver, creating it on the first call.
// If the credentials change between calls (shouldn't happen in normal use),
// the caller must call Close() first to force re-initialisation.
func (r *Neo4jRepository) ensureDriver() (neo4j.Driver, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.driver != nil {
		return r.driver, nil
	}

	if r.cfg.URI == "" {
		return nil, fmt.Errorf("no Neo4j URI configured — set NCTL_NEO4J_URI or neo4j.uri in your config file")
	}
	if r.cfg.Username == "" {
		return nil, fmt.Errorf("no Neo4j username configured — set NCTL_NEO4J_USERNAME or neo4j.username in your config file")
	}

	driver, err := neo4j.NewDriver(
		r.cfg.URI,
		neo4j.BasicAuth(r.cfg.Username, r.cfg.Password, ""),
	)
	if err != nil {
		return nil, fmt.Errorf("create Neo4j driver: %w", err)
	}

	r.driver = driver
	return driver, nil
}

// ExecuteQuery runs cypher with optional parameters against the configured
// database and returns a *RecordSet containing all result rows. All
// driver-specific types (Node, Relationship, Path) are converted to plain Go
// maps so that callers never need to import the driver package.
func (r *Neo4jRepository) ExecuteQuery(ctx context.Context, cypher string, params map[string]interface{}) (interface{}, error) {
	driver, err := r.ensureDriver()
	if err != nil {
		return nil, err
	}

	db := r.cfg.Database
	if db == "" {
		db = "neo4j"
	}

	result, err := neo4j.ExecuteQuery(ctx, driver, cypher, params,
		neo4j.EagerResultTransformer,
		neo4j.ExecuteQueryWithDatabase(db),
	)
	if err != nil {
		return nil, fmt.Errorf("execute query: %w", err)
	}

	return toRecordSet(result), nil
}

// Close releases the driver and its connection pool. Safe to call multiple
// times. The repository can be used again after Close — the driver will be
// re-created on the next query.
func (r *Neo4jRepository) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.driver == nil {
		return nil
	}

	err := r.driver.Close(context.Background())
	r.driver = nil
	return err
}

// ---- Result conversion --------------------------------------------------

// toRecordSet converts an EagerResult into a RecordSet, translating all
// driver-specific values into plain Go types.
func toRecordSet(r *neo4j.EagerResult) *RecordSet {
	rows := make([]map[string]interface{}, 0, len(r.Records))
	for _, rec := range r.Records {
		row := make(map[string]interface{}, len(r.Keys))
		for _, key := range r.Keys {
			v, _ := rec.Get(key)
			row[key] = convertValue(v)
		}
		rows = append(rows, row)
	}
	qt := ""
	if r.Summary != nil {
		qt = r.Summary.QueryType().String()
	}
	return &RecordSet{Columns: r.Keys, Rows: rows, QueryType: qt}
}

// convertValue recursively translates neo4j driver types into plain Go values.
//
// Nodes become maps with _labels ([]string), _id (string), and all
// properties merged at the top level. Relationships become maps with _type,
// _id, _start, _end, and their properties. Paths are rendered as a string
// summary. Lists are converted element-wise. Everything else is returned as-is.
func convertValue(v interface{}) interface{} {
	if v == nil {
		return nil
	}

	switch t := v.(type) {
	case neo4j.Node:
		m := make(map[string]interface{}, len(t.Props)+2)
		m["_labels"] = t.Labels
		m["_id"] = t.ElementId
		for k, pv := range t.Props {
			m[k] = convertValue(pv)
		}
		return m

	case neo4j.Relationship:
		m := make(map[string]interface{}, len(t.Props)+4)
		m["_type"] = t.Type
		m["_id"] = t.ElementId
		m["_start"] = t.StartElementId
		m["_end"] = t.EndElementId
		for k, pv := range t.Props {
			m[k] = convertValue(pv)
		}
		return m

	case neo4j.Path:
		nodes := make([]interface{}, len(t.Nodes))
		for i, n := range t.Nodes {
			nodes[i] = convertValue(n)
		}
		rels := make([]interface{}, len(t.Relationships))
		for i, r := range t.Relationships {
			rels[i] = convertValue(r)
		}
		return map[string]interface{}{
			"_type": "path",
			"nodes": nodes,
			"rels":  rels,
		}

	case []interface{}:
		out := make([]interface{}, len(t))
		for i, item := range t {
			out[i] = convertValue(item)
		}
		return out

	case map[string]interface{}:
		out := make(map[string]interface{}, len(t))
		for k, mv := range t {
			out[k] = convertValue(mv)
		}
		return out

	default:
		return v
	}
}
