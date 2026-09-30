package cli

import (
	"context"
	"errors"

	"github.com/compozy/compozy/internal/agentidentity"
	"github.com/spf13/cobra"
)

func newWorktreeDeliverCommand(deps commandDeps) *cobra.Command {
	var workspaceRef string
	request := WorktreeExitActionRequest{Action: "deliver", Draft: true}
	cmd := &cobra.Command{
		Use:   "deliver <ref>",
		Short: "Submit a reviewed draft delivery from the bound managed session",
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, workspaceID, err := worktreeCommandContext(cmd, deps, workspaceRef)
			if err != nil {
				return err
			}
			credentials, err := requireAgentCommandIdentity(cmd.Context(), deps, client, "agent.worktree.deliver")
			if err != nil {
				return err
			}
			delivery, ok := client.(interface {
				SubmitWorktreeDelivery(
					context.Context, string, string, WorktreeExitActionRequest, agentidentity.Credentials,
				) (WorktreeExitOperationRecord, error)
			})
			if !ok {
				return errors.New("cli: managed worktree delivery unavailable")
			}
			operation, err := delivery.SubmitWorktreeDelivery(cmd.Context(), workspaceID, args[0], request, credentials)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, worktreeExitOperationBundle(operation))
		},
	}
	addWorktreeWorkspaceFlag(cmd, &workspaceRef)
	cmd.Flags().StringVar(&request.DeliveryID, "delivery-id", "", "Stable delivery identifier reused for retries")
	cmd.Flags().StringVar(&request.ExpectedHead, "expected-head", "", "Reviewed branch HEAD")
	cmd.Flags().StringVarP(&request.Message, "message", "m", "", "Commit message")
	cmd.Flags().StringVar(&request.Title, "title", "", "Draft pull request title")
	cmd.Flags().StringVar(&request.Body, "body", "", "Draft pull request body")
	cmd.Flags().StringVar(&request.Base, "base", "", "Pull request base branch")
	cmd.Flags().
		StringArrayVar(&request.IncludePaths, "include", nil, "Exact reviewed worktree-relative file (repeatable)")
	cmd.Flags().StringVar(&request.ExpectedScope, "expected-scope", "", "Fingerprint from the scoped exit plan")
	for _, flag := range []string{"delivery-id", "expected-head", "message", "base", "include", "expected-scope"} {
		if err := cmd.MarkFlagRequired(flag); err != nil {
			panic("invariant: registered delivery flag missing")
		}
	}
	return cmd
}
