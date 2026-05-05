package commands

import (
	"strings"

	"github.com/cli/go-cli-tool/internal/dispatch"
	"github.com/cli/go-cli-tool/internal/presentation"
	"github.com/cli/go-cli-tool/internal/service"
)

// BuildAdminCategory returns the admin top-level category.
func BuildAdminCategory(svc service.AdminService) *dispatch.Category {
	return dispatch.NewCategory("admin", "Administrative operations against the connected Neo4j database").
		AddCommand(&dispatch.Command{
			Name:        "show-users",
			Usage:       "show-users",
			Description: "List all database users and their roles",
			Handler: func(_ []string, ctx dispatch.Context) (dispatch.CommandResult, error) {
				users, err := svc.ShowUsers(ctx.Context)
				if err != nil {
					return dispatch.CommandResult{}, err
				}
				rows := make([][]interface{}, len(users))
				items := make([]map[string]interface{}, len(users))
				for i, u := range users {
					roles := strings.Join(u.Roles, ", ")
					rows[i] = []interface{}{u.Username, roles}
					items[i] = map[string]interface{}{
						"username": u.Username,
						"roles":    u.Roles,
					}
				}
				return dispatch.ListResult(
					presentation.NewTableData([]string{"Username", "Roles"}, rows),
					items,
				), nil
			},
		}).
		AddCommand(&dispatch.Command{
			Name:        "show-databases",
			Usage:       "show-databases",
			Description: "List all databases and their current status",
			Handler: func(_ []string, ctx dispatch.Context) (dispatch.CommandResult, error) {
				dbs, err := svc.ShowDatabases(ctx.Context)
				if err != nil {
					return dispatch.CommandResult{}, err
				}
				rows := make([][]interface{}, len(dbs))
				items := make([]map[string]interface{}, len(dbs))
				for i, db := range dbs {
					rows[i] = []interface{}{db.Name, db.Status}
					items[i] = map[string]interface{}{
						"name":   db.Name,
						"status": db.Status,
					}
				}
				return dispatch.ListResult(
					presentation.NewTableData([]string{"Name", "Status"}, rows),
					items,
				), nil
			},
		})
}
