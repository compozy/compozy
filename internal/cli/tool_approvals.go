package cli

import (
	"context"
	"strings"

	toolspkg "github.com/compozy/compozy/internal/tools"
	"github.com/spf13/cobra"
)

func newToolApprovalsCommand(deps commandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "approvals",
		Short: "Manage remembered native-tool approval decisions",
	}
	cmd.AddCommand(newToolApprovalsSetCommand(deps))
	cmd.AddCommand(newToolApprovalsListCommand(deps))
	cmd.AddCommand(newToolApprovalsRevokeCommand(deps))
	return cmd
}

func newToolApprovalsSetCommand(deps commandDeps) *cobra.Command {
	var workspaceID string
	var decision string
	var scope string
	var agentName string
	cmd := &cobra.Command{
		Use:   "set <tool_id>",
		Short: "Set an explicit agent-wide or tool-wide approval decision",
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runToolCommand(cmd, deps, func(client DaemonClient) error {
				resolvedWorkspaceID, err := resolveToolApprovalWorkspace(
					cmd.Context(),
					cmd,
					deps,
					client,
					workspaceID,
				)
				if err != nil {
					return err
				}
				request := toolspkg.ApprovalGrantSetRequest{
					ToolID:    toolspkg.ToolID(strings.TrimSpace(args[0])),
					Decision:  toolspkg.ApprovalGrantDecision(decision),
					Scope:     toolspkg.ApprovalGrantManagementScope(scope),
					AgentName: agentName,
				}.Normalize()
				if _, err := request.BuildGrant(resolvedWorkspaceID); err != nil {
					return err
				}
				grant, err := client.SetToolApprovalGrant(
					cmd.Context(),
					resolvedWorkspaceID,
					ToolApprovalGrantSetRequest{
						ToolID:    request.ToolID,
						Decision:  request.Decision,
						Scope:     request.Scope,
						AgentName: request.AgentName,
					},
				)
				if err != nil {
					return err
				}
				return writeCommandOutput(cmd, toolApprovalGrantSetBundle(grant))
			})
		},
	}
	cmd.Flags().StringVar(&workspaceID, "workspace", "", "Override workspace (ID, name, or path)")
	cmd.Flags().StringVar(&decision, "decision", "", "Remembered decision: allow or reject")
	cmd.Flags().StringVar(&scope, "scope", "", "Wider scope: agent or tool")
	cmd.Flags().StringVar(&agentName, "agent", "", "Agent name required by agent scope")
	return cmd
}

func newToolApprovalsListCommand(deps commandDeps) *cobra.Command {
	var workspaceID string
	cmd := &cobra.Command{
		Use:   agentListKey,
		Short: "List remembered native-tool approval decisions",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runToolCommand(cmd, deps, func(client DaemonClient) error {
				resolvedWorkspaceID, err := resolveToolApprovalWorkspace(
					cmd.Context(),
					cmd,
					deps,
					client,
					workspaceID,
				)
				if err != nil {
					return err
				}
				response, err := client.ListToolApprovalGrants(cmd.Context(), resolvedWorkspaceID)
				if err != nil {
					return err
				}
				return writeCommandOutput(cmd, toolApprovalGrantListBundle(response))
			})
		},
	}
	cmd.Flags().StringVar(&workspaceID, "workspace", "", "Override workspace (ID, name, or path)")
	return cmd
}

func newToolApprovalsRevokeCommand(deps commandDeps) *cobra.Command {
	var workspaceID string
	cmd := &cobra.Command{
		Use:   "revoke <grant_id>",
		Short: "Revoke one remembered native-tool approval decision",
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := strings.TrimSpace(args[0])
			return runToolCommand(cmd, deps, func(client DaemonClient) error {
				resolvedWorkspaceID, err := resolveToolApprovalWorkspace(
					cmd.Context(),
					cmd,
					deps,
					client,
					workspaceID,
				)
				if err != nil {
					return err
				}
				if err := client.RevokeToolApprovalGrant(cmd.Context(), resolvedWorkspaceID, id); err != nil {
					return err
				}
				return writeCommandOutput(cmd, toolApprovalGrantRevokeBundle(resolvedWorkspaceID, id))
			})
		},
	}
	cmd.Flags().StringVar(&workspaceID, "workspace", "", "Override workspace (ID, name, or path)")
	return cmd
}

