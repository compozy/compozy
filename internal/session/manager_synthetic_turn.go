package session

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/transcript"
)

// SyntheticTurnRecord is one daemon-authored transcript turn that does not
// require a provider runtime to execute it.
type SyntheticTurnRecord struct {
	Identity  string
	Message   string
	MessageID string
	Metadata  acp.PromptSyntheticMeta
	CreatedAt time.Time
}

// RecordSyntheticTurn durably appends one idempotent daemon-authored turn.
func (m *Manager) RecordSyntheticTurn(
	ctx context.Context,
	sessionID string,
	record SyntheticTurnRecord,
) error {
	if m == nil {
		return errors.New("session: manager is required")
	}
	if ctx == nil {
		return errors.New("session: synthetic turn context is required")
	}
	target := strings.TrimSpace(sessionID)
	identity := strings.TrimSpace(record.Identity)
	message := strings.TrimSpace(record.Message)
	if target == "" || identity == "" || message == "" {
		return errors.New("session: synthetic turn session, identity, and message are required")
	}
	metadata := record.Metadata.Normalize()
	promptMeta := acp.PromptMeta{
		TurnSource: acp.PromptTurnSourceSynthetic,
		Synthetic:  &metadata,
	}
	if err := promptMeta.Validate(); err != nil {
		return fmt.Errorf("session: validate synthetic turn metadata: %w", err)
	}
	info, err := m.Status(ctx, target)
	if err != nil {
		return err
	}
	createdAt := record.CreatedAt.UTC()
	if createdAt.IsZero() {
		createdAt = m.now().UTC()
	}
	digest := sha256.Sum256([]byte(target + "\x00" + identity))
	suffix := hex.EncodeToString(digest[:12])
	event := acp.AgentEvent{
		Type:      acp.EventTypeSyntheticReentry,
		SessionID: target,
		TurnID:    "turn_synthetic_" + suffix,
		Timestamp: createdAt,
		Text:      message,
		Synthetic: &metadata,
	}
	if messageID := strings.TrimSpace(record.MessageID); messageID != "" {
		event = event.WithMessageID(messageID)
	}
	payload, err := transcript.MarshalAgentEvent(event)
	if err != nil {
		return fmt.Errorf("session: marshal synthetic turn: %w", err)
	}
	persisted, err := m.appendDurableSessionEvent(ctx, target, store.SessionEvent{
		ID:        "ev_synthetic_" + suffix,
		SessionID: target,
		TurnID:    event.TurnID,
		Type:      event.Type,
		AgentName: info.AgentName,
		Content:   payload,
		Timestamp: createdAt,
	})
	if err != nil {
		return fmt.Errorf("session: persist synthetic turn: %w", err)
	}
	m.publishSessionEventByID(ctx, target, persisted)
	m.notifyAgentEventFromInfo(ctx, info, event)
	return nil
}
