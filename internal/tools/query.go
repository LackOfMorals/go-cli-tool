package tools

import (
	"fmt"
	"strings"

	"github.com/cli/go-cli-tool/internal/presentation"
	"github.com/cli/go-cli-tool/internal/service"
	"github.com/cli/go-cli-tool/internal/tool"
)

// QueryTool executes a Cypher query via CypherService and prints the result.
// In non-interactive (--exec) mode the exec_limit from config is applied.
type QueryTool struct {
	*tool.BaseTool
	svc service.CypherService
}

func NewQueryTool(svc service.CypherService) *QueryTool {
	return &QueryTool{
		BaseTool: tool.NewBaseTool("query", "Execute a Cypher query", "1.0.0"),
		svc:      svc,
	}
}

// MutationMode returns ModeConditional because whether a Cypher statement
// modifies state depends on the query content. The dispatcher will run an
// EXPLAIN pre-check in agent mode without --rw rather than blocking blindly.
func (t *QueryTool) MutationMode() tool.MutationMode { return tool.ModeConditional }

func (t *QueryTool) Execute(ctx tool.Context) (tool.Result, error) {
	if len(ctx.Args) == 0 {
		return tool.ErrorResult("usage: query <cypher>"), fmt.Errorf("no query provided")
	}

	query := ctx.Args[0]

	// Apply exec-mode LIMIT if not already present and query has RETURN.
	execLimit := 100
	if lim := ctx.GetConfigInt("cypher.exec_limit", 0); lim > 0 {
		execLimit = lim
	}
	upper := strings.ToUpper(query)
	if strings.Contains(upper, "RETURN") && !strings.Contains(upper, "LIMIT") {
		query = fmt.Sprintf("%s LIMIT %d", query, execLimit)
	}

	result, err := t.svc.Execute(ctx.Context, query, nil)
	if err != nil {
		return tool.Result{}, err
	}

	data := serviceQueryResultToTableData(result)

	if ctx.Presenter != nil {
		output, fmtErr := ctx.Presenter.Format(data)
		if fmtErr == nil {
			return tool.SuccessResult(output), nil
		}
	}

	// Fallback: plain aligned text when no presenter is available.
	return tool.SuccessResult(fallbackRender(result)), nil
}

// serviceQueryResultToTableData converts a service.QueryResult to a
// presentation.TableData suitable for passing to PresentationService.Format.
func serviceQueryResultToTableData(r service.QueryResult) *presentation.TableData {
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

// fallbackRender produces plain aligned text when no presenter is wired.
func fallbackRender(r service.QueryResult) string {
	if len(r.Columns) == 0 || len(r.Rows) == 0 {
		return "(no results)"
	}
	widths := make([]int, len(r.Columns))
	for i, col := range r.Columns {
		widths[i] = len(col)
	}
	for _, row := range r.Rows {
		for i, col := range r.Columns {
			if cell := fmt.Sprintf("%v", row[col]); len(cell) > widths[i] {
				widths[i] = len(cell)
			}
		}
	}
	var b strings.Builder
	for i, col := range r.Columns {
		if i > 0 {
			b.WriteString("  ")
		}
		fmt.Fprintf(&b, "%-*s", widths[i], col)
	}
	b.WriteByte('\n')
	for i, w := range widths {
		if i > 0 {
			b.WriteString("  ")
		}
		b.WriteString(strings.Repeat("-", w))
	}
	b.WriteByte('\n')
	for _, row := range r.Rows {
		for i, col := range r.Columns {
			if i > 0 {
				b.WriteString("  ")
			}
			fmt.Fprintf(&b, "%-*s", widths[i], fmt.Sprintf("%v", row[col]))
		}
		b.WriteByte('\n')
	}
	return strings.TrimRight(b.String(), "\n")
}
