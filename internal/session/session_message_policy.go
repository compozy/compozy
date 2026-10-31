package session

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/acp"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/store"
)

var (
	// ErrSessionMessageSelfTarget rejects a session message addressed to the calling session.
	//nolint:revive,staticcheck // This sentinel is the exact public message of the session message contract.
	ErrSessionMessageSelfTarget = errors.New("session_prompt cannot target the calling session.")
	// ErrSessionMessageRateLimited reports an exhausted per sender→target message budget.
	ErrSessionMessageRateLimited = errors.New("session message rate limit reached")
)

// MessageRateLimitError carries the budget that refused a session message.
type MessageRateLimitError struct {
	TargetSessionID string
	Limit           int
	Window          time.Duration
	RetryAfter      time.Duration
}

func (e *MessageRateLimitError) Error() string {
	return fmt.Sprintf(
		"Message rate limit reached: %d messages to session %s within %s. Retry after %s, "+
			"and send fewer, milestone-level messages.",
		e.Limit, e.TargetSessionID, e.Window, e.RetryAfter,
	)
}

func (e *MessageRateLimitError) Unwrap() error { return ErrSessionMessageRateLimited }

// MessageOriginRequest describes one agent session sending a message to another session.
type MessageOriginRequest struct {
	SenderSessionID  string
	AgentName        string
	TargetSessionID  string
	InReplyTo        string
	NotifyOnComplete bool
}

// SessionMessageOrigin stamps the immutable sender origin for one agent-to-agent message.
//
// A message answering a session that drives the sender's active turn, or one whose in_reply_to names a
// message that target sent to the sender, keeps the turn's hop instead of adding one: hop measures
// forwarding depth, not conversation length. The per sender→target rate budget is the flood guard.
func (m *Manager) SessionMessageOrigin(
	ctx context.Context,
	req MessageOriginRequest,
) (*acp.PromptOriginMeta, error) {
	sender := strings.TrimSpace(req.SenderSessionID)
	target := strings.TrimSpace(req.TargetSessionID)
	inReplyTo := strings.TrimSpace(req.InReplyTo)
	if sender == "" || target == "" {
		return nil, errors.New("session: message sender and target are required")
	}
	if err := acp.ValidatePromptOriginInReplyTo(inReplyTo); err != nil {
		return nil, err
	}
	if sender == target {
		m.logger.WarnContext(ctx, "session.message.rejected", "reason", "self_target", "sender_session_id", sender)
		return nil, ErrSessionMessageSelfTarget
	}
	turn, err := m.currentTurnMessageContext(ctx, sender)
	if err != nil {
		return nil, err
	}
	senderInfo, err := m.Status(ctx, sender)
	if err != nil {
		return nil, err
	}
	answer := turn.drivenBy(target)
	if !answer && inReplyTo != "" {
		answer, err = m.repliesToTargetMessage(ctx, senderInfo.WorkspaceID, sender, target, inReplyTo)
		if err != nil {
			return nil, err
		}
	}
	hop := turn.hop + 1
	if answer {
		hop = max(turn.hop, 1)
	}
	if hop > acp.MaxSessionMessageHops {
		m.logger.WarnContext(ctx, "session.message.rejected", "reason", "hop_limit", "sender_session_id", sender)
		return nil, ErrSessionMessageHopLimit
	}
	if err := m.checkSessionMessageBudget(ctx, sender, target); err != nil {
		return nil, err
	}
	origin := acp.PromptOriginMeta{
		Kind: acp.PromptOriginKindSession, SessionID: sender, WorkspaceID: senderInfo.WorkspaceID,
		AgentName: strings.TrimSpace(req.AgentName), TitleAtSend: senderInfo.Name, Hop: hop,
		NotifyOnComplete: req.NotifyOnComplete, InReplyTo: inReplyTo,
	}.Normalize()
	if err := origin.Validate(); err != nil {
		return nil, err
	}
	return &origin, nil
}

// CanReplyToSession verifies a reply against durable incoming message provenance.
func (m *Manager) CanReplyToSession(ctx context.Context, sender, target, messageID string) (bool, error) {
	if sender == "" || target == "" || messageID == "" || sender == target {
		return false, nil
	}
	if err := acp.ValidatePromptOriginInReplyTo(messageID); err != nil {
		return false, err
	}
	info, err := m.Status(ctx, sender)
	if err != nil {
		return false, err
	}
	return m.repliesToTargetMessage(ctx, info.WorkspaceID, sender, target, messageID)
}

// repliesToTargetMessage reports whether messageID was admitted into the sender from the target session.
func (m *Manager) repliesToTargetMessage(
	ctx context.Context,
	senderWorkspaceID, sender, target, messageID string,
) (bool, error) {
	ledger, ok := m.promptAdmissionStore.(store.SessionMessageLedger)
	if !ok {
		return false, nil
	}
	admission, found, err := ledger.LookupSessionPromptAdmissionByMessageID(ctx, senderWorkspaceID, sender, messageID)
	if err != nil || !found {
		return false, err
	}
	origin, err := decodePromptOrigin(admission.Origin)
	if err != nil {
		return false, err
	}
	return origin != nil && origin.SessionID == target, nil
}

func (m *Manager) messagingConfig() compozyconfig.SessionMessagingConfig {
	return m.messaging.Normalize()
}

// checkSessionMessageBudget refuses a send once the sender spent its budget for this target.
func (m *Manager) checkSessionMessageBudget(ctx context.Context, sender, target string) error {
	cfg := m.messagingConfig()
	ledger, ok := m.promptAdmissionStore.(store.SessionMessageLedger)
	if !ok || cfg.RateLimitMessages == compozyconfig.SessionMessagingRateLimitDisabled {
		return nil
	}
	targetInfo, err := m.Status(ctx, target)
	if err != nil {
		return err
	}
	now := m.now()
	window, err := ledger.SessionMessageWindow(
		ctx, targetInfo.WorkspaceID, target, sender, now.Add(-cfg.RateLimitWindow),
	)
	if err != nil {
		return err
	}
	if window.Count < cfg.RateLimitMessages {
		return nil
	}
	retryAfter := time.Second
	if !window.OldestAt.IsZero() {
		retryAfter = max(window.OldestAt.Add(cfg.RateLimitWindow).Sub(now), time.Second).Round(time.Second)
	}
	m.logger.WarnContext(
		ctx, "session.message.rejected", "reason", "rate_limited",
		"sender_session_id", sender, "target_session_id", target, "count", window.Count,
	)
	return &MessageRateLimitError{
		TargetSessionID: target, Limit: cfg.RateLimitMessages, Window: cfg.RateLimitWindow, RetryAfter: retryAfter,
	}
}
