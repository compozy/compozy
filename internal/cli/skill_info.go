package cli

import (
	"github.com/spf13/cobra"
)

const (
	skillCommandsInfoEntryValue = "info <name>"
)

func newSkillInfoCommand(deps commandDeps) *cobra.Command {
	var workspace string
	var agentName string

	cmd := &cobra.Command{
		Use:   skillCommandsInfoEntryValue,
		Short: "Inspect installed skill metadata and resources",
		Example: `  # Inspect an installed skill's metadata and resource list
  compozy skill inspect code-review`,
		Args: exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			scope, err := resolveSkillCommandScope(cmd.Context(), cmd, deps)
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
		"Override workspace context (ID, name, or path)",
	)
	cmd.Flags().StringVar(&agentName, "for-agent", "", "Resolve the effective skill set for one logical agent")
	return cmd
}
