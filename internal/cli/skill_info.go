package cli

import (
	"github.com/compozy/agh/internal/api/contract"
	"github.com/spf13/cobra"
)

const (
	skillCommandsInfoEntryValue   = "info <entry_id>"
	skillCommandsInspectNameValue = "inspect <name>"
)

func newSkillInfoCommand(deps commandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   skillCommandsInfoEntryValue,
		Short: "Show one Marketplace skill entry",
		Example: `  # Inspect a Marketplace skill before installation
  agh skill info skill_code_review`,
		Args: exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			response, err := client.MarketplaceInfo(
				cmd.Context(),
				contract.MarketplaceKindSkill,
				args[0],
				"",
				MarketplaceReadScope{Scope: contract.SettingsWorkspaceScopeGlobal},
			)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, marketplaceEntryBundle(response))
		},
	}
}

func newSkillInspectCommand(deps commandDeps) *cobra.Command {
	var workspace string
	var agentName string

	cmd := &cobra.Command{
		Use:   skillCommandsInspectNameValue,
		Short: "Inspect installed skill metadata and resources",
		Example: `  # Inspect an installed skill's metadata and resource list
  agh skill inspect code-review`,
		Args: exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			scope, err := resolveSkillCommandScope(cmd.Context(), cmd, deps, agentActionCLI("skill.inspect"))
			if err != nil {
				return err
			}
			if scope.useDaemon {
				client, err := clientFromDeps(deps)
				if err != nil {
					return err
				}
				record, err := client.GetSkill(cmd.Context(), args[0], scope.query)
				if err != nil {
					return err
				}
				return writeCommandOutput(cmd, skillInfoBundle(skillInfoItemFromRecord(record)))
			}

			ctx, err := loadSkillCommandContext(cmd.Context(), deps, scope.query.ForAgent)
			if err != nil {
				return err
			}
			skill, err := findSkillByName(ctx.skills, args[0])
			if err != nil {
				return err
			}
			resources, err := listSkillResources(skill, ctx.bundledFS)
			if err != nil {
				return err
			}
			item := skillInfoItemFromSkill(skill, resources, cliNow(deps.now))
			return writeCommandOutput(cmd, skillInfoBundle(item))
		},
	}
	cmd.Flags().StringVar(
		&workspace,
		workspaceSkillSource,
		"",
		"Resolve the daemon-managed skill from a workspace id, name, or path",
	)
	cmd.Flags().StringVar(&agentName, "for-agent", "", "Resolve the effective skill set for one logical agent")
	return cmd
}
