package terminal

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
)

const subscriptionFrameCapacity = 128

type subscription struct {
	session    *session
	id         uint64
	mode       string
	flow       string
	actor      Actor
	frames     chan Frame
	leaseToken uint64
	mu         sync.Mutex
	closed     bool
	droppedTo  uint64
	closeOnce  sync.Once
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
	mode := options.Mode
	if mode == "" {
		mode = "read"
	}
	if mode != "read" && mode != "write" {
		return nil, &Error{Code: "terminal_attach_mode_invalid", Message: "terminal attach mode must be read or write", Err: ErrUnsupported}
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
		s.mu.Unlock()
		return nil, &Error{Code: "subscriber_limit_reached", Message: "terminal subscriber limit reached", Current: settings.MaxSubscribers, Max: settings.MaxSubscribers, Err: ErrSubscriberLimit}
	}
	s.nextSubID++
	subscriber := &subscription{
		session: s, id: s.nextSubID, mode: mode, flow: options.Flow,
		actor: options.Actor, frames: make(chan Frame, subscriptionFrameCapacity),
	}
	if mode == "write" {
		subscriber.leaseToken = s.lease.attachWriter(options.Actor)
	}
	s.subscribers[subscriber.id] = subscriber
	s.info.Viewers = len(s.subscribers)
	s.lastActivity = s.manager.now()
	info := s.infoSnapshotLocked()
	s.mu.Unlock()
	replay := s.ring.ReplayFrom(options.AfterSeq)
	attached, marshalErr := json.Marshal(map[string]any{
		"seq": replay.Seq, "truncated": replay.Truncated, "cols": options.Cols,
		"rows": options.Rows, "lease": info.Lease, "mode": info.Mode,
	})
	if marshalErr != nil {
		closeErr := subscriber.Close()
		return nil, errors.Join(marshalErr, closeErr)
	}
	subscriber.frames <- Frame{Op: 0x02, Seq: replay.Seq, Payload: attached}
	if len(replay.Payload) > 0 {
		subscriber.frames <- Frame{Op: 0x01, Seq: replay.Seq - uint64(len(replay.Payload)), Payload: replay.Payload}
	}
	if options.Cols > 0 && options.Rows > 0 && mode == "write" {
		if err := subscriber.Resize(options.Cols, options.Rows); err != nil {
			closeErr := subscriber.Close()
			return nil, errors.Join(err, closeErr)
		}
	}
	return subscriber, nil
}

func (s *subscription) Frames() <-chan Frame { return s.frames }

func (s *subscription) Ack(int) {}

func (s *subscription) Resize(cols, rows uint16) error {
	if s.mode != "write" || cols == 0 || rows == 0 {
		return nil
	}
	if err := s.session.proc.Resize(cols, rows); err != nil {
		return err
	}
	return s.session.vt.Resize(context.Background(), int(cols), int(rows))
}

func (s *subscription) Close() error {
	s.closeOnce.Do(func() {
		s.mu.Lock()
		s.closed = true
		s.mu.Unlock()
		s.session.removeSubscriber(s)
		close(s.frames)
	})
	return nil
}

func (s *subscription) deliver(frame Frame, end uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	select {
	case s.frames <- frame:
	default:
		s.droppedTo = end
	}
}

func (s *subscription) finish(exit Exit) {
	payload, err := json.Marshal(map[string]any{
		"cause": exit.Cause, "exit_code": exit.Code, "signal": exit.Signal,
	})
	if err == nil {
		s.deliver(Frame{Op: 0x03, Payload: payload}, 0)
	}
	_ = s.Close()
}

func (s *session) removeSubscriber(subscriber *subscription) {
	s.mu.Lock()
	delete(s.subscribers, subscriber.id)
	s.info.Viewers = len(s.subscribers)
	s.lastActivity = s.manager.now()
	s.mu.Unlock()
	if subscriber.leaseToken != 0 {
		s.lease.detachWriter(subscriber.leaseToken)
	}
}
