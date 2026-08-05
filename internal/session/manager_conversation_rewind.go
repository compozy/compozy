package session

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/transcript"
)

// ConversationRewindOptions identifies one durable user message and the
// transcript version the caller observed before requesting a rewind.
type ConversationRewindOptions struct {
	MessageID           string
	IdempotencyKey      string
	ExpectedEpoch       int64
	ExpectedGeneration  int64
	ExpectedMaxSequence int64
}

// ConversationRewindResult describes the committed transcript cut and the
// fresh runtime session that now owns the retained conversation prefix.
type ConversationRewindResult struct {
	Session         *Session
	TranscriptEpoch int64
	TargetMessageID string
	ArchivedFrom    int64
	ArchivedThrough int64
	ArchivedEvents  int64
	Generation      int64
	MaxSequence     int64
	DraftText       string
	Replayed        bool
}

type conversationRewindIdentity struct {
	SessionID           string `json:"session_id"`
	MessageID           string `json:"message_id"`
	ExpectedEpoch       int64  `json:"expected_epoch"`
	ExpectedGeneration  int64  `json:"expected_generation"`
	ExpectedMaxSequence int64  `json:"expected_max_sequence"`
}

type conversationRewindRequest struct {
	sessionID      string
	messageID      string
	idempotencyKey string
	requestHash    string
	options        ConversationRewindOptions
}

// RewindConversation archives the selected user message and the active suffix,
// then restarts the same logical session with a fresh ACP conversation context.
func (m *Manager) RewindConversation(
	ctx context.Context,
	id string,
	opts ConversationRewindOptions,
) (ConversationRewindResult, error) {
	if m == nil {
		return ConversationRewindResult{}, errors.New("session: manager is required")
	}
	if ctx == nil {
		return ConversationRewindResult{}, errors.New("session: conversation rewind context is required")
	}
	if err := m.checkNewWorkAdmission(ctx); err != nil {
		return ConversationRewindResult{}, err
	}
	request, err := normalizeConversationRewindRequest(id, opts)
	if err != nil {
		return ConversationRewindResult{}, err
	}
	if m.isPending(request.sessionID) {
		return ConversationRewindResult{}, fmt.Errorf("%w: %s", ErrConversationRewindBusy, request.sessionID)
	}
	ctx, unlockConversation, err := m.lockConversationOperation(ctx, request.sessionID)
	if err != nil {
		return ConversationRewindResult{}, err
	}
	defer unlockConversation()
	if m.isPending(request.sessionID) {
		return ConversationRewindResult{}, fmt.Errorf("%w: %s", ErrConversationRewindBusy, request.sessionID)
	}
	request.requestHash, err = conversationRewindRequestHash(request.sessionID, request.messageID, opts)
	if err != nil {
		return ConversationRewindResult{}, err
	}
	return m.rewindConversationLocked(ctx, request)
}

func normalizeConversationRewindRequest(
	id string,
	options ConversationRewindOptions,
) (conversationRewindRequest, error) {
	request := conversationRewindRequest{
		sessionID:      strings.TrimSpace(id),
		messageID:      strings.TrimSpace(options.MessageID),
		idempotencyKey: strings.TrimSpace(options.IdempotencyKey),
		options:        options,
	}
	if request.sessionID == "" || request.messageID == "" || request.idempotencyKey == "" {
		return conversationRewindRequest{}, fmt.Errorf(
			"%w: session id, message id, and idempotency key are required",
			ErrValidation,
		)
	}
	if options.ExpectedEpoch < 0 || options.ExpectedGeneration < 0 || options.ExpectedMaxSequence < 0 {
		return conversationRewindRequest{}, fmt.Errorf(
			"%w: conversation rewind fences cannot be negative",
			ErrValidation,
		)
	}
	return request, nil
}

