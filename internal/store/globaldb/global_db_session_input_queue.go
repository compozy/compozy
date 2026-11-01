package globaldb

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	commandpkg "github.com/compozy/compozy/internal/command"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

const sessionInputGenerationColumn = "input_generation"

// EnqueueSessionInput appends one queued operator input entry under the configured cap.
func (g *SessionRepo) EnqueueSessionInput(
	ctx context.Context,
	req store.SessionInputQueueInsert,
) (entry store.SessionInputQueueEntry, position int, err error) {
	if err := g.checkReady(ctx, "enqueue session input"); err != nil {
		return store.SessionInputQueueEntry{}, 0, err
	}
	normalized := req.Normalize()
	if normalized.Mode == "" {
		normalized.Mode = store.SessionInputQueueModeQueue
	}
	if normalized.Delivery == "" {
		normalized.Delivery = store.SessionInputDeliveryAfterTurn
	}
	if err := normalized.Validate(); err != nil {
		return store.SessionInputQueueEntry{}, 0, err
	}
	if err := validateCallerOwnedQueueIdentity(normalized); err != nil {
		return store.SessionInputQueueEntry{}, 0, err
	}

	err = g.withImmediateTransaction(ctx, "enqueue session input", func(exec globalSQLExecutor) error {
		if normalized.OwnerKind == store.SessionInputOwnerSynthetic && normalized.MessageID != "" {
			existing, lookupErr := sqlcgen.New(exec).GetSyntheticSessionInputByMessage(
				ctx,
				sqlcgen.GetSyntheticSessionInputByMessageParams{
					SessionID: normalized.SessionID,
					MessageID: normalized.MessageID,
				},
			)
			if lookupErr == nil {
				entry, lookupErr = sessionInputQueueFromGenerated(&existing)
				return lookupErr
			}
			if !errors.Is(lookupErr, sql.ErrNoRows) {
				return lookupErr
			}
		}
		if capErr := checkSessionInputQueueCap(ctx, exec, normalized); capErr != nil {
			return capErr
		}
		inserted, insertErr := insertSessionInputQueueEntry(ctx, exec, normalized)
		if insertErr != nil {
			return insertErr
		}
		rank, rankErr := sessionInputQueuePosition(ctx, exec, normalized.SessionID, inserted.ID)
		if rankErr != nil {
			return rankErr
		}
		entry = inserted
		position = rank
		return nil
	})
	if err != nil {
		return store.SessionInputQueueEntry{}, 0, err
	}
	return entry, position, nil
}

// StageSessionSteer replaces any active staged steer entry for the session.
func (g *SessionRepo) StageSessionSteer(
	ctx context.Context,
	req store.SessionInputQueueInsert,
) (entry store.SessionInputQueueEntry, err error) {
	if err := g.checkReady(ctx, "stage session steer"); err != nil {
		return store.SessionInputQueueEntry{}, err
	}
	normalized := req.Normalize()
	normalized.Mode = store.SessionInputQueueModeSteer
	normalized.Delivery = store.SessionInputDeliveryInterruptThenPrompt
	if err := normalized.Validate(); err != nil {
		return store.SessionInputQueueEntry{}, err
	}
	if err := validateCallerOwnedQueueIdentity(normalized); err != nil {
		return store.SessionInputQueueEntry{}, err
	}

	err = g.withImmediateTransaction(ctx, "stage session steer", func(exec globalSQLExecutor) error {
		nowRaw := store.FormatTimestamp(normalized.Now)
		superseded, cancelErr := sqlcgen.New(exec).CancelPriorSessionSteers(ctx, sqlcgen.CancelPriorSessionSteersParams{
			CanceledStatus: store.SessionInputQueueStatusCanceled, CanceledAt: nullableSessionTime(normalized.Now),
			UpdatedAt: nowRaw, SessionID: normalized.SessionID, SteerMode: store.SessionInputQueueModeSteer,
			QueuedStatus: store.SessionInputQueueStatusQueued, OriginSessionID: normalized.OriginSessionID,
		})
		if cancelErr != nil {
			return fmt.Errorf("store: cancel prior session steer input: %w", cancelErr)
		}
		inserted, insertErr := insertSessionInputQueueEntry(ctx, exec, normalized)
		if insertErr != nil {
			return insertErr
		}
		inserted.SupersededIDs = superseded
		entry = inserted
		return nil
	})
	if err != nil {
		return store.SessionInputQueueEntry{}, err
	}
	return entry, nil
}

