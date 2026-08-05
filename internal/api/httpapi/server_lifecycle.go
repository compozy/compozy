package httpapi

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
)

type serverState uint8

const (
	serverStateStopped serverState = iota
	serverStateStarting
	serverStateRunning
	serverStateStopping
)

var (
	errServerAlreadyStarted     = errors.New("httpapi: server already started")
	errServerStartInProgress    = errors.New("httpapi: server start in progress")
	errServerShutdownInProgress = errors.New("httpapi: server shutdown in progress")
)

type serverGeneration struct {
	httpServer   *http.Server
	listener     net.Listener
	serveDone    chan struct{}
	streamCancel context.CancelFunc
	startDone    chan struct{}
	startCancel  context.CancelFunc
	serveErr     error
	shutdownDone chan struct{}
	shutdownErr  error
}

func (s *Server) beginStart(ctx context.Context) (*serverGeneration, context.Context, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	switch s.state {
	case serverStateStopped:
		startCtx, startCancel := context.WithCancel(ctx)
		generation := &serverGeneration{
			startDone:   make(chan struct{}),
			startCancel: startCancel,
		}
		s.generation = generation
		s.state = serverStateStarting
		return generation, startCtx, nil
	case serverStateStarting:
		return nil, nil, errServerStartInProgress
	case serverStateRunning:
		return nil, nil, errServerAlreadyStarted
	case serverStateStopping:
		return nil, nil, errServerShutdownInProgress
	default:
		return nil, nil, errors.New("httpapi: unknown server state")
	}
}

func (s *Server) completeStart(generation *serverGeneration, started bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.generation == generation {
		if started {
			s.state = serverStateRunning
		} else {
			s.generation = nil
			s.state = serverStateStopped
			s.actualPort = 0
		}
	}
	close(generation.startDone)
}

func (s *Server) shutdownTarget() (*serverGeneration, <-chan struct{}, context.CancelFunc) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.state == serverStateStopped {
		return nil, nil, nil
	}
	if s.generation == nil {
		return nil, nil, nil
	}
	if s.state == serverStateStarting {
		return s.generation, s.generation.startDone, s.generation.startCancel
	}
	s.state = serverStateStopping
	return s.generation, nil, nil
}

func (s *Server) claimShutdownCleanup(generation *serverGeneration) (bool, <-chan struct{}, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.generation != generation {
		return false, nil, generation.shutdownErr
	}
	if generation.shutdownDone != nil {
		return false, generation.shutdownDone, nil
	}
	generation.shutdownDone = make(chan struct{})
	return true, nil, nil
}

func (s *Server) releaseShutdownCleanup(generation *serverGeneration) {
	s.mu.Lock()
	defer s.mu.Unlock()

	shutdownDone := generation.shutdownDone
	generation.shutdownDone = nil
	if shutdownDone != nil {
		close(shutdownDone)
	}
}

func (s *Server) completeShutdown(generation *serverGeneration) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.generation != generation {
		return generation.shutdownErr
	}
	generation.shutdownErr = generation.serveErr
	s.generation = nil
	s.state = serverStateStopped
	s.actualPort = 0
	shutdownDone := generation.shutdownDone
	generation.shutdownDone = nil
	if shutdownDone != nil {
		close(shutdownDone)
	}
	return generation.shutdownErr
}

func (s *Server) serveError(generation *serverGeneration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return generation.serveErr
}

func (s *Server) serve(generation *serverGeneration, address string) {
	err := generation.httpServer.Serve(generation.listener)
	if err != nil && !errors.Is(err, http.ErrServerClosed) && !errors.Is(err, net.ErrClosed) {
		err = fmt.Errorf("httpapi: serve %q: %w", address, err)
	} else {
		err = nil
	}

	s.mu.Lock()
	if s.generation == generation {
		generation.serveErr = err
		if s.state == serverStateRunning {
			s.state = serverStateStopping
		}
	}
	s.mu.Unlock()
	close(generation.serveDone)
}

func waitForServeDone(ctx context.Context, done <-chan struct{}) error {
	if done == nil {
		return nil
	}

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("httpapi: wait for serve shutdown: %w", ctx.Err())
	}
}

func waitForStartDone(ctx context.Context, done <-chan struct{}) error {
	if done == nil {
		return nil
	}

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("httpapi: wait for server start: %w", ctx.Err())
	}
}

func waitForShutdownCleanup(ctx context.Context, done <-chan struct{}) error {
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("httpapi: wait for generation shutdown cleanup: %w", ctx.Err())
	}
}
