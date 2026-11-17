package monitor

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// OutputStream reads one terminal's output for an until=stream monitor. Next blocks until output has
// settled for settle, the terminal exits, or ctx ends, and returns what arrived since the previous read.
// A stream has one reader: its monitor's goroutine.
type OutputStream interface {
	Next(ctx context.Context, settle time.Duration) StreamBatch
}

// StreamBatch is one read of a terminal's output.
type StreamBatch struct {
	// Text is untrusted terminal output printed since the previous read.
	Text string
	// Gap counts output units dropped before Text because the stream fell behind the scrollback.
	Gap uint64
	// Exited reports that the terminal exited; ExitCode is its code when it has one.
	Exited   bool
	ExitCode *int
	// Closed reports that the terminal ended without an exit record.
	Closed bool
}

// maxStreamCarry bounds the unfinished line a pattern stream holds back while it waits for its newline.
const maxStreamCarry = maxOutputBytes

// streamFire is one wake a stream produces. A final fire ends the monitor once its wake is admitted.
type streamFire struct {
	reason     string
	exitCode   *int
	text       string
	gap        uint64
	outputLost bool
	final      bool
}

// admitStreamLocked enforces the per-session stream cap. A stream also holds the ordinary caps.
func (s *Service) admitStreamLocked(item *entry) error {
	if item.state.Until != UntilStream {
		return nil
	}
	streams := 0
	for _, existing := range s.sessions[item.state.SessionID] {
		if existing.occupies() && existing.state.Until == UntilStream {
			streams++
		}
	}
	if streams >= MaxStreamsPerSession {
		return fmt.Errorf("%w: at most %d streams per session", ErrLimit, MaxStreamsPerSession)
	}
	return nil
}

// runStream owns one stream monitor until it ends. Each settled batch of new output is one wake, and the
// stream stays armed after it. It ends when the terminal exits, the timeout passes, its session stops, or
// the fire cap is reached. A batch the pattern rejects wakes nothing.
func (s *Service) runStream(ctx context.Context, item *entry, stream OutputStream, outputLost bool) {
	defer func() {
		if ctx.Err() != nil {
			s.logInterrupted(item)
		}
	}()
	s.mu.Lock()
	deadline, pattern := item.state.DeadlineAt, item.state.Pattern
	s.mu.Unlock()
	lines, err := newStreamLines(pattern)
	if err != nil {
		// Creation validated the pattern, so this is unreachable; a terminal fire keeps the monitor honest anyway.
		s.fireStream(ctx, item, streamFire{reason: ReasonTerminalFailed, final: true})
		return
	}
	lost := outputLost
	for {
		readCtx, cancel := context.WithDeadline(ctx, deadline)
		batch := stream.Next(readCtx, StreamSettle)
		timedOut := readCtx.Err() != nil && ctx.Err() == nil
		cancel()
		if ctx.Err() != nil {
			return
		}
		fire := streamFire{
			reason:     ReasonStreamOutput,
			text:       lines.take(batch.Text, batch.Exited || batch.Closed || timedOut),
			gap:        batch.Gap,
			outputLost: lost,
		}
		switch {
		case batch.Exited:
			fire.reason, fire.exitCode, fire.final = ReasonExit, batch.ExitCode, true
		case batch.Closed:
			fire.reason, fire.final = ReasonTerminalFailed, true
		case timedOut:
			fire.reason, fire.final = ReasonTimeout, true
		case fire.text == "" && fire.gap == 0:
			continue
		}
		if !s.fireStream(ctx, item, fire) {
			return
		}
		lost = false
	}
}

// fireStream records a fire durably before admitting its wake, then settles the monitor when the fire is
// final or re-arms it. It reports whether the stream goes on.
func (s *Service) fireStream(ctx context.Context, item *entry, fire streamFire) bool {
	s.mu.Lock()
	if item.state.Status != StatusActive {
		s.mu.Unlock()
		return false
	}
	seq := item.state.FireCount + 1
	if seq >= MaxStreamFires {
		fire.final = true
		if fire.reason == ReasonStreamOutput {
			fire.reason = ReasonFireCapReached
		}
	}
	item.state.Status = StatusDelivering
	item.state.FireCount = seq
	item.state.LastFiredAt = s.now()
	item.state.Reason = fire.reason
	item.state.ExitCode = fire.exitCode
	s.logLifecycle("monitor.fired", item.state, fire.reason)
	state, record := item.state, s.decideLocked(item)
	s.mu.Unlock()
	s.notify(state.SessionID)
	if !s.persistFired(ctx, item, record) {
		return false
	}
	return s.concludeWake(ctx, item, fire.final, s.admitWithRetry(ctx, Notification{
		Monitor: state,
		Outcome: Outcome{
			Reason:     fire.reason,
			ExitCode:   fire.exitCode,
			Output:     fire.text,
			OutputGap:  fire.gap,
			OutputLost: fire.outputLost,
		},
	}))
}

