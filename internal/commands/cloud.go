package commands

import (
	"fmt"
	"strings"

	"github.com/cli/go-cli-tool/internal/dispatch"
	"github.com/cli/go-cli-tool/internal/presentation"
	"github.com/cli/go-cli-tool/internal/service"
	"github.com/cli/go-cli-tool/internal/tool"
)

// BuildCloudCategory returns the cloud top-level category.
func BuildCloudCategory(svc service.CloudService) *dispatch.Category {
	return dispatch.NewCategory("cloud", "Manage Neo4j Aura cloud resources").
		AddSubcategory(buildInstancesCategory(svc)).
		AddSubcategory(buildProjectsCategory(svc))
}

// ---- Instances ----------------------------------------------------------

func buildInstancesCategory(svc service.CloudService) *dispatch.Category {
	return dispatch.NewCategory("instances", "Manage Aura DB instances").
		AddCommand(instanceListCmd(svc)).
		AddCommand(instanceGetCmd(svc)).
		AddCommand(instanceCreateCmd(svc)).
		AddCommand(instanceUpdateCmd(svc)).
		AddCommand(instancePauseCmd(svc)).
		AddCommand(instanceResumeCmd(svc)).
		AddCommand(instanceDeleteCmd(svc))
}

func instanceListCmd(svc service.CloudService) *dispatch.Command {
	return &dispatch.Command{
		Name:        "list",
		Aliases:     []string{"ls"},
		Usage:       "list",
		Description: "List all Aura instances",
		Handler: func(_ []string, ctx dispatch.Context) (dispatch.CommandResult, error) {
			instances, err := svc.Instances().List(ctx.Context)
			if err != nil {
				return dispatch.CommandResult{}, err
			}

			cols := []string{"ID", "Name", "Project", "Cloud"}
			rows := make([][]any, len(instances))
			items := make([]map[string]interface{}, len(instances))
			for i, inst := range instances {
				rows[i] = []any{inst.ID, inst.Name, orDash(inst.TenantID), orDash(inst.CloudProvider)}
				items[i] = map[string]interface{}{
					"id":             inst.ID,
					"name":           inst.Name,
					"tenant_id":      inst.TenantID,
					"cloud_provider": inst.CloudProvider,
				}
			}
			return dispatch.ListResult(presentation.NewTableData(cols, rows), items), nil
		},
	}
}

