package commands

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/cli/go-cli-tool/internal/presentation"
	"github.com/cli/go-cli-tool/internal/service"
	"github.com/cli/go-cli-tool/internal/dispatch"
)

// limitPattern detects a LIMIT clause already present in a Cypher query.
var limitPattern = regexp.MustCompile(`(?i)\bLIMIT\s+\d+`)

// BuildCypherCategory returns the cypher top-level category.
//
// The input after "cypher" is treated as a Cypher query with optional flags:
//
//	neo4j> cypher MATCH (n) RETURN n
//	neo4j> cypher --param name=Alice MATCH (n:Person {name:$name}) RETURN n
//	neo4j> cypher --format graph MATCH (n)-[r]->(m) RETURN n,r,m
//	neo4j> cypher --format json MATCH (n) RETURN n
//	neo4j> cypher --limit 50 MATCH (n) RETURN n
//
// Flags:
//
//	--param key=value            Add a query parameter (repeatable).
//	                             Values are auto-typed: int, float, bool, string.
//	--format table|graph|json|pretty-json
//	                             Override the output format for this query.
//	                             Defaults to cypher.output_format in config.
//	--limit N                    Override the auto-injected row limit.
func BuildCypherCategory(svc service.CypherService) *dispatch.Category {
	return dispatch.NewCategory("cypher", "Execute a Cypher query against the connected Neo4j database").
		AllowEmptyDirectHandler().
		SetDirectHandler(func(args []string, ctx dispatch.Context) (string, error) {
			opts := parseCypherFlags(args)

			// No query on the command line — prompt interactively.
			if opts.query == "" {
				stmt, promptedParams, err := promptForCypher(ctx)
				if err != nil {
					return "", err
				}
				opts.query = stmt
				for k, v := range promptedParams {
					if opts.params == nil {
						opts.params = make(map[string]interface{})
					}
					if _, exists := opts.params[k]; !exists {
						opts.params[k] = v
					}
				}
			}

			// Resolve format: per-query flag > config default.
			format := presentation.OutputFormat(opts.format)
			if format == "" {
				format = presentation.OutputFormat(ctx.Config.Cypher.OutputFormat)
			}
			if format == "" {
				format = presentation.OutputFormatTable
			}

			// Resolve limit: per-query flag > config shell limit > built-in default.
			limit := opts.limit
			if limit <= 0 {
				limit = ctx.Config.Cypher.ShellLimit
			}
			if limit <= 0 {
				limit = 25
			}

			result, err := svc.Execute(ctx.Context, injectLimit(opts.query, limit), opts.params)
			if err != nil {
				return "", err
			}

			data := queryResultToTableData(result)

			if ctx.Presenter == nil {
				// Fallback for tests that don't wire a presenter.
				cols := presentation.NewTableData(result.Columns, toInterfaceRows(result)).Columns()
				if len(cols) == 0 {
					return "", nil
				}
				return cols[0], nil
			}

			return ctx.Presenter.FormatAs(data, format)
		})
}

// queryResultToTableData converts a service.QueryResult into a
// presentation.TableData so the presenter can render it in any format.
func queryResultToTableData(r service.QueryResult) *presentation.TableData {
	rows := make([][]interface{}, len(r.Rows))
	for i, row := range r.Rows {
		cells := make([]interface{}, len(r.Columns))
		for j, col := range r.Columns {
			cells[j] = row[col]
		}
		rows[i] = cells
	}
	return presentation.NewTableData(r.Columns, rows)
}

// toInterfaceRows is a minimal fallback used when ctx.Presenter is nil.
func toInterfaceRows(r service.QueryResult) [][]interface{} {
	rows := make([][]interface{}, len(r.Rows))
	for i, row := range r.Rows {
		cells := make([]interface{}, len(r.Columns))
		for j, col := range r.Columns {
			cells[j] = row[col]
		}
		rows[i] = cells
	}
	return rows
}

// ---- Interactive prompt ------------------------------------------------

