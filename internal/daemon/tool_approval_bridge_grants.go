package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	acpsdk "github.com/coder/acp-go-sdk"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

func (b *toolApprovalBridge) consumeDurableToolApproval(
	ctx context.Context,
	scope toolspkg.Scope,
	call toolspkg.CallRequest,
	toolID toolspkg.ToolID,
) (bool, error) {
	if b == nil || b.grants == nil {
		return false, nil
	}
	key, err := toolApprovalGrantKey(scope, call, toolID)
	if err != nil {
		b.logger.Warn("daemon: skip invalid durable tool approval lookup", "tool_id", toolID, "error", err)
		return false, nil
	}
	grant, found, err := b.grants.LookupApprovalGrant(ctx, key)
	if err != nil {
		b.logger.Warn(
			"daemon: durable tool approval lookup failed open",
			"workspace_id", key.WorkspaceID,
			"agent_name", key.AgentName,
			"tool_id", key.ToolID,
			"error", err,
		)
		return false, nil
	}
	if !found {
		return false, nil
	}
	if terminalApprovalRequiresExactGrant(toolID) &&
		(grant.AgentName != key.AgentName || grant.InputDigest != key.InputDigest) {
		return false, nil
	}
	switch grant.Decision {
	case toolspkg.ApprovalGrantAllow:
		return true, nil
	case toolspkg.ApprovalGrantReject:
		return true, toolApprovalError(toolID, "tool approval was rejected", toolspkg.ReasonApprovalRequired)
	default:
		b.logger.Warn(
			"daemon: ignore invalid durable tool approval decision",
			"grant_id", grant.ID,
			"decision", grant.Decision,
		)
		return false, nil
	}
}

func (b *toolApprovalBridge) applyToolApprovalOutcome(
	ctx context.Context,
	scope toolspkg.Scope,
	call toolspkg.CallRequest,
	toolID toolspkg.ToolID,
	outcome acpsdk.RequestPermissionOutcome,
	remember bool,
) error {
	if err := outcome.Validate(); err != nil {
		return toolApprovalError(toolID, "tool approval returned no outcome", toolspkg.ReasonApprovalUnreachable)
	}
	if outcome.Selected == nil {
		return toolApprovalError(toolID, "tool approval was canceled", toolspkg.ReasonApprovalCanceled)
	}
	switch outcome.Selected.OptionId {
	case toolApprovalAllowOnceID:
		return nil
	case toolApprovalRejectOnceID:
		return toolApprovalError(toolID, "tool approval was rejected", toolspkg.ReasonApprovalRequired)
	case toolApprovalAllowAlwaysID:
		if !remember {
			return toolApprovalError(
				toolID,
				"tool approval selected an unavailable option",
				toolspkg.ReasonApprovalUnreachable,
			)
		}
		return b.persistToolApprovalGrant(ctx, scope, call, toolID, toolspkg.ApprovalGrantAllow)
	case toolApprovalRejectAlwaysID:
		if !remember {
			return toolApprovalError(
				toolID,
				"tool approval selected an unavailable option",
				toolspkg.ReasonApprovalUnreachable,
			)
		}
		if err := b.persistToolApprovalGrant(
			ctx,
			scope,
			call,
			toolID,
			toolspkg.ApprovalGrantReject,
		); err != nil {
			return err
		}
		return toolApprovalError(toolID, "tool approval was rejected", toolspkg.ReasonApprovalRequired)
	default:
		return toolApprovalError(toolID, "tool approval selected an unknown option", toolspkg.ReasonApprovalUnreachable)
	}
}

func (b *toolApprovalBridge) persistToolApprovalGrant(
	ctx context.Context,
	scope toolspkg.Scope,
	call toolspkg.CallRequest,
	toolID toolspkg.ToolID,
	decision toolspkg.ApprovalGrantDecision,
) error {
	if b == nil || b.grants == nil {
		return toolApprovalGrantBackendError(toolID, errors.New("durable approval store is unavailable"))
	}
	key, err := toolApprovalGrantKey(scope, call, toolID)
	if err != nil {
		return toolApprovalGrantBackendError(toolID, err)
	}
	if _, err := b.grants.PutApprovalGrant(ctx, toolspkg.ApprovalGrant{
		ApprovalGrantKey: key,
		Decision:         decision,
	}); err != nil {
		return toolApprovalGrantBackendError(toolID, err)
	}
	return nil
}

func toolApprovalGrantKey(
	scope toolspkg.Scope,
	call toolspkg.CallRequest,
	toolID toolspkg.ToolID,
) (toolspkg.ApprovalGrantKey, error) {
	workspaceID := strings.TrimSpace(call.WorkspaceID)
	if workspaceID == "" {
		workspaceID = strings.TrimSpace(scope.WorkspaceID)
	}
	agentName := strings.TrimSpace(call.AgentName)
	if agentName == "" {
		agentName = strings.TrimSpace(scope.AgentName)
	}
	inputDigest, err := toolApprovalInputDigest(call.Input, toolID)
	if err != nil {
		return toolspkg.ApprovalGrantKey{}, err
	}
	key := toolspkg.ApprovalGrantKey{
		ProfileID:   strings.TrimSpace(scope.ProfileID),
		WorkspaceID: workspaceID,
		AgentName:   agentName,
		ToolID:      toolID,
		InputDigest: inputDigest,
	}.Normalize()
	if err := key.Validate(); err != nil {
		return toolspkg.ApprovalGrantKey{}, err
	}
	return key, nil
}

func terminalApprovalRequiresExactGrant(toolID toolspkg.ToolID) bool {
	return toolID == toolspkg.ToolIDTerminalExec || toolID == toolspkg.ToolIDTerminalWrite
}

func toolApprovalInputDigest(input json.RawMessage, toolID toolspkg.ToolID) (string, error) {
	switch toolID {
	case toolspkg.ToolIDTerminalExec:
		var decoded terminalExecInput
		if err := json.Unmarshal(input, &decoded); err != nil {
			return "", fmt.Errorf("decode terminal exec approval shape: %w", err)
		}
		shape, err := json.Marshal(struct {
			Command string            `json:"command"`
			Args    []string          `json:"args,omitempty"`
			Cwd     string            `json:"cwd,omitempty"`
			Env     map[string]string `json:"env,omitempty"`
		}{Command: decoded.Command, Args: decoded.Args, Cwd: decoded.Cwd, Env: decoded.Env})
		if err != nil {
			return "", fmt.Errorf("encode terminal exec approval shape: %w", err)
		}
		return toolspkg.ApprovalInputDigest(shape, "")
	case toolspkg.ToolIDTerminalWrite:
		var decoded struct {
			TerminalID      string `json:"terminal_id"`
			GrantGeneration uint64 `json:"grant_generation"`
		}
		if err := json.Unmarshal(input, &decoded); err != nil {
			return "", fmt.Errorf("decode terminal write approval identity: %w", err)
		}
		identity, err := json.Marshal(decoded)
		if err != nil {
			return "", fmt.Errorf("encode terminal write approval identity: %w", err)
		}
		return toolspkg.ApprovalInputDigest(identity, "")
	default:
		return toolspkg.ApprovalInputDigest(input, "")
	}
}

func toolApprovalGrantBackendError(id toolspkg.ToolID, err error) error {
	return toolspkg.NewToolError(
		toolspkg.ErrorCodeBackendFailed,
		id,
		"tool approval decision could not be remembered",
		fmt.Errorf("%w: persist durable tool approval grant: %v", toolspkg.ErrToolBackendFailed, err),
		toolspkg.ReasonBackendUnhealthy,
		toolspkg.ReasonApprovalRequired,
	)
}
