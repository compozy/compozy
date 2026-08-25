package terminal

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	terminalwire "github.com/compozy/compozy/internal/terminal/wire"
)

type subscription struct {
	session    *session
	id         uint64
	mode       string
	actor      Actor
	queue      *terminalwire.Queue
	leaseToken uint64
	cols       uint16
	rows       uint16
	removeOnce sync.Once
	finishOnce sync.Once
}

func (s *session) Attach(_ context.Context, options AttachOptions) (Subscription, error) {
	if err := s.authorizeProfile(options.Actor); err != nil {
		return nil, err
	}
	if s.Info().Mode != ModePTY {
		return nil, &Error{Code: "terminal_not_interactive", Message: "terminal is not interactive", Err: ErrNotInteractive}
	}
	if err := s.runningGate(); err != nil {
		return nil, err
	}
	mode, flow, err := normalizeAttachOptions(options)
	if err != nil {
		return nil, err
	}
	if mode == "write" {
		if err := s.lease.authorize(options.Actor); err != nil {
			return nil, err
		}
	}
	settings := s.settings(context.Background())
	s.mu.Lock()
	if s.reaping {
		s.mu.Unlock()
		return nil, &Error{Code: "terminal_expired", Message: "terminal has expired", Err: ErrExpired}
	}
	if s.exit != nil {
		s.mu.Unlock()
		return nil, &Error{Code: "terminal_exited", Message: "terminal has exited", Err: ErrExited}
	}
	if len(s.subscribers) >= settings.MaxSubscribers {
		current := len(s.subscribers)
		s.mu.Unlock()
		s.emitSubscriberLimit(options.Actor, current, settings.MaxSubscribers)
		return nil, &Error{Code: "subscriber_limit_reached", Message: "terminal subscriber limit reached", Current: current, Max: settings.MaxSubscribers, Err: ErrSubscriberLimit}
	}
	s.nextSubID++
	subscriber := &subscription{session: s, id: s.nextSubID, mode: mode, actor: options.Actor}
	subscriber.queue = terminalwire.NewQueue(terminalwire.QueueOptions{
		Flow: terminalwire.Flow(flow), Now: s.manager.now,
		Demoted: subscriber.demoted, Evicted: subscriber.evict,
	})
	s.flow.Add(subscriber.queue)
	if mode == "write" {
		subscriber.leaseToken = s.lease.attachWriter(options.Actor)
	}
	s.subscribers[subscriber.id] = subscriber
	s.info.Viewers = len(s.subscribers)
	s.lastActivity = s.manager.now()
	info := s.infoSnapshotLocked()
	cols, rows := s.cols, s.rows
	s.mu.Unlock()
	if err := subscriber.enqueueInitialFrames(options, info, cols, rows); err != nil {
		closeErr := subscriber.Close()
		return nil, errors.Join(err, closeErr)
	}
	if options.Cols > 0 && options.Rows > 0 && mode == "write" {
		if err := subscriber.Resize(options.Cols, options.Rows); err != nil {
			closeErr := subscriber.Close()
			return nil, errors.Join(err, closeErr)
		}
	}
	return subscriber, nil
}

func normalizeAttachOptions(options AttachOptions) (string, string, error) {
	mode := options.Mode
	if mode == "" {
		mode = "read"
	}
	if mode != "read" && mode != "write" {
		return "", "", &Error{Code: "terminal_attach_mode_invalid", Message: "terminal attach mode must be read or write", Err: ErrUnsupported}
	}
	flow := options.Flow
	if flow == "" && mode == "write" {
		flow = string(terminalwire.FlowAck)
	}
	if flow == "" {
		flow = string(terminalwire.FlowDrop)
	}
	if flow != string(terminalwire.FlowAck) && flow != string(terminalwire.FlowDrop) {
		return "", "", &Error{Code: "terminal_flow_invalid", Message: "terminal flow must be ack or drop", Err: ErrUnsupported}
	}
	return mode, flow, nil
}

func (s *subscription) enqueueInitialFrames(options AttachOptions, info Info, cols, rows uint16) error {
	replay := s.session.ring.ReplayFrom(options.AfterSeq)
	attached, err := json.Marshal(map[string]any{
		"seq": replay.Seq, "truncated": replay.Truncated, "cols": cols,
		"rows": rows, "lease": info.Lease, "mode": info.Mode,
	})
	if err != nil {
		return fmt.Errorf("terminal: encode ATTACHED frame: %w", err)
	}
	s.queue.Enqueue(Frame{Op: terminalwire.ServerOpAttached, Seq: replay.Seq, Payload: attached}, replay.Seq)
	if len(replay.Payload) > 0 {
		start := replay.Seq - min(replay.Seq, uint64(len(replay.Payload)))
		s.queue.Enqueue(Frame{Op: terminalwire.ServerOpOutput, Seq: start, Payload: replay.Payload}, replay.Seq)
	}
	return nil
}

func (s *subscription) Frames() <-chan Frame { return s.queue.Frames() }

func (s *subscription) Ack(bytes int) { s.queue.Ack(bytes) }

