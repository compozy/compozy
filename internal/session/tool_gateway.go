package session

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/acp"
	hookspkg "github.com/compozy/compozy/internal/hooks"
	"github.com/compozy/compozy/internal/shellwait"
)

// RuntimeModeVerdictOnly disables provider-native tools for isolated verdict sessions.
const RuntimeModeVerdictOnly = "verdict-only"

type providerNativeToolGateway struct {
	manager     *Manager
	session     *Session
	runtimeMode string
	// shellWaitGuard refuses foreground shell waits at the execution boundary, which
	// is the only enforcement point for providers without a pre-tool hook.
	shellWaitGuard bool
}

var _ acp.ToolExecutionGateway = (*providerNativeToolGateway)(nil)

func newProviderNativeToolGateway(
	manager *Manager,
	session *Session,
	runtimeMode string,
	shellWaitGuard bool,
) acp.ToolExecutionGateway {
	if manager == nil || session == nil {
		return nil
	}
	return &providerNativeToolGateway{
		manager:        manager,
		session:        session,
		runtimeMode:    strings.TrimSpace(runtimeMode),
		shellWaitGuard: shellWaitGuard,
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
	if g.session.Info().Type == SessionTypeTriage && (!req.ReadOnly || req.Execute) {
		return acp.ToolExecutionRequest{}, acp.ErrPermissionDenied
	}
	if g.shellWaitGuard && req.Execute {
		if finding, found := shellwait.DetectToolInput(req.Input); found {
			return acp.ToolExecutionRequest{}, fmt.Errorf(
				"%w: %s",
				acp.ErrPermissionDenied,
				shellwait.DenyReason(finding),
			)
		}
	}

	dispatchCtx := hookDispatchContext(ctx, g.manager, g.session)
	payload, err := g.manager.hooks.tools().DispatchToolPreCall(dispatchCtx, hookspkg.ToolPreCallPayload{
		Event:          hookspkg.HookToolPreCall,
		Timestamp:      g.manager.now(),
		SessionContext: hookSessionContext(g.session),
		TurnID:         strings.TrimSpace(g.session.CurrentTurnID()),
		ToolID:         strings.TrimSpace(req.ToolID),
		ReadOnly:       req.ReadOnly,
		ToolInput:      acp.CloneRawMessage(req.Input),
	})
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return acp.ToolExecutionRequest{}, err
		}
		return acp.ToolExecutionRequest{}, fmt.Errorf("%w: %w", acp.ErrPermissionDenied, err)
	}

	if g.session.Info().Type == SessionTypeTriage && (!payload.ReadOnly || payload.ToolID != req.ToolID) {
		return acp.ToolExecutionRequest{}, acp.ErrPermissionDenied
	}
	return acp.ToolExecutionRequest{
		ToolID:   strings.TrimSpace(payload.ToolID),
		ReadOnly: payload.ReadOnly,
		Input:    acp.CloneRawMessage(payload.ToolInput),
	}, nil
}
