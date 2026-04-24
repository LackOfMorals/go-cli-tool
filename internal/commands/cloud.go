package commands

import (
	"fmt"
	"strings"

	"github.com/cli/go-cli-tool/internal/presentation"
	"github.com/cli/go-cli-tool/internal/service"
	"github.com/cli/go-cli-tool/internal/shell"
)

// BuildCloudCategory returns the cloud top-level category.
//
//	neo4j> cloud instances list
//	neo4j> cloud instances get <id>
//	neo4j> cloud instances create name=<n> [tenant=<id>] [cloud=<provider>] [region=<r>] [type=<t>] [version=<v>] [memory=<size>]
//	neo4j> cloud instances update <id> [name=<n>] [memory=<size>]
//	neo4j> cloud instances pause <id>
//	neo4j> cloud instances resume <id>
//	neo4j> cloud instances delete <id>
//	neo4j> cloud projects list
//	neo4j> cloud projects get <id>
func BuildCloudCategory(svc service.CloudService) *shell.Category {
	return shell.NewCategory("cloud", "Manage Neo4j Aura cloud resources").
		AddSubcategory(buildInstancesCategory(svc)).
		AddSubcategory(buildProjectsCategory(svc))
}

// ---- Instances ----------------------------------------------------------

func buildInstancesCategory(svc service.CloudService) *shell.Category {
	return shell.NewCategory("instances", "Manage Aura DB instances").
		AddCommand(instanceListCmd(svc)).
		AddCommand(instanceGetCmd(svc)).
		AddCommand(instanceCreateCmd(svc)).
		AddCommand(instanceUpdateCmd(svc)).
		AddCommand(instancePauseCmd(svc)).
		AddCommand(instanceResumeCmd(svc)).
		AddCommand(instanceDeleteCmd(svc))
}

func instanceListCmd(svc service.CloudService) *shell.Command {
	return &shell.Command{
		Name:        "list",
		Aliases:     []string{"ls"},
		Usage:       "list",
		Description: "List all Aura instances",
		Handler: func(args []string, ctx shell.ShellContext) (string, error) {
			instances, err := svc.Instances().List(ctx.Context)
			if err != nil {
				return "", err
			}
			if len(instances) == 0 {
				return "No instances found.", nil
			}

			cols := []string{"ID", "Name", "Project", "Cloud"}
			rows := make([][]any, len(instances))
			for i, inst := range instances {
				rows[i] = []any{
					inst.ID, inst.Name,
					orDash(inst.TenantID),
					orDash(inst.CloudProvider),
				}
			}
			return ctx.Presenter.Format(presentation.NewTableData(cols, rows))
		},
	}
}

func instanceGetCmd(svc service.CloudService) *shell.Command {
	return &shell.Command{
		Name:        "get",
		Usage:       "get <id>",
		Description: "Show full details for an instance",
		Handler: func(args []string, ctx shell.ShellContext) (string, error) {
			if len(args) == 0 {
				return "", fmt.Errorf("usage: cloud instances get <id>")
			}
			inst, err := svc.Instances().Get(ctx.Context, args[0])
			if err != nil {
				return "", err
			}
			return ctx.Presenter.Format(instanceToDetail(inst))
		},
	}
}

func instanceToDetail(inst *service.Instance) *presentation.DetailData {
	return presentation.NewDetailData("Instance", []presentation.DetailField{
		{Label: "ID", Value: inst.ID},
		{Label: "Name", Value: inst.Name},
		{Label: "Status", Value: orDash(inst.Status)},
		{Label: "Type", Value: orDash(inst.Tier)},
		{Label: "Memory", Value: orDash(inst.Memory)},
		{Label: "Region", Value: orDash(inst.Region)},
		{Label: "Cloud", Value: orDash(inst.CloudProvider)},
		{Label: "Tenant ID", Value: orDash(inst.TenantID)},
		{Label: "Connection URL", Value: orDash(inst.ConnectionURL)},
	})
}