func (s *subscription) Resize(cols, rows uint16) error {
	if s.mode != "write" {
		return nil
	}
	cols, rows, ok := terminalwire.ClampDimensions(cols, rows)
	if !ok {
		return nil
	}
	s.session.mu.Lock()
	s.cols, s.rows = cols, rows
	nextCols, nextRows := s.session.resizeVoteLocked()
	changed := nextCols != s.session.cols || nextRows != s.session.rows
	s.session.cols, s.session.rows = nextCols, nextRows
	s.session.mu.Unlock()
	if !changed {
		return nil
	}
	return s.session.applyResize(nextCols, nextRows)
}

func (s *subscription) Close() error {
	s.removeOnce.Do(func() { s.session.removeSubscriber(s) })
	s.queue.Close()
	return nil
}

func (s *subscription) deliver(frame Frame, end uint64) {
	s.queue.Enqueue(frame, end)
}

func (s *subscription) finish(exit Exit) {
	payload, err := json.Marshal(map[string]any{
		"cause": exit.Cause, "exit_code": exit.Code, "signal": exit.Signal, "seq": s.session.ringNext(),
	})
	if err != nil {
		if closeErr := s.Close(); closeErr != nil {
			s.session.manager.logger.Warn("terminal: close subscription after exit encoding failure", "error", closeErr)
		}
		return
	}
	s.finishOnce.Do(func() {
		s.queue.Enqueue(Frame{Op: terminalwire.ServerOpExit, Payload: payload}, 0)
		s.removeOnce.Do(func() { s.session.removeSubscriber(s) })
		s.queue.Finish()
	})
}

func (s *subscription) evict(reason string) {
	s.emitFlowTransition(reason)
	if err := s.Close(); err != nil {
		s.session.manager.logger.Warn("terminal: close evicted subscription", "error", err)
	}
}

func (s *subscription) demoted(reason string) {
	s.emitFlowTransition(reason)
}

func (s *subscription) emitFlowTransition(reason string) {
	info := s.session.Info()
	s.session.manager.events.Emit(context.Background(), TerminalEvent{
		Kind: EventKindSubscriberEvicted, WorkspaceID: info.WS, ProfileID: info.ProfileID,
		ProfileName: s.session.profileName,
		TerminalID:  info.ID, Actor: s.actor,
		Reason: reason, Detail: EventDetail{Flow: string(s.queue.Flow())}, At: s.session.manager.now(),
	})
}

func (s *session) emitSubscriberLimit(actor Actor, current, maximum int) {
	info := s.Info()
	s.manager.events.Emit(context.Background(), TerminalEvent{
		Kind: EventKindLimitRejected, WorkspaceID: info.WS, ProfileID: info.ProfileID,
		TerminalID: info.ID, Actor: actor,
		Detail: EventDetail{Limit: "subscribers", Current: current, Max: maximum}, At: s.manager.now(),
	})
}

func (s *session) removeSubscriber(subscriber *subscription) {
	s.mu.Lock()
	delete(s.subscribers, subscriber.id)
	s.info.Viewers = len(s.subscribers)
	s.lastActivity = s.manager.now()
	cols, rows := s.resizeVoteLocked()
	changed := cols != s.cols || rows != s.rows
	s.cols, s.rows = cols, rows
	s.mu.Unlock()
	s.flow.Remove(subscriber.queue)
	if subscriber.leaseToken != 0 {
		s.lease.detachWriter(subscriber.leaseToken)
	}
	if changed {
		if err := s.applyResize(cols, rows); err != nil {
			s.manager.logger.Warn("terminal: resize after subscriber departure", "terminal_id", s.Info().ID, "error", err)
		}
	}
}

func (s *session) resizeVoteLocked() (uint16, uint16) {
	cols, rows := s.cols, s.rows
	first := true
	for _, subscriber := range s.subscribers {
		if subscriber.mode != "write" || subscriber.cols == 0 || subscriber.rows == 0 {
			continue
		}
		if first {
			cols, rows, first = subscriber.cols, subscriber.rows, false
			continue
		}
		cols, rows = min(cols, subscriber.cols), min(rows, subscriber.rows)
	}
	return cols, rows
}

func (s *session) applyResize(cols, rows uint16) error {
	if err := s.proc.Resize(cols, rows); err != nil {
		return fmt.Errorf("terminal: resize process: %w", err)
	}
	if err := s.vt.Resize(context.Background(), int(cols), int(rows)); err != nil {
		return fmt.Errorf("terminal: resize emulator: %w", err)
	}
	payload, err := json.Marshal(map[string]uint16{"cols": cols, "rows": rows})
	if err != nil {
		return fmt.Errorf("terminal: encode RESIZED frame: %w", err)
	}
	s.mu.RLock()
	subscribers := make([]*subscription, 0, len(s.subscribers))
	for _, subscriber := range s.subscribers {
		subscribers = append(subscribers, subscriber)
	}
	s.mu.RUnlock()
	for _, subscriber := range subscribers {
		subscriber.deliver(Frame{Op: terminalwire.ServerOpResized, Payload: payload}, 0)
	}
	return nil
}

func (s *session) ringNext() uint64 {
	_, next := s.ring.Bounds()
	return next
}
