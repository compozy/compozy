package runs

import (
	"context"
	"errors"

	"github.com/compozy/compozy/pkg/compozy/events"
)

// Tail replays historical events from fromSeq and then follows the daemon stream for
// new events until ctx is canceled.
func (r *Run) Tail(ctx context.Context, fromSeq uint64) (<-chan events.Event, <-chan error) {
	out := make(chan events.Event)
	errs := make(chan error, 4)

	go func() {
		defer close(out)
		defer close(errs)

		if err := validateTailRun(r); err != nil {
			sendRunError(ctx, errs, err)
			return
		}

		cutoff, ok := tailSnapshotCursor(ctx, r, errs)
		if !ok {
			return
		}
		if !replayHistoricalWindow(ctx, r.client, r.summary.RunID, fromSeq, cutoff, out, errs) {
			return
		}
		if !followTailLiveEvents(ctx, r, cutoff, out, errs) {
			return
		}
	}()

	return out, errs
}

func validateTailRun(r *Run) error {
	if r == nil {
		return errors.New("tail run: nil run")
	}
	if r.client == nil {
		return wrapDaemonUnavailable("tail run", errors.New("daemon client is required"))
	}
	return nil
}

func tailSnapshotCursor(ctx context.Context, r *Run, errs chan<- error) (RemoteCursor, bool) {
	snapshot, err := r.client.GetRunSnapshot(ctx, r.summary.RunID)
	if err != nil {
		sendRunError(ctx, errs, err)
		return RemoteCursor{}, false
	}
	if snapshot.NextCursor == nil {
		return RemoteCursor{}, true
	}
	return *snapshot.NextCursor, true
}

func followTailLiveEvents(
	ctx context.Context,
	r *Run,
	after RemoteCursor,
	out chan<- events.Event,
	errs chan<- error,
) bool {
	liveEvents, liveErrs := watchRemoteAfter(ctx, r.client, r.summary.RunID, after)
	for liveEvents != nil || liveErrs != nil {
		if !handleTailLiveUpdate(ctx, out, errs, &liveEvents, &liveErrs) {
			return false
		}
	}
	return true
}

func handleTailLiveUpdate(
	ctx context.Context,
	out chan<- events.Event,
	errs chan<- error,
	liveEvents *<-chan events.Event,
	liveErrs *<-chan error,
) bool {
	select {
	case <-ctx.Done():
		return false
	case err, ok := <-*liveErrs:
		if !ok {
			*liveErrs = nil
			return true
		}
		if err == nil {
			return true
		}
		return sendRunError(ctx, errs, err)
	case ev, ok := <-*liveEvents:
		if !ok {
			*liveEvents = nil
			return true
		}
		return sendRunEvent(ctx, out, ev)
	}
}
