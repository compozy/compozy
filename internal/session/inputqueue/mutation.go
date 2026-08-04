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

type mutationApply func(
	context.Context,
	string,
	string,
	store.SessionInputQueueInsert,
) (store.SessionInputQueueEntry, bool, error)

type mutationSpec struct {
	sessionID      string
	entryID        string
	text           string
	targetTurnID   string
	messageID      string
	idempotencyKey string
	mode           string
	delivery       string
	apply          mutationApply
}

// Replace atomically replaces one queued input with a new queue identity.
func (s *Service) Replace(
	ctx context.Context,
	sessionID string,
	entryID string,
	text string,
	messageID string,
	idempotencyKey string,
) (store.SessionInputQueueEntry, bool, error) {
	return s.mutate(ctx, mutationSpec{
		sessionID:      sessionID,
		entryID:        entryID,
		text:           text,
		messageID:      messageID,
		idempotencyKey: idempotencyKey,
		mode:           store.SessionInputQueueModeQueue,
		delivery:       store.SessionInputDeliveryAfterTurn,
		apply:          s.store.ReplaceSessionInput,
	})
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
	return s.mutate(ctx, mutationSpec{
		sessionID:      sessionID,
		entryID:        entryID,
		text:           text,
		targetTurnID:   targetTurnID,
		messageID:      messageID,
		idempotencyKey: idempotencyKey,
		mode:           store.SessionInputQueueModeSteer,
		delivery:       store.SessionInputDeliveryInterruptThenPrompt,
		apply:          s.store.PromoteSessionInputToSteer,
	})
}

func (s *Service) mutate(
	ctx context.Context,
	spec mutationSpec,
) (store.SessionInputQueueEntry, bool, error) {
	targetSessionID := strings.TrimSpace(spec.sessionID)
	targetEntryID := strings.TrimSpace(spec.entryID)
	mutationID := durableMutationID(targetSessionID, targetEntryID, spec.idempotencyKey)
	if replayed, ok, err := s.replayMutation(
		ctx,
		targetSessionID,
		mutationID,
		spec.text,
		spec.messageID,
		spec.idempotencyKey,
		spec.mode,
		spec.delivery,
		spec.targetTurnID,
	); err != nil || ok {
		return replayed, false, err
	}
	existing, err := s.Get(ctx, targetSessionID, targetEntryID)
	if err != nil {
		return store.SessionInputQueueEntry{}, false, err
	}
	replacement, err := s.newInsert(insertSpec{
		sessionID:    targetSessionID,
		text:         spec.text,
		mode:         spec.mode,
		delivery:     spec.delivery,
		targetTurnID: spec.targetTurnID,
		generation:   existing.SessionGeneration,
		runtime:      existing.Runtime,
	})
	if err != nil {
		return store.SessionInputQueueEntry{}, false, err
	}
	replacement.ID = mutationID
	replacement.MessageID = strings.TrimSpace(spec.messageID)
	replacement.IdempotencyKey = strings.TrimSpace(spec.idempotencyKey)
	return spec.apply(ctx, targetSessionID, targetEntryID, replacement)
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
