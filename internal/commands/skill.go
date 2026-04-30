package commands

import (
	"fmt"

	"github.com/cli/go-cli-tool/internal/dispatch"
	"github.com/cli/go-cli-tool/internal/presentation"
	"github.com/cli/go-cli-tool/internal/service"
	"github.com/cli/go-cli-tool/internal/tool"
)

// BuildSkillCategory returns the skill top-level category. The category has
// no prerequisite — install/remove/list operate purely on the local
// filesystem and never touch Neo4j or Aura.
func BuildSkillCategory(svc service.SkillService) *dispatch.Category {
	return dispatch.NewCategory("skill", "Manage the embedded neo4j-cli SKILL.md across supported AI agents").
		AddCommand(skillInstallCmd(svc)).
		AddCommand(skillRemoveCmd(svc)).
		AddCommand(skillListCmd(svc))
}

// ---- skill install ------------------------------------------------------

func skillInstallCmd(svc service.SkillService) *dispatch.Command {
	return &dispatch.Command{
		Name:         "install",
		MutationMode: tool.ModeWrite,
		Usage:        "install [agent]",
		Description:  "Install the embedded SKILL.md into one agent (or every detected agent when omitted)",
		Handler: func(args []string, ctx dispatch.Context) (dispatch.CommandResult, error) {
			agentName := ""
			if len(args) > 0 {
				agentName = args[0]
			}
			results, err := svc.Install(ctx.Context, agentName)
			if err != nil {
				return dispatch.CommandResult{}, fmt.Errorf("install skill: %w", err)
			}
			cols := []string{"Agent", "Path"}
			rows := make([][]interface{}, len(results))
			items := make([]map[string]interface{}, len(results))
			for i, r := range results {
				rows[i] = []interface{}{r.Agent, r.Path}
				items[i] = map[string]interface{}{
					"agent": r.Agent,
					"path":  r.Path,
				}
			}
			return dispatch.ListResult(presentation.NewTableData(cols, rows), items), nil
		},
	}
}

// ---- skill remove -------------------------------------------------------

func skillRemoveCmd(svc service.SkillService) *dispatch.Command {
	return &dispatch.Command{
		Name:         "remove",
		MutationMode: tool.ModeWrite,
		Usage:        "remove [agent]",
		Description:  "Remove the SKILL.md from one agent (or every agent that has it when omitted)",
		Handler: func(args []string, ctx dispatch.Context) (dispatch.CommandResult, error) {
			agentName := ""
			if len(args) > 0 {
				agentName = args[0]
			}
			results, err := svc.Remove(ctx.Context, agentName)
			if err != nil {
				return dispatch.CommandResult{}, fmt.Errorf("remove skill: %w", err)
			}
			cols := []string{"Agent"}
			rows := make([][]interface{}, len(results))
			items := make([]map[string]interface{}, len(results))
			for i, r := range results {
				rows[i] = []interface{}{r.Agent}
				items[i] = map[string]interface{}{
					"agent": r.Agent,
				}
			}
			return dispatch.ListResult(presentation.NewTableData(cols, rows), items), nil
		},
	}
}

// ---- skill list ---------------------------------------------------------

func skillListCmd(svc service.SkillService) *dispatch.Command {
	return &dispatch.Command{
		Name:        "list",
		Aliases:     []string{"ls"},
		Usage:       "list",
		Description: "Show every supported agent with its detected/installed status",
		Handler: func(_ []string, ctx dispatch.Context) (dispatch.CommandResult, error) {
			statuses, err := svc.List(ctx.Context)
			if err != nil {
				return dispatch.CommandResult{}, fmt.Errorf("list skills: %w", err)
			}
			cols := []string{"Name", "Display Name", "Detected", "Installed"}
			rows := make([][]interface{}, len(statuses))
			items := make([]map[string]interface{}, len(statuses))
			for i, s := range statuses {
				rows[i] = []interface{}{s.Name, s.DisplayName, boolYesNo(s.Detected), boolYesNo(s.Installed)}
				items[i] = map[string]interface{}{
					"name":         s.Name,
					"display_name": s.DisplayName,
					"detected":     s.Detected,
					"installed":    s.Installed,
				}
			}
			return dispatch.ListResult(presentation.NewTableData(cols, rows), items), nil
		},
	}
}

// boolYesNo formats a boolean for human-mode tables. Yes/No is more scannable
// than true/false for status columns.
func boolYesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}
