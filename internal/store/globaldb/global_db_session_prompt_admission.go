package globaldb

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

// ClaimSessionPromptAdmission reserves one exact external prompt command.
func (g *SessionRepo) ClaimSessionPromptAdmission(
	ctx context.Context,
	req store.SessionPromptAdmissionRequest,
) (admission store.SessionPromptAdmission, created bool, err error) {
	if err := g.checkReady(ctx, "claim session prompt admission"); err != nil {
		return store.SessionPromptAdmission{}, false, err
	}
	normalized := req.Normalize()
	if err := normalized.Validate(); err != nil {
		return store.SessionPromptAdmission{}, false, err
	}
	err = g.withImmediateTransaction(ctx, "claim session prompt admission", func(exec globalSQLExecutor) error {
		var claimErr error
		admission, created, claimErr = claimSessionPromptAdmission(ctx, exec, normalized)
		return claimErr
	})
	if err != nil {
		return store.SessionPromptAdmission{}, false, err
	}
	return admission, created, nil
}

// CommitSessionPromptDispatch crosses the at-most-once boundary before any non-idempotent effect.
func (g *SessionRepo) CommitSessionPromptDispatch(
	ctx context.Context,
	workspaceID string,
	sessionID string,
	idempotencyKey string,
	now time.Time,
) error {
	if err := g.checkReady(ctx, "commit session prompt dispatch"); err != nil {
		return err
	}
	workspaceID, sessionID, idempotencyKey, now, err := normalizePromptAdmissionScope(
		workspaceID,
		sessionID,
		idempotencyKey,
		now,
	)
	if err != nil {
		return err
	}
	affected, err := g.queries.CommitSessionPromptDispatch(ctx, sqlcgen.CommitSessionPromptDispatchParams{
		DispatchCommittedState: store.SessionPromptAdmissionDispatchCommitted,
		DispatchCommittedAt:    nullableSessionTime(now), UpdatedAt: store.FormatTimestamp(now),
		WorkspaceID: workspaceID, SessionID: sessionID, IdempotencyKey: idempotencyKey,
		ReservedState: store.SessionPromptAdmissionReserved,
	})
	if err != nil {
		return fmt.Errorf("store: commit session prompt dispatch: %w", err)
	}
	if affected != 1 {
		return g.promptAdmissionCASFailure(ctx, workspaceID, sessionID, idempotencyKey)
	}
	return nil
}

// CompleteSessionPromptAdmission stores the stable replay result for one command.
func (g *SessionRepo) CompleteSessionPromptAdmission(
	ctx context.Context,
	workspaceID string,
	sessionID string,
	idempotencyKey string,
	result store.SessionPromptAdmissionResult,
	now time.Time,
) (store.SessionPromptAdmission, error) {
	if err := g.checkReady(ctx, "complete session prompt admission"); err != nil {
		return store.SessionPromptAdmission{}, err
	}
	workspaceID, sessionID, idempotencyKey, now, err := normalizePromptAdmissionScope(
		workspaceID,
		sessionID,
		idempotencyKey,
		now,
	)
	if err != nil {
		return store.SessionPromptAdmission{}, err
	}
	var admission store.SessionPromptAdmission
	err = g.withImmediateTransaction(ctx, "complete session prompt admission", func(exec globalSQLExecutor) error {
		completed, completeErr := completeSessionPromptAdmission(
			ctx,
			exec,
			workspaceID,
			sessionID,
			idempotencyKey,
			result,
			now,
			store.SessionPromptAdmissionDispatchCommitted,
		)
		if completeErr != nil {
			return completeErr
		}
		admission = completed
		return nil
	})
	if err != nil {
		return store.SessionPromptAdmission{}, err
	}
	return admission, nil
}