func (m *Manager) rewindConversationLocked(
	ctx context.Context,
	request conversationRewindRequest,
) (ConversationRewindResult, error) {
	receipt, found, preflightTarget, err := m.readConversationRewindPreflight(
		ctx, request.sessionID, request.idempotencyKey, request.requestHash, request.messageID,
	)
	if err != nil {
		return ConversationRewindResult{}, err
	}
	if found {
		return m.resumeKnownConversationRewind(ctx, request.sessionID, receipt)
	}
	info, err := m.Status(ctx, request.sessionID)
	if err != nil {
		return ConversationRewindResult{}, err
	}
	if info.Type != SessionTypeUser || conversationHasParentLineage(info.Lineage) {
		return ConversationRewindResult{}, fmt.Errorf(
			"%w: %s",
			ErrConversationRewindManaged,
			request.sessionID,
		)
	}
	if info.TranscriptEpoch != request.options.ExpectedEpoch ||
		preflightTarget.Generation != request.options.ExpectedGeneration ||
		preflightTarget.MaxSequence != request.options.ExpectedMaxSequence {
		return ConversationRewindResult{}, fmt.Errorf(
			"%w: %s",
			ErrConversationRewindFenceConflict,
			request.sessionID,
		)
	}
	active, activeBefore := m.Get(request.sessionID)
	return m.executeConversationRewind(ctx, request, active, activeBefore)
}

func (m *Manager) resumeKnownConversationRewind(
	ctx context.Context,
	sessionID string,
	receipt store.ConversationRewindResult,
) (ConversationRewindResult, error) {
	if _, active := m.Get(sessionID); !active {
		if err := m.beginConversationFinalization(sessionID); err != nil {
			return ConversationRewindResult{}, err
		}
		defer m.endConversationFinalization(sessionID)
	}
	return m.resumeCommittedConversationRewind(ctx, sessionID, receipt)
}

func (m *Manager) executeConversationRewind(
	ctx context.Context,
	request conversationRewindRequest,
	active *Session,
	activeBefore bool,
) (ConversationRewindResult, error) {
	if activeBefore {
		if err := active.reserveConversationRewind(); err != nil {
			return ConversationRewindResult{}, err
		}
		defer active.releaseConversationRewind()
	}
	queue, err := m.InputQueueSummary(ctx, request.sessionID)
	if err != nil {
		return ConversationRewindResult{}, err
	}
	if queue.PendingInputs > 0 {
		return ConversationRewindResult{}, fmt.Errorf(
			"%w: %s",
			ErrConversationRewindBusy,
			request.sessionID,
		)
	}
	_, replayedAfterReservation, quiescedTarget, err := m.readConversationRewindPreflight(
		ctx, request.sessionID, request.idempotencyKey, request.requestHash, request.messageID,
	)
	if err != nil {
		return ConversationRewindResult{}, err
	}
	if replayedAfterReservation ||
		quiescedTarget.Generation != request.options.ExpectedGeneration ||
		quiescedTarget.MaxSequence != request.options.ExpectedMaxSequence {
		return ConversationRewindResult{}, fmt.Errorf(
			"%w: %s",
			ErrConversationRewindFenceConflict,
			request.sessionID,
		)
	}
	prefixDigest, err := m.conversationActivePrefixDigest(
		ctx,
		request.sessionID,
		request.options.ExpectedMaxSequence,
	)
	if err != nil {
		return ConversationRewindResult{}, err
	}
	if activeBefore {
		if err := m.StopWithCause(
			ctx,
			request.sessionID,
			CauseConversationRewind,
			"conversation rewound",
		); err != nil {
			return ConversationRewindResult{}, err
		}
	}
	if err := m.beginConversationFinalization(request.sessionID); err != nil {
		return ConversationRewindResult{}, err
	}
	defer m.endConversationFinalization(request.sessionID)
	return m.commitStoppedConversationRewind(ctx, request, active, activeBefore, prefixDigest)
}

