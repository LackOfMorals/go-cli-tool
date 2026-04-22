package commands

import (
	"context"
	"fmt"
	"strings"

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
				users, err := svc.ShowUsers(context.Background())
				if err != nil {
					return "", err
				}
				if len(users) == 0 {
					return "No users found.", nil
				}
				var b strings.Builder
				fmt.Fprintf(&b, "%-20s  %s\n", "Username", "Roles")
				fmt.Fprintln(&b, strings.Repeat("-", 50))
				for _, u := range users {
					fmt.Fprintf(&b, "%-20s  %s\n", u.Username, strings.Join(u.Roles, ", "))
				}
				return strings.TrimRight(b.String(), "\n"), nil
			},
		}).
		AddCommand(&shell.Command{
			Name:        "show-databases",
			Usage:       "show-databases",
			Description: "List all databases and their current status",
			Handler: func(args []string, ctx shell.ShellContext) (string, error) {
				dbs, err := svc.ShowDatabases(context.Background())
				if err != nil {
					return "", err
				}
				if len(dbs) == 0 {
					return "No databases found.", nil
				}
				var b strings.Builder
				fmt.Fprintf(&b, "%-20s  %s\n", "Name", "Status")
				fmt.Fprintln(&b, strings.Repeat("-", 32))
				for _, db := range dbs {
					fmt.Fprintf(&b, "%-20s  %s\n", db.Name, db.Status)
				}
				return strings.TrimRight(b.String(), "\n"), nil
			},
		})
}
