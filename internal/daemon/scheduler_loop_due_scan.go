package daemon

import (
	"context"
	"errors"
	"sync"
	"time"

	looppkg "github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/store/globaldb"
	taskpkg "github.com/compozy/compozy/internal/task"
)

const defaultLoopRetryDueScanPageSize = 100

var errLoopRetryDueWakeStoreRequired = errors.New(
	"daemon: scheduler task store must support loop retry due-time recovery",
)

var errLoopAdmissionClaimSweeperRequired = errors.New(
	"daemon: scheduler task store must support loop admission claim sweeping",
)

var _ loopRetryDueWakeStore = (*globaldb.GlobalDB)(nil)

var _ loopWaitDueStore = (*globaldb.GlobalDB)(nil)

var _ loopWaitEscalationStore = (*globaldb.GlobalDB)(nil)

var _ loopAdmissionClaimSweeper = (*globaldb.GlobalDB)(nil)

type loopRetryDueWakeStore interface {
	EnqueueDueLoopRetryWakesPage(
		context.Context,
		taskpkg.Origin,
		time.Time,
		looppkg.RetryDueCursor,
		int,
	) ([]taskpkg.Run, looppkg.RetryDueCursor, error)
}

type loopWaitDueStore interface {
	ResumeDueLoopWaitsPage(
		context.Context,
		time.Time,
		looppkg.WaitDueCursor,
		int,
	) ([]taskpkg.Run, looppkg.WaitDueCursor, error)
}

type loopWaitEscalationStore interface {
	EscalateDueLoopWaitsPage(
		context.Context,
		time.Time,
		looppkg.WaitEscalationCursor,
		int,
	) (looppkg.WaitEscalationCursor, error)
}

type loopAdmissionClaimSweeper interface {
	SweepAdmissionClaims(context.Context, time.Time, int) (int, error)
}

type loopRetryDueScanState struct {
	mu     sync.Mutex
	cursor looppkg.RetryDueCursor
}

type loopWaitDueScanState struct {
	mu     sync.Mutex
	cursor looppkg.WaitDueCursor
}

type loopWaitEscalationScanState struct {
	mu     sync.Mutex
	cursor looppkg.WaitEscalationCursor
}

func newLoopRetryDueScanState() *loopRetryDueScanState {
	return &loopRetryDueScanState{}
}

func newLoopWaitDueScanState() *loopWaitDueScanState {
	return &loopWaitDueScanState{}
}

func newLoopWaitEscalationScanState() *loopWaitEscalationScanState {
	return &loopWaitEscalationScanState{}
}

func (s schedulerTaskSource) enqueueDueLoopRetryWakes(
	ctx context.Context,
	origin taskpkg.Origin,
	now time.Time,
) error {
	if s.loopRetryDueScan == nil {
		return nil
	}
	dueWakes, ok := s.store.(loopRetryDueWakeStore)
	if !ok {
		return errLoopRetryDueWakeStoreRequired
	}
	s.loopRetryDueScan.mu.Lock()
	defer s.loopRetryDueScan.mu.Unlock()
	_, next, err := dueWakes.EnqueueDueLoopRetryWakesPage(
		ctx,
		origin,
		now,
		s.loopRetryDueScan.cursor,
		defaultLoopRetryDueScanPageSize,
	)
	if err != nil {
		return err
	}
	s.loopRetryDueScan.cursor = next
	return nil
}

func (s schedulerTaskSource) resumeDueLoopWaits(ctx context.Context, now time.Time) error {
	if s.loopWaitDueScan == nil {
		return nil
	}
	store, ok := s.store.(loopWaitDueStore)
	if !ok {
		return errors.New("daemon: scheduler task store must support Loop wait recovery")
	}
	s.loopWaitDueScan.mu.Lock()
	defer s.loopWaitDueScan.mu.Unlock()
	_, next, err := store.ResumeDueLoopWaitsPage(
		ctx, now, s.loopWaitDueScan.cursor, defaultLoopRetryDueScanPageSize,
	)
	if err != nil {
		return err
	}
	s.loopWaitDueScan.cursor = next
	return nil
}

func (s schedulerTaskSource) escalateDueLoopWaits(ctx context.Context, now time.Time) error {
	if s.loopWaitEscalationScan == nil {
		return nil
	}
	store, ok := s.store.(loopWaitEscalationStore)
	if !ok {
		return errors.New("daemon: scheduler task store must support Loop wait escalation")
	}
	s.loopWaitEscalationScan.mu.Lock()
	defer s.loopWaitEscalationScan.mu.Unlock()
	next, err := store.EscalateDueLoopWaitsPage(
		ctx, now, s.loopWaitEscalationScan.cursor, defaultLoopRetryDueScanPageSize,
	)
	if err != nil {
		return err
	}
	s.loopWaitEscalationScan.cursor = next
	return nil
}

func (s schedulerTaskSource) sweepLoopAdmissionClaims(ctx context.Context, now time.Time) error {
	sweeper, ok := s.store.(loopAdmissionClaimSweeper)
	if !ok {
		return errLoopAdmissionClaimSweeperRequired
	}
	_, err := sweeper.SweepAdmissionClaims(
		ctx,
		now.UTC(),
		defaultLoopRetryDueScanPageSize,
	)
	return err
}
