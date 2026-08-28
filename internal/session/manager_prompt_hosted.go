package session

import (
	"context"
	"errors"
	"strings"

	"github.com/compozy/compozy/internal/acp"
)

func (m *Manager) startHostedPromptRun(
	ctx context.Context,
	session *Session,
	process *AgentProcess,
	state *promptTurnDispatchState,
	request acp.PromptRequest,
) (<-chan acp.AgentEvent, error) {
	if session == nil || state == nil || strings.TrimSpace(state.runID) == "" || request.Generation <= 0 {
		return nil, errors.New("session: prompt run identity is incomplete")
	}
	state.generation = request.Generation
	if m.hostedMCP != nil {
		if err := m.hostedMCP.BindRun(ctx, session.ID, state.runID, request.Generation); err != nil {
			return nil, err
		}
	}
	if state.recovery == nil {
		state.recovery = &promptRecoveryState{}
	}
	state.recovery.executionCtx = ctx
	state.recovery.request = clonePromptRecoveryRequest(request)
	if err := m.publishOrReplaceActivePromptRun(session, state); err != nil {
		m.releaseHostedPromptRun(session, state)
		return nil, err
	}
	source, err := m.driver.Prompt(ctx, process, request)
	if err != nil {
		m.clearActivePromptRunForState(session, state)
		m.releaseHostedPromptRun(session, state)
		return nil, err
	}
	return source, nil
}

func (m *Manager) releaseHostedPromptRun(session *Session, state *promptTurnDispatchState) {
	if m == nil || m.hostedMCP == nil || session == nil || state == nil || state.recovery == nil {
		return
	}
	m.hostedMCP.ReleaseRun(session.ID, state.runID, state.recovery.request.Generation)
}
