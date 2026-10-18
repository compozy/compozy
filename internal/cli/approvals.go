package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	toolspkg "github.com/compozy/compozy/internal/tools"
	"github.com/spf13/cobra"
)

func newApprovalsCommand(deps commandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   observeApprovalsLabel,
		Short: "Inspect, decide, or cancel pending tool approvals",
	}
	cmd.AddCommand(
		newApprovalShowCommand(deps),
		newApprovalResolveCommand(deps),
		newApprovalCancelCommand(deps),
	)
	return cmd
}

func newApprovalShowCommand(deps commandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show <id>",
		Short: "Show one tool approval lifecycle",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			approvalID, err := requiredCmdPaletteID(args[0], "approval ID")
			if err != nil {
				return err
			}
			client, err := cmdPaletteClientFromDeps(deps)
			if err != nil {
				return err
			}
			status, err := client.GetPendingToolApproval(cmd.Context(), approvalID)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, approvalStatusOutput(status))
		},
	}
	configureSingleProfileCommand(cmd, deps)
	return cmd
}

func newApprovalCancelCommand(deps commandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cancel <id>",
		Short: "Cancel one pending tool approval",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			approvalID, err := requiredCmdPaletteID(args[0], "approval ID")
			if err != nil {
				return err
			}
			client, err := cmdPaletteClientFromDeps(deps)
			if err != nil {
				return err
			}
			status, err := client.CancelPendingToolApproval(cmd.Context(), approvalID)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, approvalStatusOutput(status))
		},
	}
	configureProfileMutationCommand(cmd, deps)
	return cmd
}

func newApprovalResolveCommand(deps commandDeps) *cobra.Command {
	var decision string
	cmd := &cobra.Command{
		Use: "resolve <id>", Short: "Approve or deny one pending tool approval", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			approvalID, err := requiredCmdPaletteID(args[0], "approval ID")
			if err != nil {
				return err
			}
			outcome := toolspkg.ApprovalOutcome(strings.TrimSpace(decision))
			if outcome != toolspkg.ApprovalApproved && outcome != toolspkg.ApprovalDenied {
				return withCommandExitCode(2, fmt.Errorf("cli: decision must be approved or denied"))
			}
			client, err := cmdPaletteClientFromDeps(deps)
			if err != nil {
				return err
			}
			status, err := client.ResolvePendingToolApproval(cmd.Context(), approvalID,
				contract.ResolveToolApprovalRequest{Decision: outcome})
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, approvalStatusOutput(status))
		},
	}
	cmd.Flags().StringVar(&decision, "decision", "", "Decision: approved or denied")
	mustMarkFlagRequired(cmd, "decision")
	configureProfileMutationCommand(cmd, deps)
	return cmd
}

func cmdPaletteClientFromDeps(deps commandDeps) (CmdPaletteClient, error) {
	client, err := clientFromDeps(deps)
	if err != nil {
		return nil, err
	}
	paletteClient, ok := client.(CmdPaletteClient)
	if !ok {
		return nil, fmt.Errorf("cli: command palette client is unavailable")
	}
	return paletteClient, nil
}

func approvalStatusOutput(status contract.ToolApprovalStatusResponse) outputBundle {
	expiresAt := ""
	if status.ExpiresAt != nil {
		expiresAt = status.ExpiresAt.Format(time.RFC3339)
	}
	return outputBundle{
		jsonValue: status,
		jsonl:     func(cmd *cobra.Command) error { return writeJSONLine(cmd, status) },
		human: func() (string, error) {
			encoded, err := json.Marshal(status)
			if err != nil {
				return "", fmt.Errorf("cli: encode tool approval output: %w", err)
			}
			return string(encoded), nil
		},
		toon: func() (string, error) {
			return renderToonObject(
				"approval",
				[]string{"approval_status", "execution_status", mcpAuthExpiresAtKey},
				[]string{
					string(status.ApprovalStatus), string(status.ExecutionStatus), expiresAt,
				},
			), nil
		},
	}
}
