package globaldb

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

// ClaimSessionInputBatch leases the consecutive dispatch-ordered entries that follow an already
// claimed head, stopping at the first one accept refuses. The candidates use the dispatch filter,
// so a Goal cutoff, a newer generation, or a different owner always ends the batch.
func (g *SessionRepo) ClaimSessionInputBatch(
	ctx context.Context,
	sessionID string,
	limit int,
	accept func(*store.SessionInputQueueEntry) bool,
	now time.Time,
) (claimed []store.SessionInputQueueEntry, err error) {
	if err := g.checkReady(ctx, "claim session input batch"); err != nil {
		return nil, err
	}
	target := strings.TrimSpace(sessionID)
	if target == "" {
		return nil, errors.New("store: session id is required")
	}
	if limit <= 0 || accept == nil {
		return nil, nil
	}
	if now.IsZero() {
		now = g.now()
	}
	now = now.UTC()
	err = g.withImmediateTransaction(ctx, "claim session input batch", func(exec globalSQLExecutor) error {
		claimed = nil
		queries := sqlcgen.New(exec)
		rows, listErr := queries.ListDispatchableSessionInputs(ctx, sqlcgen.ListDispatchableSessionInputsParams{
			SessionID: target, RowLimit: int64(limit),
		})
		if listErr != nil {
			return fmt.Errorf("store: list dispatchable session inputs: %w", listErr)
		}
		nowRaw := store.FormatTimestamp(now)
		for index := range rows {
			candidate, mapErr := sessionInputQueueFromGenerated(&rows[index])
			if mapErr != nil {
				return mapErr
			}
			if !accept(&candidate) {
				return nil
			}
			entry, claimErr := claimSessionInputInTransaction(ctx, exec, target, &candidate, now, nowRaw)
			if claimErr != nil {
				return claimErr
			}
			claimed = append(claimed, entry)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return claimed, nil
}

func claimSessionInputInTransaction(
	ctx context.Context,
	exec globalSQLExecutor,
	sessionID string,
	candidate *store.SessionInputQueueEntry,
	now time.Time,
	nowRaw string,
) (store.SessionInputQueueEntry, error) {
	if err := commitQueuedPromptAdmissionDispatch(ctx, exec, candidate, nowRaw); err != nil {
		return store.SessionInputQueueEntry{}, err
	}
	affected, err := sqlcgen.New(exec).ClaimSessionInput(ctx, sqlcgen.ClaimSessionInputParams{
		DispatchingStatus: store.SessionInputQueueStatusDispatching,
		Now:               nullableSessionTime(now), UpdatedAt: nowRaw, ID: candidate.ID, SessionID: sessionID,
		QueuedStatus: store.SessionInputQueueStatusQueued,
	})
	if err != nil {
		return store.SessionInputQueueEntry{}, fmt.Errorf("store: mark session input dispatching: %w", err)
	}
	if affected != 1 {
		return store.SessionInputQueueEntry{}, goalControlStaleError(
			"mark session input dispatching lost compare-and-swap",
		)
	}
	return getSessionInputQueueEntry(ctx, exec, sessionID, candidate.ID)
}

// PromoteQueuedAgentSessionInputToSteer turns one queued agent message into staged steering for the
// target's live turn in place, keeping its message, event, admission, and origin identity. It reports
// false when the entry is no longer eligible: already dispatched, carrying attachments, operator input
// waiting, or the sender already holding a queued steer.
func (g *SessionRepo) PromoteQueuedAgentSessionInputToSteer(
	ctx context.Context,
	sessionID string,
	entryID string,
	targetTurnID string,
	now time.Time,
) (entry store.SessionInputQueueEntry, promoted bool, err error) {
	if err := g.checkReady(ctx, "promote queued agent session input"); err != nil {
		return store.SessionInputQueueEntry{}, false, err
	}
	target := strings.TrimSpace(sessionID)
	entryID = strings.TrimSpace(entryID)
	targetTurnID = strings.TrimSpace(targetTurnID)
	if target == "" || entryID == "" || targetTurnID == "" {
		return store.SessionInputQueueEntry{}, false, errors.New(
			"store: session id, queue entry id, and target turn id are required",
		)
	}
	if now.IsZero() {
		now = g.now()
	}
	err = g.withImmediateTransaction(ctx, "promote queued agent session input", func(exec globalSQLExecutor) error {
		existing, getErr := getSessionInputQueueEntry(ctx, exec, target, entryID)
		if getErr != nil {
			return getErr
		}
		if len(existing.Attachments) > 0 || len(existing.SkillInvocations) > 0 {
			return nil
		}
		affected, updateErr := sqlcgen.New(exec).PromoteQueuedAgentSessionInputToSteer(
			ctx,
			sqlcgen.PromoteQueuedAgentSessionInputToSteerParams{
				TargetTurnID: targetTurnID, Now: store.FormatTimestamp(now.UTC()), SessionID: target, ID: entryID,
			},
		)
		if updateErr != nil {
			return fmt.Errorf("store: promote queued agent session input: %w", updateErr)
		}
		if affected != 1 {
			return nil
		}
		entry, getErr = getSessionInputQueueEntry(ctx, exec, target, entryID)
		promoted = getErr == nil
		return getErr
	})
	if err != nil {
		return store.SessionInputQueueEntry{}, false, err
	}
	return entry, promoted, nil
}

// RevertPromotedAgentSessionInput returns a promoted agent message whose live delivery never started to
// after-turn delivery, so a failed promotion defers the message instead of dropping it. A reservation
// already committed the message's admission dispatch; reverting a reserved row completes that admission
// again in the same transaction, so the later ordinary claim can commit it.
func (g *SessionRepo) RevertPromotedAgentSessionInput(
	ctx context.Context,
	sessionID string,
	entryID string,
	now time.Time,
) (bool, error) {
	if err := g.checkReady(ctx, "revert promoted agent session input"); err != nil {
		return false, err
	}
	if now.IsZero() {
		now = g.now()
	}
	target, id := strings.TrimSpace(sessionID), strings.TrimSpace(entryID)
	nowRaw := store.FormatTimestamp(now.UTC())
	reverted := false
	err := g.withImmediateTransaction(ctx, "revert promoted agent session input", func(exec globalSQLExecutor) error {
		existing, err := getSessionInputQueueEntry(ctx, exec, target, id)
		if err != nil {
			return err
		}
		affected, err := sqlcgen.New(exec).RevertPromotedAgentSessionInput(
			ctx, sqlcgen.RevertPromotedAgentSessionInputParams{Now: nowRaw, SessionID: target, ID: id},
		)
		if err != nil {
			return fmt.Errorf("store: revert promoted agent session input: %w", err)
		}
		if affected != 1 {
			return nil
		}
		reverted = true
		if existing.Status != store.SessionInputQueueStatusDispatching {
			return nil
		}
		return completeQueuedPromptAdmissionDispatch(ctx, exec, &existing, nowRaw)
	})
	if err != nil {
		return false, err
	}
	return reverted, nil
}
