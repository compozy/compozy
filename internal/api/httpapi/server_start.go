package httpapi

import (
	"context"
	"errors"
	"fmt"

	"net"
	"net/http"
	"strconv"
	"strings"
)

// Port reports the effective HTTP port.
func (s *Server) Port() int {
	if s == nil {
		return 0
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.actualPort > 0 {
		return s.actualPort
	}
	return s.port
}

// Start begins serving the API over the configured TCP address.
func (s *Server) Start(ctx context.Context) error {
	if s == nil {
		return errors.New("httpapi: server is required")
	}
	if ctx == nil {
		return errors.New("httpapi: start context is required")
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	address := net.JoinHostPort(strings.TrimSpace(s.host), strconv.Itoa(s.port))
	var listenConfig net.ListenConfig
	ln, err := listenConfig.Listen(ctx, "tcp", address)
	if err != nil {
		return fmt.Errorf("httpapi: listen on %q: %w", address, err)
	}

	streamCtx, streamCancel := context.WithCancel(context.WithoutCancel(ctx))
	httpServer := &http.Server{
		Handler:           s.engine,
		ReadHeaderTimeout: defaultReadHeaderTimeout,
		IdleTimeout:       defaultIdleTimeout,
	}
	httpServer.RegisterOnShutdown(streamCancel)
	serveDone := make(chan struct{})

	actualPort := s.port
	if tcpAddr, ok := ln.Addr().(*net.TCPAddr); ok && tcpAddr.Port > 0 {
		actualPort = tcpAddr.Port
	}

	s.mu.Lock()
	if s.started {
		s.mu.Unlock()
		streamCancel()
		startErr := errors.New("httpapi: server already started")
		if closeErr := ln.Close(); closeErr != nil {
			return errors.Join(startErr, fmt.Errorf("httpapi: close duplicate listener: %w", closeErr))
		}
		return startErr
	}
	s.handlers.setStreamDone(streamCtx.Done())
	s.handlers.setHTTPPort(actualPort)
	s.httpServer = httpServer
	s.listener = ln
	s.serveDone = serveDone
	s.serveErr = nil
	s.streamCancel = streamCancel
	s.started = true
	s.actualPort = actualPort
	s.mu.Unlock()

	s.logger.Info("httpapi: static web assets source selected", "source", s.staticSource)

	go func() {
		defer close(serveDone)
		if err := httpServer.Serve(
			ln,
		); err != nil && !errors.Is(err, http.ErrServerClosed) &&
			!errors.Is(err, net.ErrClosed) {
			s.mu.Lock()
			s.serveErr = fmt.Errorf("httpapi: serve %q: %w", address, err)
			s.mu.Unlock()
		}
	}()

	return nil
}
