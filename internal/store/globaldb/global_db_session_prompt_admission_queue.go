package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	commandpkg "github.com/compozy/compozy/internal/command"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

type admittedSessionInputResult struct {
	admission store.SessionPromptAdmission
	entry     store.SessionInputQueueEntry
	position  int
	created   bool
	canceled  int
}

// EnqueueAdmittedSessionInput atomically binds a command receipt and one queue entry.
func (g *SessionRepo) EnqueueAdmittedSessionInput(
	ctx context.Context,
	admissionReq store.SessionPromptAdmissionRequest,
	queueReq store.SessionInputQueueInsert,
) (
	admission store.SessionPromptAdmission,
	entry store.SessionInputQueueEntry,
	position int,
	created bool,
	err error,
) {
	if err := g.checkReady(ctx, "enqueue admitted session input"); err != nil {
		return admission, entry, 0, false, err
	}
	mode := queueReq.Normalize().Mode
	if mode == "" {
		mode = admissionReq.Normalize().Mode
	}
	if mode != store.SessionInputQueueModeQueue && mode != store.SessionInputQueueModeInterrupt {
		return admission, entry, 0, false, fmt.Errorf("store: invalid admitted queued input mode %q", mode)
	}
	admissionReq, queueReq, err = normalizeAdmittedSessionInput(
		admissionReq,
		queueReq,
		mode,
		mode,
	)
	if err != nil {
		return admission, entry, 0, false, err
	}
	result, err := g.enqueueAdmittedSessionInput(ctx, admissionReq, queueReq)
	return result.admission, result.entry, result.position, result.created, err
}

func (g *SessionRepo) enqueueAdmittedSessionInput(
	ctx context.Context,
	admissionReq store.SessionPromptAdmissionRequest,
	queueReq store.SessionInputQueueInsert,
) (admittedSessionInputResult, error) {
	var result admittedSessionInputResult
	err := g.withImmediateTransaction(ctx, "enqueue admitted session input", func(exec globalSQLExecutor) error {
		return enqueueAdmittedSessionInputInTransaction(ctx, exec, admissionReq, queueReq, &result)
	})
	return result, err
}

func enqueueAdmittedSessionInputInTransaction(
	ctx context.Context,
	exec globalSQLExecutor,
	admissionReq store.SessionPromptAdmissionRequest,
	queueReq store.SessionInputQueueInsert,
	result *admittedSessionInputResult,
) error {
	claimed, replayed, err := claimOrReplayAdmittedSessionInput(ctx, exec, admissionReq, result)
	if err != nil || replayed {
		return err
	}
	if queueReq.Mode == store.SessionInputQueueModeInterrupt {
		queueReq, result.canceled, err = prepareInterruptSessionInput(ctx, exec, queueReq)
		if err != nil {
			return err
		}
	}
	count, err := countPendingSessionInputs(ctx, exec, queueReq.SessionID)
	if err != nil {
		return err
	}
	if count >= queueReq.QueueCap {
		return fmt.Errorf(
			"%w: session %s cap %d",
			store.ErrSessionInputQueueFull,
			queueReq.SessionID,
			queueReq.QueueCap,
		)
	}
	inserted, err := insertSessionInputQueueEntry(ctx, exec, bindQueueAdmission(queueReq, claimed))
	if err != nil {
		return err
	}
	result.position = count + 1
	completed, err := completeSessionPromptAdmission(
		ctx,
		exec,
		claimed.WorkspaceID,
		claimed.SessionID,
		claimed.IdempotencyKey,
		queuedPromptAdmissionResult(&inserted, result.position, result.canceled),
		admissionReq.Now,
		store.SessionPromptAdmissionReserved,
	)
	if err != nil {
		return err
	}
	result.admission = completed
	result.entry = inserted
	result.created = true
	return nil
}

