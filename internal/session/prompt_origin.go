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
	turn, err := m.currentTurnMessageContext(ctx, sessionID)
	return turn.hop, err
}

// turnMessageContext is what the active turn knows about the session messages driving it.
type turnMessageContext struct {
	hop int
	// senders holds every session whose input started or joined the active turn, including reply wakes.
	senders map[string]struct{}
}

func (c turnMessageContext) drivenBy(sessionID string) bool {
	_, ok := c.senders[strings.TrimSpace(sessionID)]
	return ok
}

func (m *Manager) currentTurnMessageContext(ctx context.Context, sessionID string) (turnMessageContext, error) {
	s, err := m.lookup(strings.TrimSpace(sessionID))
	if err != nil {
		return turnMessageContext{}, err
	}
	for {
		s.mu.RLock()
		turnID, hop, meta := s.currentTurnID, s.currentTurnHopFloor, s.currentPromptMeta
		s.mu.RUnlock()
		if turnID == "" {
			return turnMessageContext{}, nil
		}
		turn := turnMessageContext{hop: hop, senders: map[string]struct{}{}}
		turn.addPromptMeta(meta)
		if err := m.addTurnInputEvents(ctx, sessionID, turnID, &turn); err != nil {
			return turnMessageContext{}, err
		}
		s.mu.RLock()
		sameTurn := s.currentTurnID == turnID
		turn.hop = max(turn.hop, s.currentTurnHopFloor)
		s.mu.RUnlock()
		if sameTurn {
			return turn, nil
		}
		if err := ctx.Err(); err != nil {
			return turnMessageContext{}, err
		}
	}
}

// addTurnInputEvents folds in every durable input of the turn: operator and agent messages, and the
// reply wakes a batched dispatch carried after the turn's own input.
func (m *Manager) addTurnInputEvents(ctx context.Context, sessionID, turnID string, turn *turnMessageContext) error {
	for _, eventType := range []string{acp.EventTypeUserMessage, acp.EventTypeSyntheticReentry} {
		query := store.EventQuery{TurnID: turnID, Type: eventType, Limit: 200}
		for {
			page, err := m.Events(ctx, sessionID, query)
			if err != nil {
				return err
			}
			for _, stored := range page {
				event, err := transcript.UnmarshalAgentEvent(stored.Content)
				if err != nil {
					return err
				}
				turn.addPromptMeta(acp.PromptMeta{Origin: event.PromptOrigin(), Synthetic: event.Synthetic})
			}
			if len(page) < query.Limit {
				break
			}
			query.BeforeSequence = page[0].Sequence
		}
	}
	return nil
}

func (c *turnMessageContext) addOrigin(origin *acp.PromptOriginMeta) {
	if origin == nil {
		return
	}
	c.hop = max(c.hop, origin.Hop)
	if id := strings.TrimSpace(origin.SessionID); id != "" {
		c.senders[id] = struct{}{}
	}
}

func (c *turnMessageContext) addPromptMeta(meta acp.PromptMeta) {
	c.addOrigin(meta.Origin)
	c.hop = max(c.hop, promptMetaHop(meta))
	if meta.Synthetic != nil && meta.Synthetic.Kind == acp.PromptSyntheticKindSessionReply {
		if id := strings.TrimSpace(meta.Synthetic.ChildSessionID); id != "" {
			c.senders[id] = struct{}{}
		}
	}
}

type promptAdmissionLookup interface {
	LookupSessionPromptAdmission(context.Context, string, string, string) (store.SessionPromptAdmission, bool, error)
}

// PromptAdmissionReplayOrigin reports whether this message identity was already admitted for the target and
// returns the origin frozen with it (nil for operator or pre-origin admissions). A retry of an admitted send
// replays that receipt even when a new send from the caller would now be refused.
func (m *Manager) PromptAdmissionReplayOrigin(
	ctx context.Context,
	workspaceID, sessionID, messageID, idempotencyKey string,
) (bool, *acp.PromptOriginMeta, error) {
	lookup, ok := m.promptAdmissionStore.(promptAdmissionLookup)
	if !ok {
		return false, nil, nil
	}
	admission, found, err := lookup.LookupSessionPromptAdmission(ctx, workspaceID, sessionID, idempotencyKey)
	if err != nil || !found || admission.MessageID != messageID {
		return false, nil, err
	}
	origin, err := decodePromptOrigin(admission.Origin)
	if err != nil {
		return false, nil, err
	}
	return true, origin, nil
}
