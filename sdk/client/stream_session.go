package client

import (
	"io"
	"sync"
)

// StreamSession manages a live execution stream.
type StreamSession struct {
	ExecID  string
	ExecURL string

	events <-chan StreamEvent
	errors <-chan error
	closeF func() error

	closeOnce sync.Once
}

// Events returns a receive-only channel for stream events.
func (s *StreamSession) Events() <-chan StreamEvent {
	return s.events
}

// Errors returns a receive-only channel delivering terminal stream errors.
func (s *StreamSession) Errors() <-chan error {
	return s.errors
}

// Close terminates the stream and releases resources.
func (s *StreamSession) Close() error {
	if s == nil {
		return nil
	}
	var err error
	s.closeOnce.Do(func() {
		if s.closeF != nil {
			err = s.closeF()
		}
	})
	return err
}

func newStreamSession(
	execID string,
	execURL string,
	events <-chan StreamEvent,
	errors <-chan error,
	closer func() error,
) *StreamSession {
	return &StreamSession{
		ExecID:  execID,
		ExecURL: execURL,
		events:  events,
		errors:  errors,
		closeF: func() error {
			if closer == nil {
				return nil
			}
			return closer()
		},
	}
}

func closeBody(body io.Closer) error {
	if body == nil {
		return nil
	}
	return body.Close()
}
