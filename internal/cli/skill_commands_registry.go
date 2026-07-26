package cli

import (
	"errors"
	"fmt"

	skillbundled "github.com/compozy/agh/skills"
	"github.com/spf13/cobra"
)

func newSkillSearchCommand(deps commandDeps) *cobra.Command {
	limit := defaultMarketplaceSearchLim

	cmd := &cobra.Command{
		Use:   skillCommandsSearchQueryValue,
		Short: "Search marketplace skills",
		Example: `  # Search the configured marketplace
  agh skill search "code review"

  # Limit marketplace results
  agh skill search testing --limit 5`,
		Args: exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			results, err := searchMarketplaceSkills(cmd.Context(), deps, args[0], limit)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, skillSearchBundle(results))
		},
	}
	cmd.Flags().IntVar(&limit, "limit", defaultMarketplaceSearchLim, "Maximum number of marketplace results to return")
	return cmd
}

func newSkillInstallCommand(deps commandDeps) *cobra.Command {
	version := ""
	cmd := &cobra.Command{
		Use:   "install <slug>",
		Short: "Install a marketplace skill",
		Example: `  # Install the latest marketplace version of a skill
  agh skill install @acme/code-review`,
		Args: exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			slug, err := normalizeSkillSlug(args[0])
			if err != nil {
				return err
			}

			item, err := installMarketplaceSkillForCommand(cmd.Context(), deps, slug, version)
			if err != nil {
				return err
			}

			return writeCommandOutput(cmd, skillInstallBundle(item))
		},
	}
	cmd.Flags().StringVar(&version, "version", "", "Marketplace version to install")
	return cmd
}

func newSkillRemoveCommand(deps commandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "remove <name>",
		Short: "Remove an installed marketplace skill",
		Example: `  # Remove a marketplace-installed skill by local name
  agh skill remove code-review`,
		Args: exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			name, err := normalizeSkillName(args[0])
			if err != nil {
				return err
			}

			item, err := removeMarketplaceSkillForCommand(cmd.Context(), deps, name)
			if err != nil {
				if bundledSkillExists(name) {
					return fmt.Errorf("skill %q is not a marketplace-installed skill", name)
				}
				return err
			}

			return writeCommandOutput(cmd, skillRemoveBundle(item))
		},
	}
}

func bundledSkillExists(name string) bool {
	_, err := skillbundled.LoadContent(name)
	return err == nil
}

func newSkillUpdateCommand(deps commandDeps) *cobra.Command {
	updateAll := false
	checkOnly := false

	cmd := &cobra.Command{
		Use:   "update [name]",
		Short: "Check for or install updates for marketplace skills",
		Example: `  # Check whether one installed skill has an update
  agh skill update code-review --check

  # Update every marketplace-installed skill
  agh skill update --all`,
		Args: func(_ *cobra.Command, args []string) error {
			if updateAll && len(args) > 0 {
				return errors.New("cli: update accepts either a skill name or --all, not both")
			}
			if !updateAll && len(args) != 1 {
				return errors.New("cli: update requires a skill name unless --all is set")
			}
			if len(args) == 1 {
				_, err := normalizeSkillName(args[0])
				return err
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			items, err := updateMarketplaceSkillsForCommand(
				cmd.Context(),
				deps,
				args,
				updateAll,
				checkOnly,
			)
			if err != nil {
				return err
			}

			return writeCommandOutput(cmd, skillUpdateBundle(items))
		},
	}
	cmd.Flags().BoolVar(&updateAll, "all", false, "Update every installed marketplace skill")
	cmd.Flags().BoolVar(&checkOnly, "check", false, "Only check for updates without installing them")
	return cmd
}
