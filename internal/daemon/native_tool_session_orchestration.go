package daemon

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/acp"
	core "github.com/compozy/compozy/internal/api/core"
	callspkg "github.com/compozy/compozy/internal/calls"
	"github.com/compozy/compozy/internal/session"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

type nativeSessionWaiter interface {
	WaitForBadge(context.Context, session.WaitRequest) (session.WaitOutcome, error)
}

const (
	nativePayloadOutcomeKey   = "outcome"
	nativePayloadRequestIDKey = "request_id"
	nativePayloadStateKey     = "state"
)

func (n *daemonNativeTools) sessionWait(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input nativeSessionWaitInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	target, _, err := n.nativeOrchestrationTarget(ctx, scope, req.ToolID, input.SessionID)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	timeout := session.WaitTimeoutDefault
	if input.TimeoutMS > 0 {
		timeout = time.Duration(input.TimeoutMS) * time.Millisecond
	}
	badges := make([]session.Badge, 0, len(input.Until))
	for _, value := range input.Until {
		badges = append(badges, session.Badge(strings.TrimSpace(value)))
	}
	waiter, ok := n.deps.Sessions.(nativeSessionWaiter)
	if !ok {
		return toolspkg.ToolResult{}, nativeUnavailableError(req.ToolID, "session wait service is unavailable")
	}
	outcome, err := waiter.WaitForBadge(ctx, session.WaitRequest{
		SessionID: target, Until: badges, Timeout: timeout,
	})
	if err != nil {
		return toolspkg.ToolResult{}, nativeSessionOrchestrationError(req.ToolID, err)
	}
	payload := map[string]any{
		watchEventsPayloadSessionIDKey: target,
		nativePayloadOutcomeKey:        outcome.Outcome,
		"waited_ms":                    outcome.Waited.Milliseconds(),
		"revision":                     outcome.Revision,
	}
	if outcome.State != "" {
		payload[nativePayloadStateKey] = outcome.State
	}
	return structuredResult(payload, string(outcome.Outcome))
}

func (n *daemonNativeTools) sessionStop(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input nativeSessionTargetInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	target, info, err := n.nativeOrchestrationTarget(ctx, scope, req.ToolID, input.SessionID)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	payload := map[string]any{
		watchEventsPayloadSessionIDKey: target,
		nativePayloadStateKey:          session.StateStopped,
	}
	if input.Subtree {
		calls := n.callsService()
		if calls == nil {
			return toolspkg.ToolResult{}, nativeUnavailableError(req.ToolID, "calls service is unavailable")
		}
		report, drainErr := calls.DrainSubtree(ctx, target, callspkg.Actor{
			Kind: daemonAgentSessionActorKind, ID: strings.TrimSpace(scope.SessionID),
		}, input.Reason)
		if drainErr != nil {
			return toolspkg.ToolResult{}, drainErr
		}
		payload["stopped_children"] = len(report.Stopped)
		payload["closed_calls"] = len(report.CanceledCalls)
		payload["preserved_results"] = report.PreservedResults
	}
	if info.State == session.StateStopped {
		payload[nativePayloadOutcomeKey] = "already-stopped"
		return structuredResult(payload, "already-stopped")
	}
	if err := n.deps.Sessions.StopWithCause(
		ctx, target, session.CauseUserRequested, nativeSessionStopReason(scope.SessionID, input.Reason),
	); err != nil {
		return toolspkg.ToolResult{}, nativeSessionOrchestrationError(req.ToolID, err)
	}
	return structuredResult(payload, "stopped")
}

func nativeSessionStopReason(callerID, reason string) string {
	if reason = strings.TrimSpace(reason); reason != "" {
		return reason
	}
	return "native session_stop requested by " + strings.TrimSpace(callerID)
}

func (n *daemonNativeTools) sessionApprove(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input nativeSessionApproveInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	target, _, err := n.nativeOrchestrationTarget(ctx, scope, req.ToolID, input.SessionID)
	if err != nil {
		return toolspkg.ToolResult{}, nativeApprovalSelfError(req.ToolID, err)
	}
	result, err := n.deps.Sessions.ApprovePermission(ctx, target, acp.ApproveRequest{
		RequestID:  strings.TrimSpace(input.RequestID),
		Decision:   strings.TrimSpace(input.Decision),
		ResolvedBy: daemonAgentSessionActorKind + ":" + strings.TrimSpace(scope.SessionID),
	})
	if err != nil {
		return toolspkg.ToolResult{}, nativeSessionOrchestrationError(req.ToolID, err)
	}
	payload := map[string]any{
		nativePayloadOutcomeKey:       result.Outcome,
		nativePayloadRequestIDKey:     result.RequestID,
		watchEventsPayloadDecisionKey: result.Decision,
	}
	if result.ResolvedDecision != "" {
		payload["resolved_decision"] = result.ResolvedDecision
	}
	return structuredResult(payload, result.Outcome)
}