// MarkSessionPromptAdmissionIndeterminate permanently fences an uncertain external effect.
func (g *SessionRepo) MarkSessionPromptAdmissionIndeterminate(
	ctx context.Context,
	workspaceID string,
	sessionID string,
	idempotencyKey string,
	reason string,
	now time.Time,
) error {
	if err := g.checkReady(ctx, "mark session prompt admission indeterminate"); err != nil {
		return err
	}
	workspaceID, sessionID, idempotencyKey, now, err := normalizePromptAdmissionScope(
		workspaceID,
		sessionID,
		idempotencyKey,
		now,
	)
	if err != nil {
		return err
	}
	reason = strings.TrimSpace(reason)
	if reason == "" {
		reason = "dispatch outcome is unknown"
	}
	affected, err := g.queries.MarkSessionPromptAdmissionIndeterminate(
		ctx,
		sqlcgen.MarkSessionPromptAdmissionIndeterminateParams{
			IndeterminateState:  store.SessionPromptAdmissionIndeterminate,
			IndeterminateReason: reason, UpdatedAt: store.FormatTimestamp(now),
			WorkspaceID: workspaceID, SessionID: sessionID, IdempotencyKey: idempotencyKey,
			DispatchCommittedState: store.SessionPromptAdmissionDispatchCommitted,
		},
	)
	if err != nil {
		return fmt.Errorf("store: mark session prompt admission indeterminate: %w", err)
	}
	if affected != 1 {
		return g.promptAdmissionCASFailure(ctx, workspaceID, sessionID, idempotencyKey)
	}
	return nil
}

func claimSessionPromptAdmission(
	ctx context.Context,
	exec globalSQLExecutor,
	req store.SessionPromptAdmissionRequest,
) (store.SessionPromptAdmission, bool, error) {
	queries := sqlcgen.New(exec)
	existing, err := queries.GetSessionPromptAdmissionByIdempotencyKey(
		ctx,
		sqlcgen.GetSessionPromptAdmissionByIdempotencyKeyParams{
			WorkspaceID: req.WorkspaceID, SessionID: req.SessionID, IdempotencyKey: req.IdempotencyKey,
		},
	)
	if err == nil {
		admission, mapErr := sessionPromptAdmissionFromGenerated(&existing)
		if mapErr != nil {
			return store.SessionPromptAdmission{}, false, mapErr
		}
		return classifyPromptAdmissionReplay(admission, req)
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return store.SessionPromptAdmission{}, false, err
	}
	message, err := queries.GetSessionPromptAdmissionByMessageID(
		ctx,
		sqlcgen.GetSessionPromptAdmissionByMessageIDParams{
			WorkspaceID: req.WorkspaceID, SessionID: req.SessionID, MessageID: req.MessageID,
		},
	)
	if err == nil {
		return store.SessionPromptAdmission{}, false, fmt.Errorf(
			"%w: message_id %q is already bound to idempotency_key %q",
			store.ErrSessionPromptMessageConflict,
			req.MessageID,
			message.IdempotencyKey,
		)
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return store.SessionPromptAdmission{}, false, err
	}
	nowRaw := store.FormatTimestamp(req.Now)
	if err := queries.InsertSessionPromptAdmission(ctx, sqlcgen.InsertSessionPromptAdmissionParams{
		ID: req.ID, WorkspaceID: req.WorkspaceID, SessionID: req.SessionID,
		MessageID: req.MessageID, IdempotencyKey: req.IdempotencyKey, Operation: req.Operation,
		FingerprintVersion: req.FingerprintVersion, RequestFingerprint: req.RequestFingerprint,
		State: store.SessionPromptAdmissionReserved, Mode: req.Mode, AuthoredText: req.AuthoredText,
		RuntimeProvider: req.Runtime.Provider, RuntimeModel: req.Runtime.Model,
		RuntimeReasoningEffort: req.Runtime.ReasoningEffort, RuntimeSpeed: req.Runtime.Speed,
		TurnID: req.TurnID, EventID: req.EventID, CreatedAt: nowRaw, UpdatedAt: nowRaw,
	}); err != nil {
		return store.SessionPromptAdmission{}, false, fmt.Errorf("store: insert session prompt admission: %w", err)
	}
	created, err := queries.GetSessionPromptAdmissionByIdempotencyKey(
		ctx,
		sqlcgen.GetSessionPromptAdmissionByIdempotencyKeyParams{
			WorkspaceID: req.WorkspaceID, SessionID: req.SessionID, IdempotencyKey: req.IdempotencyKey,
		},
	)
	if err != nil {
		return store.SessionPromptAdmission{}, false, err
	}
	admission, err := sessionPromptAdmissionFromGenerated(&created)
	return admission, true, err
}

