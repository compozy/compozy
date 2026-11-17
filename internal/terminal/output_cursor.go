package terminal

import (
	"context"
	"time"
)

// OutputBatch is the output one OutputCursor read observed, oldest first.
type OutputBatch struct {
	// Text is the model-facing rendering of the output read since the cursor.
	Text string
	// Lost counts retained sequence units the cursor fell behind before Text. The scrollback moved past
	// its position while nobody was reading, so that output is no longer retained.
	Lost uint64
	// Exit is set once the terminal has exited; Text then holds the output printed before the exit.
	Exit *Exit
	// Closed reports that the terminal session ended without an exit record.
	Closed bool
}

// OutputCursor is an in-memory read position in one terminal's retained output. It stores no output:
// each read renders the retained segments from its position and advances it. A cursor has a single
// reader; it is not safe for concurrent Next calls.
type OutputCursor struct {
	session *session
	next    uint64
}

// OpenOutputCursor returns a cursor at the current end of the output, so its first read observes only
// output printed afterward.
func (s *session) OpenOutputCursor() *OutputCursor {
	_, next := s.ring.Bounds()
	return &OutputCursor{session: s, next: next}
}

// Next blocks until output has been pending for settle, the terminal exits, its session closes, or ctx
// ends, then returns what it observed. A zero settle returns at the first pending output; a longer one
// collects output printed within that window into one batch. A ctx end returns the output collected so
// far without an error, so callers check ctx themselves.
func (c *OutputCursor) Next(ctx context.Context, settle time.Duration) OutputBatch {
	s := c.session
	var settled <-chan time.Time
	var timer *time.Timer
	defer func() {
		if timer != nil {
			timer.Stop()
		}
	}()
	for {
		s.mu.RLock()
		exit := cloneExit(s.exit)
		revisionReady := s.revisionReady
		s.mu.RUnlock()
		if exit != nil {
			// The exit can land before the reader drains its last output; wait for the drain first.
			select {
			case <-ctx.Done():
			case <-s.done:
			}
			return c.drain(exit, false)
		}
		if _, next := s.ring.Bounds(); next > c.next && settled == nil {
			settled = closedTimeChannel()
			if settle > 0 {
				timer = time.NewTimer(settle)
				settled = timer.C
			}
		}
		select {
		case <-ctx.Done():
			return c.drain(nil, false)
		case <-settled:
			return c.drain(nil, false)
		case <-s.done:
			s.mu.RLock()
			exit = cloneExit(s.exit)
			s.mu.RUnlock()
			return c.drain(exit, exit == nil)
		case <-revisionReady:
		}
	}
}

// closedTimeChannel returns a channel that is already ready, so a zero settle needs no timer.
func closedTimeChannel() <-chan time.Time {
	ready := make(chan time.Time, 1)
	ready <- time.Time{}
	return ready
}

// drain renders the output retained since the cursor and advances the cursor past it. When the cursor
// fell behind the retained output, the gap is reported in Lost instead of being replayed.
func (c *OutputCursor) drain(exit *Exit, closed bool) OutputBatch {
	replay := c.session.ring.ReplayFrom(c.next)
	batch := OutputBatch{Exit: exit, Closed: closed}
	if replay.Truncated {
		// A truncated replay starts at the oldest retained segment, or at the tail when none remain.
		retainedStart := replay.Seq
		if len(replay.Segments) > 0 {
			retainedStart = replay.Segments[0].Seq
		}
		batch.Lost = retainedStart - c.next
	}
	if len(replay.Segments) > 0 {
		segments := make([]OutputSegment, 0, len(replay.Segments))
		for _, segment := range replay.Segments {
			segments = append(segments, segment.Segment)
		}
		batch.Text = string(modelFacingOutput([]byte(RenderOutputSegments(segments))))
	}
	c.next = replay.Seq
	return batch
}