func (m *Manager) commitStoppedConversationRewind(
	ctx context.Context,
	request conversationRewindRequest,
	active *Session,
	activeBefore bool,
	prefixDigest [sha256.Size]byte,
) (ConversationRewindResult, error) {
	_, replayedAfterStop, stoppedTarget, err := m.readConversationRewindPreflight(
		ctx, request.sessionID, request.idempotencyKey, request.requestHash, request.messageID,
	)
	if err != nil {
		return ConversationRewindResult{}, err
	}
	if replayedAfterStop || stoppedTarget.Generation != request.options.ExpectedGeneration ||
		(!activeBefore && stoppedTarget.MaxSequence != request.options.ExpectedMaxSequence) {
		return ConversationRewindResult{}, fmt.Errorf(
			"%w: %s",
			ErrConversationRewindFenceConflict,
			request.sessionID,
		)
	}
	stoppedPrefixDigest, err := m.conversationActivePrefixDigest(
		ctx,
		request.sessionID,
		request.options.ExpectedMaxSequence,
	)
	if err != nil {
		return ConversationRewindResult{}, err
	}
	if prefixDigest != stoppedPrefixDigest {
		return ConversationRewindResult{}, fmt.Errorf(
			"%w: %s",
			ErrConversationRewindFenceConflict,
			request.sessionID,
		)
	}
	if activeBefore {
		if err := m.validateConversationRewindStopTail(
			ctx,
			request.sessionID,
			request.options.ExpectedMaxSequence,
		); err != nil {
			return ConversationRewindResult{}, err
		}
	}
	targetEpoch, err := m.nextTranscriptEpoch(ctx, active, request.sessionID)
	if err != nil {
		return ConversationRewindResult{}, err
	}
	storedResult, err := m.commitConversationRewind(
		ctx, request.sessionID, request.messageID, request.idempotencyKey, request.requestHash, targetEpoch,
		stoppedTarget.Generation, stoppedTarget.MaxSequence,
	)
	if err != nil {
		return ConversationRewindResult{}, err
	}
	return m.resumeCommittedConversationRewind(ctx, request.sessionID, storedResult)
}

func conversationHasParentLineage(lineage *store.SessionLineage) bool {
	return lineage != nil && strings.TrimSpace(lineage.ParentSessionID) != ""
}

func (m *Manager) readConversationRewindPreflight(
	ctx context.Context,
	sessionID string,
	idempotencyKey string,
	requestHash string,
	messageID string,
) (_ store.ConversationRewindResult, _ bool, _ store.ConversationRewindTarget, retErr error) {
	recorder, cleanup, err := m.openQueryRecorder(ctx, sessionID)
	if err != nil {
		return store.ConversationRewindResult{}, false, store.ConversationRewindTarget{}, err
	}
	defer func() { retErr = errors.Join(retErr, cleanup()) }()
	reader, ok := recorder.(store.ConversationRewindReader)
	if !ok {
		return store.ConversationRewindResult{}, false, store.ConversationRewindTarget{},
			errors.New("session: event recorder does not support conversation rewind")
	}
	receipt, found, err := reader.ConversationRewindReceipt(ctx, idempotencyKey, requestHash)
	if err != nil || found {
		return receipt, found, store.ConversationRewindTarget{}, err
	}
	managed, err := recorder.Query(ctx, store.EventQuery{
		Type: EventTypeGoalSnapshotChanged, Limit: 1, Archive: store.EventArchiveUnarchived,
	})
	if err != nil {
		return store.ConversationRewindResult{}, false, store.ConversationRewindTarget{},
			fmt.Errorf("session: query managed conversation state for %q: %w", sessionID, err)
	}
	if conversationHasManagedGoal(managed) {
		return store.ConversationRewindResult{}, false, store.ConversationRewindTarget{},
			fmt.Errorf("%w: %s", ErrConversationRewindManaged, sessionID)
	}
	target, err := reader.ConversationRewindTarget(ctx, messageID)
	return store.ConversationRewindResult{}, false, target, err
}

func conversationHasManagedGoal(events []store.SessionEvent) bool {
	for _, event := range events {
		var projection struct {
			RunID *string `json:"run_id"`
		}
		if err := json.Unmarshal([]byte(event.Content), &projection); err != nil {
			return true
		}
		if projection.RunID != nil && strings.TrimSpace(*projection.RunID) != "" {
			return true
		}
	}
	return false
}