// concludeWake settles a fire by its admission result: a final wake ends the monitor and any other re-arms
// it. It reports whether the monitor goes on.
func (s *Service) concludeWake(ctx context.Context, item *entry, final bool, err error) bool {
	switch {
	case err == nil:
	case ctx.Err() != nil:
		return false
	case errors.Is(err, ErrSessionGone):
		s.settle(item, StatusUndelivered, causeSessionGone)
		return false
	default:
		// The only other admission failure is an exhausted retry window.
		s.settle(item, StatusUndelivered, causeRetryExhausted)
		return false
	}
	if final {
		s.settle(item, StatusFired, "")
		return false
	}
	s.rearmStream(item)
	return true
}

// rearmStream returns a stream whose wake was admitted to active, keeping its cap slots.
func (s *Service) rearmStream(item *entry) {
	s.mu.Lock()
	if item.state.Status != StatusDelivering {
		s.mu.Unlock()
		return
	}
	item.state.Status = StatusActive
	item.admitted = true
	s.logLifecycle("monitor.wake_queued", item.state, item.state.Reason)
	record, sessionID := s.decideLocked(item), item.state.SessionID
	s.mu.Unlock()
	s.writeDecision(item, record, saveRetries)
	s.notify(sessionID)
}

// runRecoveredStream re-arms a stream after a daemon restart. Output it had not reported before the restart
// is not retained, so the stream resumes at the terminal's current tail and its next wake says so. A wake
// that was delivering is admitted again under its own id with OutputLost, and no command is rerun.
func (s *Service) runRecoveredStream(ctx context.Context, item *entry, mon Monitor) {
	if ctx.Err() != nil {
		return
	}
	stream, err := s.openRecoveredStream(ctx, item, mon)
	if err != nil {
		s.failRecoveredStream(ctx, item, mon)
		return
	}
	if mon.Status == StatusDelivering {
		if !s.concludeWake(ctx, item, isFinalStreamReason(mon.Reason), s.admitRecovered(ctx, mon)) {
			return
		}
		s.runStream(ctx, item, stream, false)
		return
	}
	s.runStream(ctx, item, stream, true)
}

// failRecoveredStream ends a recovered stream whose terminal can no longer be observed. A wake that was
// delivering is admitted as it stands; the stream is over either way.
func (s *Service) failRecoveredStream(ctx context.Context, item *entry, mon Monitor) {
	if mon.Status == StatusDelivering {
		s.concludeWake(ctx, item, true, s.admitRecovered(ctx, mon))
		return
	}
	s.fireStream(ctx, item, streamFire{reason: ReasonTerminalFailed, final: true})
}

// admitRecovered admits a recovered delivering fire under its original message id.
func (s *Service) admitRecovered(ctx context.Context, mon Monitor) error {
	return s.admitWithRetry(ctx, Notification{
		Monitor: mon,
		Outcome: Outcome{Reason: mon.Reason, ExitCode: mon.ExitCode, OutputLost: true, Targets: mon.Targets},
	})
}

func (s *Service) openRecoveredStream(ctx context.Context, item *entry, mon Monitor) (OutputStream, error) {
	if s.terminals == nil {
		return nil, errors.New("monitor: terminal service is unavailable")
	}
	return s.terminals.OpenStream(ctx, Spec{
		WorkspaceID: item.workspaceID, ProfileID: item.profileID, SessionID: mon.SessionID, Kind: mon.Kind,
		Description: mon.Description, TerminalID: mon.TerminalID, Until: mon.Until, Pattern: mon.Pattern,
	})
}

// isFinalStreamReason reports whether a stream wake with this reason ended its stream.
func isFinalStreamReason(reason string) bool {
	return reason != ReasonStreamOutput
}

// streamLines turns batches of terminal output into the text one wake reports. A pattern keeps only the
// complete lines that match it; a line still missing its newline waits in carry for the rest.
type streamLines struct {
	filter *regexp.Regexp
	carry  string
}

func newStreamLines(pattern string) (*streamLines, error) {
	if pattern == "" {
		return &streamLines{}, nil
	}
	filter, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}
	return &streamLines{filter: filter}, nil
}

// take returns the text one read reports. final flushes the unfinished line too, because the stream is ending.
func (l *streamLines) take(text string, final bool) string {
	if l.filter == nil {
		return text
	}
	text = l.carry + text
	l.carry = ""
	if final {
		return matchingLines(text, l.filter)
	}
	end := strings.LastIndexByte(text, '\n') + 1
	if len(text)-end > maxStreamCarry {
		// An unbroken line this long is judged as it stands, which keeps the carry bounded.
		end = len(text)
	}
	l.carry = text[end:]
	return matchingLines(text[:end], l.filter)
}

// matchingLines keeps the lines of text that the filter matches, without their carriage returns.
func matchingLines(text string, filter *regexp.Regexp) string {
	var kept []string
	for line := range strings.SplitSeq(text, "\n") {
		line = strings.TrimRight(line, "\r")
		if line != "" && filter.MatchString(line) {
			kept = append(kept, line)
		}
	}
	return strings.Join(kept, "\n")
}