func validateCallerOwnedQueueIdentity(req store.SessionInputQueueInsert) error {
	if req.PromptAdmissionID != "" || (req.MessageID != "" && req.OwnerKind != store.SessionInputOwnerSynthetic) ||
		req.IdempotencyKey != "" ||
		(req.TurnID != "" && req.OwnerKind != store.SessionInputOwnerSynthetic) ||
		req.EventID != "" {
		return errors.New("store: prompt admission identity is reserved for admitted session input")
	}
	return nil
}

// ClaimNextSessionInput atomically leases the next eligible input for dispatch.
func (g *SessionRepo) ClaimNextSessionInput(
	ctx context.Context,
	sessionID string,
	now time.Time,
) (entry store.SessionInputQueueEntry, ok bool, err error) {
	if err := g.checkReady(ctx, "claim session input"); err != nil {
		return store.SessionInputQueueEntry{}, false, err
	}
	target := strings.TrimSpace(sessionID)
	if target == "" {
		return store.SessionInputQueueEntry{}, false, errors.New("store: session id is required")
	}
	if now.IsZero() {
		now = g.now()
	}
	now = now.UTC()

	err = g.withImmediateTransaction(ctx, "claim session input", func(exec globalSQLExecutor) error {
		queries := sqlcgen.New(exec)
		if err := queries.FreezePreparedGoalInputBatch(ctx, sqlcgen.FreezePreparedGoalInputBatchParams{
			SessionID: target, QueuedStatus: store.SessionInputQueueStatusQueued,
		}); err != nil {
			return fmt.Errorf("store: freeze prepared Goal input batch: %w", err)
		}
		row, scanErr := queries.GetNextDispatchableSessionInput(
			ctx,
			sqlcgen.GetNextDispatchableSessionInputParams{
				SessionID: target, QueuedStatus: store.SessionInputQueueStatusQueued,
			},
		)
		if errors.Is(scanErr, sql.ErrNoRows) {
			return nil
		}
		if scanErr != nil {
			return scanErr
		}
		claimed, scanErr := sessionInputQueueFromGenerated(&row)
		if scanErr != nil {
			return scanErr
		}
		refreshed, claimErr := claimSessionInputInTransaction(
			ctx, exec, target, &claimed, now, store.FormatTimestamp(now),
		)
		if claimErr != nil {
			return claimErr
		}
		entry = refreshed
		ok = true
		return nil
	})
	if err != nil {
		return store.SessionInputQueueEntry{}, false, err
	}
	return entry, ok, nil
}

