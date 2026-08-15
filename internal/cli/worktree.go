package cli

import (
	"context"
	"strings"

	"github.com/spf13/cobra"
)

func newWorktreeCommand(deps commandDeps) *cobra.Command {
	cmd := &cobra.Command{Use: "worktree", Short: "Manage workspace worktrees"}
	cmd.AddCommand(
		newWorktreeListCommand(deps),
		newWorktreeCreateCommand(deps),
		newWorktreeCancelCommand(deps),
		newWorktreeAdoptCommand(deps),
		newWorktreeInspectCommand(deps),
		newWorktreeStatusCommand(deps),
		newWorktreeExitCommand(deps),
		newWorktreeCommitCommand(deps),
		newWorktreePushCommand(deps),
		newWorktreePRCommand(deps),
		newWorktreeExitCancelCommand(deps),
		newWorktreeRemoveCommand(deps),
		newWorktreeDismissCommand(deps),
	)
	return cmd
}

func newWorktreeListCommand(deps commandDeps) *cobra.Command {
	var workspaceRef string
	var refresh bool
	cmd := &cobra.Command{
		Use: worktreeListVerb, Short: "List registered worktrees",
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, workspaceID, err := worktreeCommandContext(cmd, deps, workspaceRef)
			if err != nil {
				return err
			}
			result, err := client.ListWorktrees(cmd.Context(), workspaceID, refresh)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, worktreeListBundle(result.Worktrees))
		},
	}
	addWorktreeWorkspaceFlag(cmd, &workspaceRef)
	cmd.Flags().BoolVar(&refresh, "refresh", false, "Refresh Git discovery and status")
	return cmd
}

func newWorktreeCreateCommand(deps commandDeps) *cobra.Command {
	var workspaceRef, branch, base, existingBranch, path string
	cmd := &cobra.Command{
		Use: "create [name]", Short: "Create a worktree", Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, workspaceID, err := worktreeCommandContext(cmd, deps, workspaceRef)
			if err != nil {
				return err
			}
			item, err := client.CreateWorktree(cmd.Context(), workspaceID, WorktreeCreateRequest{
				Name: firstArg(args), Branch: branch, BaseRef: base,
				ExistingBranch: existingBranch, Path: path,
			})
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, worktreeRecordBundle(item))
		},
	}
	addWorktreeWorkspaceFlag(cmd, &workspaceRef)
	cmd.Flags().StringVar(&branch, "branch", "", "Branch name for a new worktree")
	cmd.Flags().StringVar(&base, "base", "", "Base ref for a new branch")
	cmd.Flags().StringVar(&existingBranch, "existing-branch", "", "Attach an existing branch")
	cmd.Flags().StringVar(&path, "path", "", "Explicit worktree path")
	return cmd
}

func newWorktreeCancelCommand(deps commandDeps) *cobra.Command {
	var workspaceRef string
	cmd := &cobra.Command{
		Use: "cancel <ref>", Short: "Cancel a pending worktree creation", Args: exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, workspaceID, err := worktreeCommandContext(cmd, deps, workspaceRef)
			if err != nil {
				return err
			}
			item, err := resolveWorktreeCommandTarget(cmd.Context(), client, workspaceID, args[0])
			if err != nil {
				return err
			}
			if err := client.CancelWorktreeCreate(cmd.Context(), workspaceID, item.ID); err != nil {
				return err
			}
			return writeCommandOutput(cmd, worktreeMutationBundle("canceled", item))
		},
	}
	addWorktreeWorkspaceFlag(cmd, &workspaceRef)
	return cmd
}

func newWorktreeAdoptCommand(deps commandDeps) *cobra.Command {
	var workspaceRef string
	cmd := &cobra.Command{
		Use: "adopt <path>", Short: "Adopt an existing linked worktree", Args: exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, workspaceID, err := worktreeCommandContext(cmd, deps, workspaceRef)
			if err != nil {
				return err
			}
			item, err := client.AdoptWorktree(cmd.Context(), workspaceID, args[0])
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, worktreeRecordBundle(item))
		},
	}
	addWorktreeWorkspaceFlag(cmd, &workspaceRef)
	return cmd
}

