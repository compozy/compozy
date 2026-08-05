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

	generation, startCtx, err := s.beginStart(ctx)
	if err != nil {
		return err
	}
	defer generation.startCancel()

	address := net.JoinHostPort(strings.TrimSpace(s.host), strconv.Itoa(s.port))
	var listenConfig net.ListenConfig
	listener, err := listenConfig.Listen(startCtx, "tcp", address)
	if err != nil {
		s.completeStart(generation, false)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if startCtx.Err() != nil {
			return errServerShutdownInProgress
		}
		return fmt.Errorf("httpapi: listen on %q: %w", address, err)
	}

	streamCtx, streamCancel := context.WithCancel(context.WithoutCancel(ctx))
	httpServer := &http.Server{
		Handler:           s.engine,
		ReadHeaderTimeout: defaultReadHeaderTimeout,
		IdleTimeout:       defaultIdleTimeout,
	}
	httpServer.RegisterOnShutdown(streamCancel)

	actualPort := s.port
	if tcpAddr, ok := listener.Addr().(*net.TCPAddr); ok && tcpAddr.Port > 0 {
		actualPort = tcpAddr.Port
	}

	s.mu.Lock()
	startCanceled := s.generation != generation || s.state != serverStateStarting || startCtx.Err() != nil
	if !startCanceled {
		generation.httpServer = httpServer
		generation.listener = listener
		generation.serveDone = make(chan struct{})
		generation.streamCancel = streamCancel
		s.handlers.setStreamDone(streamCtx.Done())
		s.handlers.setHTTPPort(actualPort)
		s.actualPort = actualPort
	}
	s.mu.Unlock()

	if startCanceled {
		streamCancel()
		closeErr := listener.Close()
		s.completeStart(generation, false)
		if ctxErr := ctx.Err(); ctxErr != nil {
			if closeErr != nil && !errors.Is(closeErr, net.ErrClosed) {
				return errors.Join(ctxErr, fmt.Errorf("httpapi: close interrupted listener: %w", closeErr))
			}
			return ctxErr
		}
		if closeErr != nil && !errors.Is(closeErr, net.ErrClosed) {
			interruptedCloseErr := fmt.Errorf("httpapi: close interrupted listener: %w", closeErr)
			return errors.Join(errServerShutdownInProgress, interruptedCloseErr)
		}
		return errServerShutdownInProgress
	}

	s.completeStart(generation, true)
	s.logger.Info("httpapi: static web assets source selected", "source", s.staticSource)
	go s.serve(generation, address)
	return nil
}
