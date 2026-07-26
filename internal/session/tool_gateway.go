package session

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/acp"
	hookspkg "github.com/compozy/agh/internal/hooks"
)

// RuntimeModeVerdictOnly disables provider-native tools for isolated verdict sessions.
const RuntimeModeVerdictOnly = "verdict-only"

type providerNativeToolGateway struct {
	manager     *Manager
	session     *Session
	runtimeMode string
}

var _ acp.ToolExecutionGateway = (*providerNativeToolGateway)(nil)

func newProviderNativeToolGateway(
	manager *Manager,
	session *Session,
	runtimeMode string,
) acp.ToolExecutionGateway {
	if manager == nil || session == nil {
		return nil
	}
	return &providerNativeToolGateway{
		manager:     manager,
		session:     session,
		runtimeMode: strings.TrimSpace(runtimeMode),
	}
}

func (g *providerNativeToolGateway) Intercept(
	ctx context.Context,
	req acp.ToolExecutionRequest,
) (acp.ToolExecutionRequest, error) {
	if g == nil || g.manager == nil || g.session == nil {
		return req, nil
	}
	if strings.EqualFold(g.runtimeMode, RuntimeModeVerdictOnly) {
		return acp.ToolExecutionRequest{}, fmt.Errorf(
			"%w: provider-native tools are disabled for verdict-only sessions",
			acp.ErrPermissionDenied,
		)
	}

	dispatchCtx := hookDispatchContext(ctx, g.manager, g.session)
	payload, err := g.manager.hooks.tools().DispatchToolPreCall(dispatchCtx, hookspkg.ToolPreCallPayload{
		PayloadBase: hookspkg.PayloadBase{
			Event:     hookspkg.HookToolPreCall,
			Timestamp: g.manager.now(),
		},
		SessionContext: hookSessionContext(g.session),
		TurnContext: hookspkg.TurnContext{
			TurnID: strings.TrimSpace(g.session.CurrentTurnID()),
		},
		ToolCallRef: hookspkg.ToolCallRef{
			ToolID:   strings.TrimSpace(req.ToolID),
			ReadOnly: req.ReadOnly,
		},
		ToolInput: acp.CloneRawMessage(req.Input),
	})
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return acp.ToolExecutionRequest{}, err
		}
		return acp.ToolExecutionRequest{}, fmt.Errorf("%w: %w", acp.ErrPermissionDenied, err)
	}

	return acp.ToolExecutionRequest{
		ToolID:   strings.TrimSpace(payload.ToolID),
		ReadOnly: payload.ReadOnly,
		Input:    acp.CloneRawMessage(payload.ToolInput),
	}, nil
}
