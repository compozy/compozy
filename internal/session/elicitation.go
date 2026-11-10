package session

import (
	"context"

	"github.com/compozy/compozy/internal/acp"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

// SetClarifyBroker installs the boot-scoped operator question broker that
// provider elicitation reuses. The daemon wires it once the broker exists.
func (m *Manager) SetClarifyBroker(broker toolspkg.ClarifyBroker) {
	if m == nil {
		return
	}
	m.clarifyMu.Lock()
	m.clarify = broker
	m.clarifyMu.Unlock()
}

func (m *Manager) clarifyBroker() toolspkg.ClarifyBroker {
	if m == nil {
		return nil
	}
	m.clarifyMu.RLock()
	defer m.clarifyMu.RUnlock()
	return m.clarify
}

// elicitationAsker renders provider questions raised by one session process
// through the same operator clarification channel as compozy__clarify. Only
// operator-facing session types may block on an answer; headless sessions get
// an explicit unavailable refusal so a Loop, automation, or subagent never
// hangs on a question nobody can see.
func (m *Manager) elicitationAsker(
	sessionID, workspaceID, profileID, agentName string,
	sessionType Type,
) acp.ElicitationAsker {
	scope := toolspkg.Scope{
		ProfileID:   profileID,
		WorkspaceID: workspaceID,
		SessionID:   sessionID,
		AgentName:   agentName,
	}
	return func(ctx context.Context, question toolspkg.ClarifyQuestion) (toolspkg.ClarifyAnswer, error) {
		if !elicitationSessionIsInteractive(sessionType) {
			return toolspkg.ClarifyAnswer{}, acp.ErrElicitationUnavailable
		}
		broker := m.clarifyBroker()
		if broker == nil {
			return toolspkg.ClarifyAnswer{}, acp.ErrElicitationUnavailable
		}
		if err := ctx.Err(); err != nil {
			return toolspkg.ClarifyAnswer{}, err
		}
		return broker.Ask(ctx, scope, question)
	}
}

// elicitationSessionIsInteractive reports whether an operator can see and
// answer this session's questions in the UI.
func elicitationSessionIsInteractive(sessionType Type) bool {
	switch sessionType {
	case SessionTypeUser, SessionTypeCoordinator:
		return true
	default:
		return false
	}
}
