package cli

import (
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

const rolesListCommandUse = "list"

func newRolesCommand(deps commandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "roles",
		Short: "Inspect effective background role configuration",
	}
	cmd.AddCommand(newRolesListCommand(deps))
	cmd.AddCommand(newRolesShowCommand(deps))
	return cmd
}

func newRolesListCommand(deps commandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   rolesListCommandUse,
		Short: "List effective background roles",
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			query, err := roleQueryFromCommand(cmd)
			if err != nil {
				return err
			}
			roles, err := client.ListRoles(cmd.Context(), query)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, roleListBundle(roles))
		},
	}
	cmd.Flags().String("workspace", "", "Resolve roles from a workspace id, name, or path")
	return cmd
}

func newRolesShowCommand(deps commandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show <role>",
		Short: "Show one effective background role",
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			query, err := roleQueryFromCommand(cmd)
			if err != nil {
				return err
			}
			role, err := client.GetRole(cmd.Context(), args[0], query)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, roleBundle(role))
		},
	}
	cmd.Flags().String("workspace", "", "Resolve the role from a workspace id, name, or path")
	return cmd
}

func roleQueryFromCommand(cmd *cobra.Command) (RoleQuery, error) {
	workspace, err := commandWorkspaceFlag(cmd)
	if err != nil {
		return RoleQuery{}, err
	}
	return RoleQuery{Workspace: workspace}, nil
}

func roleListBundle(roles []RoleRecord) outputBundle {
	return listBundle(
		roles,
		roles,
		"Roles",
		[]string{
			roleOutputLabel,
			"Enabled",
			"Resolution",
			agentOutputLabel,
			agentKernelProviderValue,
			agentKernelModelValue,
			"Diagnostics",
		},
		"roles",
		[]string{
			"role",
			"enabled",
			"resolution_mode",
			agentAgentKey,
			cliProviderKey,
			agentModelKey,
			"diagnostic_count",
		},
		roleListValues,
		roleListValues,
	)
}

func roleListValues(role RoleRecord) []string {
	return []string{
		role.Role,
		strconv.FormatBool(role.Enabled),
		string(role.ResolutionMode),
		roleStatusValue(role.Agent),
		roleStatusValue(role.Provider),
		roleStatusValue(role.Model),
		strconv.Itoa(len(role.Diagnostics)),
	}
}

func roleBundle(role RoleRecord) outputBundle {
	return outputBundle{
		jsonValue: role,
		human: func() (string, error) {
			summary := renderHumanSection(roleOutputLabel, []keyValue{
				{Label: roleOutputLabel, Value: role.Role},
				{Label: "Enabled", Value: strconv.FormatBool(role.Enabled)},
				{Label: "Resolution", Value: string(role.ResolutionMode)},
				{Label: agentOutputLabel, Value: stringOrDash(roleStatusValue(role.Agent))},
				{Label: agentKernelProviderValue, Value: stringOrDash(roleStatusValue(role.Provider))},
				{Label: agentKernelModelValue, Value: stringOrDash(roleStatusValue(role.Model))},
				{Label: "Reasoning Effort", Value: stringOrDash(roleStatusValue(role.ReasoningEffort))},
				{Label: "Timeout", Value: stringOrDash(roleStatusValue(role.Timeout))},
				{Label: "Diagnostics", Value: strconv.Itoa(len(role.Diagnostics))},
			})
			return renderHumanBlocks(
				summary,
				renderHumanTable(
					"Fallback Chain",
					[]string{"Provider", "Model", "Reasoning Effort"},
					roleFallbackRows(role),
				),
				renderHumanTable("Provenance", []string{cliFieldValue, "Source"}, roleProvenanceRows(role)),
				renderHumanTable("Diagnostics", []string{"Code", "Message", "Agent"}, roleDiagnosticRows(role)),
			), nil
		},
		toon: func() (string, error) {
			return renderHumanBlocks(renderToonObject("role", []string{
				"role",
				"enabled",
				"resolution_mode",
				agentAgentKey,
				cliProviderKey,
				agentModelKey,
				"reasoning_effort",
				"timeout",
			}, []string{
				role.Role,
				strconv.FormatBool(role.Enabled),
				string(role.ResolutionMode),
				roleStatusValue(role.Agent),
				roleStatusValue(role.Provider),
				roleStatusValue(role.Model),
				roleStatusValue(role.ReasoningEffort),
				roleStatusValue(role.Timeout),
			}),
				renderToonArray(
					"fallback_chain",
					[]string{cliProviderKey, agentKernelModelKey, "reasoning_effort"},
					roleFallbackRows(role),
				),
				renderToonArray("provenance", []string{cliFieldKey, "source"}, roleProvenanceRows(role)),
				renderToonArray("diagnostics", []string{"code", "message", agentAgentKey}, roleDiagnosticRows(role)),
			), nil
		},
	}
}

func roleFallbackRows(role RoleRecord) [][]string {
	rows := make([][]string, 0, len(role.FallbackChain))
	for _, fallback := range role.FallbackChain {
		rows = append(rows, []string{fallback.Provider, fallback.Model, fallback.ReasoningEffort})
	}
	return rows
}

func roleProvenanceRows(role RoleRecord) [][]string {
	fields := make([]string, 0, len(role.Provenance))
	for field := range role.Provenance {
		fields = append(fields, field)
	}
	sort.Strings(fields)
	rows := make([][]string, 0, len(fields))
	for _, field := range fields {
		rows = append(rows, []string{field, role.Provenance[field]})
	}
	return rows
}

func roleDiagnosticRows(role RoleRecord) [][]string {
	rows := make([][]string, 0, len(role.Diagnostics))
	for _, diagnostic := range role.Diagnostics {
		rows = append(rows, []string{diagnostic.Code, diagnostic.Message, diagnostic.Agent})
	}
	return rows
}

func roleStatusValue(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}
