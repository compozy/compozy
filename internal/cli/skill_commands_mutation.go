package cli

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/skills"

	"github.com/spf13/cobra"
)

type skillMutationClient interface {
	EnableSkill(context.Context, string, SkillQuery) (SkillActionRecord, error)
	DisableSkill(context.Context, string, SkillQuery) (SkillActionRecord, error)
}

func newSkillWhereCommand(deps commandDeps) *cobra.Command {
	var workspace string
	var agentName string

	cmd := &cobra.Command{
		Use:     "where <name>",
		Short:   "Show every path participating in skill resolution",
		Example: "  # Show which skill declaration wins and which ones are shadowed\n  compozy skill where code-review",
		Args:    exactOneNonBlankArg(),
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
				detail, err := client.GetSkill(cmd.Context(), args[0], scope.query)
				if err != nil {
					return err
				}
				record, err := client.GetSkillShadows(cmd.Context(), args[0], scope.query)
				if err != nil {
					return err
				}
				return writeCommandOutput(cmd, skillWhereBundle(skillWhereItemFromRecords(detail, record)))
			}

			ctx, err := loadSkillCommandContext(cmd.Context(), deps, scope.query.ForAgent)
			if err != nil {
				return err
			}
			shadows, ok := skills.ShadowsForSkillList(ctx.skills, args[0], cliNow(deps.now))
			if !ok {
				return fmt.Errorf("skill %q not found", strings.TrimSpace(args[0]))
			}
			skill, err := findSkillByName(ctx.skills, args[0])
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, skillWhereBundle(skillWhereItemFromSkill(
				skill,
				skillShadowsRecordFromDomain(shadows),
			)))
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

func newSkillCreateCommand(deps commandDeps) *cobra.Command {
	var workspaceRef string
	var group string
	var exposeTargets string
	cmd := &cobra.Command{
		Use:   "create [name]",
		Short: "Scaffold a new workspace skill",
		Example: `  # Create .compozy/skills/api-review/SKILL.md in the current workspace
  compozy skill create api-review

  # Organize a skill beneath an optional group directory
  compozy skill create campaign-brief --group marketing`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var targets []string
			if cmd.Flags().Changed("expose") {
				parsedTargets, parseErr := parseSkillExposureTargets(exposeTargets)
				if parseErr != nil {
					return parseErr
				}
				targets = parsedTargets
			}
			name := defaultSkillName
			if len(args) == 1 {
				name = args[0]
			}

			skillName, err := normalizeSkillName(name)
			if err != nil {
				return err
			}
			groupPath := ""
			if cmd.Flags().Changed("group") {
				groupPath, err = normalizeSkillGroup(group)
				if err != nil {
					return err
				}
			}

			workspace, err := resolveCommandWorkspaceRoot(cmd, deps, workspaceRef)
			if err != nil {
				return err
			}

			skillsRoot := filepath.Join(workspace, compozyconfig.DirName, compozyconfig.SkillsDirName)
			skillDir, skillFilePath, err := createWorkspaceSkill(skillsRoot, groupPath, skillName)
			if err != nil {
				return err
			}

			created := skillCreateItem{
				Name:   skillName,
				Group:  groupPath,
				Path:   skillDir,
				File:   skillFilePath,
				Source: workspaceSkillSource,
				Status: skillCommandsCreatedKey,
			}
			if !cmd.Flags().Changed("expose") {
				return writeCommandOutput(cmd, skillCreateBundle(created))
			}
			resolution, ok := commandWorkspaceResolution(cmd)
			if !ok || strings.TrimSpace(resolution.ID) == "" {
				return errors.New("cli: created skill workspace resolution is unavailable")
			}
			client, err := skillExposureClientFromDeps(deps)
			if err != nil {
				return err
			}
			response, err := client.ExposeSkill(cmd.Context(), skillName, contract.SkillExposureRequest{
				Targets: targets, WorkspaceID: resolution.ID,
			}, SkillQuery{Workspace: resolution.ID})
			relativeFile, relativeErr := filepath.Rel(workspace, skillFilePath)
			if relativeErr != nil {
				return fmt.Errorf("cli: render created skill path: %w", relativeErr)
			}
			if err != nil {
				mode, modeErr := resolveOutputFormat(cmd)
				if modeErr != nil {
					return modeErr
				}
				if mode == OutputHuman {
					if writeErr := writeRawCommandOutput(cmd, "created "+filepath.ToSlash(relativeFile)); writeErr != nil {
						return errors.Join(&skillCreatedExposureError{cause: err}, writeErr)
					}
				}
				return &skillCreatedExposureError{cause: err}
			}
			return writeCommandOutput(cmd, skillCreateExposureBundle(
				created,
				filepath.ToSlash(relativeFile),
				skillExposureSuccess{Name: response.Name, Action: "expose", Results: response.Results},
			))
		},
	}
	cmd.Flags().StringVar(
		&workspaceRef,
		workspaceSkillSource,
		"",
		"Override the target workspace (ID, name, or path)",
	)
	cmd.Flags().StringVar(&group, "group", "", "Place the skill under a relative group path")
	cmd.Flags().StringVar(&exposeTargets, "expose", "", "Expose the created skill to comma-separated provider targets")
	return cmd
}

func newSkillEnableCommand(deps commandDeps) *cobra.Command {
	return newSkillActionCommand(deps, "enable <name>", "Enable a daemon-managed skill", func(
		ctx context.Context,
		client skillMutationClient,
		name string,
		query SkillQuery,
	) (SkillActionRecord, error) {
		return client.EnableSkill(ctx, name, query)
	})
}

func newSkillDisableCommand(deps commandDeps) *cobra.Command {
	return newSkillActionCommand(deps, "disable <name>", "Disable a daemon-managed skill", func(
		ctx context.Context,
		client skillMutationClient,
		name string,
		query SkillQuery,
	) (SkillActionRecord, error) {
		return client.DisableSkill(ctx, name, query)
	})
}

func newSkillActionCommand(
	deps commandDeps,
	use string,
	short string,
	action func(context.Context, skillMutationClient, string, SkillQuery) (SkillActionRecord, error),
) *cobra.Command {
	var workspaceRef string
	var agentName string
	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			scope, err := resolveSkillCommandScope(
				cmd.Context(),
				cmd,
				deps,
			)
			if err != nil {
				return err
			}
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			actionName := strings.Fields(use)[0]
			result, err := action(
				cmd.Context(),
				client,
				args[0],
				scope.query,
			)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, skillActionBundle(args[0], actionName, result))
		},
	}
	cmd.Flags().StringVar(
		&workspaceRef,
		workspaceSkillSource,
		"",
		"Override the workspace context (ID, name, or path)",
	)
	cmd.Flags().StringVar(&agentName, "for-agent", "", "Resolve the effective skill set for one logical agent")
	return cmd
}
