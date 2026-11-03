package globaldb

import (
	"context"
	"errors"
	"time"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

func (g *SessionRepo) SetUnattendedTurnLimit(limit int64) error {
	if limit < 0 {
		return errors.New("store: unattended turn limit must be nonnegative")
	}
	g.unattendedTurnLimit.Store(limit)
	return nil
}

func advanceLineageUnattended(
	ctx context.Context, exec globalSQLExecutor, entry *store.SessionInputQueueEntry, limit int64, now time.Time,
) (string, error) {
	q := sqlcgen.New(exec)
	caller, err := q.GetInstanceCaller(ctx, entry.SessionID)
	if err != nil {
		return "", err
	}
	id, ok := caller.OriginatingInstanceID.(string)
	if !ok {
		return "", errors.New("store: invalid originating instance")
	}
	if id == "" {
		return "", nil
	}
	stamp := store.FormatTimestamp(now)
	if store.IsHumanSessionInputOriginClass(entry.OriginClass) {
		return "", q.ResetInstanceUnattended(ctx, sqlcgen.ResetInstanceUnattendedParams{InstanceID: id, Now: stamp})
	}
	if entry.AttemptCount > 1 || entry.LoopRunID != "" || entry.OwnerKind == goalPromptOwnerKind {
		return "", nil
	}
	return advanceInstanceUnattended(ctx, q, id, entry.ID, limit, now)
}

func advanceInstanceUnattended(
	ctx context.Context,
	q *sqlcgen.Queries,
	id, key string,
	limit int64,
	now time.Time,
) (string, error) {
	stamp := store.FormatTimestamp(now)
	turns, err := q.AdvanceInstanceUnattended(ctx, id)
	if err != nil {
		return "", err
	}
	if limit == 0 || turns < limit {
		return "", nil
	}
	changed, err := q.PauseAgentInstance(
		ctx,
		sqlcgen.PauseAgentInstanceParams{ID: id, PauseReason: "unattended", UpdatedAt: stamp},
	)
	if err != nil || changed == 0 {
		return "", err
	}
	if err := q.CancelInstanceWake(ctx, id); err != nil {
		return "", err
	}
	reportID, err := store.NewID("arpt")
	if err != nil {
		return "", err
	}
	err = q.InsertAgentInstanceReport(ctx, sqlcgen.InsertAgentInstanceReportParams{
		ID: reportID, InstanceID: id, SourceKey: "unattended:" + key, Kind: instanceReportKindFailure,
		Payload:   `{"reason":"unattended","detail":"Agent paused after reaching its unattended turn limit."}`,
		CreatedAt: stamp,
	})
	if err != nil {
		return "", err
	}
	return id, nil
}

func resetLineageUnattended(ctx context.Context, q *sqlcgen.Queries, sessionID, now string) error {
	caller, err := q.GetInstanceCaller(ctx, sessionID)
	if err != nil {
		return err
	}
	id, ok := caller.OriginatingInstanceID.(string)
	if !ok {
		return errors.New("store: invalid originating instance")
	}
	if id == "" {
		return nil
	}
	return q.ResetInstanceUnattended(ctx, sqlcgen.ResetInstanceUnattendedParams{InstanceID: id, Now: now})
}

func (g *SessionRepo) ResetLineageUnattended(ctx context.Context, sessionID string) error {
	if err := g.checkReady(ctx, "reset lineage unattended turns"); err != nil {
		return err
	}
	return g.withImmediateTransaction(ctx, "reset lineage unattended turns", func(exec globalSQLExecutor) error {
		return resetLineageUnattended(ctx, sqlcgen.New(exec), sessionID, store.FormatTimestamp(g.now()))
	})
}

const instanceReportKindFailure = "failure"