func classifyPromptAdmissionReplay(
	admission store.SessionPromptAdmission,
	req store.SessionPromptAdmissionRequest,
) (store.SessionPromptAdmission, bool, error) {
	if admission.MessageID != req.MessageID || admission.FingerprintVersion != req.FingerprintVersion ||
		admission.RequestFingerprint != req.RequestFingerprint || admission.Operation != req.Operation {
		return store.SessionPromptAdmission{}, false, fmt.Errorf(
			"%w: idempotency_key %q is already bound to another request",
			store.ErrSessionPromptIdempotencyConflict,
			req.IdempotencyKey,
		)
	}
	switch admission.State {
	case store.SessionPromptAdmissionReserved:
		return admission, false, nil
	case store.SessionPromptAdmissionCompleted:
		return admission, false, nil
	case store.SessionPromptAdmissionDispatchCommitted, store.SessionPromptAdmissionIndeterminate:
		return store.SessionPromptAdmission{}, false, fmt.Errorf(
			"%w: idempotency_key %q",
			store.ErrSessionPromptDispatchIndeterminate,
			req.IdempotencyKey,
		)
	default:
		return store.SessionPromptAdmission{}, false, fmt.Errorf(
			"store: invalid session prompt admission state %q",
			admission.State,
		)
	}
}

func completeSessionPromptAdmission(
	ctx context.Context,
	exec globalSQLExecutor,
	workspaceID string,
	sessionID string,
	idempotencyKey string,
	result store.SessionPromptAdmissionResult,
	now time.Time,
	expectedState string,
) (store.SessionPromptAdmission, error) {
	encoded, err := json.Marshal(result)
	if err != nil {
		return store.SessionPromptAdmission{}, fmt.Errorf("store: encode session prompt result: %w", err)
	}
	nowRaw := store.FormatTimestamp(now)
	affected, err := sqlcgen.New(exec).CompleteSessionPromptAdmission(
		ctx,
		sqlcgen.CompleteSessionPromptAdmissionParams{
			CompletedState: store.SessionPromptAdmissionCompleted,
			ResultJson:     sql.NullString{String: string(encoded), Valid: true},
			CompletedAt:    nullableSessionTime(now), UpdatedAt: nowRaw,
			WorkspaceID: workspaceID, SessionID: sessionID, IdempotencyKey: idempotencyKey,
			ExpectedState: expectedState,
		},
	)
	if err != nil {
		return store.SessionPromptAdmission{}, fmt.Errorf("store: complete session prompt admission: %w", err)
	}
	if affected != 1 {
		return store.SessionPromptAdmission{}, store.ErrSessionPromptAdmissionInProgress
	}
	row, err := sqlcgen.New(exec).GetSessionPromptAdmissionByIdempotencyKey(
		ctx,
		sqlcgen.GetSessionPromptAdmissionByIdempotencyKeyParams{
			WorkspaceID: workspaceID, SessionID: sessionID, IdempotencyKey: idempotencyKey,
		},
	)
	if err != nil {
		return store.SessionPromptAdmission{}, err
	}
	return sessionPromptAdmissionFromGenerated(&row)
}

func normalizePromptAdmissionScope(
	workspaceID string,
	sessionID string,
	idempotencyKey string,
	now time.Time,
) (string, string, string, time.Time, error) {
	workspaceID = strings.TrimSpace(workspaceID)
	sessionID = strings.TrimSpace(sessionID)
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if workspaceID == "" || sessionID == "" || idempotencyKey == "" {
		return "", "", "", time.Time{}, errors.New(
			"store: session prompt workspace, session, and idempotency key are required",
		)
	}
	if now.IsZero() {
		now = time.Now().UTC()
	} else {
		now = now.UTC()
	}
	return workspaceID, sessionID, idempotencyKey, now, nil
}

func (g *SessionRepo) promptAdmissionCASFailure(
	ctx context.Context,
	workspaceID string,
	sessionID string,
	idempotencyKey string,
) error {
	row, err := g.queries.GetSessionPromptAdmissionByIdempotencyKey(
		ctx,
		sqlcgen.GetSessionPromptAdmissionByIdempotencyKeyParams{
			WorkspaceID: workspaceID, SessionID: sessionID, IdempotencyKey: idempotencyKey,
		},
	)
	if errors.Is(err, sql.ErrNoRows) {
		return store.ErrSessionPromptAdmissionInProgress
	}
	if err != nil {
		return err
	}
	if row.State == store.SessionPromptAdmissionDispatchCommitted ||
		row.State == store.SessionPromptAdmissionIndeterminate {
		return store.ErrSessionPromptDispatchIndeterminate
	}
	return store.ErrSessionPromptAdmissionInProgress
}
