package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/cli/go-cli-tool/internal/service"
	"github.com/cli/go-cli-tool/internal/shell"
)

// BuildCloudCategory returns the cloud top-level category with sub-categories
// for instances and projects.
//
//	neo4j> cloud instances list
//	neo4j> cloud instances get <id>
//	neo4j> cloud instances pause <id>
//	neo4j> cloud instances resume <id>
//	neo4j> cloud projects list
//	neo4j> cloud projects get <id>
func BuildCloudCategory(svc service.CloudService) *shell.Category {
	return shell.NewCategory("cloud", "Manage Neo4j Aura cloud resources").
		AddSubcategory(buildInstancesCategory(svc.Instances())).
		AddSubcategory(buildProjectsCategory(svc.Projects()))
}

func buildInstancesCategory(svc service.InstancesService) *shell.Category {
	return shell.NewCategory("instances", "Manage Aura DB instances").
		AddCommand(&shell.Command{
			Name:        "list",
			Usage:       "list",
			Description: "List all instances",
			Handler: func(args []string, ctx shell.ShellContext) (string, error) {
				instances, err := svc.List(context.Background())
				if err != nil {
					return "", err
				}
				if len(instances) == 0 {
					return "No instances found.", nil
				}
				var b strings.Builder
				fmt.Fprintf(&b, "%-36s  %-20s  %-10s  %-12s  %s\n",
					"ID", "Name", "Status", "Tier", "Region")
				fmt.Fprintln(&b, strings.Repeat("-", 96))
				for _, i := range instances {
					fmt.Fprintf(&b, "%-36s  %-20s  %-10s  %-12s  %s\n",
						i.ID, i.Name, i.Status, i.Tier, i.Region)
				}
				return strings.TrimRight(b.String(), "\n"), nil
			},
		}).
		AddCommand(&shell.Command{
			Name:        "get",
			Usage:       "get <id>",
			Description: "Get details for a specific instance",
			Handler: func(args []string, ctx shell.ShellContext) (string, error) {
				if len(args) == 0 {
					return "", fmt.Errorf("usage: cloud instances get <id>")
				}
				inst, err := svc.Get(context.Background(), args[0])
				if err != nil {
					return "", err
				}
				return fmt.Sprintf("ID:     %s\nName:   %s\nStatus: %s\nTier:   %s\nRegion: %s",
					inst.ID, inst.Name, inst.Status, inst.Tier, inst.Region), nil
			},
		}).
		AddCommand(&shell.Command{
			Name:        "pause",
			Usage:       "pause <id>",
			Description: "Pause a running instance",
			Handler: func(args []string, ctx shell.ShellContext) (string, error) {
				if len(args) == 0 {
					return "", fmt.Errorf("usage: cloud instances pause <id>")
				}
				if err := svc.Pause(context.Background(), args[0]); err != nil {
					return "", err
				}
				return fmt.Sprintf("Instance %s paused.", args[0]), nil
			},
		}).
		AddCommand(&shell.Command{
			Name:        "resume",
			Usage:       "resume <id>",
			Description: "Resume a paused instance",
			Handler: func(args []string, ctx shell.ShellContext) (string, error) {
				if len(args) == 0 {
					return "", fmt.Errorf("usage: cloud instances resume <id>")
				}
				if err := svc.Resume(context.Background(), args[0]); err != nil {
					return "", err
				}
				return fmt.Sprintf("Instance %s resumed.", args[0]), nil
			},
		}).
		AddCommand(&shell.Command{
			Name:        "delete",
			Usage:       "delete <id>",
			Description: "Permanently delete an instance",
			Handler: func(args []string, ctx shell.ShellContext) (string, error) {
				if len(args) == 0 {
					return "", fmt.Errorf("usage: cloud instances delete <id>")
				}
				if err := svc.Delete(context.Background(), args[0]); err != nil {
					return "", err
				}
				return fmt.Sprintf("Instance %s deleted.", args[0]), nil
			},
		})
}

func buildProjectsCategory(svc service.ProjectsService) *shell.Category {
	return shell.NewCategory("projects", "Manage Aura projects").
		AddCommand(&shell.Command{
			Name:        "list",
			Usage:       "list",
			Description: "List all projects",
			Handler: func(args []string, ctx shell.ShellContext) (string, error) {
				projects, err := svc.List(context.Background())
				if err != nil {
					return "", err
				}
				if len(projects) == 0 {
					return "No projects found.", nil
				}
				var b strings.Builder
				fmt.Fprintf(&b, "%-36s  %s\n", "ID", "Name")
				fmt.Fprintln(&b, strings.Repeat("-", 56))
				for _, p := range projects {
					fmt.Fprintf(&b, "%-36s  %s\n", p.ID, p.Name)
				}
				return strings.TrimRight(b.String(), "\n"), nil
			},
		}).
		AddCommand(&shell.Command{
			Name:        "get",
			Usage:       "get <id>",
			Description: "Get details for a specific project",
			Handler: func(args []string, ctx shell.ShellContext) (string, error) {
				if len(args) == 0 {
					return "", fmt.Errorf("usage: cloud projects get <id>")
				}
				proj, err := svc.Get(context.Background(), args[0])
				if err != nil {
					return "", err
				}
				return fmt.Sprintf("ID:   %s\nName: %s", proj.ID, proj.Name), nil
			},
		})
}
