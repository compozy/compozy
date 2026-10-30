package daemon

import (
	"context"
	"errors"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/session"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

type nativeSessionStopInput struct {
	Processes contract.SessionProcessPolicy `json:"processes"`
	SessionID string                        `json:"session_id"`
	Wait      *bool                         `json:"wait,omitzero"`
}

type nativeSessionStopper interface {
	RequestStopWithCause(context.Context, string, session.StopCause, string) error
	AwaitStopped(context.Context, string) (session.StopOutcome, error)
}

func (n *daemonNativeTools) sessionStop(
	ctx context.Context, scope toolspkg.Scope, req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input nativeSessionStopInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	target, info, err := n.nativeOrchestrationTarget(ctx, scope, req.ToolID, input.SessionID, true)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	if err := n.prepareSessionProcessPolicy(ctx, input.Processes); err != nil {
		return toolspkg.ToolResult{}, nativeInputError(req.ToolID, err)
	}
	detail := "native session_stop requested by " + strings.TrimSpace(scope.SessionID)
	if input.Wait == nil {
		result, err := n.legacySessionStop(ctx, req.ToolID, target, info, detail)
		if err == nil {
			err = n.finishSessionProcessPolicy(ctx, info, input.Processes)
		}
		return result, err
	}
	if info.State == session.StateStopped {
		if err := n.finishSessionProcessPolicy(ctx, info, input.Processes); err != nil {
			return toolspkg.ToolResult{}, err
		}
		return structuredResult(contract.SessionStopPayload{
			SessionID: target, Status: "already-stopped", State: info.State,
			Verified: true, Escalated: info.StopEscalated,
		}, "already-stopped")
	}
	manager, ok := n.deps.Sessions.(nativeSessionStopper)
	if !ok {
		return toolspkg.ToolResult{}, errors.New("native session stop manager is unavailable")
	}
	if err := manager.RequestStopWithCause(ctx, target, session.CauseUserRequested, detail); err != nil {
		return toolspkg.ToolResult{}, nativeSessionOrchestrationError(req.ToolID, err)
	}
	if !*input.Wait {
		if input.Processes == contract.SessionProcessesTerminate {
			n.sessionProcesses().
				AfterStop(target, func(ctx context.Context) error { _, err := manager.AwaitStopped(ctx, target); return err })
		}
		return structuredResult(contract.SessionStopPayload{
			SessionID: target, Status: "stopping", State: session.StateStopping,
			StopCause: session.CauseUserRequested.String(), Escalated: info.StopEscalated,
		}, "stopping")
	}
	outcome, err := manager.AwaitStopped(ctx, target)
	if err != nil && !errors.Is(err, session.ErrStopVerificationFailed) {
		return toolspkg.ToolResult{}, nativeSessionOrchestrationError(req.ToolID, err)
	}
	if outcome.Verified {
		if err := n.finishSessionProcessPolicy(ctx, info, input.Processes); err != nil {
			return toolspkg.ToolResult{}, err
		}
	}
	payload := core.SessionStopOutcomePayload(target, outcome)
	return structuredResult(payload, payload.Status)
}

func (n *daemonNativeTools) legacySessionStop(
	ctx context.Context, toolID toolspkg.ToolID, target string, info *session.Info, detail string,
) (toolspkg.ToolResult, error) {
	payload := map[string]any{
		watchEventsPayloadSessionIDKey: target, nativePayloadStateKey: session.StateStopped,
		"deprecation": "Specify wait:false or wait:true; implicit synchronous stop is removed in v0.5.0",
	}
	if info.State == session.StateStopped {
		payload[nativePayloadOutcomeKey] = "already-stopped"
		return structuredResult(payload, "already-stopped")
	}
	if err := n.deps.Sessions.StopWithCause(ctx, target, session.CauseUserRequested, detail); err != nil {
		return toolspkg.ToolResult{}, nativeSessionOrchestrationError(toolID, err)
	}
	return structuredResult(payload, "stopped")
}
