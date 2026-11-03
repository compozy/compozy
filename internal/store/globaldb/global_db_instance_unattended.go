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
) (*store.InstancePauseTrip, error) {
	q := sqlcgen.New(exec)
	caller, err := q.GetInstanceCaller(ctx, entry.SessionID)
	if err != nil {
		return nil, err
	}
	id, ok := caller.OriginatingInstanceID.(string)
	if !ok {
		return nil, errors.New("store: invalid originating instance")
	}
	if id == "" {
		return nil, nil
	}
	stamp := store.FormatTimestamp(now)
	if store.IsHumanSessionInputOriginClass(entry.OriginClass) {
		return nil, q.ResetInstanceUnattended(ctx, sqlcgen.ResetInstanceUnattendedParams{InstanceID: id, Now: stamp})
	}
	loopOwned, err := loopOwnedTask(ctx, q, entry.TaskRunID)
	if err != nil {
		return nil, err
	}
	if loopOwned || entry.LoopRunID != "" || entry.OwnerKind == goalPromptOwnerKind {
		return nil, nil
	}
	// The durable marker, not attempt_count, records that this row's turn was counted: released rows
	// carry steering reservations in attempt_count, and a retried claim must not count twice.
	counted, err := q.MarkSessionInputUnattendedCounted(ctx, entry.ID)
	if err != nil || counted == 0 {
		return nil, err
	}
	return advanceInstanceUnattended(ctx, q, id, entry.ID, limit, now)
}

func advanceInstanceUnattended(
	ctx context.Context,
	q *sqlcgen.Queries,
	id, key string,
	limit int64,
	now time.Time,
) (*store.InstancePauseTrip, error) {
	stamp := store.FormatTimestamp(now)
	turns, err := q.AdvanceInstanceUnattended(ctx, id)
	if err != nil {
		return nil, err
	}
	if limit == 0 || turns < limit {
		return nil, nil
	}
	changed, err := q.PauseAgentInstance(
		ctx,
		sqlcgen.PauseAgentInstanceParams{ID: id, PauseReason: "unattended", UpdatedAt: stamp},
	)
	if err != nil || changed == 0 {
		return nil, err
	}
	if err := q.CancelInstanceWake(ctx, id); err != nil {
		return nil, err
	}
	reportID, err := store.NewID("arpt")
	if err != nil {
		return nil, err
	}
	err = q.InsertAgentInstanceReport(ctx, sqlcgen.InsertAgentInstanceReportParams{
		ID: reportID, InstanceID: id, SourceKey: "unattended:" + key, Kind: instanceReportKindFailure,
		Payload:   `{"reason":"unattended","detail":"Agent paused after reaching its unattended turn limit."}`,
		CreatedAt: stamp,
	})
	if err != nil {
		return nil, err
	}
	row, err := q.GetAgentInstance(ctx, id)
	if err != nil {
		return nil, err
	}
	instance, err := instanceFromRow(row)
	if err != nil {
		return nil, err
	}
	return &store.InstancePauseTrip{
		InstanceID: instance.ID, ProfileID: instance.ProfileID, WorkspaceID: instance.WorkspaceID,
		AgentName: instance.AgentName, MainSessionID: instance.MainSessionID, Reason: instance.PauseReason,
		CreatedAt: instance.CreatedAt, PausedAt: instance.UpdatedAt,
	}, nil
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

func (g *SessionRepo) HumanSessionTurn(ctx context.Context, sessionID, turnID string) (bool, error) {
	if err := g.checkReady(ctx, "read human session turn"); err != nil {
		return false, err
	}
	return g.queries.HumanSessionTurn(ctx, sqlcgen.HumanSessionTurnParams{SessionID: sessionID, TurnID: turnID})
}
