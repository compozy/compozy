package cli

import (
	"errors"
	"fmt"

	"strings"

	"github.com/compozy/agh/internal/skills"

	"github.com/spf13/cobra"
)

const (
	skillCommandsCreatedKey       = "created"
	skillCommandsListKey          = "list"
	skillCommandsSearchQueryValue = "search <query>"
	skillCommandsSkillKey         = "skill"
)

func newSkillCommand(deps commandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   skillCommandsSkillKey,
		Short: "Manage local AgentSkills",
	}

	cmd.AddCommand(newSkillListCommand(deps))
	cmd.AddCommand(newSkillViewCommand(deps))
	cmd.AddCommand(newSkillInfoCommand(deps))
	cmd.AddCommand(newSkillInspectCommand(deps))
	cmd.AddCommand(newSkillWhereCommand(deps))
	cmd.AddCommand(newSkillSearchCommand(deps))
	cmd.AddCommand(newSkillInstallCommand(deps))
	cmd.AddCommand(newSkillRemoveCommand(deps))
	cmd.AddCommand(newSkillUpdateCommand(deps))
	cmd.AddCommand(newSkillCreateCommand(deps))
	cmd.AddCommand(newSkillEnableCommand(deps))
	cmd.AddCommand(newSkillDisableCommand(deps))
	return cmd
}

func newSkillListCommand(deps commandDeps) *cobra.Command {
	var sourceFilter string
	var workspace string
	var agentName string

	cmd := &cobra.Command{
		Use:   skillCommandsListKey,
		Short: "List locally available skills",
		Example: `  # List every skill visible in the current workspace
  agh skill list

  # Show only bundled skills
  agh skill list --source bundled`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			scope, err := resolveSkillCommandScope(cmd.Context(), cmd, deps, agentActionCLI("skill.list"))
			if err != nil {
				return err
			}
			if scope.useDaemon {
				client, err := clientFromDeps(deps)
				if err != nil {
					return err
				}
				records, err := client.ListSkills(
					cmd.Context(),
					scope.query,
				)
				if err != nil {
					return err
				}
				items, err := skillListItemsFromRecords(records, sourceFilter)
				if err != nil {
					return err
				}
				return writeCommandOutput(cmd, skillListBundle(items))
			}

			ctx, err := loadSkillCommandContext(cmd.Context(), deps, scope.query.ForAgent)
			if err != nil {
				return err
			}

			items, err := skillListItems(ctx.skills, sourceFilter)
			if err != nil {
				return err
			}

			return writeCommandOutput(cmd, skillListBundle(items))
		},
	}
	cmd.Flags().StringVar(
		&sourceFilter,
		"source",
		"",
		"Filter by source: bundled, marketplace, user, additional, workspace, or agent-local",
	)
	cmd.Flags().StringVar(
		&workspace,
		workspaceSkillSource,
		"",
		"Resolve daemon-managed skills from a workspace id, name, or path",
	)
	cmd.Flags().StringVar(&agentName, "for-agent", "", "Resolve the effective skill set for one logical agent")
	return cmd
}

func newSkillViewCommand(deps commandDeps) *cobra.Command {
	var filePath string
	var workspace string
	var agentName string

	cmd := &cobra.Command{
		Use:   "view <name>",
		Short: "Read a skill or one of its resource files",
		Example: `  # Render a skill as the XML block injected into agents
  agh skill view code-review

  # Read a resource file inside a skill directory
  agh skill view code-review --file references/checklist.md`,
		Args: exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSkillViewCommand(cmd, deps, args[0], filePath)
		},
	}
	cmd.Flags().StringVar(&filePath, "file", "", "Relative file path inside the skill directory")
	cmd.Flags().StringVar(
		&workspace,
		workspaceSkillSource,
		"",
		"Resolve the daemon-managed skill from a workspace id, name, or path",
	)
	cmd.Flags().StringVar(&agentName, "for-agent", "", "Resolve the effective skill set for one logical agent")
	return cmd
}

func runSkillViewCommand(cmd *cobra.Command, deps commandDeps, name string, filePath string) error {
	skillName := strings.TrimSpace(name)
	if skillName == "" {
		return errors.New("skill name is required")
	}
	scope, err := resolveSkillCommandScope(cmd.Context(), cmd, deps, agentActionCLI("skill.view"))
	if err != nil {
		return err
	}
	if scope.useDaemon {
		return runDaemonSkillViewCommand(cmd, deps, skillName, filePath, scope.query)
	}
	return runLocalSkillViewCommand(cmd, deps, skillName, filePath, scope.query.ForAgent)
}

func runDaemonSkillViewCommand(
	cmd *cobra.Command,
	deps commandDeps,
	name string,
	filePath string,
	query SkillQuery,
) error {
	if strings.TrimSpace(filePath) != "" {
		return fmt.Errorf("skill view --workspace does not support --file")
	}
	client, err := clientFromDeps(deps)
	if err != nil {
		return err
	}
	record, err := client.GetSkill(cmd.Context(), name, query)
	if err != nil {
		return err
	}
	content, err := client.GetSkillContent(cmd.Context(), name, query)
	if err != nil {
		return err
	}
	rendered, err := renderSkillXML(&skills.Skill{Meta: skills.SkillMeta{Name: record.Name}}, content, nil)
	if err != nil {
		return err
	}
	item := skillViewItem{
		Name:    record.Name,
		Source:  record.Source,
		Path:    record.Dir,
		Content: rendered,
	}
	return writeCommandOutput(cmd, skillViewBundle(item, rendered))
}

func runLocalSkillViewCommand(
	cmd *cobra.Command,
	deps commandDeps,
	name string,
	filePath string,
	agentName string,
) error {
	ctx, err := loadSkillCommandContext(cmd.Context(), deps, agentName)
	if err != nil {
		return err
	}
	skill, err := findSkillByName(ctx.skills, name)
	if err != nil {
		return err
	}
	if strings.TrimSpace(filePath) != "" {
		return runLocalSkillResourceViewCommand(cmd, ctx, skill, filePath)
	}
	return runLocalSkillXMLViewCommand(cmd, ctx, skill)
}

func runLocalSkillResourceViewCommand(
	cmd *cobra.Command,
	ctx skillCommandContext,
	skill *skills.Skill,
	filePath string,
) error {
	content, err := readSkillResource(skill, ctx.bundledFS, filePath)
	if err != nil {
		return err
	}
	item := skillViewItem{
		Name:    skill.Meta.Name,
		Source:  skillSourceLabel(skill.Source),
		Path:    skill.FilePath,
		File:    strings.TrimSpace(filePath),
		Content: content,
	}
	return writeCommandOutput(cmd, skillViewBundle(item, content))
}

func runLocalSkillXMLViewCommand(cmd *cobra.Command, ctx skillCommandContext, skill *skills.Skill) error {
	resources, err := listSkillResources(skill, ctx.bundledFS)
	if err != nil {
		return err
	}
	content, err := ctx.registry.LoadContent(cmd.Context(), skill)
	if err != nil {
		return err
	}
	rendered, err := renderSkillXML(skill, content, resources)
	if err != nil {
		return err
	}
	item := skillViewItem{
		Name:      skill.Meta.Name,
		Source:    skillSourceLabel(skill.Source),
		Path:      skill.FilePath,
		Content:   rendered,
		Resources: resources,
	}
	return writeCommandOutput(cmd, skillViewBundle(item, rendered))
}
