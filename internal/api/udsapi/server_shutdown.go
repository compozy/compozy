package udsapi

import (
	"context"
	"errors"
	"fmt"
	"net"
)

// Shutdown stops accepting new requests, drains active ones, and removes the socket file.
func (s *Server) Shutdown(ctx context.Context) error {
	if s == nil {
		return nil
	}
	if ctx == nil {
		return errors.New("udsapi: shutdown context is required")
	}

	s.mu.Lock()
	if s.state == serverStateStopped {
		s.mu.Unlock()
		return nil
	}
	httpServer := s.httpServer
	listener := s.listener
	serveDone := s.serveDone
	streamCancel := s.streamCancel
	socketPath := s.socketPath
	s.state = serverStateStopping
	s.mu.Unlock()

	var errs []error
	if httpServer != nil {
		if err := httpServer.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("udsapi: shutdown http server: %w", err))
		}
	}
	if streamCancel != nil {
		streamCancel()
	}
	if s.handlers != nil {
		if err := s.handlers.ShutdownWindowManagerStreams(ctx); err != nil {
			errs = append(errs, fmt.Errorf("udsapi: shutdown window-manager streams: %w", err))
		}
	}
	if listener != nil {
		if err := listener.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			errs = append(errs, fmt.Errorf("udsapi: close listener: %w", err))
		}
	}
	if serveDone != nil {
		if err := waitForServeDone(ctx, serveDone); err != nil {
			errs = append(errs, err)
		}
	}
	if err := removeSocketPath(socketPath); err != nil {
		errs = append(errs, err)
	}
	s.mu.Lock()
	serveErr := s.serveErr
	if serveErr != nil {
		errs = append(errs, serveErr)
	}
	if len(errs) == 0 {
		s.httpServer = nil
		s.listener = nil
		s.serveDone = nil
		s.streamCancel = nil
		s.serveErr = nil
		s.state = serverStateStopped
	} else {
		s.state = serverStateStopping
	}
	s.mu.Unlock()

	return errors.Join(errs...)
}
