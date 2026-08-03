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

var _ loopRetryDueWakeStore = (*globaldb.GlobalDB)(nil)

type loopRetryDueWakeStore interface {
	EnqueueDueLoopRetryWakesPage(
		context.Context,
		taskpkg.Origin,
		time.Time,
		looppkg.RetryDueCursor,
		int,
	) ([]taskpkg.Run, looppkg.RetryDueCursor, error)
}

type loopRetryDueScanState struct {
	mu     sync.Mutex
	cursor looppkg.RetryDueCursor
}

func newLoopRetryDueScanState() *loopRetryDueScanState {
	return &loopRetryDueScanState{}
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
