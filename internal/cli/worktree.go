package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
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
		newWorktreeDeliverCommand(deps),
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
	var refresh, includeRemoved bool
	cmd := &cobra.Command{
		Use: worktreeListVerb, Short: "List registered worktrees",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if wantsAllWorkspaces(cmd) {
				return writeAggregateCatalog(cmd, deps, "worktrees")
			}
			client, workspaceID, err := worktreeCommandContext(cmd, deps, workspaceRef)
			if err != nil {
				return err
			}
			result, err := client.ListWorktrees(cmd.Context(), workspaceID, refresh, includeRemoved)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, worktreeListBundle(result.Worktrees))
		},
	}
	addWorktreeWorkspaceFlag(cmd, &workspaceRef)
	cmd.Flags().BoolVar(&refresh, "refresh", false, "Refresh Git discovery and status")
	cmd.Flags().BoolVar(&includeRemoved, "include-removed", false, "Include removed worktree records")
	addAllWorkspacesFlag(cmd)
	cmd.MarkFlagsMutuallyExclusive("refresh", "all-workspaces")
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
			return writeCommandOutput(cmd, worktreeMutationBundle("canceled", item, nil))
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
	var workspaceRef, discoveredPath string
	var dryRun bool
	var opts WorktreeCleanupOptions
	cmd := &cobra.Command{
		Use: "remove <ref> [<ref>...] | --path <path>", Short: "Remove a worktree", Args: worktreeRemoveArgs(&discoveredPath),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, workspaceID, err := worktreeCommandContext(cmd, deps, workspaceRef)
			if err != nil {
				return err
			}
			if dryRun || len(args) > 1 {
				return runWorktreeRemovalBatch(cmd, client, workspaceID, args, discoveredPath, opts, dryRun)
			}
			if strings.TrimSpace(discoveredPath) != "" {
				// Sent verbatim: Git lists paths exactly, edge whitespace included.
				path := discoveredPath
				if err := client.RemoveDiscoveredWorktree(cmd.Context(), workspaceID, path, opts.Force); err != nil {
					return err
				}
				item := WorktreeRecord{WorkspaceID: workspaceID, Path: path, State: lifecycleRemovedKey}
				return writeCommandOutput(cmd, worktreeMutationBundle(lifecycleRemovedKey, item, nil))
			}
			item, err := resolveWorktreeCommandTarget(cmd.Context(), client, workspaceID, args[0])
			if err != nil {
				return err
			}
			branch, err := client.RemoveWorktree(cmd.Context(), workspaceID, item.ID, opts)
			if err != nil {
				return err
			}
			item.State = lifecycleRemovedKey
			return writeCommandOutput(cmd, worktreeMutationBundle(lifecycleRemovedKey, item, branch))
		},
	}
	addWorktreeWorkspaceFlag(cmd, &workspaceRef)
	cmd.Flags().BoolVar(&opts.Force, "force", false,
		"Confirm destructive removal; with --delete-branch, also delete an unmerged branch")
	addWorktreeDeleteBranchFlag(cmd, &opts.DeleteBranch)
	cmd.Flags().StringVar(&discoveredPath, "path", "",
		"Remove a discovered (not adopted) worktree by its listed path; its branch is kept")
	cmd.Flags().
		BoolVar(&dryRun, "dry-run", false, "Preview per-target risk without removing worktrees or running hooks")
	cmd.MarkFlagsMutuallyExclusive("path", "delete-branch")
	return cmd
}

// worktreeRemoveArgs accepts registered references or one discovered --path.
func worktreeRemoveArgs(discoveredPath *string) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if strings.TrimSpace(*discoveredPath) == "" {
			if len(args) == 0 {
				return fmt.Errorf("cli: %q requires at least one worktree reference", cmd.CommandPath())
			}
			for _, ref := range args {
				if strings.TrimSpace(ref) == "" {
					return fmt.Errorf("cli: worktree reference cannot be blank")
				}
			}
			return nil
		}
		if len(args) != 0 {
			return fmt.Errorf("cli: %q accepts either <ref> or --path, not both", cmd.CommandPath())
		}
		return nil
	}
}

func newWorktreeDismissCommand(deps commandDeps) *cobra.Command {
	var workspaceRef string
	var opts WorktreeCleanupOptions
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
			branch, err := client.DismissWorktree(cmd.Context(), workspaceID, item.ID, opts)
			if err != nil {
				return err
			}
			item.State = lifecycleDismissedKey
			return writeCommandOutput(cmd, worktreeMutationBundle(lifecycleDismissedKey, item, branch))
		},
	}
	addWorktreeWorkspaceFlag(cmd, &workspaceRef)
	addWorktreeDeleteBranchFlag(cmd, &opts.DeleteBranch)
	cmd.Flags().BoolVar(&opts.Force, "force", false, "With --delete-branch, also delete an unmerged branch")
	return cmd
}

func addWorktreeDeleteBranchFlag(cmd *cobra.Command, target *bool) {
	cmd.Flags().BoolVar(target, "delete-branch", false,
		"Also delete the local branch; root HEAD and default branches are kept, unmerged ones need --force")
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

func runWorktreeRemovalBatch(
	cmd *cobra.Command, client DaemonClient, workspaceID string, refs []string, path string,
	opts WorktreeCleanupOptions, dryRun bool,
) error {
	request := contract.RemoveWorktreesRequest{Force: opts.Force, DeleteBranch: opts.DeleteBranch, DryRun: dryRun}
	for _, ref := range refs {
		if ref = strings.TrimSpace(ref); ref != "" {
			request.Targets = append(request.Targets, contract.WorktreeRemovalTarget{Ref: ref})
		}
	}
	if strings.TrimSpace(path) != "" {
		request.Targets = append(request.Targets, contract.WorktreeRemovalTarget{Path: path})
	}
	response, err := client.RemoveWorktrees(cmd.Context(), workspaceID, request)
	if err != nil {
		return err
	}
	if err := writeCommandOutput(cmd, worktreeRemovalBatchBundle(response)); err != nil {
		return err
	}
	if !dryRun {
		for _, result := range response.Results {
			if result.State != lifecycleRemovedKey || result.Code != "" {
				return fmt.Errorf("cli: one or more worktrees could not be removed; inspect per-target results")
			}
		}
	}
	return nil
}
