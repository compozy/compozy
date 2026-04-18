package runs

import (
	"context"
	"errors"
	"iter"

	"github.com/compozy/compozy/pkg/compozy/events"
)

var errStopReplay = errors.New("stop replay")

// Replay yields all persisted events from fromSeq forward through the daemon-backed event page API.
func (r *Run) Replay(fromSeq uint64) iter.Seq2[events.Event, error] {
	return func(yield func(events.Event, error) bool) {
		if r == nil {
			yield(events.Event{}, errors.New("replay run: nil run"))
			return
		}
		if r.client == nil {
			yield(events.Event{}, wrapDaemonUnavailable("replay run", errors.New("daemon client is required")))
			return
		}

		if err := replayRemoteEvents(
			context.Background(),
			r.client,
			r.summary.RunID,
			fromSeq,
			RemoteCursor{},
			yield,
		); err != nil {
			yield(events.Event{}, err)
		}
	}
}

func replayRemoteEvents(
	ctx context.Context,
	client daemonRunReader,
	runID string,
	fromSeq uint64,
	stopAfter RemoteCursor,
	yield func(events.Event, error) bool,
) error {
	after := RemoteCursor{}

	for {
		page, err := client.ListRunEvents(ctx, runID, after, defaultRunEventPageLimit)
		if err != nil {
			return err
		}
		if len(page.Events) == 0 {
			return nil
		}

		for i := range page.Events {
			ev := page.Events[i]
			if cursorAfter(remoteCursorFromEvent(ev), stopAfter) {
				return nil
			}
			if err := validateSchemaVersion(ev.SchemaVersion); err != nil {
				return err
			}
			if ev.Seq < fromSeq {
				continue
			}
			if !yield(ev, nil) {
				return errStopReplay
			}
		}

		if !page.HasMore || page.NextCursor == nil {
			return nil
		}
		after = *page.NextCursor
	}
}

func cursorAfter(left RemoteCursor, right RemoteCursor) bool {
	if right.Sequence == 0 || right.Timestamp.IsZero() {
		return false
	}
	switch {
	case left.Timestamp.After(right.Timestamp):
		return true
	case left.Timestamp.Before(right.Timestamp):
		return false
	default:
		return left.Sequence > right.Sequence
	}
}

func replayHistoricalWindow(
	ctx context.Context,
	client daemonRunReader,
	runID string,
	fromSeq uint64,
	cutoff RemoteCursor,
	out chan<- events.Event,
	errs chan<- error,
) bool {
	replayErr := replayRemoteEvents(ctx, client, runID, fromSeq, cutoff, func(ev events.Event, err error) bool {
		if err != nil {
			return sendRunError(ctx, errs, err)
		}
		return sendRunEvent(ctx, out, ev)
	})
	switch {
	case replayErr == nil:
		return true
	case errors.Is(replayErr, errStopReplay):
		return false
	default:
		return sendRunError(ctx, errs, replayErr)
	}
}