func newWorktreeInspectCommand(deps commandDeps) *cobra.Command {
	var workspaceRef string
	cmd := &cobra.Command{
		Use: "inspect <ref>", Short: "Inspect a worktree", Args: exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, workspaceID, err := worktreeCommandContext(cmd, deps, workspaceRef)
			if err != nil {
				return err
			}
			item, err := client.InspectWorktree(cmd.Context(), workspaceID, args[0])
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, worktreeInspectBundle(item))
		},
	}
	addWorktreeWorkspaceFlag(cmd, &workspaceRef)
	return cmd
}

func newWorktreeStatusCommand(deps commandDeps) *cobra.Command {
	var workspaceRef string
	var refresh, forge bool
	cmd := &cobra.Command{
		Use: "status <ref>", Short: "Read or refresh worktree status", Args: exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, workspaceID, err := worktreeCommandContext(cmd, deps, workspaceRef)
			if err != nil {
				return err
			}
			item, err := client.GetWorktreeStatus(cmd.Context(), workspaceID, args[0], refresh, forge)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, worktreeStatusBundle(item))
		},
	}
	addWorktreeWorkspaceFlag(cmd, &workspaceRef)
	cmd.Flags().BoolVar(&refresh, "refresh", false, "Refresh local Git status")
	cmd.Flags().BoolVar(&forge, "forge", false, "Refresh the serving forge provider")
	return cmd
}

func newWorktreeRemoveCommand(deps commandDeps) *cobra.Command {
	var workspaceRef string
	var force bool
	cmd := &cobra.Command{
		Use: "remove <ref>", Short: "Remove a worktree", Args: exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, workspaceID, err := worktreeCommandContext(cmd, deps, workspaceRef)
			if err != nil {
				return err
			}
			item, err := resolveWorktreeCommandTarget(cmd.Context(), client, workspaceID, args[0])
			if err != nil {
				return err
			}
			if err := client.RemoveWorktree(cmd.Context(), workspaceID, item.ID, force); err != nil {
				return err
			}
			item.State = lifecycleRemovedKey
			return writeCommandOutput(cmd, worktreeMutationBundle(lifecycleRemovedKey, item))
		},
	}
	addWorktreeWorkspaceFlag(cmd, &workspaceRef)
	cmd.Flags().BoolVar(&force, "force", false, "Confirm destructive removal")
	return cmd
}

func newWorktreeDismissCommand(deps commandDeps) *cobra.Command {
	var workspaceRef string
	cmd := &cobra.Command{
		Use: "dismiss <ref>", Short: "Dismiss a worktree tombstone", Args: exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, workspaceID, err := worktreeCommandContext(cmd, deps, workspaceRef)
			if err != nil {
				return err
			}
			item, err := resolveWorktreeCommandTarget(cmd.Context(), client, workspaceID, args[0])
			if err != nil {
				return err
			}
			if err := client.DismissWorktree(cmd.Context(), workspaceID, item.ID); err != nil {
				return err
			}
			item.State = "dismissed"
			return writeCommandOutput(cmd, worktreeMutationBundle("dismissed", item))
		},
	}
	addWorktreeWorkspaceFlag(cmd, &workspaceRef)
	return cmd
}

func worktreeCommandContext(
	cmd *cobra.Command,
	deps commandDeps,
	workspaceRef string,
) (DaemonClient, string, error) {
	client, err := clientFromDeps(deps)
	if err != nil {
		return nil, "", err
	}
	resolution, err := resolveCommandWorkspace(
		cmd.Context(), cmd, deps, client,
		workspaceResolutionRequest{FlagRef: strings.TrimSpace(workspaceRef)},
	)
	if err != nil {
		return nil, "", err
	}
	return client, resolution.ID, nil
}

func resolveWorktreeCommandTarget(
	ctx context.Context,
	client DaemonClient,
	workspaceID string,
	ref string,
) (WorktreeRecord, error) {
	inspection, err := client.InspectWorktree(ctx, workspaceID, ref)
	if err != nil {
		return WorktreeRecord{}, err
	}
	return inspection.Worktree, nil
}

func addWorktreeWorkspaceFlag(cmd *cobra.Command, target *string) {
	cmd.Flags().StringVar(
		target, workspaceFlagName, "",
		"Override workspace context (ID, name, or path)",
	)
}