// StageAdmittedSessionSteer atomically binds a receipt and replaces the active staged steer.
func (g *SessionRepo) StageAdmittedSessionSteer(
	ctx context.Context,
	admissionReq store.SessionPromptAdmissionRequest,
	queueReq store.SessionInputQueueInsert,
) (admission store.SessionPromptAdmission, entry store.SessionInputQueueEntry, created bool, err error) {
	if err := g.checkReady(ctx, "stage admitted session steer"); err != nil {
		return admission, entry, false, err
	}
	admissionReq, queueReq, err = normalizeAdmittedSessionInput(
		admissionReq,
		queueReq,
		store.SessionInputQueueModeSteer,
		"steer",
	)
	if err != nil {
		return admission, entry, false, err
	}
	result, err := g.stageAdmittedSessionSteer(ctx, admissionReq, queueReq)
	return result.admission, result.entry, result.created, err
}

func (g *SessionRepo) stageAdmittedSessionSteer(
	ctx context.Context,
	admissionReq store.SessionPromptAdmissionRequest,
	queueReq store.SessionInputQueueInsert,
) (admittedSessionInputResult, error) {
	var result admittedSessionInputResult
	err := g.withImmediateTransaction(ctx, "stage admitted session steer", func(exec globalSQLExecutor) error {
		return stageAdmittedSessionSteerInTransaction(ctx, exec, admissionReq, queueReq, &result)
	})
	return result, err
}

func stageAdmittedSessionSteerInTransaction(
	ctx context.Context,
	exec globalSQLExecutor,
	admissionReq store.SessionPromptAdmissionRequest,
	queueReq store.SessionInputQueueInsert,
	result *admittedSessionInputResult,
) error {
	claimed, replayed, err := claimOrReplayAdmittedSessionInput(ctx, exec, admissionReq, result)
	if err != nil || replayed {
		return err
	}
	if err := cancelPriorAdmittedSessionSteers(ctx, exec, queueReq, admissionReq.Now); err != nil {
		return err
	}
	inserted, err := insertSessionInputQueueEntry(ctx, exec, bindQueueAdmission(queueReq, claimed))
	if err != nil {
		return err
	}
	completed, err := completeSessionPromptAdmission(
		ctx,
		exec,
		claimed.WorkspaceID,
		claimed.SessionID,
		claimed.IdempotencyKey,
		stagedPromptAdmissionResult(&inserted),
		admissionReq.Now,
		store.SessionPromptAdmissionReserved,
	)
	if err != nil {
		return err
	}
	result.admission = completed
	result.entry = inserted
	result.created = true
	return nil
}

func normalizeAdmittedSessionInput(
	admissionReq store.SessionPromptAdmissionRequest,
	queueReq store.SessionInputQueueInsert,
	mode string,
	kind string,
) (store.SessionPromptAdmissionRequest, store.SessionInputQueueInsert, error) {
	admissionReq = admissionReq.Normalize()
	queueReq = queueReq.Normalize()
	queueReq.Mode = mode
	if err := admissionReq.Validate(); err != nil {
		return store.SessionPromptAdmissionRequest{}, store.SessionInputQueueInsert{}, err
	}
	if err := queueReq.Validate(); err != nil {
		return store.SessionPromptAdmissionRequest{}, store.SessionInputQueueInsert{}, err
	}
	if queueReq.SessionID != admissionReq.SessionID {
		return store.SessionPromptAdmissionRequest{}, store.SessionInputQueueInsert{}, fmt.Errorf(
			"store: admitted %s session %q does not match admission session %q",
			kind,
			queueReq.SessionID,
			admissionReq.SessionID,
		)
	}
	if admissionReq.Mode != mode {
		return store.SessionPromptAdmissionRequest{}, store.SessionInputQueueInsert{}, fmt.Errorf(
			"store: admitted %s mode %q must be %q",
			kind,
			admissionReq.Mode,
			mode,
		)
	}
	if err := validateCallerOwnedQueueIdentity(queueReq); err != nil {
		return store.SessionPromptAdmissionRequest{}, store.SessionInputQueueInsert{}, err
	}
	return admissionReq, queueReq, nil
}

