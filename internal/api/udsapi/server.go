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
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/compozy/compozy/internal/api/core"
	compozyconfig "github.com/compozy/compozy/internal/config"
)

const (
	defaultReadHeaderTimeout = 5 * time.Second
	defaultIdleTimeout       = 60 * time.Second
)

// Option customizes UDS server construction.
type Option func(*Server)

// Server exposes the daemon API over a Unix domain socket.
type Server struct {
	mu sync.Mutex

	socketPath string
	handlers   *core.Handlers
	engine     *gin.Engine

	httpServer   *http.Server
	listener     net.Listener
	serveDone    chan struct{}
	serveErr     error
	streamCancel context.CancelFunc
	started      bool
}

// WithHandlers injects the shared transport handlers.
func WithHandlers(handlers *core.Handlers) Option {
	return func(server *Server) {
		server.handlers = handlers
	}
}

// WithSocketPath overrides the Unix socket path.
func WithSocketPath(path string) Option {
	return func(server *Server) {
		server.socketPath = strings.TrimSpace(path)
	}
}

// WithEngine overrides the Gin engine used by the server.
func WithEngine(engine *gin.Engine) Option {
	return func(server *Server) {
		server.engine = engine
	}
}

// New constructs a UDS API server.
func New(opts ...Option) (*Server, error) {
	paths, err := compozyconfig.ResolveHomePaths()
	if err != nil {
		return nil, fmt.Errorf("udsapi: resolve home paths: %w", err)
	}

	server := &Server{socketPath: paths.SocketPath}
	for _, opt := range opts {
		if opt != nil {
			opt(server)
		}
	}
	if err := server.finalize(); err != nil {
		return nil, err
	}
	return server, nil
}

func (s *Server) finalize() error {
	if s.handlers == nil {
		return errors.New("udsapi: handlers are required")
	}
	if strings.TrimSpace(s.socketPath) == "" {
		return errors.New("udsapi: socket path is required")
	}
	s.ensureEngine()
	return nil
}

func (s *Server) ensureEngine() {
	if s.engine != nil {
		return
	}

	s.engine = gin.New()
	s.engine.Use(core.RequestIDMiddleware())
	s.engine.Use(gin.CustomRecovery(func(c *gin.Context, recovered any) {
		core.RespondError(
			c,
			core.NewProblem(
				http.StatusInternalServerError,
				"internal_error",
				http.StatusText(http.StatusInternalServerError),
				nil,
				fmt.Errorf("panic: %v", recovered),
			),
		)
	}))
	s.engine.Use(core.ErrorMiddleware())
	RegisterRoutes(s.engine, s.handlers)
}

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

	s.mu.Lock()
	if s.started {
		s.mu.Unlock()
		return errors.New("udsapi: server already started")
	}
	s.mu.Unlock()

	socketPath := strings.TrimSpace(s.socketPath)
	if err := ensureSocketParentDir(socketPath); err != nil {
		return err
	}
	if err := removeSocketPath(socketPath); err != nil {
		return err
	}

	var listenConfig net.ListenConfig
	ln, err := listenConfig.Listen(ctx, "unix", socketPath)
	if err != nil {
		return fmt.Errorf("udsapi: listen on %q: %w", socketPath, err)
	}
	if err := os.Chmod(socketPath, 0o600); err != nil {
		_ = ln.Close()
		_ = os.Remove(socketPath)
		return fmt.Errorf("udsapi: chmod socket %q: %w", socketPath, err)
	}

	streamCtx, streamCancel := context.WithCancel(context.Background())
	httpServer := &http.Server{
		Handler:           s.engine,
		ReadHeaderTimeout: defaultReadHeaderTimeout,
		IdleTimeout:       defaultIdleTimeout,
	}
	serveDone := make(chan struct{})

	s.mu.Lock()
	s.handlers.SetStreamDone(streamCtx.Done())
	s.httpServer = httpServer
	s.listener = ln
	s.serveDone = serveDone
	s.serveErr = nil
	s.streamCancel = streamCancel
	s.started = true
	s.mu.Unlock()

	go func() {
		defer close(serveDone)
		if err := httpServer.Serve(ln); err != nil &&
			!errors.Is(err, http.ErrServerClosed) &&
			!errors.Is(err, net.ErrClosed) {
			s.mu.Lock()
			s.serveErr = fmt.Errorf("udsapi: serve %q: %w", socketPath, err)
			s.mu.Unlock()
		}
	}()

	return nil
}

// Shutdown stops accepting new requests, drains active ones, and removes the socket file.
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
	socketPath := s.socketPath
	serveErr := s.serveErr
	s.httpServer = nil
	s.listener = nil
	s.serveDone = nil
	s.streamCancel = nil
	s.serveErr = nil
	s.started = false
	s.mu.Unlock()

	var errs []error
	if streamCancel != nil {
		streamCancel()
	}
	if httpServer != nil {
		if err := httpServer.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("udsapi: shutdown http server: %w", err))
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
	if serveErr != nil {
		errs = append(errs, serveErr)
	}
	return errors.Join(errs...)
}

func waitForServeDone(ctx context.Context, done <-chan struct{}) error {
	if done == nil {
		return nil
	}
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("udsapi: wait for serve shutdown: %w", ctx.Err())
	}
}

func ensureSocketParentDir(path string) error {
	cleanPath := strings.TrimSpace(path)
	if cleanPath == "" {
		return errors.New("udsapi: socket path is required")
	}
	if err := os.MkdirAll(filepath.Dir(cleanPath), 0o700); err != nil {
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