// PeekNextSessionInput returns the oldest eligible human or managed row without claiming it.
func (g *SessionRepo) PeekNextSessionInput(
	ctx context.Context,
	sessionID string,
) (store.SessionInputQueueEntry, bool, error) {
	if err := g.checkReady(ctx, "peek next session input"); err != nil {
		return store.SessionInputQueueEntry{}, false, err
	}
	target := strings.TrimSpace(sessionID)
	if target == "" {
		return store.SessionInputQueueEntry{}, false, errors.New("store: session id is required")
	}
	row, err := g.queries.PeekNextSessionInput(ctx, sqlcgen.PeekNextSessionInputParams{
		SessionID: target, QueuedStatus: store.SessionInputQueueStatusQueued,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return store.SessionInputQueueEntry{}, false, nil
	}
	if err != nil {
		return store.SessionInputQueueEntry{}, false, err
	}
	entry, err := sessionInputQueueFromGenerated(&row)
	if err != nil {
		return store.SessionInputQueueEntry{}, false, err
	}
	return entry, true, nil
}

// GetSessionInputQueueEntry returns one exact queue entry for durable await/recovery.
func (g *SessionRepo) GetSessionInputQueueEntry(
	ctx context.Context,
	sessionID string,
	entryID string,
) (store.SessionInputQueueEntry, error) {
	if err := g.checkReady(ctx, "get session input queue entry"); err != nil {
		return store.SessionInputQueueEntry{}, err
	}
	return getSessionInputQueueEntry(ctx, g.db, strings.TrimSpace(sessionID), strings.TrimSpace(entryID))
}

// AdvanceSessionInputGeneration increments the session generation used to fence stale queue entries.
func (g *SessionRepo) AdvanceSessionInputGeneration(
	ctx context.Context,
	sessionID string,
	now time.Time,
) (int64, error) {
	if err := g.checkReady(ctx, "advance session input generation"); err != nil {
		return 0, err
	}
	target := strings.TrimSpace(sessionID)
	if target == "" {
		return 0, errors.New("store: session id is required")
	}
	if now.IsZero() {
		now = g.now()
	}
	nowRaw := store.FormatTimestamp(now.UTC())
	var generation int64
	err := g.withImmediateTransaction(ctx, "advance session input generation", func(exec globalSQLExecutor) error {
		queries := sqlcgen.New(exec)
		affected, updateErr := queries.AdvanceSessionInputGeneration(ctx, sqlcgen.AdvanceSessionInputGenerationParams{
			UpdatedAt: nowRaw, ID: target,
		})
		if updateErr != nil {
			return fmt.Errorf("store: advance session input generation: %w", updateErr)
		}
		if affected == 0 {
			return fmt.Errorf("%w: %s", store.ErrSessionInputQueueEntryNotFound, target)
		}
		var scanErr error
		generation, scanErr = queries.GetSessionInputGeneration(ctx, target)
		if scanErr != nil {
			return fmt.Errorf("store: read session input generation: %w", scanErr)
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return generation, nil
}

// CurrentSessionInputGeneration returns the persisted busy-input generation for a session.
func (g *SessionRepo) CurrentSessionInputGeneration(ctx context.Context, sessionID string) (int64, error) {
	if err := g.checkReady(ctx, "read session input generation"); err != nil {
		return 0, err
	}
	target := strings.TrimSpace(sessionID)
	if target == "" {
		return 0, errors.New("store: session id is required")
	}
	generation, err := g.queries.GetSessionInputGeneration(ctx, target)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, fmt.Errorf("%w: %s", store.ErrSessionNotFound, target)
		}
		return 0, fmt.Errorf("store: read session input generation: %w", err)
	}
	return generation, nil
}

// SessionInputQueueSummary returns the current generation and active pending counts for one session.
func (g *SessionRepo) SessionInputQueueSummary(
	ctx context.Context,
	sessionID string,
) (summary store.SessionInputQueueSummary, err error) {
	if err := g.checkReady(ctx, "read session input queue summary"); err != nil {
		return store.SessionInputQueueSummary{}, err
	}
	target := strings.TrimSpace(sessionID)
	if target == "" {
		return store.SessionInputQueueSummary{}, errors.New("store: session id is required")
	}
	summary.SessionID = target
	summary.Generation, err = g.queries.GetSessionInputGeneration(ctx, target)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return store.SessionInputQueueSummary{}, fmt.Errorf("%w: %s", store.ErrSessionNotFound, target)
		}
		return store.SessionInputQueueSummary{}, fmt.Errorf("store: read session input generation: %w", err)
	}
	rows, err := g.queries.ListSessionInputQueueSummary(ctx, sqlcgen.ListSessionInputQueueSummaryParams{
		SessionID: target, SessionGeneration: summary.Generation,
		QueuedStatus:      store.SessionInputQueueStatusQueued,
		DispatchingStatus: store.SessionInputQueueStatusDispatching,
	})
	if err != nil {
		return store.SessionInputQueueSummary{}, fmt.Errorf("store: query session input queue summary: %w", err)
	}
	for _, row := range rows {
		count := int(row.Count)
		summary.PendingActive += count
		switch strings.TrimSpace(row.Mode) {
		case store.SessionInputQueueModeQueue:
			summary.PendingQueued += count
		case store.SessionInputQueueModeSteer:
			summary.PendingSteer += count
		}
		if strings.TrimSpace(row.Status) == store.SessionInputQueueStatusDispatching {
			summary.PendingLeased += count
		}
	}
	return summary, nil
}