func instanceCreateCmd(svc service.CloudService) *shell.Command {
	return &shell.Command{
		Name:  "create",
		Usage: "create name=<n> [tenant=<id>] [cloud=<provider>] [region=<r>] [type=<t>] [version=<v>] [memory=<size>]",
		Description: "Create a new Aura instance. " +
			"Unset fields fall back to aura.instance_defaults in your config.",
		Handler: func(args []string, ctx shell.ShellContext) (string, error) {
			kv := parseKV(args)
			d := ctx.Config.Aura.InstanceDefaults

			params := &service.CreateInstanceParams{
				Name:          kvGet(kv, "name", ""),
				TenantID:      kvGet(kv, "tenant", d.TenantID),
				CloudProvider: kvGet(kv, "cloud", d.CloudProvider),
				Region:        kvGet(kv, "region", d.Region),
				Type:          kvGet(kv, "type", d.Type),
				Version:       kvGet(kv, "version", d.Version),
				Memory:        kvGet(kv, "memory", d.Memory),
			}

			if params.Name == "" {
				return "", fmt.Errorf(
					"name is required\n" +
						"  usage: cloud instances create name=<n> [tenant=<id>] [cloud=<provider>] [region=<r>] [type=<t>] [version=<v>] [memory=<size>]")
			}
			if params.TenantID == "" {
				return "", fmt.Errorf(
					"tenant is required — provide tenant=<id> or set aura.instance_defaults.tenant_id in your config")
			}

			created, err := svc.Instances().Create(ctx.Context, params)
			if err != nil {
				return "", err
			}

			detail := presentation.NewDetailData("Instance created", []presentation.DetailField{
				{Label: "ID", Value: created.ID},
				{Label: "Name", Value: created.Name},
				{Label: "Cloud", Value: orDash(created.CloudProvider)},
				{Label: "Region", Value: orDash(created.Region)},
				{Label: "Type", Value: orDash(created.Tier)},
				{Label: "Tenant ID", Value: orDash(created.TenantID)},
				{Label: "Connection URL", Value: orDash(created.ConnectionURL)},
				{Label: "Username", Value: orDash(created.Username)},
				{Label: "Password", Value: orDash(created.Password)},
			})

			if created.Password != "" {
				ctx.IO.Write("⚠  Save this password now — it will NOT be shown again.\n")
			}

			return ctx.Presenter.Format(detail)
		},
	}
}

func instanceUpdateCmd(svc service.CloudService) *shell.Command {
	return &shell.Command{
		Name:        "update",
		Usage:       "update <id> [name=<new-name>] [memory=<size>]",
		Description: "Rename or resize an existing instance",
		Handler: func(args []string, ctx shell.ShellContext) (string, error) {
			if len(args) == 0 {
				return "", fmt.Errorf("usage: cloud instances update <id> [name=<new-name>] [memory=<size>]")
			}
			id := args[0]
			kv := parseKV(args[1:])

			params := &service.UpdateInstanceParams{
				Name:   kvGet(kv, "name", ""),
				Memory: kvGet(kv, "memory", ""),
			}
			if params.Name == "" && params.Memory == "" {
				return "", fmt.Errorf("provide at least one of: name=<n>, memory=<size>")
			}

			updated, err := svc.Instances().Update(ctx.Context, id, params)
			if err != nil {
				return "", err
			}
			return ctx.Presenter.Format(presentation.NewDetailData("Instance updated", []presentation.DetailField{
				{Label: "ID", Value: updated.ID},
				{Label: "Name", Value: updated.Name},
				{Label: "Status", Value: orDash(updated.Status)},
				{Label: "Memory", Value: orDash(updated.Memory)},
			}))
		},
	}
}

