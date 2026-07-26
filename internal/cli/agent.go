package cli

import (
	"strconv"
	"strings"

	"github.com/compozy/agh/internal/api/contract"
	"github.com/spf13/cobra"
)

const (
	toolOperatorToolsValue = "Tools"
)

const (
	agentKernelProviderValue = "Provider"
)

const (
	installPermissionsValue = "Permissions"
	cliProviderKey          = "provider"
)

const (
	automationNameValue  = "Name"
	configPermissionsKey = "permissions"
)

const (
	agentKernelModelValue = "Model"
	automationNameKey     = "name"
)

const (
	agentModelKey             = "model"
	agentReasoningEffortKey   = "reasoning-effort"
	agentReasoningEffortValue = "Reasoning Effort"
	agentReasoningEffortField = "reasoning_effort"
)

const (
	agentBodyValue         = "Body"
	agentCategoryValue     = "Category"
	agentAgentKey          = "agent"
	agentCategoryKey       = "category"
	agentCommandKey        = "command"
	agentDisableSkillFlag  = "disable-skill"
	agentDisabledSkillsKey = "disabled_skills"
	agentInfoNameValue     = "info <name>"
	agentListKey           = "list"
)

func newAgentCommand(deps commandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   agentAgentKey,
		Short: "Inspect AGH agent definitions",
	}

	cmd.AddCommand(newAgentCreateCommand(deps))
	cmd.AddCommand(newAgentUpdateCommand(deps))
	cmd.AddCommand(newAgentDeleteCommand(deps))
	cmd.AddCommand(newAgentDuplicateCommand(deps))
	cmd.AddCommand(newAgentListCommand(deps))
	cmd.AddCommand(newAgentInfoCommand(deps))
	cmd.AddCommand(newAgentSoulCommand(deps))
	cmd.AddCommand(newAgentHeartbeatCommand(deps))
	return cmd
}

func newAgentListCommand(deps commandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   agentListKey,
		Short: "List installed agent definitions",
		Example: `  # Show every agent definition available to the daemon
  agh agent list

  # Show agents resolved for a workspace
  agh agent list --workspace ~/dev/ai/acme-startup

  # Emit the same list as JSON
  agh agent list -o json`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}

			query, err := agentQueryFromCommand(cmd)
			if err != nil {
				return err
			}
			agents, err := client.ListAgents(cmd.Context(), query)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, agentListBundle(agents))
		},
	}
	cmd.Flags().String("workspace", "", "Resolve agents from a workspace id, name, or path")
	return cmd
}

func newAgentInfoCommand(deps commandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   agentInfoNameValue,
		Short: "Show one agent definition",
		Example: `  # Inspect the default bootstrap agent
  agh agent info general

  # Inspect a workspace-local agent
  agh agent info reviewer --workspace ~/dev/ai/acme-startup

  # Inspect an agent definition as JSON
  agh agent info reviewer -o json`,
		Args: exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}

			query, err := agentQueryFromCommand(cmd)
			if err != nil {
				return err
			}
			agent, err := client.GetAgent(cmd.Context(), args[0], query)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, agentBundle(agent))
		},
	}
	cmd.Flags().String("workspace", "", "Resolve the agent from a workspace id, name, or path")
	return cmd
}

func agentQueryFromCommand(cmd *cobra.Command) (AgentQuery, error) {
	workspace, err := commandWorkspaceFlag(cmd)
	if err != nil {
		return AgentQuery{}, err
	}
	return AgentQuery{Workspace: workspace}, nil
}