func insertSessionInputQueueEntry(
	ctx context.Context,
	exec globalSQLExecutor,
	req store.SessionInputQueueInsert,
) (store.SessionInputQueueEntry, error) {
	normalized := req.Normalize()
	nowRaw := store.FormatTimestamp(normalized.Now)
	var runGeneration sql.NullInt64
	if normalized.RunGeneration != nil {
		runGeneration = sql.NullInt64{Int64: *normalized.RunGeneration, Valid: true}
	}
	skillInvocations := append([]commandpkg.Invocation{}, normalized.SkillInvocations...)
	skillInvocationsJSON, err := json.Marshal(skillInvocations)
	if err != nil {
		return store.SessionInputQueueEntry{}, fmt.Errorf("store: encode session input skill invocations: %w", err)
	}
	attachmentsJSON, err := encodeSessionInputAttachments(normalized.Attachments)
	if err != nil {
		return store.SessionInputQueueEntry{}, err
	}
	runtimeOptionsJSON, err := encodeSessionACPOptions(
		normalized.Runtime.ACPOptions,
		"session input runtime ACP options",
	)
	if err != nil {
		return store.SessionInputQueueEntry{}, err
	}
	syntheticJSON, err := encodeSyntheticQueuePrompt(normalized.SyntheticPrompt)
	if err != nil {
		return store.SessionInputQueueEntry{}, err
	}
	if err := sqlcgen.New(exec).InsertSessionInputQueueEntry(ctx, sqlcgen.InsertSessionInputQueueEntryParams{
		Priority:  int64(normalized.Priority),
		ID:        normalized.ID,
		SessionID: normalized.SessionID,
		PromptAdmissionID: sql.NullString{
			String: normalized.PromptAdmissionID,
			Valid:  normalized.PromptAdmissionID != "",
		},
		MessageID:      normalized.MessageID,
		IdempotencyKey: normalized.IdempotencyKey,
		TurnID:         normalized.TurnID,
		TargetTurnID:   normalized.TargetTurnID,
		EventID:        normalized.EventID,
		Status:         store.SessionInputQueueStatusQueued,
		Mode:           normalized.Mode,
		Delivery:       normalized.Delivery,
		SteerDelivery: sql.NullString{
			String: string(normalized.SteerDelivery),
			Valid:  normalized.SteerDelivery != "",
		},
		Text:                   normalized.Text,
		OwnerKind:              sql.NullString{String: normalized.OwnerKind, Valid: normalized.OwnerKind != ""},
		SyntheticPromptJson:    syntheticJSON,
		SkillInvocationsJson:   string(skillInvocationsJSON),
		MentionsJson:           store.NormalizeMentionsJSON(normalized.MentionsJSON),
		HumanIngress:           boolToInt64(normalized.HumanIngress),
		OriginJson:             sql.NullString{String: string(normalized.Origin), Valid: len(normalized.Origin) > 0},
		AttachmentsJson:        attachmentsJSON,
		RuntimeProvider:        normalized.Runtime.Provider,
		RuntimeModel:           normalized.Runtime.Model,
		RuntimeReasoningEffort: normalized.Runtime.ReasoningEffort,
		RuntimeSpeed:           normalized.Runtime.Speed,
		RuntimeAcpOptionsJson:  runtimeOptionsJSON,
		SessionGeneration:      normalized.SessionGeneration,
		TaskRunID:              normalized.TaskRunID,
		RunGeneration:          runGeneration,
		EnqueuedAt:             nowRaw,
		UpdatedAt:              nowRaw,
		OriginClass:            normalized.OriginClass,
		OriginSessionID:        normalized.OriginSessionID,
	}); err != nil {
		return store.SessionInputQueueEntry{}, fmt.Errorf("store: insert session input queue entry: %w", err)
	}
	return getSessionInputQueueEntry(ctx, exec, normalized.SessionID, normalized.ID)
}

