package udsapi

import (
	"context"
	"errors"
	"fmt"

	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	mcppkg "github.com/compozy/agh/internal/mcp"
)

// Path reports the served Unix domain socket path.
func (s *Server) Path() string {
	if s == nil {
		return ""
	}
	return s.socketPath
}

// Start begins serving the API over the configured Unix domain socket.
func (s *Server) Start(ctx context.Context) error {
	if s == nil {
		return errors.New("udsapi: server is required")
	}
	if ctx == nil {
		return errors.New("udsapi: start context is required")
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	socketPath := strings.TrimSpace(s.socketPath)
	if socketPath == "" {
		return errors.New("udsapi: socket path is required")
	}

	s.mu.Lock()
	if s.state != serverStateStopped {
		err := errors.New("udsapi: server already started")
		if s.state == serverStateStopping {
			err = errors.New("udsapi: server shutdown in progress")
		}
		s.mu.Unlock()
		return err
	}
	if err := ensureSocketParentDir(socketPath); err != nil {
		s.mu.Unlock()
		return err
	}
	if err := removeSocketPath(socketPath); err != nil {
		s.mu.Unlock()
		return err
	}

	var listenConfig net.ListenConfig
	ln, err := listenConfig.Listen(ctx, "unix", socketPath)
	if err != nil {
		s.mu.Unlock()
		return fmt.Errorf("udsapi: listen on %q: %w", socketPath, err)
	}
	if err := os.Chmod(socketPath, 0o600); err != nil {
		cleanupErr := cleanupSocketStartFailure(ln, socketPath)
		s.mu.Unlock()
		return errors.Join(fmt.Errorf("udsapi: chmod socket %q: %w", socketPath, err), cleanupErr)
	}

	streamCtx, streamCancel := context.WithCancel(context.WithoutCancel(ctx))
	httpServer := &http.Server{
		Handler:           s.engine,
		ReadHeaderTimeout: defaultReadHeaderTimeout,
		IdleTimeout:       defaultIdleTimeout,
		ConnContext: func(ctx context.Context, conn net.Conn) context.Context {
			peer, err := mcppkg.PeerInfoFromConn(conn)
			return mcppkg.ContextWithPeerInfo(ctx, peer, err)
		},
	}
	httpServer.RegisterOnShutdown(streamCancel)
	serveDone := make(chan struct{})

	s.handlers.setStreamDone(streamCtx.Done())
	s.httpServer = httpServer
	s.listener = ln
	s.serveDone = serveDone
	s.serveErr = nil
	s.streamCancel = streamCancel
	s.state = serverStateRunning
	s.mu.Unlock()

	go func() {
		defer close(serveDone)
		if err := httpServer.Serve(
			ln,
		); err != nil && !errors.Is(err, http.ErrServerClosed) &&
			!errors.Is(err, net.ErrClosed) {
			s.mu.Lock()
			s.serveErr = fmt.Errorf("udsapi: serve socket %q: %w", socketPath, err)
			s.mu.Unlock()
		}
	}()

	return nil
}

func cleanupSocketStartFailure(ln net.Listener, socketPath string) error {
	var errs []error
	if ln != nil {
		if err := ln.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			errs = append(errs, fmt.Errorf("udsapi: close startup listener: %w", err))
		}
	}
	if err := removeSocketPath(socketPath); err != nil {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

func ensureSocketParentDir(path string) error {
	cleanPath := strings.TrimSpace(path)
	if cleanPath == "" {
		return errors.New("udsapi: socket path is required")
	}
	if err := os.MkdirAll(filepath.Dir(cleanPath), 0o755); err != nil {
		return fmt.Errorf("udsapi: create socket directory for %q: %w", cleanPath, err)
	}
	return nil
}

func removeSocketPath(path string) error {
	cleanPath := strings.TrimSpace(path)
	if cleanPath == "" {
		return nil
	}

	info, err := os.Lstat(cleanPath)
	switch {
	case errors.Is(err, os.ErrNotExist):
		return nil
	case err != nil:
		return fmt.Errorf("udsapi: stat socket %q: %w", cleanPath, err)
	case info.Mode()&os.ModeSocket == 0:
		return fmt.Errorf("udsapi: existing path %q is not a unix socket", cleanPath)
	}

	if err := os.Remove(cleanPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("udsapi: remove socket %q: %w", cleanPath, err)
	}
	return nil
}
