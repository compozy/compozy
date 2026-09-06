package session

import (
	"context"
	"encoding/json"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/events"
)

type turnQuiescedPayload struct {
	WorkspaceID string    `json:"workspace_id"`
	SessionID   string    `json:"session_id"`
	TurnID      string    `json:"turn_id"`
	Scope       string    `json:"scope"`
	Verified    bool      `json:"verified"`
	Escalated   bool      `json:"escalated"`
	Phase       StopPhase `json:"phase"`
	ElapsedMS   int64     `json:"elapsed_ms"`
	StopCause   string    `json:"stop_cause"`
}

// recordTurnQuiesced commits completion before turn admission is released.
// Escalation events precede an attempted action and cannot establish this fact.
const stopScopeTurn = "turn"

func (m *Manager) recordTurnQuiesced(ctx context.Context, run *turnStopRun, cause StopCause) error {
	info := run.session.Info()
	raw, err := json.Marshal(turnQuiescedPayload{
		WorkspaceID: info.WorkspaceID, SessionID: info.ID, TurnID: run.turnID,
		Scope: stopScopeTurn, Verified: true, Escalated: run.outcome.Escalated,
		Phase: run.outcome.Phase, ElapsedMS: run.outcome.Elapsed.Milliseconds(), StopCause: cause.String(),
	})
	if err != nil {
		return err
	}
	event := m.normalizeEvent(run.session, run.turnID, acp.AgentEvent{
		Type: events.SessionTurnQuiesced, Raw: raw, Timestamp: m.now(),
	}).WithEventID("turn-quiesced:" + run.turnID)
	if err := m.recordEventWithWriter(ctx, run.session, event, "", recordIdempotentSessionEvent); err != nil {
		return err
	}
	m.notifyAgentEvent(ctx, run.session, event)
	return nil
}
