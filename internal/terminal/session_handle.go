package terminal

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

func (s *session) Info() Info {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.infoSnapshotLocked()
}

func (s *session) infoSnapshotLocked() Info {
	info := s.info
	info.Exit = cloneExit(s.info.Exit)
	return info
}

func (s *session) Screen(ctx context.Context, options ReadOptions) (*ReadResult, error) {
	if err := requestContextError(ctx, "read"); err != nil {
		return nil, err
	}
	view := strings.TrimSpace(options.View)
	if view == "" {
		view = terminalViewScreen
	}
	if s.Info().Mode == ModePipe && view == terminalViewScreen {
		mode := s.Info().Mode
		return nil, &Error{
			Code:    ErrorCodeNotInteractive,
			Message: "pipe terminals do not have a screen",
			Mode:    mode,
			Err:     ErrNotInteractive,
		}
	}
	s.touch()
	switch view {
	case terminalViewScreen:
		snapshot, err := s.vt.Screen(ctx)
		if err != nil {
			return nil, fmt.Errorf("terminal: read screen: %w", err)
		}
		return &ReadResult{
			Content: string(modelFacingOutput([]byte(snapshot.Content))),
			Seq:     snapshot.Seq, Busy: snapshot.Busy, Untrusted: true,
		}, nil
	case terminalViewTail:
		return s.readTail(options)
	case "lines":
		return s.readLines(options)
	default:
		return nil, fmt.Errorf("terminal read view must be screen, tail, or lines: %w", ErrUnsupported)
	}
}

func (s *session) readTail(options ReadOptions) (*ReadResult, error) {
	var content []byte
	var segments []OutputSegment
	var seq uint64
	truncated := false
	if options.SinceSeq == 0 {
		segments, seq = s.ring.SnapshotSegments()
		content = []byte(RenderOutputSegments(segments))
		oldest, _ := s.ring.Bounds()
		truncated = oldest > 0
	} else {
		replay := s.ring.ReplayFrom(options.SinceSeq)
		content, seq, truncated = replay.Bytes(), replay.Seq, replay.Truncated
		segments = make([]OutputSegment, 0, len(replay.Segments))
		for _, segment := range replay.Segments {
			segments = append(segments, segment.Segment)
		}
	}
	content = modelFacingOutput(content)
	if options.Grep != "" {
		var err error
		content, err = grepOutput(content, options.Grep)
		if err != nil {
			return nil, err
		}
	}
	if options.MaxBytes > 0 && len(content) > options.MaxBytes {
		content = boundedTail(content, options.MaxBytes)
		truncated = true
	}
	return &ReadResult{
		Content: string(content), Segments: segments, Seq: seq, Truncated: truncated, Untrusted: true,
	}, nil
}

func trimPartialLeadingRune(content []byte) []byte {
	for len(content) > 0 && !validUTF8LeadingByte(content[0]) {
		content = content[1:]
	}
	return content
}

func validUTF8LeadingByte(value byte) bool {
	return value < utf8.RuneSelf || value >= 0xC2 && value <= 0xF4
}

func boundedTail(content []byte, maxBytes int) []byte {
	if maxBytes <= 0 || len(content) <= maxBytes {
		return content
	}
	return trimPartialLeadingRune(content[len(content)-maxBytes:])
}

func (s *session) readLines(options ReadOptions) (*ReadResult, error) {
	data, seq := s.ring.Snapshot()
	data = modelFacingOutput(data)
	lines := strings.Split(string(data), "\n")
	from := max(options.FromLine, 0)
	to := options.ToLine
	if to <= 0 || to > len(lines) {
		to = len(lines)
	}
	if from > to {
		from = to
	}
	selected := strings.Join(lines[from:to], "\n")
	if options.Grep != "" {
		matched, err := grepOutput([]byte(selected), options.Grep)
		if err != nil {
			return nil, err
		}
		selected = string(matched)
	}
	truncated := false
	if options.MaxBytes > 0 && len(selected) > options.MaxBytes {
		selected = string(boundedTail([]byte(selected), options.MaxBytes))
		truncated = true
	}
	return &ReadResult{Content: selected, Seq: seq, Truncated: truncated, Untrusted: true}, nil
}