func instanceGetCmd(svc service.CloudService) *dispatch.Command {
	return &dispatch.Command{
		Name:        "get",
		Usage:       "get <id>",
		Description: "Show full details for an instance",
		Handler: func(args []string, ctx dispatch.Context) (dispatch.CommandResult, error) {
			if len(args) == 0 {
				return dispatch.CommandResult{}, fmt.Errorf("usage: cloud instances get <id>")
			}
			inst, err := svc.Instances().Get(ctx.Context, args[0])
			if err != nil {
				return dispatch.CommandResult{}, err
			}
			return dispatch.ItemResult(instanceToDetail(inst), instanceToMap(inst)), nil
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

// instanceToMap converts an Instance to a snake_case JSON map.
func instanceToMap(inst *service.Instance) map[string]interface{} {
	return map[string]interface{}{
		"id":             inst.ID,
		"name":           inst.Name,
		"status":         inst.Status,
		"type":           inst.Tier,
		"memory":         inst.Memory,
		"region":         inst.Region,
		"cloud_provider": inst.CloudProvider,
		"tenant_id":      inst.TenantID,
		"connection_url": inst.ConnectionURL,
	}
}

func instanceCreateCmd(svc service.CloudService) *dispatch.Command {
	return &dispatch.Command{
		Name:         "create",
		MutationMode: tool.ModeWrite,
		Usage:        "create name=<n> [tenant=<id>] [cloud=<provider>] [region=<r>] [type=<t>] [version=<v>] [memory=<size>]",
		Description:  "Create a new Aura instance. Unset fields fall back to aura.instance_defaults in your config.",
		Handler: func(args []string, ctx dispatch.Context) (dispatch.CommandResult, error) {
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
				return dispatch.CommandResult{}, fmt.Errorf(
					"name is required\n  usage: cloud instances create name=<n> [tenant=<id>] [cloud=<provider>] [region=<r>] [type=<t>] [version=<v>] [memory=<size>]")
			}
			if params.TenantID == "" {
				return dispatch.CommandResult{}, fmt.Errorf(
					"tenant is required — provide tenant=<id> or set aura.instance_defaults.tenant_id in your config")
			}

			created, err := svc.Instances().Create(ctx.Context, params)
			if err != nil {
				return dispatch.CommandResult{}, err
			}

			if created.Password != "" {
				ctx.IO.Write("⚠  Save this password now — it will NOT be shown again.\n")
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

			item := map[string]interface{}{
				"id":             created.ID,
				"name":           created.Name,
				"status":         created.Status,
				"type":           created.Tier,
				"memory":         created.Memory,
				"region":         created.Region,
				"cloud_provider": created.CloudProvider,
				"tenant_id":      created.TenantID,
				"connection_url": created.ConnectionURL,
				"username":       created.Username,
				"password":       created.Password,
			}
			return dispatch.ItemResult(detail, item), nil
		},
	}
}

func instanceUpdateCmd(svc service.CloudService) *dispatch.Command {
	return &dispatch.Command{
		Name:         "update",
		MutationMode: tool.ModeWrite,
		Usage:        "update <id> [name=<new-name>] [memory=<size>]",
		Description:  "Rename or resize an existing instance",
		Handler: func(args []string, ctx dispatch.Context) (dispatch.CommandResult, error) {
			if len(args) == 0 {
				return dispatch.CommandResult{}, fmt.Errorf("usage: cloud instances update <id> [name=<new-name>] [memory=<size>]")
			}
			id := args[0]
			kv := parseKV(args[1:])

			params := &service.UpdateInstanceParams{
				Name:   kvGet(kv, "name", ""),
				Memory: kvGet(kv, "memory", ""),
			}
			if params.Name == "" && params.Memory == "" {
				return dispatch.CommandResult{}, fmt.Errorf("provide at least one of: name=<n>, memory=<size>")
			}

			updated, err := svc.Instances().Update(ctx.Context, id, params)
			if err != nil {
				return dispatch.CommandResult{}, err
			}

			detail := presentation.NewDetailData("Instance updated", []presentation.DetailField{
				{Label: "ID", Value: updated.ID},
				{Label: "Name", Value: updated.Name},
				{Label: "Status", Value: orDash(updated.Status)},
				{Label: "Memory", Value: orDash(updated.Memory)},
			})
			item := map[string]interface{}{
				"id":     updated.ID,
				"name":   updated.Name,
				"status": updated.Status,
				"memory": updated.Memory,
			}
			return dispatch.ItemResult(detail, item), nil
		},
	}
}

func instancePauseCmd(svc service.CloudService) *dispatch.Command {
	return &dispatch.Command{
		Name:         "pause",
		MutationMode: tool.ModeWrite,
		Usage:        "pause <id>",
		Description:  "Pause a running instance",
		Handler: func(args []string, ctx dispatch.Context) (dispatch.CommandResult, error) {
			if len(args) == 0 {
				return dispatch.CommandResult{}, fmt.Errorf("usage: cloud instances pause <id>")
			}
			if err := svc.Instances().Pause(ctx.Context, args[0]); err != nil {
				return dispatch.CommandResult{}, err
			}
			return dispatch.ItemResult(nil, map[string]interface{}{
				"id":      args[0],
				"status":  "pausing",
				"message": fmt.Sprintf("Instance %s is pausing.", args[0]),
			}), nil
		},
	}
}

func instanceResumeCmd(svc service.CloudService) *dispatch.Command {
	return &dispatch.Command{
		Name:         "resume",
		MutationMode: tool.ModeWrite,
		Usage:        "resume <id>",
		Description:  "Resume a paused instance",
		Handler: func(args []string, ctx dispatch.Context) (dispatch.CommandResult, error) {
			if len(args) == 0 {
				return dispatch.CommandResult{}, fmt.Errorf("usage: cloud instances resume <id>")
			}
			if err := svc.Instances().Resume(ctx.Context, args[0]); err != nil {
				return dispatch.CommandResult{}, err
			}
			return dispatch.ItemResult(nil, map[string]interface{}{
				"id":      args[0],
				"status":  "resuming",
				"message": fmt.Sprintf("Instance %s is resuming.", args[0]),
			}), nil
		},
	}
}

func instanceDeleteCmd(svc service.CloudService) *dispatch.Command {
	return &dispatch.Command{
		Name:         "delete",
		Aliases:      []string{"rm"},
		MutationMode: tool.ModeWrite,
		Usage:        "delete <id>",
		Description:  "Permanently delete an instance (prompts for confirmation outside agent mode)",
		Handler: func(args []string, ctx dispatch.Context) (dispatch.CommandResult, error) {
			if len(args) == 0 {
				return dispatch.CommandResult{}, fmt.Errorf("usage: cloud instances delete <id>")
			}
			id := args[0]
			// In agent mode the dispatcher has already enforced --rw, so skip
			// the interactive confirmation. In human mode, require explicit "yes".
			if !ctx.AgentMode {
				ctx.IO.Write("Permanently delete instance %s? Type 'yes' to confirm: ", id)
				confirm, err := ctx.IO.Read()
				if err != nil {
					return dispatch.CommandResult{}, fmt.Errorf("read confirmation: %w", err)
				}
				if strings.TrimSpace(confirm) != "yes" {
					return dispatch.MessageResult("Delete cancelled."), nil
				}
			}
			if err := svc.Instances().Delete(ctx.Context, id); err != nil {
				return dispatch.CommandResult{}, err
			}
			return dispatch.ItemResult(nil, map[string]interface{}{
				"id":      id,
				"status":  "deleted",
				"message": fmt.Sprintf("Instance %s deleted.", id),
			}), nil
		},
	}
}

// ---- Projects -----------------------------------------------------------

func buildProjectsCategory(svc service.CloudService) *dispatch.Category {
	return dispatch.NewCategory("projects", "Manage Aura projects / tenants").
		AddCommand(&dispatch.Command{
			Name:        "list",
			Aliases:     []string{"ls"},
			Usage:       "list",
			Description: "List all projects",
			Handler: func(_ []string, ctx dispatch.Context) (dispatch.CommandResult, error) {
				projects, err := svc.Projects().List(ctx.Context)
				if err != nil {
					return dispatch.CommandResult{}, err
				}
				rows := make([][]interface{}, len(projects))
				items := make([]map[string]interface{}, len(projects))
				for i, p := range projects {
					rows[i] = []interface{}{p.ID, p.Name}
					items[i] = map[string]interface{}{"id": p.ID, "name": p.Name}
				}
				return dispatch.ListResult(
					presentation.NewTableData([]string{"ID", "Name"}, rows),
					items,
				), nil
			},
		}).
		AddCommand(&dispatch.Command{
			Name:        "get",
			Usage:       "get <id>",
			Description: "Show details for a project",
			Handler: func(args []string, ctx dispatch.Context) (dispatch.CommandResult, error) {
				if len(args) == 0 {
					return dispatch.CommandResult{}, fmt.Errorf("usage: cloud projects get <id>")
				}
				proj, err := svc.Projects().Get(ctx.Context, args[0])
				if err != nil {
					return dispatch.CommandResult{}, err
				}
				detail := presentation.NewDetailData("Project", []presentation.DetailField{
					{Label: "ID", Value: proj.ID},
					{Label: "Name", Value: proj.Name},
				})
				item := map[string]interface{}{"id": proj.ID, "name": proj.Name}
				return dispatch.ItemResult(detail, item), nil
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
