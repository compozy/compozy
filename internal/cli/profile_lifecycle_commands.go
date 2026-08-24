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
			choice, err := profileRepoSelectionForCommand(cmd, deps, repos, plan)
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
	cmd.Flags().StringVar(&repos, "repos", "", "Repo folders to rename: all, none, or comma-separated workspace IDs")
	return cmd
}

func profileRepoSelectionForCommand(
	cmd *cobra.Command,
	deps commandDeps,
	value string,
	plan contract.RenameProfilePlan,
) ([]string, error) {
	if cmd.Flags().Changed("repos") {
		return profileRepoSelection(value)
	}
	mode, err := resolveInheritedOutputFormat(cmd)
	if err != nil {
		return nil, err
	}
	if mode != OutputHuman || deps.inputIsTerminal == nil || !deps.inputIsTerminal(cmd.InOrStdin()) {
		return []string{profileRepoNoneValue}, nil
	}
	if len(plan.RepoCandidates) == 0 {
		return []string{profileRepoNoneValue}, nil
	}
	answer, err := promptProfileRepoSelection(cmd, plan.RepoCandidates)
	if err != nil {
		return nil, err
	}
	if answer == "" || answer == "y" || answer == yesFlagName {
		return selectedProfileRepoCandidates(plan.RepoCandidates), nil
	}
	if answer == "n" || answer == "no" {
		return []string{profileRepoNoneValue}, nil
	}
	return profileRepoSelection(answer)
}

func promptProfileRepoSelection(
	cmd *cobra.Command,
	candidates []contract.ProfileFolderRef,
) (string, error) {
	if _, err := fmt.Fprintln(cmd.ErrOrStderr(), "Repository folders found:"); err != nil {
		return "", fmt.Errorf("cli: write profile rename repository candidates: %w", err)
	}
	for _, candidate := range candidates {
		workspace := firstNonEmpty(candidate.WorkspaceName, candidate.WorkspaceID)
		if _, err := fmt.Fprintf(cmd.ErrOrStderr(), "  %s: %s\n", workspace, candidate.Path); err != nil {
			return "", fmt.Errorf("cli: write profile rename repository candidate: %w", err)
		}
	}
	if _, err := fmt.Fprint(cmd.ErrOrStderr(), "Rename these repository folders? [Y/n] "); err != nil {
		return "", fmt.Errorf("cli: write profile rename confirmation: %w", err)
	}
	line, readErr := bufio.NewReader(cmd.InOrStdin()).ReadString('\n')
	if readErr != nil && !errors.Is(readErr, io.EOF) {
		return "", fmt.Errorf("cli: read profile rename repository selection: %w", readErr)
	}
	return strings.TrimSpace(strings.ToLower(line)), nil
}

func selectedProfileRepoCandidates(candidates []contract.ProfileFolderRef) []string {
	selected := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		if workspaceID := strings.TrimSpace(candidate.WorkspaceID); workspaceID != "" {
			selected = append(selected, workspaceID)
		}
	}
	if len(selected) == 0 {
		return []string{profileRepoNoneValue}
	}
	return selected
}

func profileRepoSelection(value string) ([]string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" || trimmed == profileRepoNoneValue {
		return []string{profileRepoNoneValue}, nil
	}
	if trimmed == providerModelViewAll {
		return []string{providerModelViewAll}, nil
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
		Use: cliNamedDeleteUse, Short: "Delete an empty profile", Args: exactOneNonBlankArg(),
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
	if answer != "y" && answer != yesFlagName {
		return errors.New("cli: profile deletion declined")
	}
	return nil
}
