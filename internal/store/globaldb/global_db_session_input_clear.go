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

var _ store.SessionInputClearStore = (*SessionRepo)(nil)

func (g *SessionRepo) ClearSessionInputs(
	ctx context.Context,
	req store.SessionInputClearRequest,
) (store.SessionInputClearResult, error) {
	if err := g.checkReady(ctx, "clear session input queue"); err != nil {
		return store.SessionInputClearResult{}, err
	}
	req.SessionID = strings.TrimSpace(req.SessionID)
	req.ActorKind = strings.TrimSpace(req.ActorKind)
	req.ActorID = strings.TrimSpace(req.ActorID)
	if req.SessionID == "" || req.ActorKind == "" || req.ActorID == "" {
		return store.SessionInputClearResult{}, errors.New("store: clear queue requires session and actor identity")
	}
	if req.Now.IsZero() {
		req.Now = g.now()
	}
	result := store.SessionInputClearResult{Inputs: []store.SessionInputQueueEntry{}}
	err := g.withImmediateTransaction(ctx, "clear session input queue", func(exec globalSQLExecutor) error {
		queries := sqlcgen.New(exec)
		rows, err := queries.ListPendingSessionInputs(ctx, sqlcgen.ListPendingSessionInputsParams{
			SessionID: req.SessionID, QueuedStatus: store.SessionInputQueueStatusQueued,
			DispatchingStatus: store.SessionInputQueueStatusDispatching,
		})
		if err != nil {
			return fmt.Errorf("store: list inputs for clear: %w", err)
		}
		now := store.FormatTimestamp(req.Now)
		affected, err := queries.AdvanceSessionInputGeneration(ctx, sqlcgen.AdvanceSessionInputGenerationParams{
			ID: req.SessionID, UpdatedAt: now,
		})
		if err != nil {
			return err
		}
		if affected != 1 {
			return fmt.Errorf("%w: %s", store.ErrSessionNotFound, req.SessionID)
		}
		result.Generation, err = queries.GetSessionInputGeneration(ctx, req.SessionID)
		if err != nil {
			return err
		}
		for index := range rows {
			entry, err := sessionInputQueueFromGenerated(&rows[index])
			if err != nil {
				return err
			}
			if entry.Status == store.SessionInputQueueStatusQueued {
				if err := clearSessionInputWithTrace(ctx, queries, &entry, req, now); err != nil {
					return &store.SessionInputClearError{EntryID: entry.ID, Cause: err}
				}
				result.Cleared++
			} else {
				entry.SessionGeneration = result.Generation
			}
			result.Inputs = append(result.Inputs, entry)
		}
		return queries.RebaseDispatchingSessionInputs(ctx, sqlcgen.RebaseDispatchingSessionInputsParams{
			SessionID: req.SessionID, Generation: result.Generation,
		})
	})
	if err != nil {
		return store.SessionInputClearResult{}, fmt.Errorf("store: clear session input queue: %w", err)
	}
	return result, nil
}

func clearSessionInputWithTrace(
	ctx context.Context,
	queries *sqlcgen.Queries,
	entry *store.SessionInputQueueEntry,
	req store.SessionInputClearRequest,
	now string,
) error {
	affected, err := queries.ClearQueuedSessionInput(ctx, sqlcgen.ClearQueuedSessionInputParams{
		SessionID: req.SessionID, EntryID: entry.ID, Now: nullableSessionTime(req.Now),
	})
	if err != nil {
		return err
	}
	if affected != 1 {
		return fmt.Errorf("%w: %s", store.ErrSessionInputQueueEntryNotQueued, entry.ID)
	}
	if err := queries.InsertSessionInputClearTrace(ctx, sqlcgen.InsertSessionInputClearTraceParams{
		SessionID: req.SessionID, EntryID: entry.ID, TurnID: req.TurnID,
		ActorKind: req.ActorKind, ActorID: req.ActorID, QueueGeneration: entry.SessionGeneration, CreatedAt: now,
	}); err != nil {
		return err
	}
	entry.Status = store.SessionInputQueueStatusCanceled
	entry.Dispatchable = false
	entry.CanceledAt = &req.Now
	return nil
}

func (g *SessionRepo) ListPendingSessionInputClearTraces(
	ctx context.Context,
	sessionID string,
) ([]store.SessionInputClearTrace, error) {
	if err := g.checkReady(ctx, "list queue clear traces"); err != nil {
		return nil, err
	}
	rows, err := g.queries.ListPendingSessionInputClearTraces(ctx, strings.TrimSpace(sessionID))
	if err != nil {
		return nil, err
	}
	traces := make([]store.SessionInputClearTrace, 0, len(rows))
	for _, row := range rows {
		created, err := store.ParseTimestamp(row.CreatedAt)
		if err != nil {
			return nil, err
		}
		traces = append(traces, store.SessionInputClearTrace{
			EntryID: row.EntryID, SessionID: row.SessionID, TurnID: row.TurnID,
			ActorKind: row.ActorKind, ActorID: row.ActorID, Generation: row.QueueGeneration, CreatedAt: created,
		})
	}
	return traces, nil
}

func (g *SessionRepo) MarkSessionInputClearTraceProjected(
	ctx context.Context,
	sessionID, entryID string,
	now time.Time,
) error {
	if err := g.checkReady(ctx, "mark queue clear trace projected"); err != nil {
		return err
	}
	return g.queries.MarkSessionInputClearTraceProjected(ctx, sqlcgen.MarkSessionInputClearTraceProjectedParams{
		SessionID: sessionID, EntryID: entryID, Now: nullableSessionTime(now),
	})
}
