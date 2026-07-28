package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

const (
	networkInvitationScopeWorkspace = "workspace"
	networkInvitationScopeTask      = "task"
)

func registerNetworkPublicSurfaceCommands(cmd *cobra.Command, deps commandDeps, workspaceRef *string) {
	cmd.AddCommand(newNetworkCoordinationCommand(deps, workspaceRef))
	cmd.AddCommand(newNetworkInvitationCommand(deps, workspaceRef))
	cmd.AddCommand(newNetworkUsageCommand(deps, workspaceRef))
}

func newNetworkCoordinationCommand(deps commandDeps, workspaceRef *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "coordination",
		Short: "Inspect or change workspace network coordination",
	}
	cmd.AddCommand(newNetworkCoordinationStatusCommand(deps, workspaceRef))
	cmd.AddCommand(newNetworkCoordinationEnableCommand(deps, workspaceRef))
	cmd.AddCommand(newNetworkCoordinationDisableCommand(deps, workspaceRef))
	return cmd
}

func newNetworkCoordinationStatusCommand(deps commandDeps, workspaceRef *string) *cobra.Command {
	var scope string
	var taskID string
	var runID string
	cmd := &cobra.Command{
		Use:   configStatusKey,
		Short: "Show workspace network coordination and invitation state",
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			workspaceID, err := resolveNetworkWorkspaceRef(cmd, deps, client, workspaceRef)
			if err != nil {
				return err
			}
			ref, err := normalizeNetworkCoordinationRef(scope, taskID, runID)
			if err != nil {
				return err
			}
			payload, err := client.GetNetworkCoordination(cmd.Context(), workspaceID, ref)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, coordinationOutputBundle(payload))
		},
	}
	addNetworkCoordinationScopeFlags(cmd, &scope, &taskID, &runID)
	return cmd
}

func newNetworkCoordinationEnableCommand(deps commandDeps, workspaceRef *string) *cobra.Command {
	return newNetworkCoordinationToggleCommand(
		deps,
		workspaceRef,
		true,
		"enable",
		"Enable workspace coordination conversations",
	)
}

func newNetworkCoordinationDisableCommand(deps commandDeps, workspaceRef *string) *cobra.Command {
	return newNetworkCoordinationToggleCommand(
		deps,
		workspaceRef,
		false,
		"disable",
		"Disable workspace coordination conversations",
	)
}

func newNetworkCoordinationToggleCommand(
	deps commandDeps,
	workspaceRef *string,
	enabled bool,
	use string,
	short string,
) *cobra.Command {
	var scope string
	var taskID string
	var runID string
	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			workspaceID, err := resolveNetworkWorkspaceRef(cmd, deps, client, workspaceRef)
			if err != nil {
				return err
			}
			ref, err := normalizeNetworkCoordinationRef(scope, taskID, runID)
			if err != nil {
				return err
			}
			current, err := client.GetNetworkCoordination(cmd.Context(), workspaceID, ref)
			if err != nil {
				return err
			}
			payload, err := client.PutNetworkCoordination(
				cmd.Context(),
				workspaceID,
				PutNetworkCoordinationRequest{
					Scope:            ref.Scope,
					TaskID:           ref.TaskID,
					RunID:            ref.RunID,
					Enabled:          &enabled,
					ExpectedRevision: &current.Revision,
				},
			)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, coordinationOutputBundle(payload))
		},
	}
	addNetworkCoordinationScopeFlags(cmd, &scope, &taskID, &runID)
	return cmd
}

func newNetworkInvitationCommand(deps commandDeps, workspaceRef *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "invitation",
		Short: "Dismiss or reset the coordination invitation",
	}
	cmd.AddCommand(newNetworkInvitationDismissCommand(deps, workspaceRef))
	cmd.AddCommand(newNetworkInvitationResetCommand(deps, workspaceRef))
	return cmd
}

func newNetworkInvitationDismissCommand(deps commandDeps, workspaceRef *string) *cobra.Command {
	return newNetworkInvitationMutationCommand(
		deps,
		workspaceRef,
		true,
		"dismiss",
		"Dismiss the coordination invitation for a scope",
	)
}

func newNetworkInvitationResetCommand(deps commandDeps, workspaceRef *string) *cobra.Command {
	return newNetworkInvitationMutationCommand(
		deps,
		workspaceRef,
		false,
		"reset",
		"Reset the coordination invitation for a scope",
	)
}

func newNetworkInvitationMutationCommand(
	deps commandDeps,
	workspaceRef *string,
	dismissed bool,
	use string,
	short string,
) *cobra.Command {
	var scope string
	var taskID string
	var runID string
	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			workspaceID, err := resolveNetworkWorkspaceRef(cmd, deps, client, workspaceRef)
			if err != nil {
				return err
			}
			ref, err := normalizeNetworkCoordinationRef(scope, taskID, runID)
			if err != nil {
				return err
			}
			current, err := client.GetNetworkCoordination(cmd.Context(), workspaceID, ref)
			if err != nil {
				return err
			}
			payload, err := client.PutNetworkCoordinationInvitation(
				cmd.Context(),
				workspaceID,
				PutNetworkCoordinationInvitationRequest{
					Scope:            ref.Scope,
					TaskID:           ref.TaskID,
					RunID:            ref.RunID,
					Dismissed:        &dismissed,
					ExpectedRevision: &current.Revision,
				},
			)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, coordinationOutputBundle(payload))
		},
	}
	addNetworkCoordinationScopeFlags(cmd, &scope, &taskID, &runID)
	return cmd
}

func addNetworkCoordinationScopeFlags(
	cmd *cobra.Command,
	scope *string,
	taskID *string,
	runID *string,
) {
	cmd.Flags().StringVar(scope, "scope", networkInvitationScopeWorkspace, "Coordination scope: workspace or task")
	cmd.Flags().StringVar(taskID, "task-id", "", "Task id when --scope=task")
	cmd.Flags().StringVar(runID, "run-id", "", "Task run id for server-owned invitation eligibility")
}

func normalizeNetworkCoordinationRef(scope string, taskID string, runID string) (NetworkCoordinationRef, error) {
	normalized := NetworkCoordinationRef{
		Scope:  strings.TrimSpace(strings.ToLower(scope)),
		TaskID: strings.TrimSpace(taskID),
		RunID:  strings.TrimSpace(runID),
	}
	if normalized.Scope != networkInvitationScopeWorkspace && normalized.Scope != networkInvitationScopeTask {
		return NetworkCoordinationRef{}, fmt.Errorf("cli: --scope must be workspace or task")
	}
	if normalized.Scope == networkInvitationScopeWorkspace && normalized.TaskID != "" {
		return NetworkCoordinationRef{}, fmt.Errorf("cli: --task-id requires --scope=task")
	}
	if normalized.Scope == networkInvitationScopeTask && normalized.TaskID == "" {
		return NetworkCoordinationRef{}, fmt.Errorf("cli: --task-id is required when --scope=task")
	}
	return normalized, nil
}
