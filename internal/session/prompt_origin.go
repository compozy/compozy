package session

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/transcript"
)

//nolint:revive,staticcheck // This sentinel is the exact public message required by the session message contract.
var ErrSessionMessageHopLimit = errors.New("Message chain limit reached (8 hops). Ask the operator to continue.")

func encodePromptOrigin(origin *acp.PromptOriginMeta) (json.RawMessage, error) {
	if origin == nil {
		return nil, nil
	}
	encoded, err := json.Marshal(origin)
	if err != nil {
		return nil, fmt.Errorf("session: encode prompt origin: %w", err)
	}
	return encoded, nil
}

func decodePromptOrigin(raw json.RawMessage) (*acp.PromptOriginMeta, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var origin acp.PromptOriginMeta
	if err := json.Unmarshal(raw, &origin); err != nil {
		return nil, fmt.Errorf("session: decode prompt origin: %w", err)
	}
	if err := origin.Validate(); err != nil {
		return nil, err
	}
	return acp.ClonePromptOriginMeta(&origin), nil
}

func promptOriginMessage(origin *acp.PromptOriginMeta, message string) string {
	origin = acp.ClonePromptOriginMeta(origin)
	if origin == nil {
		return message
	}
	reply := fmt.Sprintf("To reply, call compozy__session_prompt with session_id %q.", origin.SessionID)
	if origin.NotifyOnComplete {
		reply = "Your final answer in this turn is sent back to it automatically."
	}
	identity := origin.SessionID
	if origin.AgentName != "" {
		identity += ", agent " + origin.AgentName
	}
	return fmt.Sprintf(
		"[Message from session %q (%s) via compozy__session_prompt — another agent, not the operator. %s]\n\n%s",
		origin.TitleAtSend,
		identity,
		reply,
		message,
	)
}

func originHop(origin *acp.PromptOriginMeta) int {
	if origin == nil {
		return 0
	}
	return origin.Hop
}

func promptMetaHop(meta acp.PromptMeta) int {
	hop := originHop(meta.Origin)
	if meta.Synthetic != nil && meta.Synthetic.Kind == acp.PromptSyntheticKindSessionReply {
		hop = max(hop, meta.Synthetic.Hop)
	}
	return hop
}

func (s *Session) raiseTurnHop(turnID string, hop int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.currentTurnID == turnID {
		s.currentTurnHopFloor = max(s.currentTurnHopFloor, hop)
	}
}

// CurrentTurnEffectiveHop includes the pre-provider fence and every durable input in the active turn.
func (m *Manager) CurrentTurnEffectiveHop(ctx context.Context, sessionID string) (int, error) {
	s, err := m.lookup(strings.TrimSpace(sessionID))
	if err != nil {
		return 0, err
	}
	for {
		s.mu.RLock()
		turnID, hop := s.currentTurnID, s.currentTurnHopFloor
		s.mu.RUnlock()
		if turnID == "" {
			return 0, nil
		}
		query := store.EventQuery{TurnID: turnID, Type: acp.EventTypeUserMessage, Limit: 200}
		for {
			page, err := m.Events(ctx, sessionID, query)
			if err != nil {
				return 0, err
			}
			for _, stored := range page {
				event, err := transcript.UnmarshalAgentEvent(stored.Content)
				if err != nil {
					return 0, err
				}
				hop = max(hop, originHop(event.PromptOrigin()))
			}
			if len(page) < query.Limit {
				break
			}
			query.BeforeSequence = page[0].Sequence
		}
		s.mu.RLock()
		sameTurn := s.currentTurnID == turnID
		hop = max(hop, s.currentTurnHopFloor)
		s.mu.RUnlock()
		if sameTurn {
			return hop, nil
		}
		if err := ctx.Err(); err != nil {
			return 0, err
		}
	}
}
