package tools

import (
	"context"
	"fmt"
	"github.com/cli/go-cli-tool/internal/service"
	"github.com/cli/go-cli-tool/internal/tool"
)

type QueryTool struct {
	*tool.BaseTool
	service *service.GraphService
}

func NewQueryTool(svc *service.GraphService) *QueryTool {
	return &QueryTool{
		BaseTool: tool.NewBaseTool("query", "Execute a Cypher query", "1.0.0"),
		service:  svc,
	}
}

func (t *QueryTool) Execute(ctx tool.Context) (tool.Result, error) {
	if len(ctx.Args) == 0 {
		return tool.Result{Success: false, Output: "Usage: query <cypher>"}, nil
	}

	cypher := ctx.Args[0]
	result, err := t.service.RunQuery(context.Background(), cypher)
	if err != nil {
		return tool.Result{Success: false, Error: err}, err
	}

	output := ""
	if ctx.Presenter != nil {
		output, _ = ctx.Presenter.Format(result)
	} else {
		output = fmt.Sprintf("Query Result: %v", result)
	}

	return tool.Result{
		Success: true,
		Output:  output,
		Data:    map[string]interface{}{"result": result},
	}, nil
}