func (n *daemonNativeTools) sessionClarifyAnswer(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input nativeSessionClarifyAnswerInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	target, info, err := n.nativeOrchestrationTarget(ctx, scope, req.ToolID, input.SessionID)
	if err != nil {
		return toolspkg.ToolResult{}, nativeApprovalSelfError(req.ToolID, err)
	}
	broker := n.clarifyBroker()
	if broker == nil {
		return toolspkg.ToolResult{}, nativeUnavailableError(req.ToolID, "clarification broker is unavailable")
	}
	var choiceIndex *int
	if input.Choice != nil {
		index := *input.Choice - 1
		choiceIndex = &index
	}
	result, err := broker.Answer(session.WithActingSession(ctx, scope.SessionID), toolspkg.Scope{
		WorkspaceID: info.WorkspaceID, SessionID: target, AgentName: info.AgentName, Operator: true,
	}, strings.TrimSpace(input.RequestID), toolspkg.ClarifyAnswerRequest{
		ChoiceIndex: choiceIndex, Text: strings.TrimSpace(input.Text),
	})
	if err != nil {
		return toolspkg.ToolResult{}, nativeSessionOrchestrationError(req.ToolID, err)
	}
	payload := map[string]any{
		nativePayloadOutcomeKey:   result.Outcome,
		nativePayloadRequestIDKey: result.RequestID,
	}
	if result.ResolvedAnswer != "" {
		payload["resolved_answer"] = result.ResolvedAnswer
	}
	return structuredResult(payload, result.Outcome)
}

func (n *daemonNativeTools) sessionPromptCancel(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input nativeSessionTargetInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	target, _, err := n.nativeOrchestrationTarget(ctx, scope, req.ToolID, input.SessionID)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	result, err := n.deps.Sessions.CancelPrompt(session.WithActingSession(ctx, scope.SessionID), target)
	if err != nil {
		return toolspkg.ToolResult{}, nativeSessionOrchestrationError(req.ToolID, err)
	}
	payload := map[string]any{
		watchEventsPayloadSessionIDKey: target,
		nativePayloadOutcomeKey:        result.Outcome,
	}
	if result.TurnID != "" {
		payload["turn_id"] = result.TurnID
	}
	return structuredResult(payload, string(result.Outcome))
}

func (n *daemonNativeTools) nativeOrchestrationTarget(
	ctx context.Context,
	scope toolspkg.Scope,
	id toolspkg.ToolID,
	sessionID string,
) (string, *session.Info, error) {
	target, err := requiredNativeString(id, "session_id", sessionID)
	if err != nil {
		return "", nil, err
	}
	if target == strings.TrimSpace(scope.SessionID) {
		return "", nil, nativeSelfTargetError(id)
	}
	resolved, err := n.nativeResolvedWorkspace(ctx, id, "", scope)
	if err != nil {
		return "", nil, err
	}
	workspaceID, err := nativeResolvedRegistryWorkspaceID(&resolved)
	if err != nil {
		return "", nil, nativeNetworkInputError(id, err)
	}
	info, err := n.nativeSessionInWorkspace(ctx, id, workspaceID, target)
	if err != nil {
		if toolErr, ok := errors.AsType[*toolspkg.ToolError](err); ok && toolErr != nil {
			return "", nil, err
		}
		return "", nil, nativeSessionOrchestrationError(id, err)
	}
	if strings.TrimSpace(info.ProfileID) != strings.TrimSpace(scope.ProfileID) {
		return "", nil, nativeWorkspaceAccessDeniedError(id)
	}
	return target, info, nil
}

func nativeSelfTargetError(id toolspkg.ToolID) error {
	err := errors.New("daemon: session orchestration cannot target the caller session")
	return toolspkg.NewToolError(
		toolspkg.ErrorCodeDenied, id, err.Error(), fmt.Errorf("%w: %w", toolspkg.ErrToolDenied, err),
		toolspkg.ReasonSelfTargetDenied, toolspkg.ReasonPolicyDenied,
	)
}

func nativeApprovalSelfError(id toolspkg.ToolID, err error) error {
	if reason, ok := toolspkg.ReasonOf(err); !ok || reason != toolspkg.ReasonSelfTargetDenied {
		return err
	}
	return toolspkg.NewToolError(
		toolspkg.ErrorCodeDenied, id, err.Error(), fmt.Errorf("%w: %w", toolspkg.ErrToolDenied, err),
		toolspkg.ReasonApprovalSelfDenied, toolspkg.ReasonPolicyDenied,
	)
}

func nativeSessionOrchestrationError(id toolspkg.ToolID, err error) error {
	if err == nil {
		return nil
	}
	status := core.StatusForSessionError(err)
	switch {
	case errors.Is(err, toolspkg.ErrClarifyInvalid):
		status = 422
	case errors.Is(err, toolspkg.ErrClarifyNotFound):
		status = 404
	case errors.Is(err, toolspkg.ErrClarifyPending), errors.Is(err, toolspkg.ErrClarifyCanceled):
		status = 409
	case errors.Is(err, session.ErrSpawnPermissionDenied):
		status = 403
	case errors.Is(err, session.ErrSpawnLimitExceeded):
		status = 409
	case errors.Is(err, session.ErrSpawnValidation),
		errors.Is(err, session.ErrWaitValidation),
		errors.Is(err, session.ErrWaitStateInvalid):
		status = 422
	}
	return nativeHTTPStatusToolError(id, err, status)
}
