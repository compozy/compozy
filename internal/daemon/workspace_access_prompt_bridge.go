package daemon

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	acpsdk "github.com/coder/acp-go-sdk"
	"github.com/compozy/compozy/internal/acp"
	toolspkg "github.com/compozy/compozy/internal/tools"
	"github.com/compozy/compozy/internal/workspaceaccess"
)

const (
	workspaceAccessAllowOnceID     acpsdk.PermissionOptionId = "allow_once"
	workspaceAccessAllowSessionID  acpsdk.PermissionOptionId = "allow_session"
	workspaceAccessRejectOnceID    acpsdk.PermissionOptionId = "reject_once"
	workspaceAccessRejectSessionID acpsdk.PermissionOptionId = "reject_session"
)

type workspaceAccessPrompter interface {
	RequestWorkspaceAccess(
		ctx context.Context,
		scope toolspkg.Scope,
		call toolspkg.CallRequest,
		descriptor toolspkg.Descriptor,
		targetWorkspaceID string,
	) (bool, error)
}

type workspaceAccessPromptBridge struct {
	sessions func() sessionPermissionRequester
	timeout  time.Duration
	consent  workspaceaccess.SessionConsentCache
}

var _ workspaceAccessPrompter = (*workspaceAccessPromptBridge)(nil)

func newWorkspaceAccessPromptBridge(
	sessions func() sessionPermissionRequester,
	timeout time.Duration,
	consent workspaceaccess.SessionConsentCache,
) *workspaceAccessPromptBridge {
	return &workspaceAccessPromptBridge{sessions: sessions, timeout: timeout, consent: consent}
}

func (b *workspaceAccessPromptBridge) RequestWorkspaceAccess(
	ctx context.Context,
	scope toolspkg.Scope,
	call toolspkg.CallRequest,
	descriptor toolspkg.Descriptor,
	targetWorkspaceID string,
) (bool, error) {
	if ctx == nil {
		return false, errors.New("daemon: workspace access prompt context is required")
	}
	if b == nil || b.sessions == nil {
		return false, errors.New("daemon: workspace access prompt channel is unavailable")
	}
	sessions := b.sessions()
	if sessions == nil {
		return false, errors.New("daemon: workspace access prompt channel is unavailable")
	}
	sessionID := strings.TrimSpace(call.SessionID)
	if sessionID == "" {
		sessionID = strings.TrimSpace(scope.SessionID)
	}
	if sessionID == "" {
		return false, errors.New("daemon: workspace access prompt session is unavailable")
	}
	timeout := b.timeout
	if timeout <= 0 {
		timeout = 120 * time.Second
	}
	detached := context.WithoutCancel(ctx)
	promptCtx, cancel := context.WithTimeout(detached, timeout)
	defer cancel()
	response, err := sessions.RequestPermission(promptCtx, sessionID, acp.RequestPermissionRequest{
		SessionId: acpsdk.SessionId(sessionID),
		ToolCall: acpsdk.ToolCallUpdate{
			ToolCallId: acpsdk.ToolCallId(toolApprovalCallID(call, nil)),
			Title:      new(fmt.Sprintf("Access workspace %s", strings.TrimSpace(targetWorkspaceID))),
			RawInput: map[string]any{
				"tool_id":             descriptor.ID.String(),
				"target_workspace_id": strings.TrimSpace(targetWorkspaceID),
			},
			Status: acpsdk.Ptr(acpsdk.ToolCallStatusPending),
		},
		Options: workspaceAccessPromptOptions(),
	})
	if err != nil {
		if errors.Is(promptCtx.Err(), context.DeadlineExceeded) {
			return false, fmt.Errorf("daemon: workspace access prompt timed out: %w", promptCtx.Err())
		}
		return false, fmt.Errorf("daemon: workspace access prompt failed: %w", err)
	}
	return b.applyWorkspaceAccessOutcome(promptCtx, sessionID, response.Outcome)
}

func (b *workspaceAccessPromptBridge) applyWorkspaceAccessOutcome(
	ctx context.Context,
	sessionID string,
	outcome acpsdk.RequestPermissionOutcome,
) (bool, error) {
	if err := outcome.Validate(); err != nil {
		return false, fmt.Errorf("daemon: validate workspace access prompt outcome: %w", err)
	}
	if outcome.Selected == nil {
		return false, errors.New("daemon: workspace access prompt returned no selected outcome")
	}
	switch outcome.Selected.OptionId {
	case workspaceAccessAllowOnceID:
		return true, nil
	case workspaceAccessRejectOnceID:
		return false, nil
	case workspaceAccessAllowSessionID:
		if b.consent == nil {
			return false, errors.New("daemon: workspace access consent cache is unavailable")
		}
		b.consent.PutConsent(ctx, sessionID, workspaceaccess.ConsentAllow)
		return true, nil
	case workspaceAccessRejectSessionID:
		if b.consent == nil {
			return false, errors.New("daemon: workspace access consent cache is unavailable")
		}
		b.consent.PutConsent(ctx, sessionID, workspaceaccess.ConsentReject)
		return false, nil
	default:
		return false, errors.New("daemon: workspace access prompt selected an unknown option")
	}
}

func workspaceAccessPromptOptions() []acpsdk.PermissionOption {
	return []acpsdk.PermissionOption{
		{Kind: acpsdk.PermissionOptionKindAllowOnce, Name: "Allow once", OptionId: workspaceAccessAllowOnceID},
		{
			Kind:     acpsdk.PermissionOptionKindAllowAlways,
			Name:     "Allow for this session",
			OptionId: workspaceAccessAllowSessionID,
		},
		{Kind: acpsdk.PermissionOptionKindRejectOnce, Name: "Reject once", OptionId: workspaceAccessRejectOnceID},
		{
			Kind:     acpsdk.PermissionOptionKindRejectAlways,
			Name:     "Reject for this session",
			OptionId: workspaceAccessRejectSessionID,
		},
	}
}
