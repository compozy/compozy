package daemon

import (
	"context"
	"errors"
	"fmt"
	"strings"

	toolspkg "github.com/compozy/agh/internal/tools"
)

type toolApprovalsListInput struct {
	WorkspaceID string `json:"workspace_id,omitempty"`
}

type toolApprovalsSetInput struct {
	ToolID      toolspkg.ToolID                       `json:"tool_id"`
	Decision    toolspkg.ApprovalGrantDecision        `json:"decision"`
	Scope       toolspkg.ApprovalGrantManagementScope `json:"scope"`
	AgentName   string                                `json:"agent_name,omitempty"`
	WorkspaceID string                                `json:"workspace_id,omitempty"`
}

type toolApprovalsRevokeInput struct {
	ID          string `json:"id"`
	WorkspaceID string `json:"workspace_id,omitempty"`
}

type nativeToolApprovalListResponse struct {
	Grants []toolspkg.ApprovalGrant `json:"grants"`
	Total  int                      `json:"total"`
}

type nativeToolApprovalSetResponse struct {
	Grant toolspkg.ApprovalGrant `json:"grant"`
}

func (n *daemonNativeTools) toolApprovalBindings(
	availability toolspkg.NativeAvailabilityFunc,
) map[toolspkg.ToolID]nativeToolBinding {
	return map[toolspkg.ToolID]nativeToolBinding{
		toolspkg.ToolIDToolApprovalsSet: {
			call:         n.toolApprovalsSet,
			availability: availability,
		},
		toolspkg.ToolIDToolApprovalsList: {
			call:         n.toolApprovalsList,
			availability: availability,
		},
		toolspkg.ToolIDToolApprovalsRevoke: {
			call:         n.toolApprovalsRevoke,
			availability: availability,
		},
	}
}

func (n *daemonNativeTools) toolApprovalsSet(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input toolApprovalsSetInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	workspaceID, err := nativeToolApprovalWorkspace(req.ToolID, scope.WorkspaceID, input.WorkspaceID)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	grant, err := (toolspkg.ApprovalGrantSetRequest{
		ToolID:    input.ToolID,
		Decision:  input.Decision,
		Scope:     input.Scope,
		AgentName: input.AgentName,
	}).BuildGrant(workspaceID)
	if err != nil {
		return toolspkg.ToolResult{}, nativeToolApprovalGrantError(req.ToolID, err)
	}
	service, err := n.toolApprovalGrantService(req.ToolID)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	stored, err := service.PutApprovalGrant(ctx, grant)
	if err != nil {
		return toolspkg.ToolResult{}, nativeToolApprovalGrantError(req.ToolID, err)
	}
	return structuredResult(
		nativeToolApprovalSetResponse{Grant: stored},
		fmt.Sprintf("Remembered %s-wide tool approval set", input.Scope),
	)
}

func (n *daemonNativeTools) toolApprovalsList(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input toolApprovalsListInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	workspaceID, err := nativeToolApprovalWorkspace(req.ToolID, scope.WorkspaceID, input.WorkspaceID)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	service, err := n.toolApprovalGrantService(req.ToolID)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	grants, err := service.ListApprovalGrants(ctx, workspaceID)
	if err != nil {
		return toolspkg.ToolResult{}, nativeToolApprovalGrantError(req.ToolID, err)
	}
	return structuredResult(
		nativeToolApprovalListResponse{Grants: grants, Total: len(grants)},
		fmt.Sprintf("%d remembered tool approvals", len(grants)),
	)
}

func (n *daemonNativeTools) toolApprovalsRevoke(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input toolApprovalsRevokeInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	input.ID = strings.TrimSpace(input.ID)
	if input.ID == "" {
		return toolspkg.ToolResult{}, nativeToolApprovalValidationError(req.ToolID, "id", "id is required")
	}
	workspaceID, err := nativeToolApprovalWorkspace(req.ToolID, scope.WorkspaceID, input.WorkspaceID)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	service, err := n.toolApprovalGrantService(req.ToolID)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	if err := service.RevokeApprovalGrant(ctx, workspaceID, input.ID); err != nil {
		return toolspkg.ToolResult{}, nativeToolApprovalGrantError(req.ToolID, err)
	}
	return structuredResult(map[string]string{
		"revoked_id":   input.ID,
		"workspace_id": workspaceID,
	}, "Remembered tool approval revoked")
}

func (n *daemonNativeTools) toolApprovalGrantService(
	id toolspkg.ToolID,
) (toolspkg.ApprovalGrantStore, error) {
	if n == nil || n.deps == nil || n.deps.ApprovalGrants == nil {
		return nil, toolspkg.NewToolError(
			toolspkg.ErrorCodeBackendFailed,
			id,
			"tool approval grant service is unavailable",
			fmt.Errorf("%w: durable tool approval service is unavailable", toolspkg.ErrToolBackendFailed),
			toolspkg.ReasonDependencyMissing,
		)
	}
	return n.deps.ApprovalGrants, nil
}

func nativeToolApprovalWorkspace(
	id toolspkg.ToolID,
	scoped string,
	requested string,
) (string, error) {
	scoped = strings.TrimSpace(scoped)
	requested = strings.TrimSpace(requested)
	if scoped != "" && requested != "" && scoped != requested {
		return "", toolspkg.NewToolError(
			toolspkg.ErrorCodeDenied,
			id,
			"tool approval workspace does not match the caller scope",
			fmt.Errorf("%w: workspace_id scope mismatch", toolspkg.ErrToolDenied),
			toolspkg.ReasonScopeMismatch,
		)
	}
	workspaceID := scoped
	if workspaceID == "" {
		workspaceID = requested
	}
	if workspaceID == "" {
		return "", nativeToolApprovalValidationError(id, "workspace_id", "workspace_id is required")
	}
	return workspaceID, nil
}

func nativeToolApprovalValidationError(id toolspkg.ToolID, field, detail string) error {
	return toolspkg.NewToolError(
		toolspkg.ErrorCodeInvalidInput,
		id,
		detail,
		fmt.Errorf(
			"%w: %w",
			toolspkg.ErrToolInvalidInput,
			toolspkg.NewValidationError(field, toolspkg.ReasonSchemaInvalid, detail),
		),
		toolspkg.ReasonSchemaInvalid,
	)
}

func nativeToolApprovalGrantError(id toolspkg.ToolID, err error) error {
	if errors.Is(err, toolspkg.ErrApprovalGrantNotFound) {
		return toolspkg.NewToolError(
			toolspkg.ErrorCodeNotFound,
			id,
			"remembered tool approval not found",
			fmt.Errorf("%w: %w", toolspkg.ErrToolNotFound, toolspkg.ErrApprovalGrantNotFound),
		)
	}
	if errors.Is(err, toolspkg.ErrApprovalGrantInvalid) {
		return nativeToolApprovalValidationError(id, "approval_grant", err.Error())
	}
	return toolspkg.NewToolError(
		toolspkg.ErrorCodeBackendFailed,
		id,
		"remembered tool approval operation failed",
		fmt.Errorf("%w: durable tool approval operation: %v", toolspkg.ErrToolBackendFailed, err),
		toolspkg.ReasonBackendUnhealthy,
	)
}
