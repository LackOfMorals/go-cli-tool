package commands

import (
	"fmt"
	"strings"

	"github.com/cli/go-cli-tool/internal/presentation"
	"github.com/cli/go-cli-tool/internal/service"
	"github.com/cli/go-cli-tool/internal/shell"
)

// BuildAdminCategory returns the admin top-level category.
//
//	neo4j> admin show-users
//	neo4j> admin show-databases
func BuildAdminCategory(svc service.AdminService) *shell.Category {
	return shell.NewCategory("admin", "Administrative operations against the connected Neo4j database").
		AddCommand(&shell.Command{
			Name:        "show-users",
			Usage:       "show-users",
			Description: "List all database users and their roles",
			Handler: func(args []string, ctx shell.ShellContext) (string, error) {
				users, err := svc.ShowUsers(ctx.Context)
				if err != nil {
					return "", err
				}
				if len(users) == 0 {
					return "No users found.", nil
				}
				rows := make([][]interface{}, len(users))
				for i, u := range users {
					rows[i] = []interface{}{u.Username, strings.Join(u.Roles, ", ")}
				}
				return ctx.Presenter.Format(presentation.NewTableData(
					[]string{"Username", "Roles"}, rows,
				))
			},
		}).
		AddCommand(&shell.Command{
			Name:        "show-databases",
			Usage:       "show-databases",
			Description: "List all databases and their current status",
			Handler: func(args []string, ctx shell.ShellContext) (string, error) {
				dbs, err := svc.ShowDatabases(ctx.Context)
				if err != nil {
					return "", err
				}
				if len(dbs) == 0 {
					return "No databases found.", nil
				}
				rows := make([][]interface{}, len(dbs))
				for i, db := range dbs {
					rows[i] = []interface{}{db.Name, db.Status}
				}
				return ctx.Presenter.Format(presentation.NewTableData(
					[]string{"Name", "Status"}, rows,
				))
			},
		})
}

// formatInstance is a shared helper for building instance detail output.
// Used by instanceGetCmd and instanceCreateCmd.
func formatInstance(inst *service.Instance) string {
	var b strings.Builder
	row := func(label, value string) {
		fmt.Fprintf(&b, "  %-16s  %s\n", label+":", value)
	}
	fmt.Fprintln(&b, strings.Repeat("─", 48))
	row("ID", inst.ID)
	row("Name", inst.Name)
	row("Status", orDash(inst.Status))
	row("Type", orDash(inst.Tier))
	row("Memory", orDash(inst.Memory))
	row("Region", orDash(inst.Region))
	row("Cloud", orDash(inst.CloudProvider))
	row("Tenant ID", orDash(inst.TenantID))
	row("Connection URL", orDash(inst.ConnectionURL))
	return strings.TrimRight(b.String(), "\n")
}
