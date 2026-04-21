package httpapi

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/compozy/compozy/internal/api/core"
)

const (
	defaultHost              = "127.0.0.1"
	defaultReadHeaderTimeout = 5 * time.Second
	defaultIdleTimeout       = 60 * time.Second
)

// PortUpdater persists the selected HTTP port in daemon state.
type PortUpdater interface {
	SetHTTPPort(context.Context, int) error
}

// Option customizes HTTP server construction.
type Option func(*Server)

// Server exposes the daemon API over localhost HTTP.
type Server struct {
	mu sync.Mutex

	host        string
	port        int
	logger      *slog.Logger
	handlers    *core.Handlers
	engine      *gin.Engine
	portUpdater PortUpdater
	staticFS    fs.FS
	devProxy    *devProxyHandler
	devProxyURL string

	httpServer   *http.Server
	listener     net.Listener
	serveDone    chan struct{}
	serveErr     error
	streamCancel context.CancelFunc
	started      bool
	actualPort   int
}

// WithHandlers injects the shared transport handlers.
func WithHandlers(handlers *core.Handlers) Option {
	return func(server *Server) {
		server.handlers = handlers
	}
}

// WithHost overrides the HTTP bind host.
func WithHost(host string) Option {
	return func(server *Server) {
		server.host = strings.TrimSpace(host)
	}
}

// WithPort overrides the HTTP bind port.
func WithPort(port int) Option {
	return func(server *Server) {
		server.port = port
	}
}

// WithLogger injects the server logger.
func WithLogger(logger *slog.Logger) Option {
	return func(server *Server) {
		server.logger = logger
	}
}

// WithEngine overrides the Gin engine used by the server.
func WithEngine(engine *gin.Engine) Option {
	return func(server *Server) {
		server.engine = engine
	}
}

// WithPortUpdater persists the selected port after the listener binds.
func WithPortUpdater(updater PortUpdater) Option {
	return func(server *Server) {
		server.portUpdater = updater
	}
}

// WithDevProxyTarget enables development fallback proxying to a Vite server.
func WithDevProxyTarget(target string) Option {
	return func(server *Server) {
		server.devProxyURL = strings.TrimSpace(target)
	}
}

// New constructs a localhost HTTP API server.
func New(opts ...Option) (*Server, error) {
	server := &Server{
		host:   defaultHost,
		port:   0,
		logger: slog.Default(),
	}
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
		return errors.New("httpapi: handlers are required")
	}
	s.handlers = s.handlers.Clone()
	if strings.TrimSpace(s.host) == "" {
		s.host = defaultHost
	}
	if s.host != defaultHost {
		return fmt.Errorf("httpapi: host must be %s", defaultHost)
	}
	if s.port < 0 || s.port > 65535 {
		return fmt.Errorf("httpapi: invalid port %d", s.port)
	}
	if s.devProxyURL != "" {
		devProxy, err := newDevProxyHandler(s.devProxyURL)
		if err != nil {
			return fmt.Errorf("httpapi: configure development frontend proxy: %w", err)
		}
		s.devProxy = devProxy
	} else {
		staticFS, err := newStaticFS()
		if err != nil {
			return fmt.Errorf("httpapi: load embedded frontend bundle: %w", err)
		}
		s.staticFS = staticFS
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
	s.engine.Use(s.hostValidationMiddleware())
	s.engine.Use(s.originValidationMiddleware())
	s.engine.Use(s.activeWorkspaceMiddleware())
	s.engine.Use(s.csrfMiddleware())
	if s.devProxy != nil {
		RegisterRoutes(s.engine, s.handlers, s.devProxy.serve)
		return
	}
	staticHandler := newStaticHandler(s.staticFS, s.handlers.Now())
	if staticHandler != nil {
		RegisterRoutes(s.engine, s.handlers, staticHandler.serve)
		return
	}
	RegisterRoutes(s.engine, s.handlers)
}

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

func (s *Server) reserveStart() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.started {
		return errors.New("httpapi: server already started")
	}
	s.started = true
	return nil
}

func (s *Server) rollbackStart() {
	s.mu.Lock()
	s.httpServer = nil
	s.listener = nil
	s.serveDone = nil
	s.serveErr = nil
	s.streamCancel = nil
	s.started = false
	s.actualPort = 0
	s.mu.Unlock()
}

func (s *Server) publishStartState(
	streamDone <-chan struct{},
	httpServer *http.Server,
	ln net.Listener,
	serveDone chan struct{},
	streamCancel context.CancelFunc,
	actualPort int,
) {
	s.mu.Lock()
	s.handlers.SetStreamDone(streamDone)
	s.handlers.SetHTTPPort(actualPort)
	s.httpServer = httpServer
	s.listener = ln
	s.serveDone = serveDone
	s.serveErr = nil
	s.streamCancel = streamCancel
	s.actualPort = actualPort
	s.mu.Unlock()
}

// Start begins serving the API over localhost TCP.
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

	if err := s.reserveStart(); err != nil {
		return err
	}

	address := net.JoinHostPort(s.host, strconv.Itoa(s.port))
	var listenConfig net.ListenConfig
	ln, err := listenConfig.Listen(ctx, "tcp", address)
	if err != nil {
		s.rollbackStart()
		return fmt.Errorf("httpapi: listen on %q: %w", address, err)
	}

	actualPort := s.port
	if tcpAddr, ok := ln.Addr().(*net.TCPAddr); ok && tcpAddr.Port > 0 {
		actualPort = tcpAddr.Port
	}
	if s.portUpdater != nil {
		if err := s.portUpdater.SetHTTPPort(ctx, actualPort); err != nil {
			_ = ln.Close()
			s.rollbackStart()
			return fmt.Errorf("httpapi: persist http port: %w", err)
		}
	}

	streamCtx, streamCancel := context.WithCancel(context.Background())
	httpServer := &http.Server{
		Handler:           s.engine,
		ReadHeaderTimeout: defaultReadHeaderTimeout,
		IdleTimeout:       defaultIdleTimeout,
	}
	serveDone := make(chan struct{})

	s.publishStartState(streamCtx.Done(), httpServer, ln, serveDone, streamCancel, actualPort)

	go func() {
		defer close(serveDone)
		if err := httpServer.Serve(ln); err != nil &&
			!errors.Is(err, http.ErrServerClosed) &&
			!errors.Is(err, net.ErrClosed) {
			s.mu.Lock()
			s.serveErr = fmt.Errorf("httpapi: serve %q: %w", address, err)
			s.mu.Unlock()
		}
	}()

	return nil
}

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
	if streamCancel != nil {
		streamCancel()
	}
	if httpServer != nil {
		if err := httpServer.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("httpapi: shutdown http server: %w", err))
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