func (m *Manager) commitConversationRewind(
	ctx context.Context,
	sessionID string,
	messageID string,
	idempotencyKey string,
	requestHash string,
	transcriptEpoch int64,
	expectedGeneration int64,
	expectedMaxSequence int64,
) (_ store.ConversationRewindResult, retErr error) {
	recorder, cleanup, err := m.openMutationRecorder(ctx, sessionID)
	if err != nil {
		return store.ConversationRewindResult{}, err
	}
	defer func() { retErr = errors.Join(retErr, cleanup()) }()
	rewinder, ok := recorder.(store.ConversationRewinder)
	if !ok {
		return store.ConversationRewindResult{}, errors.New(
			"session: event recorder does not support conversation rewind",
		)
	}
	target, err := rewinder.ConversationRewindTarget(ctx, messageID)
	if err != nil {
		return store.ConversationRewindResult{}, err
	}
	events, err := recorder.Query(ctx, store.EventQuery{
		BeforeSequence: target.StartSequence,
		Archive:        store.EventArchiveUnarchived,
	})
	if err != nil {
		return store.ConversationRewindResult{}, fmt.Errorf("session: query retained conversation prefix: %w", err)
	}
	messages, err := transcript.Assemble(events)
	if err != nil {
		return store.ConversationRewindResult{}, fmt.Errorf("session: assemble retained conversation prefix: %w", err)
	}
	messages = transcript.Prune(messages, transcript.PruneOptions{Dedup: true})
	baseline, err := json.Marshal(messages)
	if err != nil {
		return store.ConversationRewindResult{}, fmt.Errorf("session: marshal retained conversation prefix: %w", err)
	}
	result, err := rewinder.RewindConversation(ctx, store.ConversationRewindRequest{
		MessageID: messageID, IdempotencyKey: idempotencyKey, RequestHash: requestHash,
		ExpectedGeneration: expectedGeneration, ExpectedMaxSequence: expectedMaxSequence,
		TranscriptEpoch: transcriptEpoch, BaselineJSON: string(baseline),
	})
	if err != nil {
		if errors.Is(err, store.ErrConversationRewindFenceConflict) {
			return store.ConversationRewindResult{}, ErrConversationRewindFenceConflict
		}
		return store.ConversationRewindResult{}, err
	}
	return result, nil
}

func (m *Manager) validateConversationRewindStopTail(
	ctx context.Context,
	sessionID string,
	expectedMaxSequence int64,
) (retErr error) {
	recorder, cleanup, err := m.openQueryRecorder(ctx, sessionID)
	if err != nil {
		return err
	}
	defer func() { retErr = errors.Join(retErr, cleanup()) }()
	events, err := recorder.Query(ctx, store.EventQuery{
		AfterSequence: expectedMaxSequence,
		Archive:       store.EventArchiveUnarchived,
	})
	if err != nil {
		return fmt.Errorf("session: query conversation rewind stop tail: %w", err)
	}
	if len(events) != 1 {
		return fmt.Errorf("%w: %s", ErrConversationRewindFenceConflict, sessionID)
	}
	event := events[0]
	if event.Type != EventTypeSessionStopped {
		return fmt.Errorf("%w: %s", ErrConversationRewindFenceConflict, sessionID)
	}
	var payload struct {
		StopReason string `json:"stop_reason"`
	}
	if err := json.Unmarshal([]byte(event.Content), &payload); err != nil ||
		payload.StopReason != string(store.StopCompleted) {
		return fmt.Errorf("%w: %s", ErrConversationRewindFenceConflict, sessionID)
	}
	return nil
}

func (m *Manager) conversationActivePrefixDigest(
	ctx context.Context,
	sessionID string,
	maxSequence int64,
) (_ [sha256.Size]byte, retErr error) {
	recorder, cleanup, err := m.openQueryRecorder(ctx, sessionID)
	if err != nil {
		return [sha256.Size]byte{}, err
	}
	defer func() { retErr = errors.Join(retErr, cleanup()) }()
	events, err := recorder.Query(ctx, store.EventQuery{
		BeforeSequence: maxSequence + 1,
		Archive:        store.EventArchiveUnarchived,
	})
	if err != nil {
		return [sha256.Size]byte{}, fmt.Errorf("session: query conversation rewind prefix fence: %w", err)
	}
	payload, err := json.Marshal(events)
	if err != nil {
		return [sha256.Size]byte{}, fmt.Errorf("session: marshal conversation rewind prefix fence: %w", err)
	}
	return sha256.Sum256(payload), nil
}

func conversationRewindRequestHash(
	sessionID string,
	messageID string,
	opts ConversationRewindOptions,
) (string, error) {
	payload, err := json.Marshal(conversationRewindIdentity{
		SessionID: sessionID, MessageID: messageID, ExpectedEpoch: opts.ExpectedEpoch,
		ExpectedGeneration: opts.ExpectedGeneration, ExpectedMaxSequence: opts.ExpectedMaxSequence,
	})
	if err != nil {
		return "", fmt.Errorf("session: marshal conversation rewind identity: %w", err)
	}
	digest := sha256.Sum256(payload)
	return hex.EncodeToString(digest[:]), nil
}
