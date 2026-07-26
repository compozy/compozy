package session

import (
	"context"

	"github.com/compozy/agh/internal/acp"
)

func (m *Manager) notifyManagedPromptEvent(
	ctx context.Context,
	session *Session,
	turnState *promptTurnDispatchState,
	event acp.AgentEvent,
) {
	reportManagedInputUsage(turnState, event.Usage)
	m.notifyAgentEvent(ctx, session, event)
}

func reportManagedInputUsage(turnState *promptTurnDispatchState, usage *acp.TokenUsage) {
	if turnState == nil || turnState.managed == nil ||
		turnState.managed.submission.UsageReporter == nil {
		return
	}
	tokens, reported, err := managedInputTokens(usage)
	if err != nil || !reported {
		return
	}
	turnState.managed.submission.UsageReporter.ReportActionTokensUsed(tokens)
}
