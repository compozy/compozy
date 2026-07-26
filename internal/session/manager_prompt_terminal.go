package session

import (
	"context"

	"github.com/compozy/agh/internal/acp"
	"github.com/compozy/agh/internal/transcript"
)

func (m *Manager) finishPromptTurnIfNeeded(
	ctx context.Context,
	turnState *promptTurnDispatchState,
	loop *promptPumpLoopState,
	normalized acp.AgentEvent,
) bool {
	if !isPromptTerminalEvent(normalized.Type) {
		return false
	}
	m.dispatchTurnEnd(ctx, turnState, normalized.Timestamp)
	return loop.turnEndedShouldReturn()
}

func (m *Manager) emitFileMutationMarkerBeforeTerminalNotification(
	ctx context.Context,
	session *Session,
	turnState *promptTurnDispatchState,
	loop *promptPumpLoopState,
	event acp.AgentEvent,
) {
	if loop == nil || loop.fileMutationMarked || !isPromptTerminalEvent(event.Type) {
		return
	}
	summary, evidence, ok := loop.fileMutations.Marker()
	if !ok {
		return
	}
	m.emitTranscriptMarker(
		ctx,
		session,
		turnState.turnID,
		transcript.MarkerFileMutationUnverified,
		summary,
		evidence,
	)
	loop.fileMutationMarked = true
}
