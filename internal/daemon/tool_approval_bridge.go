package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	acpsdk "github.com/coder/acp-go-sdk"
	"github.com/compozy/compozy/internal/acp"
	terminalpkg "github.com/compozy/compozy/internal/terminal"
	toolspkg "github.com/compozy/compozy/internal/tools"
	"github.com/google/uuid"
)

const (
	toolApprovalAllowOnceID       acpsdk.PermissionOptionId = "allow_once"
	toolApprovalAllowAlwaysID     acpsdk.PermissionOptionId = "allow_always"
	toolApprovalRejectOnceID      acpsdk.PermissionOptionId = "reject_once"
	toolApprovalRejectAlwaysID    acpsdk.PermissionOptionId = "reject_always"
	toolApprovalApprovedOnceLabel                           = "approved_once"
)

type sessionPermissionRequester interface {
	RequestPermission(
		ctx context.Context,
		id string,
		req acp.RequestPermissionRequest,
	) (acp.RequestPermissionResponse, error)
}

type toolApprovalBridge struct {
	sessions  func() sessionPermissionRequester
	timeout   time.Duration
	approvals toolspkg.ApprovalTokenConsumer
	grants    toolspkg.ApprovalGrantStore
	logger    *slog.Logger
}

var _ toolspkg.ApprovalBridge = (*toolApprovalBridge)(nil)

func newToolApprovalBridge(
	sessions func() sessionPermissionRequester,
	timeout time.Duration,
	approvals toolspkg.ApprovalTokenConsumer,
	grants toolspkg.ApprovalGrantStore,
	logger *slog.Logger,
) *toolApprovalBridge {
	if logger == nil {
		logger = slog.Default()
	}
	return &toolApprovalBridge{
		sessions:  sessions,
		timeout:   timeout,
		approvals: approvals,
		grants:    grants,
		logger:    logger,
	}
}

func (b *toolApprovalBridge) RequestToolApproval(
	ctx context.Context,
	scope toolspkg.Scope,
	request *toolspkg.CallRequest,
	view *toolspkg.ToolView,
) error {
	if request == nil {
		return toolApprovalError("", "tool approval request is unavailable", toolspkg.ReasonApprovalUnreachable)
	}
	call := *request
	toolID := toolApprovalID(call, view)
	forcePrompt, err := terminalApprovalForcePrompt(call, toolID)
	if err != nil {
		return err
	}
	if handled, err := b.consumeLocalToolApproval(ctx, scope, call); handled {
		if err == nil {
			setToolApprovalLabel(request, toolApprovalApprovedOnceLabel)
		}
		return err
	}
	if !forcePrompt {
		if handled, durableErr := b.consumeDurableToolApproval(ctx, scope, call, toolID); handled {
			if durableErr == nil {
				setToolApprovalLabel(request, "approved_always")
			}
			return durableErr
		}
	}
	if b == nil || b.sessions == nil {
		return toolApprovalError(
			toolID,
			"tool approval channel is unreachable",
			toolspkg.ReasonApprovalUnreachable,
		)
	}
	sessions := b.sessions()
	if sessions == nil {
		return toolApprovalError(
			toolID,
			"tool approval channel is unreachable",
			toolspkg.ReasonApprovalUnreachable,
		)
	}
	sessionID := strings.TrimSpace(call.SessionID)
	if sessionID == "" {
		sessionID = strings.TrimSpace(scope.SessionID)
	}
	if sessionID == "" {
		return toolApprovalError(
			toolID,
			"tool approval session is unavailable",
			toolspkg.ReasonApprovalUnreachable,
		)
	}
	descriptor := toolApprovalDescriptor(call, view)
	response, err := b.requestSessionToolApproval(ctx, sessions, sessionID, call, descriptor, view)
	if err != nil {
		return err
	}
	err = b.applyToolApprovalOutcome(ctx, scope, call, toolID, response.Outcome)
	if err == nil {
		setToolApprovalLabel(request, toolApprovalOutcomeLabel(response.Outcome))
	}
	return err
}

func terminalApprovalForcePrompt(call toolspkg.CallRequest, toolID toolspkg.ToolID) (bool, error) {
	if toolID != toolspkg.ToolIDTerminalExec {
		return false, nil
	}
	var input terminalExecInput
	if err := json.Unmarshal(call.Input, &input); err != nil {
		return false, toolspkg.NewToolError(
			toolspkg.ErrorCodeInvalidInput,
			toolID,
			"terminal command approval input is invalid",
			errors.Join(toolspkg.ErrToolInvalidInput, err),
			toolspkg.ReasonSchemaInvalid,
		)
	}
	classification := terminalpkg.ClassifyArgv(append([]string{input.Command}, input.Args...), nil)
	if classification.Verdict == terminalpkg.CommandVerdictDenied {
		return false, nativeCommandToolError(
			toolspkg.ErrorCodeDenied,
			toolID,
			"approval_rejected — terminal command is blocked by the irreversible-operation policy",
			toolspkg.ErrToolDenied,
			toolspkg.ReasonSessionDenied,
		)
	}
	return classification.Reason == "unclassifiable", nil
}

func setToolApprovalLabel(call *toolspkg.CallRequest, label string) {
	if call != nil {
		call.ApprovalGranted = true
		call.ApprovalLabel = label
	}
}

func toolApprovalOutcomeLabel(outcome acpsdk.RequestPermissionOutcome) string {
	if outcome.Selected != nil && outcome.Selected.OptionId == toolApprovalAllowAlwaysID {
		return "approved_always"
	}
	return toolApprovalApprovedOnceLabel
}

