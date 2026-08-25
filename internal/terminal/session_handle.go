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
	info.Controller = cloneActor(s.info.Controller)
	info.Exit = cloneExit(s.info.Exit)
	return info
}

func (s *session) Write(_ context.Context, actor Actor, input []byte) error {
	if err := s.authorizeProfile(actor); err != nil {
		return err
	}
	if s.Info().Mode != ModePTY {
		return &Error{Code: "terminal_not_interactive", Message: "terminal is not interactive", Err: ErrNotInteractive}
	}
	if s.audit.Blocked() {
		return &Error{Code: "journal_unavailable", Message: "terminal input is blocked while journal delivery is unavailable", Err: ErrJournalUnavailable}
	}
	if err := s.runningGate(); err != nil {
		return err
	}
	filtered := s.filter.FilterInput(input)
	info := s.Info()
	reservation, admitted := s.manager.reserveJournalInput(info, filtered)
	if !admitted {
		return &Error{Code: "journal_unavailable", Message: "terminal input is blocked while the journal lane is full", Err: ErrJournalUnavailable}
	}
	if err := s.lease.deliver(actor, filtered); err != nil {
		s.manager.releaseJournalInput(info, reservation)
		return err
	}
	s.manager.commitJournalInput(info, actor, filtered, reservation)
	return nil
}

func (s *session) Screen(ctx context.Context, options ReadOptions) (*ReadResult, error) {
	view := strings.TrimSpace(options.View)
	if view == "" {
		view = "screen"
	}
	if s.Info().Mode == ModePipe && view == "screen" {
		return nil, &Error{Code: "terminal_not_interactive", Message: "pipe terminals do not have a screen", Err: ErrNotInteractive}
	}
	s.touch()
	switch view {
	case "screen":
		snapshot, err := s.vt.Screen(ctx)
		if err != nil {
			return nil, fmt.Errorf("terminal: read screen: %w", err)
		}
		_, seq := s.ring.Snapshot()
		return &ReadResult{Content: snapshot.Content, Seq: seq, Busy: snapshot.Busy, Untrusted: true}, nil
	case "tail":
		return s.readTail(options), nil
	case "lines":
		return s.readLines(options), nil
	default:
		return nil, &Error{Code: "terminal_read_view_invalid", Message: "terminal read view must be screen, tail, or lines", Err: ErrUnsupported}
	}
}

func (s *session) readTail(options ReadOptions) *ReadResult {
	var content []byte
	var seq uint64
	truncated := false
	if options.SinceSeq == 0 {
		content, seq = s.ring.Snapshot()
		oldest, _ := s.ring.Bounds()
		truncated = oldest > 0
	} else {
		replay := s.ring.ReplayFrom(options.SinceSeq)
		content, seq, truncated = replay.Payload, replay.Seq, replay.Truncated
	}
	if options.MaxBytes > 0 && len(content) > options.MaxBytes {
		content = content[len(content)-options.MaxBytes:]
		content = trimPartialLeadingRune(content)
		truncated = true
	}
	return &ReadResult{Content: string(content), Seq: seq, Truncated: truncated, Untrusted: true}
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

func (s *session) readLines(options ReadOptions) *ReadResult {
	data, seq := s.ring.Snapshot()
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
		matches := make([]string, 0)
		for _, line := range strings.Split(selected, "\n") {
			if strings.Contains(line, options.Grep) {
				matches = append(matches, line)
			}
		}
		selected = strings.Join(matches, "\n")
	}
	return &ReadResult{Content: selected, Seq: seq, Untrusted: true}
}

func (s *session) Takeover(_ context.Context, actor Actor, force bool) error {
	if err := s.authorizeProfile(actor); err != nil {
		return err
	}
	if err := s.runningGate(); err != nil {
		return err
	}
	return s.lease.takeover(actor, force)
}

func (s *session) Yield(_ context.Context, actor Actor) error {
	if err := s.authorizeProfile(actor); err != nil {
		return err
	}
	if err := s.runningGate(); err != nil {
		return err
	}
	return s.lease.yield(actor)
}

