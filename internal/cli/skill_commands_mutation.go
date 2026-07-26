package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/skills"

	"github.com/spf13/cobra"
)

func newSkillWhereCommand(deps commandDeps) *cobra.Command {
	var workspace string
	var agentName string

	cmd := &cobra.Command{
		Use:     "where <name>",
		Short:   "Show every path participating in skill resolution",
		Example: "  # Show which skill declaration wins and which ones are shadowed\n  agh skill where code-review",
		Args:    exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			scope, err := resolveSkillCommandScope(cmd.Context(), cmd, deps, agentActionCLI("skill.where"))
			if err != nil {
				return err
			}
			if scope.useDaemon {
				client, err := clientFromDeps(deps)
				if err != nil {
					return err
				}
				record, err := client.GetSkillShadows(cmd.Context(), args[0], scope.query)
				if err != nil {
					return err
				}
				return writeCommandOutput(cmd, skillWhereBundle(record))
			}

			ctx, err := loadSkillCommandContext(cmd.Context(), deps, scope.query.ForAgent)
			if err != nil {
				return err
			}
			shadows, ok := skills.ShadowsForSkillList(ctx.skills, args[0], cliNow(deps.now))
			if !ok {
				return fmt.Errorf("skill %q not found", strings.TrimSpace(args[0]))
			}
			return writeCommandOutput(cmd, skillWhereBundle(skillShadowsRecordFromDomain(shadows)))
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

func newSkillCreateCommand(deps commandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "create [name]",
		Short: "Scaffold a new workspace skill",
		Example: `  # Create .agh/skills/api-review/SKILL.md in the current workspace
  agh skill create api-review`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := defaultSkillName
			if len(args) == 1 {
				name = args[0]
			}

			skillName, err := normalizeSkillName(name)
			if err != nil {
				return err
			}

			workspace, err := resolveCLIWorkspaceRoot(deps)
			if err != nil {
				return err
			}

			skillDir := filepath.Join(workspace, aghconfig.DirName, aghconfig.SkillsDirName, skillName)
			if _, err := os.Stat(skillDir); err == nil {
				return fmt.Errorf("skill %q already exists at %s", skillName, skillDir)
			} else if !errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("cli: inspect skill directory %q: %w", skillDir, err)
			}

			if err := os.MkdirAll(skillDir, 0o755); err != nil {
				return fmt.Errorf("cli: create skill directory %q: %w", skillDir, err)
			}

			skillFilePath := filepath.Join(skillDir, skillMarkdownFileName)
			content := defaultSkillTemplate(skillName)
			if err := os.WriteFile(skillFilePath, []byte(content), 0o600); err != nil {
				return fmt.Errorf("cli: write skill template %q: %w", skillFilePath, err)
			}

			if _, err := skills.ParseSkillFile(skillFilePath); err != nil {
				return fmt.Errorf("cli: validate generated skill %q: %w", skillFilePath, err)
			}

			return writeCommandOutput(cmd, skillCreateBundle(skillCreateItem{
				Name:   skillName,
				Path:   skillDir,
				File:   skillFilePath,
				Source: workspaceSkillSource,
				Status: skillCommandsCreatedKey,
			}))
		},
	}
}

func newSkillEnableCommand(deps commandDeps) *cobra.Command {
	return newSkillActionCommand(deps, "enable <name>", "Enable a daemon-managed skill", func(
		ctx context.Context,
		client DaemonClient,
		name string,
		query SkillQuery,
	) (SkillActionRecord, error) {
		return client.EnableSkill(ctx, name, query)
	})
}

func newSkillDisableCommand(deps commandDeps) *cobra.Command {
	return newSkillActionCommand(deps, "disable <name>", "Disable a daemon-managed skill", func(
		ctx context.Context,
		client DaemonClient,
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
	action func(context.Context, DaemonClient, string, SkillQuery) (SkillActionRecord, error),
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
				agentActionCLI("skill."+strings.Fields(use)[0]),
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
	cmd.Flags().StringVar(&workspaceRef, workspaceSkillSource, "", "Workspace id, name, or path")
	cmd.Flags().StringVar(&agentName, "for-agent", "", "Resolve the effective skill set for one logical agent")
	return cmd
}