func instancePauseCmd(svc service.CloudService) *shell.Command {
	return &shell.Command{
		Name:        "pause",
		Usage:       "pause <id>",
		Description: "Pause a running instance",
		Handler: func(args []string, ctx shell.ShellContext) (string, error) {
			if len(args) == 0 {
				return "", fmt.Errorf("usage: cloud instances pause <id>")
			}
			if err := svc.Instances().Pause(ctx.Context, args[0]); err != nil {
				return "", err
			}
			return fmt.Sprintf("✓ Instance %s is pausing.", args[0]), nil
		},
	}
}

func instanceResumeCmd(svc service.CloudService) *shell.Command {
	return &shell.Command{
		Name:        "resume",
		Usage:       "resume <id>",
		Description: "Resume a paused instance",
		Handler: func(args []string, ctx shell.ShellContext) (string, error) {
			if len(args) == 0 {
				return "", fmt.Errorf("usage: cloud instances resume <id>")
			}
			if err := svc.Instances().Resume(ctx.Context, args[0]); err != nil {
				return "", err
			}
			return fmt.Sprintf("✓ Instance %s is resuming.", args[0]), nil
		},
	}
}

func instanceDeleteCmd(svc service.CloudService) *shell.Command {
	return &shell.Command{
		Name:        "delete",
		Aliases:     []string{"rm"},
		Usage:       "delete <id>",
		Description: "Permanently delete an instance (prompts for confirmation)",
		Handler: func(args []string, ctx shell.ShellContext) (string, error) {
			if len(args) == 0 {
				return "", fmt.Errorf("usage: cloud instances delete <id>")
			}
			id := args[0]
			ctx.IO.Write("Permanently delete instance %s? Type 'yes' to confirm: ", id)
			confirm, err := ctx.IO.Read()
			if err != nil {
				return "", fmt.Errorf("read confirmation: %w", err)
			}
			if strings.TrimSpace(confirm) != "yes" {
				return "Delete cancelled.", nil
			}
			if err := svc.Instances().Delete(ctx.Context, id); err != nil {
				return "", err
			}
			return fmt.Sprintf("✓ Instance %s deleted.", id), nil
		},
	}
}

// ---- Projects -----------------------------------------------------------

func buildProjectsCategory(svc service.CloudService) *shell.Category {
	return shell.NewCategory("projects", "Manage Aura projects / tenants").
		AddCommand(&shell.Command{
			Name:        "list",
			Aliases:     []string{"ls"},
			Usage:       "list",
			Description: "List all projects",
			Handler: func(args []string, ctx shell.ShellContext) (string, error) {
				projects, err := svc.Projects().List(ctx.Context)
				if err != nil {
					return "", err
				}
				if len(projects) == 0 {
					return "No projects found.", nil
				}
				rows := make([][]interface{}, len(projects))
				for i, p := range projects {
					rows[i] = []interface{}{p.ID, p.Name}
				}
				return ctx.Presenter.Format(presentation.NewTableData(
					[]string{"ID", "Name"}, rows,
				))
			},
		}).
		AddCommand(&shell.Command{
			Name:        "get",
			Usage:       "get <id>",
			Description: "Show details for a project",
			Handler: func(args []string, ctx shell.ShellContext) (string, error) {
				if len(args) == 0 {
					return "", fmt.Errorf("usage: cloud projects get <id>")
				}
				proj, err := svc.Projects().Get(ctx.Context, args[0])
				if err != nil {
					return "", err
				}
				return ctx.Presenter.Format(presentation.NewDetailData("Project", []presentation.DetailField{
					{Label: "ID", Value: proj.ID},
					{Label: "Name", Value: proj.Name},
				}))
			},
		})
}

// ---- helpers ------------------------------------------------------------

func parseKV(args []string) map[string]string {
	out := make(map[string]string, len(args))
	for _, a := range args {
		if k, v, ok := strings.Cut(a, "="); ok {
			out[strings.ToLower(k)] = v
		}
	}
	return out
}

func kvGet(m map[string]string, key, def string) string {
	if v, ok := m[key]; ok && v != "" {
		return v
	}
	return def
}

func orDash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}
