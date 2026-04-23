package tools

import (
	"fmt"

	"github.com/cli/go-cli-tool/internal/service"
	"github.com/cli/go-cli-tool/internal/tool"
)

// QueryTool executes a Cypher query via CypherService and prints the result.
// It uses the same service as the 'cypher' shell category so there is a
// single code path for query execution regardless of how it is invoked.
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

func (t *QueryTool) Execute(ctx tool.Context) (tool.Result, error) {
	if len(ctx.Args) == 0 {
		return tool.ErrorResult("usage: query <cypher>"), fmt.Errorf("no query provided")
	}

	output, err := t.svc.Execute(ctx.Context, ctx.Args[0])
	if err != nil {
		return tool.Result{}, err
	}

	if ctx.Presenter != nil {
		if formatted, fmtErr := ctx.Presenter.Format(output); fmtErr == nil {
			output = formatted
		}
	}

	return tool.SuccessResult(output), nil
}