func agentListBundle(items []AgentRecord) outputBundle {
	return listBundle(
		items,
		items,
		"Agents",
		[]string{
			automationNameValue,
			agentKernelProviderValue,
			agentKernelModelValue,
			agentCategoryValue,
			taskOriginValue,
			automationWorkspaceValue,
			"Disabled Skills",
			"Definition Digest",
			toolOperatorToolsValue,
			installPermissionsValue,
		},
		"agents",
		[]string{
			automationNameKey,
			cliProviderKey,
			agentModelKey,
			agentCategoryKey,
			taskOriginKey,
			automationWorkspaceIDKey,
			agentDisabledSkillsKey,
			"definition_digest",
			"tool_count",
			configPermissionsKey,
		},
		func(item AgentRecord) []string {
			return []string{
				stringOrDash(item.Name),
				stringOrDash(item.Provider),
				stringOrDash(item.Model),
				stringOrDash(agentCategoryLabel(item.CategoryPath)),
				stringOrDash(string(item.Origin)),
				stringOrDash(item.WorkspaceID),
				stringOrDash(agentSkillsLabel(item.Skills)),
				stringOrDash(item.DefinitionDigest),
				strconv.Itoa(len(item.Tools)),
				stringOrDash(item.Permissions),
			}
		},
		func(item AgentRecord) []string {
			return []string{
				item.Name,
				item.Provider,
				item.Model,
				agentCategoryLabel(item.CategoryPath),
				string(item.Origin),
				item.WorkspaceID,
				agentSkillsLabel(item.Skills),
				item.DefinitionDigest,
				strconv.Itoa(len(item.Tools)),
				item.Permissions,
			}
		},
	)
}

func agentBundle(item AgentRecord) outputBundle {
	return outputBundle{
		jsonValue: item,
		human: func() (string, error) {
			base := renderHumanSection("Agent", []keyValue{
				{Label: automationNameValue, Value: stringOrDash(item.Name)},
				{Label: agentKernelProviderValue, Value: stringOrDash(item.Provider)},
				{Label: cliCommandValue, Value: stringOrDash(item.Command)},
				{Label: agentKernelModelValue, Value: stringOrDash(item.Model)},
				{Label: agentReasoningEffortValue, Value: stringOrDash(string(item.ReasoningEffort))},
				{Label: agentCategoryValue, Value: stringOrDash(agentCategoryLabel(item.CategoryPath))},
				{Label: taskOriginValue, Value: stringOrDash(string(item.Origin))},
				{Label: automationWorkspaceValue, Value: stringOrDash(item.WorkspaceID)},
				{Label: "Disabled Skills", Value: stringOrDash(agentSkillsLabel(item.Skills))},
				{Label: "Definition Digest", Value: stringOrDash(item.DefinitionDigest)},
				{Label: toolOperatorToolsValue, Value: stringOrDash(strings.Join(item.Tools, ", "))},
				{Label: installPermissionsValue, Value: stringOrDash(item.Permissions)},
			})

			servers := make([][]string, 0, len(item.MCPServers))
			for _, server := range item.MCPServers {
				servers = append(servers, []string{
					stringOrDash(server.Name),
					stringOrDash(server.Command),
					stringOrDash(strings.Join(server.Args, " ")),
				})
			}
			mcp := renderHumanTable("MCP Servers", []string{automationNameValue, cliCommandValue, "Args"}, servers)
			prompt := renderHumanSection(
				"Prompt",
				[]keyValue{{Label: agentBodyValue, Value: stringOrDash(item.Prompt)}},
			)
			return renderHumanBlocks(base, mcp, prompt), nil
		},
		toon: func() (string, error) {
			// Detail output emits tool names; list output keeps the table dense with tool_count.
			return renderToonObject(agentAgentKey, []string{
				automationNameKey,
				cliProviderKey,
				agentCommandKey,
				agentModelKey,
				agentReasoningEffortField,
				agentCategoryKey,
				taskOriginKey,
				automationWorkspaceIDKey,
				agentDisabledSkillsKey,
				"definition_digest",
				"tools",
				configPermissionsKey,
				clientToolsPromptKey,
			}, []string{
				item.Name,
				item.Provider,
				item.Command,
				item.Model,
				string(item.ReasoningEffort),
				agentCategoryLabel(item.CategoryPath),
				string(item.Origin),
				item.WorkspaceID,
				agentSkillsLabel(item.Skills),
				item.DefinitionDigest,
				strings.Join(item.Tools, "|"),
				item.Permissions,
				item.Prompt,
			}), nil
		},
	}
}

func agentCategoryLabel(path []string) string {
	if len(path) == 0 {
		return ""
	}
	// AGENT.md category paths render as a single space-delimited CLI path.
	return strings.Join(path, " / ")
}

func agentSkillsLabel(skills *contract.CreateAgentSkillsConfig) string {
	if skills == nil {
		return ""
	}
	return strings.Join(skills.Disabled, ", ")
}
