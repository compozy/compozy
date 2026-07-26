package daemon

import (
	"context"
	"errors"
	"sync"
	"time"

	looppkg "github.com/compozy/agh/internal/loop"
	"github.com/compozy/agh/internal/store/globaldb"
	taskpkg "github.com/compozy/agh/internal/task"
)

const defaultWatchEventsGapScanPageSize = 100

var errLoopWatchEventsGapWakeStoreRequired = errors.New(
	"daemon: scheduler task store must support watch-events gap recovery",
)

var _ loopWatchEventsGapWakeStore = (*globaldb.GlobalDB)(nil)

type loopWatchEventsGapWakeStore interface {
	EnqueueWatchEventsGapWakesPage(
		context.Context,
		taskpkg.Origin,
		time.Time,
		looppkg.ParkedWatchEventScanCursor,
		int,
	) ([]taskpkg.Run, looppkg.ParkedWatchEventScanCursor, error)
}

type loopWatchEventsGapScanState struct {
	mu     sync.Mutex
	cursor looppkg.ParkedWatchEventScanCursor
}

func newLoopWatchEventsGapScanState() *loopWatchEventsGapScanState {
	return &loopWatchEventsGapScanState{}
}

func (s schedulerTaskSource) enqueueWatchEventsGapWakes(
	ctx context.Context,
	origin taskpkg.Origin,
	now time.Time,
) error {
	if s.watchEventsGapScan == nil {
		return nil
	}
	gapWakes, ok := s.store.(loopWatchEventsGapWakeStore)
	if !ok {
		return errLoopWatchEventsGapWakeStoreRequired
	}
	s.watchEventsGapScan.mu.Lock()
	defer s.watchEventsGapScan.mu.Unlock()
	_, next, err := gapWakes.EnqueueWatchEventsGapWakesPage(
		ctx,
		origin,
		now,
		s.watchEventsGapScan.cursor,
		defaultWatchEventsGapScanPageSize,
	)
	s.watchEventsGapScan.cursor = next
	return err
}