func resolveToolApprovalWorkspace(
	ctx context.Context,
	cmd *cobra.Command,
	deps commandDeps,
	client DaemonClient,
	workspaceRef string,
) (string, error) {
	resolution, err := resolveCommandWorkspace(
		ctx,
		cmd,
		deps,
		client,
		workspaceResolutionRequest{FlagRef: workspaceRef},
	)
	if err != nil {
		return "", err
	}
	return resolution.ID, nil
}

func toolApprovalGrantListBundle(response ToolApprovalGrantListRecord) outputBundle {
	return listBundle(
		response,
		response.Grants,
		"Remembered Tool Approvals",
		[]string{"ID", toolOperatorToolIDHeader, "AGENT", "DECISION", "INPUT DIGEST", "LAST USED"},
		"tool_approval_grants",
		[]string{"id", toolOperatorToolIDKey, installAgentNameKey, "decision", "input_digest", "last_used_at"},
		func(grant ToolApprovalGrantRecord) []string {
			return []string{
				grant.ID,
				grant.ToolID.String(),
				stringOrDash(grant.AgentName),
				string(grant.Decision),
				stringOrDash(grant.InputDigest),
				formatTime(grant.LastUsedAt),
			}
		},
		func(grant ToolApprovalGrantRecord) []string {
			return []string{
				grant.ID,
				grant.ToolID.String(),
				grant.AgentName,
				string(grant.Decision),
				grant.InputDigest,
				formatTime(grant.LastUsedAt),
			}
		},
	)
}

func toolApprovalGrantSetBundle(grant ToolApprovalGrantRecord) outputBundle {
	scope := toolToolKey
	if grant.AgentName != "" {
		scope = agentAgentKey
	}
	return outputBundle{
		jsonValue: grant,
		human: func() (string, error) {
			return renderHumanSection("Remembered Tool Approval Set", []keyValue{
				{Label: "Grant ID", Value: grant.ID},
				{Label: "Workspace", Value: grant.WorkspaceID},
				{Label: "Tool ID", Value: grant.ToolID.String()},
				{Label: "Decision", Value: string(grant.Decision)},
				{Label: automationScopeValue, Value: scope},
				{Label: agentOutputLabel, Value: stringOrDash(grant.AgentName)},
			}), nil
		},
		toon: func() (string, error) {
			return renderToonObject(
				"tool_approval_grant",
				[]string{
					"id", taskWorkspaceIDKey, "tool_id", "decision", bridgeScopeKey, installAgentNameKey,
				},
				[]string{
					grant.ID,
					grant.WorkspaceID,
					grant.ToolID.String(),
					string(grant.Decision),
					scope,
					grant.AgentName,
				},
			), nil
		},
	}
}

func toolApprovalGrantRevokeBundle(workspaceID string, id string) outputBundle {
	payload := struct {
		RevokedID   string `json:"revoked_id"`
		WorkspaceID string `json:"workspace_id"`
	}{RevokedID: id, WorkspaceID: workspaceID}
	return outputBundle{
		jsonValue: payload,
		human: func() (string, error) {
			return renderHumanSection("Remembered Tool Approval Revoked", []keyValue{
				{Label: "Grant ID", Value: id},
				{Label: "Workspace", Value: workspaceID},
			}), nil
		},
		toon: func() (string, error) {
			return renderToonObject(
				"tool_approval_grant_revoked",
				[]string{"revoked_id", taskWorkspaceIDKey},
				[]string{id, workspaceID},
			), nil
		},
	}
}