func (s *session) Signal(_ context.Context, actor Actor, signal Signal) error {
	if err := s.authorizeProfile(actor); err != nil {
		return err
	}
	if err := s.runningGate(); err != nil {
		return err
	}
	if err := s.authorizeClose(actor); err != nil {
		return err
	}
	if err := s.proc.Kill(terminalSignal(signal)); err != nil {
		return fmt.Errorf("terminal: signal %q: %w", s.Info().ID, err)
	}
	return nil
}

func (s *session) authorizeClose(actor Actor) error {
	if err := s.authorizeProfile(actor); err != nil {
		return err
	}
	if actor.Kind == ActorKindHuman || actor.Kind == ActorKindSystem {
		return nil
	}
	state, controller := s.lease.snapshot()
	if actor.Kind == ActorKindAgent && controller != nil && sameRun(actor, *controller) &&
		actor.Generation != controller.Generation {
		return &Error{Code: "generation_fenced", Message: "terminal action came from a stale runtime generation", Err: ErrGenerationFenced}
	}
	if state == LeaseAgentOwned && controller != nil && sameActor(actor, *controller) {
		return nil
	}
	return &Error{Code: "write_owner_held", Message: "terminal is controlled by another actor", Controller: controller, Err: ErrWriteOwnerHeld}
}

func (s *session) authorizeProfile(actor Actor) error {
	if actor.ProfileID != s.Info().ProfileID {
		return &Error{Code: "terminal_not_found", Message: "terminal not found", Err: ErrNotFound}
	}
	return nil
}

func (s *session) RequestInput(context.Context, InputRequest) (*InputOutcome, error) {
	if s.Info().Mode != ModePTY {
		return nil, &Error{Code: "terminal_not_interactive", Message: "terminal is not interactive", Err: ErrNotInteractive}
	}
	return nil, &Error{Code: "terminal_input_requests_unavailable", Message: "terminal input requests are not available yet", Err: ErrUnsupported}
}

func (s *session) AnswerInput(_ context.Context, actor Actor, _ InputRequestID, _ InputAnswer) error {
	if err := s.authorizeProfile(actor); err != nil {
		return err
	}
	return &Error{Code: "terminal_input_requests_unavailable", Message: "terminal input requests are not available yet", Err: ErrUnsupported}
}

func (s *session) RejectInput(_ context.Context, actor Actor, _ InputRequestID, _ string) error {
	if err := s.authorizeProfile(actor); err != nil {
		return err
	}
	return &Error{Code: "terminal_input_requests_unavailable", Message: "terminal input requests are not available yet", Err: ErrUnsupported}
}

func (s *session) StartRecording(_ context.Context, actor Actor) (RecordingRef, error) {
	if err := s.authorizeProfile(actor); err != nil {
		return RecordingRef{}, err
	}
	return s.startRecording(actor)
}

func (s *session) StopRecording(ctx context.Context, actor Actor) (RecordingRef, error) {
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
		return &Error{Code: "terminal_expired", Message: "terminal has expired", Err: ErrExpired}
	}
	return &Error{Code: "terminal_exited", Message: "terminal has exited", Err: ErrExited}
}

func (s *session) touch() {
	s.mu.Lock()
	s.lastActivity = s.manager.now()
	s.mu.Unlock()
}

func (s *session) leaseChanged(from, to LeaseState, reason string, actor Actor) {
	s.mu.Lock()
	s.info.Lease = to
	_, controller := s.lease.snapshot()
	s.info.Controller = controller
	info := s.infoSnapshotLocked()
	s.mu.Unlock()
	s.manager.events.Emit(context.Background(), TerminalEvent{
		Kind: EventKindLeaseChanged, WorkspaceID: info.WS, ProfileID: info.ProfileID,
		ProfileName: s.profileName,
		TerminalID:  info.ID, Actor: actor, Info: &info,
		Reason: reason, Detail: EventDetail{LeaseFrom: from, LeaseTo: to}, At: s.manager.now(),
	})
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

func ignoreExited(err error) error {
	if errors.Is(err, ErrExited) {
		return nil
	}
	return err
}

var _ Handle = (*session)(nil)
