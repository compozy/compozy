package globaldb

import (
	"context"
	"sync"
	"time"

	taskpkg "github.com/compozy/compozy/internal/task"
)

var _ taskpkg.RunLeaseSettlementReservationStore = (*GlobalDB)(nil)

// ReserveRunLeaseSettlement registers a token-fenced in-process reservation
// without acquiring the SQLite writer. Recovery checks it after writer admission,
// so a recovery already waiting for SQLite cannot reclaim the pending result.
func (g *TaskRunRepo) ReserveRunLeaseSettlement(
	ctx context.Context,
	runID, claimToken string,
	at time.Time,
	actor taskpkg.ActorContext,
) (func(), error) {
	if err := g.checkReady(ctx, "reserve task run lease settlement"); err != nil {
		return nil, err
	}
	if err := actor.Validate(); err != nil {
		return nil, err
	}
	if at.IsZero() {
		at = g.now()
	}
	for {
		version := g.leaseRecoveryVersion.Load()
		run, err := g.tasks.getTaskRunWithExecutor(ctx, g.db, runID)
		if err != nil {
			return nil, err
		}
		// Do not retain pooled connections while waiting for recovery's commit.
		// If recovery overlapped this read, reload its committed ownership first.
		g.leaseSettlementsMu.Lock()
		if version != g.leaseRecoveryVersion.Load() {
			g.leaseSettlementsMu.Unlock()
			continue
		}
		release, err := g.reserveLoadedRunLease(run, claimToken, at, actor)
		g.leaseSettlementsMu.Unlock()
		return release, err
	}
}

func (g *TaskRunRepo) reserveLoadedRunLease(
	run taskpkg.Run,
	claimToken string,
	at time.Time,
	actor taskpkg.ActorContext,
) (func(), error) {
	if err := requireCurrentRunLease(run, claimToken, at); err != nil {
		return nil, err
	}
	if err := taskpkg.RequireLeaseSettlementActor(run, actor); err != nil {
		return nil, err
	}
	if _, exists := g.leaseSettlements[run.ID]; exists {
		return nil, taskpkg.ErrTerminalRunCommandInProgress
	}
	if g.leaseSettlements == nil {
		g.leaseSettlements = make(map[string]string)
	}
	g.leaseSettlements[run.ID] = run.ClaimTokenHash
	return sync.OnceFunc(func() {
		g.leaseSettlementsMu.Lock()
		defer g.leaseSettlementsMu.Unlock()
		delete(g.leaseSettlements, run.ID)
	}), nil
}