// promptForCypher asks the user to enter a Cypher statement when neither
// a query nor parameters were provided on the command line. Input is
// accumulated line by line until a semicolon terminates the statement,
// matching the behaviour of the interactive shell.
func promptForCypher(ctx dispatch.Context) (query string, params map[string]interface{}, err error) {
	if ctx.IO == nil {
		return "", nil, fmt.Errorf("no IO handler available for interactive prompt")
	}

	ctx.IO.Write("Cypher (end with ;):\n")

	var lines []string
	for {
		ctx.IO.Write("...> ")
		rawLine, readErr := ctx.IO.Read()
		line := strings.TrimSpace(rawLine)

		// A read error or a blank line both signal end-of-input.
		// If lines have been accumulated, execute them; otherwise error.
		if readErr != nil || line == "" {
			if len(lines) > 0 {
				break
			}
			if readErr != nil {
				return "", nil, fmt.Errorf("read cypher statement: %w", readErr)
			}
			return "", nil, fmt.Errorf("cypher statement is required")
		}

		if strings.HasSuffix(line, ";") {
			line = strings.TrimRight(strings.TrimSuffix(line, ";"), " \t")
			if line != "" {
				lines = append(lines, line)
			}
			break
		}
		lines = append(lines, line)
	}

	stmt := strings.Join(lines, " ")
	if stmt == "" {
		return "", nil, fmt.Errorf("cypher statement is required")
	}

	ctx.IO.Write("Parameters (key=value ..., or blank to skip): ")
	paramStr, _ := ctx.IO.Read()
	paramStr = strings.TrimSpace(paramStr)

	if paramStr != "" {
		params = make(map[string]interface{})
		for _, token := range strings.Fields(paramStr) {
			if k, v, ok := splitKV(token); ok {
				params[k] = coerceParamValue(v)
			}
		}
	}

	return stmt, params, nil
}

// ---- Flag parsing -------------------------------------------------------

type cypherFlags struct {
	query  string
	params map[string]interface{}
	format string
	limit  int
}

func parseCypherFlags(args []string) cypherFlags {
	var opts cypherFlags
	var queryParts []string

	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--param" && i+1 < len(args):
			i++
			if k, v, ok := splitKV(args[i]); ok {
				if opts.params == nil {
					opts.params = make(map[string]interface{})
				}
				opts.params[k] = coerceParamValue(v)
			}
		case strings.HasPrefix(args[i], "--param="):
			kv := strings.TrimPrefix(args[i], "--param=")
			if k, v, ok := splitKV(kv); ok {
				if opts.params == nil {
					opts.params = make(map[string]interface{})
				}
				opts.params[k] = coerceParamValue(v)
			}
		case args[i] == "--format" && i+1 < len(args):
			i++
			opts.format = strings.ToLower(args[i])
		case strings.HasPrefix(args[i], "--format="):
			opts.format = strings.ToLower(strings.TrimPrefix(args[i], "--format="))
		case args[i] == "--limit" && i+1 < len(args):
			i++
			if n, err := strconv.Atoi(args[i]); err == nil && n > 0 {
				opts.limit = n
			}
		case strings.HasPrefix(args[i], "--limit="):
			s := strings.TrimPrefix(args[i], "--limit=")
			if n, err := strconv.Atoi(s); err == nil && n > 0 {
				opts.limit = n
			}
		default:
			queryParts = append(queryParts, args[i])
		}
	}

	opts.query = strings.Join(queryParts, " ")
	return opts
}

func splitKV(s string) (key, value string, ok bool) {
	k, v, found := strings.Cut(s, "=")
	return k, v, found
}

func coerceParamValue(s string) interface{} {
	if s == "true" {
		return true
	}
	if s == "false" {
		return false
	}
	if n, err := strconv.ParseInt(s, 10, 64); err == nil {
		return n
	}
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return f
	}
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}

// ---- LIMIT injection ----------------------------------------------------

func injectLimit(query string, n int) string {
	upper := strings.ToUpper(query)
	if !strings.Contains(upper, "RETURN") {
		return query
	}
	if limitPattern.MatchString(query) {
		return query
	}
	return fmt.Sprintf("%s LIMIT %d", query, n)
}
