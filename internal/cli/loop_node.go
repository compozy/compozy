package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/agentidentity"
	"github.com/compozy/compozy/internal/api/contract"
	"github.com/spf13/cobra"
)

const (
	loopNodeIDKey = "node"
	loopModeKey   = "mode"
	loopReasonKey = "reason"
	loopItemKey   = "item"
)

func newLoopNodeCommand(deps commandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   loopNodeKey,
		Short: "Manage one authored Loop node",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(newLoopNodePauseCommand(deps))
	cmd.AddCommand(newLoopNodeResumeCommand(deps))
	cmd.AddCommand(newLoopNodeSimpleCommand(deps, loopCancelKey, "Cancel one Loop node"))
	cmd.AddCommand(newLoopNodeSimpleCommand(deps, loopKillKey, "Kill one Loop node"))
	cmd.AddCommand(newLoopNodeSimpleCommand(deps, loopRequeueKey, "Requeue one quarantined Loop node"))
	return cmd
}

func newLoopNodePauseCommand(deps commandDeps) *cobra.Command {
	var workspaceRef, runID, nodeID, mode, reason string
	cmd := &cobra.Command{
		Use:   loopPauseKey,
		Short: "Pause one Loop node",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, workspaceID, err := loopClientAndWorkspace(cmd, deps, workspaceRef)
			if err != nil {
				return err
			}
			response, err := client.PauseLoopNode(
				cmd.Context(),
				workspaceID,
				strings.TrimSpace(runID),
				strings.TrimSpace(nodeID),
				contract.LoopNodePauseRequest{Mode: strings.TrimSpace(mode), Reason: strings.TrimSpace(reason)},
				agentCredentialsFromEnv(deps),
			)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, loopMutationOutputBundle(response, loopPauseKey))
		},
	}
	addLoopNodeIdentityFlags(cmd, &workspaceRef, &runID, &nodeID)
	cmd.Flags().StringVar(&mode, loopModeKey, "drain", "Pause mode: drain or cancel")
	cmd.Flags().StringVar(&reason, loopReasonKey, "", "Operator reason")
	return cmd
}

func newLoopNodeResumeCommand(deps commandDeps) *cobra.Command {
	var workspaceRef, runID, nodeID, mode, payload string
	var itemIndex int
	cmd := &cobra.Command{
		Use:   loopResumeKey,
		Short: "Resume one Loop node or manual wait",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, workspaceID, err := loopClientAndWorkspace(cmd, deps, workspaceRef)
			if err != nil {
				return err
			}
			request, err := loopNodeResumeRequest(cmd, mode, payload, itemIndex)
			if err != nil {
				return err
			}
			response, err := client.ResumeLoopNode(
				cmd.Context(),
				workspaceID,
				strings.TrimSpace(runID),
				strings.TrimSpace(nodeID),
				request,
				agentCredentialsFromEnv(deps),
			)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, loopMutationOutputBundle(response, loopResumeKey))
		},
	}
	addLoopNodeIdentityFlags(cmd, &workspaceRef, &runID, &nodeID)
	cmd.Flags().StringVar(&mode, loopModeKey, "plain", "Resume mode: plain, reset_attempts, or immediate")
	cmd.Flags().StringVar(&payload, loopPayloadKey, "", "JSON payload for a manual wait")
	cmd.Flags().IntVar(&itemIndex, loopItemKey, 0, "Wait item index")
	return cmd
}

func newLoopNodeSimpleCommand(deps commandDeps, verb string, short string) *cobra.Command {
	var workspaceRef, runID, nodeID, reason string
	cmd := &cobra.Command{
		Use:   verb,
		Short: short,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, workspaceID, err := loopClientAndWorkspace(cmd, deps, workspaceRef)
			if err != nil {
				return err
			}
			response, err := executeLoopNodeSimpleAction(
				cmd,
				client,
				verb,
				workspaceID,
				strings.TrimSpace(runID),
				strings.TrimSpace(nodeID),
				contract.LoopNodeMutationRequest{Reason: strings.TrimSpace(reason)},
				agentCredentialsFromEnv(deps),
			)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, loopMutationOutputBundle(response, verb))
		},
	}
	addLoopNodeIdentityFlags(cmd, &workspaceRef, &runID, &nodeID)
	cmd.Flags().StringVar(&reason, loopReasonKey, "", "Operator reason")
	return cmd
}

func executeLoopNodeSimpleAction(
	cmd *cobra.Command,
	client loopCommandClient,
	verb string,
	workspaceID string,
	runID string,
	nodeID string,
	request contract.LoopNodeMutationRequest,
	credentials agentidentity.Credentials,
) (contract.LoopMutationResponse, error) {
	switch verb {
	case loopCancelKey:
		return client.CancelLoopNode(cmd.Context(), workspaceID, runID, nodeID, request, credentials)
	case loopKillKey:
		return client.KillLoopNode(cmd.Context(), workspaceID, runID, nodeID, request, credentials)
	case loopRequeueKey:
		return client.RequeueLoopNode(cmd.Context(), workspaceID, runID, nodeID, request, credentials)
	default:
		return contract.LoopMutationResponse{}, fmt.Errorf("cli: unsupported Loop node action %q", verb)
	}
}

func addLoopNodeIdentityFlags(
	cmd *cobra.Command,
	workspaceRef *string,
	runID *string,
	nodeID *string,
) {
	cmd.Flags().StringVar(workspaceRef, loopWorkspaceKey, "", "Override workspace (ID, name, or path)")
	cmd.Flags().StringVar(runID, loopRunIDKey, "", "Loop run ID")
	cmd.Flags().StringVar(nodeID, loopNodeIDKey, "", "Loop node ID")
	mustMarkFlagRequired(cmd, loopRunIDKey)
	mustMarkFlagRequired(cmd, loopNodeIDKey)
}

func loopNodeResumeRequest(
	cmd *cobra.Command,
	mode string,
	payload string,
	itemIndex int,
) (contract.LoopNodeResumeRequest, error) {
	request := contract.LoopNodeResumeRequest{Mode: strings.TrimSpace(mode)}
	trimmedPayload := strings.TrimSpace(payload)
	if trimmedPayload != "" {
		if !json.Valid([]byte(trimmedPayload)) {
			return contract.LoopNodeResumeRequest{}, errors.New("cli: --payload must be valid JSON")
		}
		request.Payload = json.RawMessage(trimmedPayload)
	}
	if cmd.Flags().Changed(loopItemKey) {
		if itemIndex < 0 {
			return contract.LoopNodeResumeRequest{}, errors.New("cli: --item must be nonnegative")
		}
		request.ItemIndex = &itemIndex
	}
	return request, nil
}