func (b *toolApprovalBridge) requestSessionToolApproval(
	ctx context.Context,
	sessions sessionPermissionRequester,
	sessionID string,
	call toolspkg.CallRequest,
	descriptor toolspkg.Descriptor,
	view *toolspkg.ToolView,
) (acp.RequestPermissionResponse, error) {
	toolID := toolApprovalID(call, view)
	timeout := b.timeout
	if timeout <= 0 {
		timeout = 120 * time.Second
	}
	approvalCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	requestID := toolApprovalCallID(call)

	response, err := sessions.RequestPermission(
		approvalCtx,
		sessionID,
		acp.RequestPermissionRequest{
			Meta: map[string]any{
				acp.PermissionRequestIDMetaKey: requestID,
				acp.PermissionToolIDMetaKey:    toolID.String(),
			},
			SessionId: acpsdk.SessionId(sessionID),
			ToolCall: acpsdk.ToolCallUpdate{
				ToolCallId: acpsdk.ToolCallId(requestID),
				Title:      new(toolApprovalTitle(descriptor)),
				Kind:       new(toolApprovalKind(descriptor)),
				RawInput:   toolApprovalRawInput(call.Input),
				Status:     acpsdk.Ptr(acpsdk.ToolCallStatusPending),
			},
			Options: toolApprovalOptions(),
		},
	)
	if err != nil {
		switch {
		case errors.Is(approvalCtx.Err(), context.DeadlineExceeded):
			return acp.RequestPermissionResponse{}, toolApprovalError(
				toolID,
				"tool approval timed out",
				toolspkg.ReasonApprovalTimedOut,
			)
		case errors.Is(ctx.Err(), context.Canceled), errors.Is(err, context.Canceled):
			return acp.RequestPermissionResponse{}, toolApprovalError(
				toolID,
				"tool approval was canceled",
				toolspkg.ReasonApprovalCanceled,
			)
		default:
			return acp.RequestPermissionResponse{}, toolApprovalError(
				toolID,
				fmt.Sprintf("tool approval channel is unreachable: %v", err),
				toolspkg.ReasonApprovalUnreachable,
			)
		}
	}
	return response, nil
}

func (b *toolApprovalBridge) consumeLocalToolApproval(
	ctx context.Context,
	scope toolspkg.Scope,
	call toolspkg.CallRequest,
) (bool, error) {
	if b == nil || b.approvals == nil {
		return false, nil
	}
	if !scope.Operator && strings.TrimSpace(call.ApprovalToken) == "" {
		return false, nil
	}
	return true, b.approvals.ConsumeToolApproval(ctx, scope, call)
}

func toolApprovalID(call toolspkg.CallRequest, view *toolspkg.ToolView) toolspkg.ToolID {
	if view != nil && view.Descriptor.ID != "" {
		return view.Descriptor.ID
	}
	return call.ToolID
}

func toolApprovalDescriptor(call toolspkg.CallRequest, view *toolspkg.ToolView) toolspkg.Descriptor {
	if view != nil {
		return view.Descriptor
	}
	return toolspkg.Descriptor{ID: call.ToolID}
}

func toolApprovalOptions() []acpsdk.PermissionOption {
	return []acpsdk.PermissionOption{
		{Kind: acpsdk.PermissionOptionKindAllowOnce, Name: "Allow once", OptionId: toolApprovalAllowOnceID},
		{Kind: acpsdk.PermissionOptionKindAllowAlways, Name: "Allow always", OptionId: toolApprovalAllowAlwaysID},
		{Kind: acpsdk.PermissionOptionKindRejectOnce, Name: "Reject once", OptionId: toolApprovalRejectOnceID},
		{Kind: acpsdk.PermissionOptionKindRejectAlways, Name: "Reject always", OptionId: toolApprovalRejectAlwaysID},
	}
}

func toolApprovalCallID(call toolspkg.CallRequest) string {
	for _, value := range []string{call.ToolCallID, call.CorrelationID} {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return uuid.NewString()
}

func toolApprovalTitle(descriptor toolspkg.Descriptor) string {
	if title := strings.TrimSpace(descriptor.Presentation().DisplayTitle); title != "" {
		return title
	}
	return descriptor.ID.String()
}

func toolApprovalKind(descriptor toolspkg.Descriptor) acpsdk.ToolKind {
	switch {
	case descriptor.ReadOnly && descriptor.Risk == toolspkg.RiskRead:
		return acpsdk.ToolKindRead
	case descriptor.Destructive || descriptor.Risk == toolspkg.RiskDestructive:
		return acpsdk.ToolKindDelete
	case descriptor.OpenWorld || descriptor.Risk == toolspkg.RiskOpenWorld:
		return acpsdk.ToolKindFetch
	default:
		return acpsdk.ToolKindExecute
	}
}

func toolApprovalRawInput(input json.RawMessage) any {
	if len(input) == 0 {
		return map[string]any{}
	}
	var value any
	if err := json.Unmarshal(input, &value); err != nil {
		return string(input)
	}
	return value
}

func toolApprovalError(id toolspkg.ToolID, message string, reason toolspkg.ReasonCode) error {
	return toolspkg.NewToolError(
		toolspkg.ErrorCodeApprovalRequired,
		id,
		message,
		toolspkg.ErrToolApprovalRequired,
		toolspkg.ReasonApprovalRequired,
		reason,
	)
}
