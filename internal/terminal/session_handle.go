package terminal

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	terminalwire "github.com/compozy/compozy/internal/terminal/wire"
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

func (s *session) Write(ctx context.Context, actor Actor, input []byte) error {
	return s.deliverInputMode(ctx, actor, input, false, false)
}

type inputVisibilityProc interface {
	InputVisible() (bool, error)
	WriteRedacted([]byte) (int, error)
}

func requireInputVisibilityProc(proc Proc) (inputVisibilityProc, error) {
	visibilityProc, ok := proc.(inputVisibilityProc)
	if !ok {
		return nil, &Error{
			Code:    "terminal_not_interactive",
			Message: "terminal input requests require an echo-aware interactive terminal",
			Err:     ErrNotInteractive,
		}
	}
	return visibilityProc, nil
}

func (s *session) deliverInputMode(
	ctx context.Context,
	actor Actor,
	input []byte,
	clientRedact bool,
	answerHandoff bool,
) error {
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
	if actor.Kind == ActorKindAgent {
		if s.manager.typingGrants == nil {
			return &Error{Code: "typing_grant_rejected", Message: "agent typing requires a one-time terminal grant", Err: ErrTypingGrant}
		}
		if err := s.manager.typingGrants.AuthorizeTerminalInput(ctx, actor, s.Info()); err != nil {
			return err
		}
	}
	filtered := s.filter.FilterInput(input)
	info := s.Info()
	redacted := clientRedact
	writer := s.proc.Write
	if clientRedact {
		visibilityProc, err := requireInputVisibilityProc(s.proc)
		if err != nil {
			return err
		}
		writer = visibilityProc.WriteRedacted
	}
	if visibilityProc, ok := s.proc.(inputVisibilityProc); ok {
		inputVisible, err := visibilityProc.InputVisible()
		if err != nil {
			return err
		}
		redacted = redacted || !inputVisible
		if redacted && !clientRedact {
			writer = visibilityProc.WriteRedacted
		}
	}
	auditInput := filtered
	if redacted {
		auditInput = nil
	}
	reservation, admitted := s.manager.reserveJournalInput(info, auditInput)
	if !admitted {
		return &Error{Code: "journal_unavailable", Message: "terminal input is blocked while the journal lane is full", Err: ErrJournalUnavailable}
	}
	var deliveryErr error
	if answerHandoff {
		deliveryErr = s.lease.answerHandoff(actor, filtered, writer)
	} else {
		deliveryErr = s.lease.deliverWith(actor, filtered, writer)
	}
	if deliveryErr != nil {
		s.manager.releaseJournalInput(info, reservation)
		return deliveryErr
	}
	s.manager.commitJournalInput(info, actor, auditInput, reservation)
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
		return &ReadResult{
			Content: string(modelFacingOutput([]byte(snapshot.Content))),
			Seq:     snapshot.Seq, Busy: snapshot.Busy, Untrusted: true,
		}, nil
	case "tail":
		return s.readTail(options)
	case "lines":
		return s.readLines(options)
	default:
		return nil, &Error{Code: "terminal_read_view_invalid", Message: "terminal read view must be screen, tail, or lines", Err: ErrUnsupported}
	}
}

func (s *session) readTail(options ReadOptions) (*ReadResult, error) {
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
	return &ReadResult{Content: string(content), Seq: seq, Truncated: truncated, Untrusted: true}, nil
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

func (s *session) Takeover(ctx context.Context, actor Actor, force bool) error {
	if err := s.authorizeProfile(actor); err != nil {
		return err
	}
	if err := s.runningGate(); err != nil {
		return err
	}
	_, before := s.lease.snapshot()
	if err := s.lease.takeover(actor, force); err != nil {
		return err
	}
	if before == nil || !sameActor(actor, *before) {
		s.supersedeInputRequests(context.WithoutCancel(ctx), actor)
	}
	return nil
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

func (s *session) claim(actor Actor) error {
	if err := s.authorizeProfile(actor); err != nil {
		return err
	}
	if err := s.runningGate(); err != nil {
		return err
	}
	return s.lease.claim(actor)
}

func (s *session) runtimeRecovered(previous, current Actor) bool {
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
	s.lease.runtimeRecovered(previous, current)
	return true
}

func (s *session) runEnded(actor Actor) bool {
	info := s.Info()
	if info.BoundRun == nil || info.BoundRun.SessionID != actor.SessionID ||
		info.BoundRun.RunID != actor.RunID || info.BoundRun.Generation != actor.Generation {
		return false
	}
	s.lease.runEnded(actor, "run_ended")
	s.supersedeInputRequests(context.Background(), actor)
	return true
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
	info := s.Info()
	if actor.ProfileID != info.ProfileID {
		return &Error{Code: "terminal_not_found", Message: "terminal not found", Err: ErrNotFound}
	}
	if actor.Kind == ActorKindAgent && info.BoundRun != nil && actor.SessionID == info.BoundRun.SessionID &&
		actor.RunID == info.BoundRun.RunID && actor.Generation != info.BoundRun.Generation {
		return &Error{Code: "generation_fenced", Message: "terminal action came from a stale runtime generation", Err: ErrGenerationFenced}
	}
	return nil
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

func (s *session) leaseChanged(from, to LeaseState, reason string, actor Actor, controller *Actor) {
	s.mu.Lock()
	s.info.Lease = to
	s.info.Controller = cloneActor(controller)
	if from == LeaseAgentOwned && to != LeaseAgentOwned && reason != "answer_handoff" {
		s.info.TypingGeneration++
	}
	info := s.infoSnapshotLocked()
	subscribers := make([]*subscription, 0, len(s.subscribers))
	for _, subscriber := range s.subscribers {
		subscribers = append(subscribers, subscriber)
	}
	s.mu.Unlock()
	actorKind := ActorKind("")
	actorID := ""
	if controller != nil {
		actorKind = controller.Kind
		actorID = controller.ID
	}
	payload, err := json.Marshal(struct {
		Lease     LeaseState `json:"lease"`
		ActorKind ActorKind  `json:"actor_kind,omitempty"`
		ActorID   string     `json:"actor_id,omitempty"`
		Reason    string     `json:"reason"`
	}{Lease: to, ActorKind: actorKind, ActorID: actorID, Reason: reason})
	if err != nil {
		s.manager.logger.Warn("terminal: encode owner frame", "terminal_id", info.ID, "error", err)
	} else {
		for _, subscriber := range subscribers {
			subscriber.deliver(Frame{Op: terminalwire.ServerOpOwner, Payload: payload}, 0)
		}
	}
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

var _ Handle = (*session)(nil)
