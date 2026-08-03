package inputqueue

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/store"
)

// Replace atomically replaces one queued input with a new queue identity.
func (s *Service) Replace(
	ctx context.Context,
	sessionID string,
	entryID string,
	text string,
	messageID string,
	idempotencyKey string,
) (store.SessionInputQueueEntry, bool, error) {
	targetSessionID := strings.TrimSpace(sessionID)
	targetEntryID := strings.TrimSpace(entryID)
	mutationID := durableMutationID(targetSessionID, targetEntryID, idempotencyKey)
	if replayed, ok, err := s.replayMutation(
		ctx,
		targetSessionID,
		mutationID,
		text,
		messageID,
		idempotencyKey,
		store.SessionInputQueueModeQueue,
		store.SessionInputDeliveryAfterTurn,
		"",
	); err != nil || ok {
		return replayed, false, err
	}
	existing, err := s.Get(ctx, sessionID, entryID)
	if err != nil {
		return store.SessionInputQueueEntry{}, false, err
	}
	replacement, err := s.newInsert(
		sessionID,
		text,
		store.SessionInputQueueModeQueue,
		store.SessionInputDeliveryAfterTurn,
		"",
		existing.SessionGeneration,
		existing.Runtime,
	)
	if err != nil {
		return store.SessionInputQueueEntry{}, false, err
	}
	replacement.ID = mutationID
	replacement.MessageID = strings.TrimSpace(messageID)
	replacement.IdempotencyKey = strings.TrimSpace(idempotencyKey)
	return s.store.ReplaceSessionInput(ctx, targetSessionID, targetEntryID, replacement)
}

// PromoteToSteer atomically replaces one queued input with priority steering input.
func (s *Service) PromoteToSteer(
	ctx context.Context,
	sessionID string,
	entryID string,
	text string,
	targetTurnID string,
	messageID string,
	idempotencyKey string,
) (store.SessionInputQueueEntry, bool, error) {
	targetSessionID := strings.TrimSpace(sessionID)
	targetEntryID := strings.TrimSpace(entryID)
	mutationID := durableMutationID(targetSessionID, targetEntryID, idempotencyKey)
	if replayed, ok, err := s.replayMutation(
		ctx,
		targetSessionID,
		mutationID,
		text,
		messageID,
		idempotencyKey,
		store.SessionInputQueueModeSteer,
		store.SessionInputDeliveryInterruptThenPrompt,
		targetTurnID,
	); err != nil || ok {
		return replayed, false, err
	}
	existing, err := s.Get(ctx, sessionID, entryID)
	if err != nil {
		return store.SessionInputQueueEntry{}, false, err
	}
	replacement, err := s.newInsert(
		sessionID,
		text,
		store.SessionInputQueueModeSteer,
		store.SessionInputDeliveryInterruptThenPrompt,
		targetTurnID,
		existing.SessionGeneration,
		existing.Runtime,
	)
	if err != nil {
		return store.SessionInputQueueEntry{}, false, err
	}
	replacement.ID = mutationID
	replacement.MessageID = strings.TrimSpace(messageID)
	replacement.IdempotencyKey = strings.TrimSpace(idempotencyKey)
	return s.store.PromoteSessionInputToSteer(
		ctx,
		targetSessionID,
		targetEntryID,
		replacement,
	)
}

// ReplayPromotion returns an exact prior queue-to-steer mutation without changing queue state.
func (s *Service) ReplayPromotion(
	ctx context.Context,
	sessionID string,
	entryID string,
	text string,
	targetTurnID string,
	messageID string,
	idempotencyKey string,
) (store.SessionInputQueueEntry, bool, error) {
	return s.replayMutation(
		ctx,
		strings.TrimSpace(sessionID),
		durableMutationID(sessionID, entryID, idempotencyKey),
		text,
		messageID,
		idempotencyKey,
		store.SessionInputQueueModeSteer,
		store.SessionInputDeliveryInterruptThenPrompt,
		targetTurnID,
	)
}

func (s *Service) replayMutation(
	ctx context.Context,
	sessionID string,
	mutationID string,
	text string,
	messageID string,
	idempotencyKey string,
	mode string,
	delivery string,
	targetTurnID string,
) (store.SessionInputQueueEntry, bool, error) {
	entry, err := s.store.GetSessionInputQueueEntry(ctx, sessionID, mutationID)
	if errors.Is(err, store.ErrSessionInputQueueEntryNotFound) {
		return store.SessionInputQueueEntry{}, false, nil
	}
	if err != nil {
		return store.SessionInputQueueEntry{}, false, err
	}
	if !sameMutation(entry, text, messageID, idempotencyKey, mode, delivery, targetTurnID) {
		return store.SessionInputQueueEntry{}, false, fmt.Errorf(
			"%w: %s",
			store.ErrSessionInputMutationConflict,
			mutationID,
		)
	}
	return entry, true, nil
}

func durableMutationID(sessionID string, entryID string, idempotencyKey string) string {
	digest := sha256.Sum256([]byte(strings.Join([]string{
		strings.TrimSpace(sessionID),
		strings.TrimSpace(entryID),
		strings.TrimSpace(idempotencyKey),
	}, "\x00")))
	return "inq_mut_" + hex.EncodeToString(digest[:16])
}

func sameMutation(
	entry store.SessionInputQueueEntry,
	text string,
	messageID string,
	idempotencyKey string,
	mode string,
	delivery string,
	targetTurnID string,
) bool {
	return entry.Text == strings.TrimSpace(text) &&
		entry.MessageID == strings.TrimSpace(messageID) &&
		entry.IdempotencyKey == strings.TrimSpace(idempotencyKey) &&
		entry.Mode == mode &&
		entry.Delivery == delivery &&
		entry.TargetTurnID == strings.TrimSpace(targetTurnID)
}
