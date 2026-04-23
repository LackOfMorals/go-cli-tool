package commands

import (
	"fmt"
	"strings"

	"github.com/cli/go-cli-tool/internal/service"
	"github.com/cli/go-cli-tool/internal/shell"
)

// BuildCypherCategory returns the cypher top-level category.
//
// The entire remainder of the input line is treated as the Cypher query:
//
//	neo4j> cypher MATCH (n) RETURN n LIMIT 5
func BuildCypherCategory(svc service.CypherService) *shell.Category {
	return shell.NewCategory("cypher", "Execute a Cypher query against the connected Neo4j database").
		SetDirectHandler(func(args []string, ctx shell.ShellContext) (string, error) {
			if len(args) == 0 {
				return "", fmt.Errorf("usage: cypher <query>  (e.g. cypher MATCH (n) RETURN n LIMIT 5)")
			}
			return svc.Execute(ctx.Context, strings.Join(args, " "))
		})
}
