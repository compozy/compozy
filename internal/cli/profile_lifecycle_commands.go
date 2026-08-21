package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/spf13/cobra"
)

func newProfileRenameCommand(deps commandDeps) *cobra.Command {
	var repos string
	cmd := &cobra.Command{
		Use: "rename <old> <new>", Short: "Rename a profile", Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			profiles, _, err := profileClientFromDeps(deps)
			if err != nil {
				return err
			}
			oldName, newName := strings.TrimSpace(args[0]), strings.TrimSpace(args[1])
			plan, err := profiles.PrepareProfileRename(cmd.Context(), oldName, newName)
			if err != nil {
				return err
			}
			choice, err := profileRepoSelection(repos)
			if err != nil {
				return err
			}
			result, err := profiles.RenameProfile(cmd.Context(), oldName, contract.RenameProfileRequest{
				NewName: newName, Repos: choice, PlanRevision: plan.Revision,
			})
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, profileRenameBundle(oldName, newName, plan, result))
		},
	}
	cmd.Flags().StringVar(&repos, "repos", "none", "Repo folders to rename: all, none, or comma-separated workspace IDs")
	return cmd
}

func profileRepoSelection(value string) ([]string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" || trimmed == "none" {
		return []string{"none"}, nil
	}
	if trimmed == "all" {
		return []string{"all"}, nil
	}
	values := trimmedUniqueStrings(strings.Split(trimmed, ","))
	if len(values) == 0 {
		return nil, errors.New("cli: --repos requires all, none, or workspace IDs")
	}
	return values, nil
}

func newProfileArchiveCommand(deps commandDeps) *cobra.Command {
	return &cobra.Command{
		Use: "archive <name>", Short: "Archive a profile", Args: exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			profiles, _, err := profileClientFromDeps(deps)
			if err != nil {
				return err
			}
			name := strings.TrimSpace(args[0])
			plan, err := profiles.PrepareProfileArchive(cmd.Context(), name)
			if err != nil {
				return err
			}
			result, err := profiles.ArchiveProfile(cmd.Context(), name, plan.Revision)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, profileArchiveBundle(name, result))
		},
	}
}

func newProfileUnarchiveCommand(deps commandDeps) *cobra.Command {
	return &cobra.Command{
		Use: "unarchive <name>", Short: "Unarchive a profile", Args: exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			profiles, _, err := profileClientFromDeps(deps)
			if err != nil {
				return err
			}
			name := strings.TrimSpace(args[0])
			result, err := profiles.UnarchiveProfile(cmd.Context(), name)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, profileUnarchiveBundle(name, result))
		},
	}
}

func newProfileDeleteCommand(deps commandDeps) *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use: "delete <name>", Short: "Delete an empty profile", Args: exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			profiles, _, err := profileClientFromDeps(deps)
			if err != nil {
				return err
			}
			name := strings.TrimSpace(args[0])
			plan, err := profiles.PrepareProfileDelete(cmd.Context(), name)
			if err != nil {
				return err
			}
			if err := confirmProfileDelete(cmd, deps, name, plan, yes); err != nil {
				return err
			}
			result, err := profiles.DeleteProfile(cmd.Context(), name, plan.Revision)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, profileDeleteBundle(name, result))
		},
	}
	cmd.Flags().BoolVar(&yes, yesFlagName, false, "Confirm permanent profile deletion")
	return cmd
}

func confirmProfileDelete(
	cmd *cobra.Command,
	deps commandDeps,
	name string,
	plan contract.DeleteProfilePlan,
	yes bool,
) error {
	if yes {
		return nil
	}
	mode, err := resolveInheritedOutputFormat(cmd)
	if err != nil {
		return err
	}
	if mode != OutputHuman {
		return errors.New("cli: profile delete requires --yes for structured output")
	}
	if deps.inputIsTerminal == nil || !deps.inputIsTerminal(cmd.InOrStdin()) {
		return errors.New("cli: profile delete requires --yes when stdin is not interactive")
	}
	preview := renderProfileDeletePreview(name, plan.Removed)
	if _, err := fmt.Fprint(cmd.ErrOrStderr(), preview+"\nDelete? [y/N] "); err != nil {
		return fmt.Errorf("cli: write profile delete confirmation: %w", err)
	}
	line, err := bufio.NewReader(cmd.InOrStdin()).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return fmt.Errorf("cli: read profile delete confirmation: %w", err)
	}
	answer := strings.ToLower(strings.TrimSpace(line))
	if answer != "y" && answer != "yes" {
		return errors.New("cli: profile deletion declined")
	}
	return nil
}