func (s *session) runtimeRecovered(previous, current Actor) bool {
	s.lifecycleMu.Lock()
	defer s.lifecycleMu.Unlock()
	s.mu.Lock()
	bound := s.info.BoundRun
	if bound == nil || bound.SessionID != previous.SessionID || bound.RunID != previous.RunID ||
		bound.Generation != previous.Generation || current.Generation <= previous.Generation {
		s.mu.Unlock()
		return false
	}
	s.info.BoundRun = &RunRef{
		SessionID: current.SessionID, RunID: current.RunID, Generation: current.Generation,
	}
	s.mu.Unlock()
	return true
}

func (s *session) runEnded(actor Actor) bool {
	s.lifecycleMu.Lock()
	defer s.lifecycleMu.Unlock()
	s.mu.Lock()
	bound := s.info.BoundRun
	if bound == nil || bound.SessionID != actor.SessionID || bound.RunID != actor.RunID ||
		bound.Generation != actor.Generation {
		s.mu.Unlock()
		return false
	}
	s.info.BoundRun = nil
	s.mu.Unlock()
	s.supersedeInputRequests(s.ctx, actor)
	return true
}

func (s *session) Signal(ctx context.Context, actor Actor, signal Signal) error {
	if err := requestContextError(ctx, "signal"); err != nil {
		return err
	}
	if err := s.authorizeProfile(actor); err != nil {
		return err
	}
	if err := s.runningGate(); err != nil {
		return err
	}
	if err := s.proc.Kill(terminalSignal(signal)); err != nil {
		return fmt.Errorf("terminal: signal %q: %w", s.Info().ID, err)
	}
	return nil
}

func (s *session) authorizeClose(actor Actor) error {
	return s.authorizeProfile(actor)
}

func (s *session) authorizeProfile(actor Actor) error {
	info := s.Info()
	if actor.ProfileID != info.ProfileID {
		return &Error{Code: ErrorCodeNotFound, Message: errorMessageNotFound, Err: ErrNotFound}
	}
	if actor.Kind == ActorKindAgent && info.BoundRun != nil && actor.SessionID == info.BoundRun.SessionID &&
		actor.RunID == info.BoundRun.RunID && actor.Generation != info.BoundRun.Generation {
		return &Error{
			Code:    ErrorCodeGenerationFenced,
			Message: errorMessageGenerationFenced,
			Err:     ErrGenerationFenced,
		}
	}
	return nil
}

func (s *session) StartRecording(ctx context.Context, actor Actor) (RecordingRef, error) {
	if err := requestContextError(ctx, "start recording"); err != nil {
		return RecordingRef{}, err
	}
	if err := s.authorizeProfile(actor); err != nil {
		return RecordingRef{}, err
	}
	return s.startRecording(ctx, actor)
}

func (s *session) StopRecording(ctx context.Context, actor Actor) (RecordingRef, error) {
	if err := requestContextError(ctx, "stop recording"); err != nil {
		return RecordingRef{}, err
	}
	if err := s.authorizeProfile(actor); err != nil {
		return RecordingRef{}, err
	}
	return s.stopRecording(ctx, actor, "manual")
}

func isRecordingNotActive(err error) bool {
	var terminalErr *Error
	return errors.As(err, &terminalErr) && terminalErr.Code == "recording_not_active"
}

func (s *session) runningGate() error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.exit == nil && !s.reaping {
		return nil
	}
	if s.reaping {
		return &Error{Code: ErrorCodeExpired, Message: errorMessageExpired, Err: ErrExpired}
	}
	return &Error{Code: ErrorCodeExited, Message: errorMessageExited, Err: ErrExited}
}

func (s *session) touch() {
	s.mu.Lock()
	s.lastActivity = s.manager.now()
	s.mu.Unlock()
}

func (s *session) exited() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.exit != nil
}

func (s *session) detachedSince() (bool, time.Time) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.subscribers) == 0, s.lastActivity
}

func (s *session) claimDetachedReap(now time.Time, ttl time.Duration) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.exit != nil || s.reaping || len(s.subscribers) != 0 || now.Sub(s.lastActivity) < ttl {
		return false
	}
	s.reaping = true
	return true
}

func (s *session) cancelDetachedReap() {
	s.mu.Lock()
	s.reaping = false
	s.mu.Unlock()
}

func (s *session) exitAt() (time.Time, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.exit == nil {
		return time.Time{}, false
	}
	return s.exit.At, true
}

var _ Handle = (*session)(nil)