// countPendingSessionInputs counts the capacity pool of originClass: human
// input and agent/synthetic input are capped separately so other sessions can
// never exhaust the operator's slots.
func countPendingSessionInputs(
	ctx context.Context,
	exec globalSQLExecutor,
	sessionID string,
	originClass string,
) (int, error) {
	pool := store.SessionInputOriginClassAgent
	if store.IsHumanSessionInputOriginClass(originClass) {
		pool = store.SessionInputOriginClassHuman
	}
	count, err := sqlcgen.New(exec).CountPendingSessionInputs(ctx, sqlcgen.CountPendingSessionInputsParams{
		SessionID: sessionID, QueuedStatus: store.SessionInputQueueStatusQueued,
		DispatchingStatus: store.SessionInputQueueStatusDispatching, PoolClass: pool,
	})
	if err != nil {
		return 0, fmt.Errorf("store: count pending session inputs: %w", err)
	}
	return int(count), nil
}

// checkSessionInputQueueCap refuses an insert whose capacity pool is full.
func checkSessionInputQueueCap(
	ctx context.Context,
	exec globalSQLExecutor,
	req store.SessionInputQueueInsert,
) error {
	normalized := req.Normalize()
	count, err := countPendingSessionInputs(ctx, exec, normalized.SessionID, normalized.OriginClass)
	if err != nil {
		return err
	}
	if count >= normalized.QueueCap {
		return &store.SessionInputQueueFullError{
			SessionID: normalized.SessionID, Cap: normalized.QueueCap, Count: count,
		}
	}
	return nil
}

// sessionInputQueuePosition reports the 1-based rank of one entry in the
// pending list's dispatch order, matching what queue readers display.
func sessionInputQueuePosition(
	ctx context.Context,
	exec globalSQLExecutor,
	sessionID string,
	entryID string,
) (int, error) {
	rows, err := sqlcgen.New(exec).ListPendingSessionInputs(ctx, sqlcgen.ListPendingSessionInputsParams{
		SessionID: sessionID, QueuedStatus: store.SessionInputQueueStatusQueued,
		DispatchingStatus: store.SessionInputQueueStatusDispatching,
	})
	if err != nil {
		return 0, fmt.Errorf("store: rank pending session input: %w", err)
	}
	for index := range rows {
		if rows[index].ID == entryID {
			return index + 1, nil
		}
	}
	return 0, fmt.Errorf("%w: %s", store.ErrSessionInputQueueEntryNotFound, entryID)
}

// SyntheticSessionInputAdmitted reports whether a synthetic input with messageID was already queued,
// dispatched, or sent for the session, so a replayed wake does not start a second turn.
func (g *SessionRepo) SyntheticSessionInputAdmitted(ctx context.Context, sessionID, messageID string) (bool, error) {
	if err := g.checkReady(ctx, "read synthetic session input"); err != nil {
		return false, err
	}
	admitted, err := g.queries.SyntheticSessionInputAdmitted(ctx, sqlcgen.SyntheticSessionInputAdmittedParams{
		SessionID: sessionID, MessageID: messageID,
	})
	if err != nil {
		return false, fmt.Errorf("store: read synthetic session input: %w", err)
	}
	return admitted, nil
}
