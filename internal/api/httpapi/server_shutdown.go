package httpapi

import (
	"context"
	"errors"
	"fmt"
	"net"
)

// Shutdown stops accepting new requests and drains active ones.
func (s *Server) Shutdown(ctx context.Context) error {
	if s == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}

	s.mu.Lock()
	httpServer := s.httpServer
	listener := s.listener
	serveDone := s.serveDone
	streamCancel := s.streamCancel
	serveErr := s.serveErr
	s.httpServer = nil
	s.listener = nil
	s.serveDone = nil
	s.streamCancel = nil
	s.serveErr = nil
	s.started = false
	s.actualPort = 0
	s.mu.Unlock()

	var errs []error
	if httpServer != nil {
		if err := httpServer.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("httpapi: shutdown http server: %w", err))
		}
	}
	if streamCancel != nil {
		streamCancel()
	}
	if s.handlers != nil {
		if err := s.handlers.ShutdownWindowManagerStreams(ctx); err != nil {
			errs = append(errs, fmt.Errorf("httpapi: shutdown window-manager streams: %w", err))
		}
	}
	if listener != nil {
		if err := listener.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			errs = append(errs, fmt.Errorf("httpapi: close listener: %w", err))
		}
	}
	if serveDone != nil {
		if err := waitForServeDone(ctx, serveDone); err != nil {
			errs = append(errs, err)
		}
	}
	if serveErr != nil {
		errs = append(errs, serveErr)
	}

	return errors.Join(errs...)
}
