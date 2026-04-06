package runs

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	tailpkg "github.com/nxadm/tail"

	"github.com/compozy/compozy/pkg/compozy/events"
)

type tailState struct {
	fromSeq uint64
	lastSeq uint64
}

// Tail replays historical events from fromSeq and then follows events.jsonl for
// new events until ctx is canceled.
func (r *Run) Tail(ctx context.Context, fromSeq uint64) (<-chan events.Event, <-chan error) {
	out := make(chan events.Event)
	errs := make(chan error, 4)

	go func() {
		defer close(out)
		defer close(errs)

		if r == nil {
			sendRunError(ctx, errs, errors.New("tail run: nil run"))
			return
		}

		startOffset, ok := r.snapshotTailOffset(ctx, errs)
		if !ok {
			return
		}

		state := &tailState{fromSeq: fromSeq}
		if !r.replayTailHistory(ctx, errs, out, state) {
			return
		}

		r.followTailStream(ctx, startOffset, errs, out, state)
	}()

	return out, errs
}

func (r *Run) snapshotTailOffset(ctx context.Context, errs chan<- error) (int64, bool) {
	startOffset, err := liveTailOffsetSnapshot(r.eventsPath)
	if err != nil {
		sendRunError(ctx, errs, fmt.Errorf("snapshot run tail offset: %w", err))
		return 0, false
	}
	return startOffset, true
}

func (r *Run) replayTailHistory(
	ctx context.Context,
	errs chan<- error,
	out chan<- events.Event,
	state *tailState,
) bool {
	for ev, replayErr := range r.Replay(state.fromSeq) {
		if replayErr != nil {
			if !sendRunError(ctx, errs, replayErr) {
				return false
			}
			continue
		}
		state.lastSeq = ev.Seq
		if !sendRunEvent(ctx, out, ev) {
			return false
		}
	}
	return true
}

func (r *Run) followTailStream(
	ctx context.Context,
	startOffset int64,
	errs chan<- error,
	out chan<- events.Event,
	state *tailState,
) {
	tailer, err := r.openTailFollower(startOffset)
	if err != nil {
		sendRunError(ctx, errs, fmt.Errorf("start run tail: %w", err))
		return
	}
	defer tailer.Kill(nil)
	defer tailer.Cleanup()

	for {
		select {
		case <-ctx.Done():
			tailer.Kill(ctx.Err())
			return
		case line, ok := <-tailer.Lines:
			if !ok {
				sendTailerErr(ctx, errs, tailer.Err())
				return
			}
			if !handleTailLine(ctx, errs, out, state, line) {
				return
			}
		}
	}
}

func (r *Run) openTailFollower(startOffset int64) (*tailpkg.Tail, error) {
	return tailpkg.TailFile(r.eventsPath, tailpkg.Config{
		Location:      &tailpkg.SeekInfo{Offset: startOffset, Whence: io.SeekStart},
		Follow:        true,
		ReOpen:        true,
		MustExist:     false,
		CompleteLines: true,
		Logger:        tailpkg.DiscardingLogger,
	})
}

func sendTailerErr(ctx context.Context, errs chan<- error, err error) {
	if err == nil || errors.Is(err, tailpkg.ErrStop) {
		return
	}
	_ = sendRunError(ctx, errs, fmt.Errorf("tail run events: %w", err))
}

func handleTailLine(
	ctx context.Context,
	errs chan<- error,
	out chan<- events.Event,
	state *tailState,
	line *tailpkg.Line,
) bool {
	if line == nil {
		return true
	}
	if line.Err != nil {
		return sendRunError(ctx, errs, line.Err)
	}

	text := strings.TrimSpace(line.Text)
	if text == "" {
		return true
	}

	ev, err := decodeEventLine([]byte(text), line.Num)
	if err != nil {
		return sendRunError(ctx, errs, err)
	}
	if ev.Seq < state.fromSeq || ev.Seq <= state.lastSeq {
		return true
	}

	state.lastSeq = ev.Seq
	return sendRunEvent(ctx, out, ev)
}

func sendRunEvent(ctx context.Context, dst chan<- events.Event, ev events.Event) bool {
	if err := ctx.Err(); err != nil {
		return false
	}
	select {
	case <-ctx.Done():
		return false
	case dst <- ev:
		return true
	}
}

func sendRunError(ctx context.Context, dst chan<- error, err error) bool {
	if err == nil {
		return true
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		return false
	}
	select {
	case <-ctx.Done():
		return false
	case dst <- err:
		return true
	}
}