func claimOrReplayAdmittedSessionInput(
	ctx context.Context,
	exec globalSQLExecutor,
	req store.SessionPromptAdmissionRequest,
	result *admittedSessionInputResult,
) (store.SessionPromptAdmission, bool, error) {
	existing, found, err := findPromptAdmissionReplay(ctx, exec, req)
	if err != nil {
		return store.SessionPromptAdmission{}, false, err
	}
	if found && existing.State == store.SessionPromptAdmissionCompleted {
		if existing.Result == nil {
			return store.SessionPromptAdmission{}, false, store.ErrSessionPromptAdmissionInProgress
		}
		entry, err := getSessionInputQueueEntry(ctx, exec, existing.SessionID, existing.Result.QueueEntryID)
		if err != nil {
			return store.SessionPromptAdmission{}, false, err
		}
		result.admission = existing
		result.entry = entry
		result.position = existing.Result.QueuePosition
		return existing, true, nil
	}
	if found {
		return existing, false, nil
	}
	claimed, _, err := claimSessionPromptAdmission(ctx, exec, req)
	return claimed, false, err
}

func cancelPriorAdmittedSessionSteers(
	ctx context.Context,
	exec globalSQLExecutor,
	queueReq store.SessionInputQueueInsert,
	now time.Time,
) error {
	nowRaw := store.FormatTimestamp(now)
	err := sqlcgen.New(exec).CancelPriorSessionSteers(
		ctx,
		sqlcgen.CancelPriorSessionSteersParams{
			CanceledStatus: store.SessionInputQueueStatusCanceled,
			CanceledAt:     nullableSessionTime(now),
			UpdatedAt:      nowRaw,
			SessionID:      queueReq.SessionID,
			SteerMode:      store.SessionInputQueueModeSteer,
			QueuedStatus:   store.SessionInputQueueStatusQueued,
		},
	)
	if err != nil {
		return fmt.Errorf("store: cancel prior admitted session steer: %w", err)
	}
	return nil
}

func queuedPromptAdmissionResult(
	entry *store.SessionInputQueueEntry,
	position int,
	canceled int,
) store.SessionPromptAdmissionResult {
	status := store.SessionPromptResultStatusQueued
	if entry.Mode == store.SessionInputQueueModeInterrupt {
		status = store.SessionPromptResultStatusInterrupting
	}
	return store.SessionPromptAdmissionResult{
		Status:                status,
		Mode:                  entry.Mode,
		QueueEntryID:          entry.ID,
		QueuePosition:         position,
		QueueGeneration:       entry.SessionGeneration,
		Delivery:              entry.Delivery,
		CanceledQueuedEntries: canceled,
	}
}

func stagedPromptAdmissionResult(entry *store.SessionInputQueueEntry) store.SessionPromptAdmissionResult {
	return store.SessionPromptAdmissionResult{
		Status:          store.SessionPromptResultStatusSteering,
		Mode:            store.SessionInputQueueModeSteer,
		QueueEntryID:    entry.ID,
		QueueGeneration: entry.SessionGeneration,
		Delivery:        entry.Delivery,
	}
}

func findPromptAdmissionReplay(
	ctx context.Context,
	exec globalSQLExecutor,
	req store.SessionPromptAdmissionRequest,
) (store.SessionPromptAdmission, bool, error) {
	row, err := sqlcgen.New(exec).GetSessionPromptAdmissionByIdempotencyKey(
		ctx,
		sqlcgen.GetSessionPromptAdmissionByIdempotencyKeyParams{
			WorkspaceID:    req.WorkspaceID,
			SessionID:      req.SessionID,
			IdempotencyKey: req.IdempotencyKey,
		},
	)
	if errors.Is(err, sql.ErrNoRows) {
		return store.SessionPromptAdmission{}, false, nil
	}
	if err != nil {
		return store.SessionPromptAdmission{}, false, err
	}
	admission, err := sessionPromptAdmissionFromGenerated(&row)
	if err != nil {
		return store.SessionPromptAdmission{}, false, err
	}
	admission, _, err = classifyPromptAdmissionReplay(admission, req)
	return admission, true, err
}

func bindQueueAdmission(
	req store.SessionInputQueueInsert,
	admission store.SessionPromptAdmission,
) store.SessionInputQueueInsert {
	req.PromptAdmissionID = admission.ID
	req.MessageID = admission.MessageID
	req.IdempotencyKey = admission.IdempotencyKey
	req.TurnID = admission.TurnID
	req.EventID = admission.EventID
	req.Text = admission.AuthoredText
	req.Runtime = admission.Runtime
	req.SkillInvocations = append([]commandpkg.Invocation(nil), admission.SkillInvocations...)
	return req
}
