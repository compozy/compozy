package cli

import (
	"errors"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/spf13/cobra"
)

func newWorktreeExitCommand(deps commandDeps) *cobra.Command {
	var workspaceRef string
	cmd := &cobra.Command{
		Use: "exit <ref>", Short: "Read the worktree exit plan", Args: exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, workspaceID, err := worktreeCommandContext(cmd, deps, workspaceRef)
			if err != nil {
				return err
			}
			plan, err := client.GetWorktreeExitPlan(cmd.Context(), workspaceID, args[0])
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, worktreeExitPlanBundle(plan))
		},
	}
	addWorktreeWorkspaceFlag(cmd, &workspaceRef)
	return cmd
}

func newWorktreeCommitCommand(deps commandDeps) *cobra.Command {
	var workspaceRef, message string
	var push bool
	cmd := &cobra.Command{
		Use: "commit <ref>", Short: "Commit worktree changes", Args: exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			action := contract.WorktreeExitAction("commit")
			if push {
				action = worktreeCommitPushAction
			}
			return runWorktreeExitCommand(cmd, deps, workspaceRef, args[0], WorktreeExitActionRequest{
				Action: action, Message: message,
			})
		},
	}
	addWorktreeWorkspaceFlag(cmd, &workspaceRef)
	cmd.Flags().StringVarP(&message, "message", "m", "", "Commit message")
	cmd.Flags().BoolVar(&push, "push", false, "Push after committing")
	return cmd
}

func newWorktreePushCommand(deps commandDeps) *cobra.Command {
	var workspaceRef string
	cmd := &cobra.Command{
		Use: "push <ref>", Short: "Push the worktree branch", Args: exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWorktreeExitCommand(cmd, deps, workspaceRef, args[0], WorktreeExitActionRequest{
				Action: worktreePushAction,
			})
		},
	}
	addWorktreeWorkspaceFlag(cmd, &workspaceRef)
	return cmd
}

func newWorktreePRCommand(deps commandDeps) *cobra.Command {
	var workspaceRef, title, body, base string
	var draft bool
	cmd := &cobra.Command{
		Use: "pr <ref>", Short: "Open a pull request for the worktree", Args: exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWorktreeExitCommand(cmd, deps, workspaceRef, args[0], WorktreeExitActionRequest{
				Action: "open_pr", Title: title, Body: body, Draft: draft, Base: base,
			})
		},
	}
	addWorktreeWorkspaceFlag(cmd, &workspaceRef)
	cmd.Flags().StringVar(&title, "title", "", "Pull request title")
	cmd.Flags().StringVar(&body, "body", "", "Pull request body")
	cmd.Flags().BoolVar(&draft, "draft", false, "Open as a draft")
	cmd.Flags().StringVar(&base, "base", "", "Pull request base branch")
	return cmd
}

func newWorktreeExitCancelCommand(deps commandDeps) *cobra.Command {
	var workspaceRef, opID string
	cmd := &cobra.Command{
		Use: "exit-cancel <ref>", Short: "Cancel a running worktree exit action", Args: exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			opID = strings.TrimSpace(opID)
			if opID == "" {
				return errors.New("cli: --op is required")
			}
			client, workspaceID, err := worktreeCommandContext(cmd, deps, workspaceRef)
			if err != nil {
				return err
			}
			item, err := resolveWorktreeCommandTarget(cmd.Context(), client, workspaceID, args[0])
			if err != nil {
				return err
			}
			if err := client.CancelWorktreeExitAction(cmd.Context(), workspaceID, item.ID, opID); err != nil {
				return err
			}
			return writeCommandOutput(cmd, worktreeExitCancelBundle(item.ID, opID))
		},
	}
	addWorktreeWorkspaceFlag(cmd, &workspaceRef)
	cmd.Flags().StringVar(&opID, "op", "", "Exit operation ID")
	return cmd
}

func runWorktreeExitCommand(
	cmd *cobra.Command,
	deps commandDeps,
	workspaceRef string,
	ref string,
	request WorktreeExitActionRequest,
) error {
	client, workspaceID, err := worktreeCommandContext(cmd, deps, workspaceRef)
	if err != nil {
		return err
	}
	operation, err := client.RunWorktreeExitAction(cmd.Context(), workspaceID, ref, request)
	if err != nil {
		return err
	}
	return writeCommandOutput(cmd, worktreeExitOperationBundle(operation))
}
