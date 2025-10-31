package embedded

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/compozy/compozy/pkg/logger"
	"go.temporal.io/server/common/log"
	"go.temporal.io/server/temporal"
)

const (
	readyPollInterval = 100 * time.Millisecond
	readyDialTimeout  = 50 * time.Millisecond
)

var (
	errNilContext     = errors.New("context is required")
	errAlreadyStarted = errors.New("embedded temporal server already started")
)

// Server wraps an embedded Temporal server instance.
type Server struct {
	mu           sync.Mutex // protects state fields like started
	opMu         sync.Mutex // serializes start/stop operations
	server       temporal.Server
	config       *Config
	frontendAddr string
	uiServer     *UIServer
	started      bool
}

// NewServer creates but does not start an embedded Temporal server.
// Validates configuration, prepares persistence, and instantiates Temporal services.
func NewServer(ctx context.Context, cfg *Config) (*Server, error) {
	if ctx == nil {
		return nil, errNilContext
	}
	if err := validateConfig(cfg); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}
	applyDefaults(cfg)

	server, frontendAddr, err := buildEmbeddedTemporalServer(ctx, cfg)
	if err != nil {
		return nil, err
	}
	uiSrv := newUIServer(cfg)

	s := &Server{
		server:       server,
		config:       cfg,
		frontendAddr: frontendAddr,
		uiServer:     uiSrv,
	}

	logger.FromContext(ctx).Debug(
		"Embedded Temporal server prepared",
		"frontend_addr", s.frontendAddr,
		"database", cfg.DatabaseFile,
		"cluster", cfg.ClusterName,
	)
	if uiSrv == nil {
		logger.FromContext(ctx).Debug("Temporal UI disabled for embedded server")
	} else {
		logger.FromContext(ctx).Debug("Temporal UI prepared", "ui_addr", uiSrv.address)
	}

	return s, nil
}

func buildEmbeddedTemporalServer(ctx context.Context, cfg *Config) (temporal.Server, string, error) {
	serverConfig, err := buildTemporalConfig(cfg)
	if err != nil {
		return nil, "", fmt.Errorf("build temporal config: %w", err)
	}
	if err := createNamespace(ctx, serverConfig, cfg); err != nil {
		return nil, "", fmt.Errorf("create namespace: %w", err)
	}

	if err := ensurePortsAvailable(ctx, cfg.BindIP, servicePorts(cfg)); err != nil {
		return nil, "", err
	}

	temporalLogger := log.NewZapLogger(log.BuildZapLogger(buildLogConfig(cfg)))
	server, err := temporal.NewServer(
		temporal.WithConfig(serverConfig),
		temporal.ForServices(temporal.DefaultServices),
		temporal.WithStaticHosts(buildStaticHosts(cfg)),
		temporal.WithLogger(temporalLogger),
	)
	if err != nil {
		return nil, "", fmt.Errorf("create temporal server: %w", err)
	}

	return server, net.JoinHostPort(cfg.BindIP, strconv.Itoa(cfg.FrontendPort)), nil
}

// Start boots the embedded Temporal server and waits for readiness.
func (s *Server) Start(ctx context.Context) error {
	if ctx == nil {
		return errNilContext
	}

	log := logger.FromContext(ctx)

	s.opMu.Lock()
	defer s.opMu.Unlock()

	s.mu.Lock()
	if s.started {
		s.mu.Unlock()
		return errAlreadyStarted
	}
	s.started = true
	s.mu.Unlock()

	duration, err := s.startCore(ctx, log)
	if err != nil {
		s.setStarted(false)
		return err
	}

	if err := s.startUIServer(ctx, log); err != nil {
		s.setStarted(false)
		return err
	}

	log.Info(
		"Embedded Temporal server started",
		"frontend_addr", s.frontendAddr,
		"duration", duration,
	)

	return nil
}

// Stop gracefully shuts down the embedded Temporal server.
func (s *Server) Stop(ctx context.Context) error {
	if ctx == nil {
		return errNilContext
	}

	ctx = context.WithoutCancel(ctx)

	s.opMu.Lock()
	defer s.opMu.Unlock()

	s.mu.Lock()
	if !s.started {
		s.mu.Unlock()
		return nil
	}
	s.started = false
	s.mu.Unlock()

	stopStart := time.Now()
	log := logger.FromContext(ctx)
	log.Info("Stopping embedded Temporal server", "frontend_addr", s.frontendAddr)

	if s.uiServer != nil {
		if err := s.uiServer.Stop(ctx); err != nil {
			log.Warn("Failed to stop Temporal UI server", "error", err)
		}
	}

	stopErr := make(chan error, 1)
	go func() {
		stopErr <- s.server.Stop()
	}()

	select {
	case err := <-stopErr:
		if err != nil {
			return fmt.Errorf("stop temporal server: %w", err)
		}
	case <-ctx.Done():
		log.Warn("Temporal server stop exceeded context deadline", "elapsed", time.Since(stopStart), "error", ctx.Err())
		return fmt.Errorf("temporal server stop timeout: %w", ctx.Err())
	}

	log.Info(
		"Embedded Temporal server stopped",
		"frontend_addr", s.frontendAddr,
		"duration", time.Since(stopStart),
	)

	return nil
}

// FrontendAddress returns the gRPC address for the Temporal frontend service.
func (s *Server) FrontendAddress() string {
	return s.frontendAddr
}

// waitForReady polls the frontend service until ready or the context ends.
func (s *Server) waitForReady(ctx context.Context) error {
	if ctx == nil {
		return errNilContext
	}
	dialer := &net.Dialer{Timeout: readyDialTimeout}
	ticker := time.NewTicker(readyPollInterval)
	defer ticker.Stop()

	host, port, err := net.SplitHostPort(s.frontendAddr)
	if err != nil {
		return fmt.Errorf("parse frontend address %q: %w", s.frontendAddr, err)
	}
	target := net.JoinHostPort(dialHost(host), port)

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("temporal frontend %s not ready before deadline: %w", target, ctx.Err())
		case <-ticker.C:
			conn, err := dialer.DialContext(ctx, "tcp", target)
			if err == nil {
				_ = conn.Close()
				return nil
			}
		}
	}
}

func ensurePortsAvailable(ctx context.Context, bindIP string, ports []int) error {
	dialer := &net.Dialer{Timeout: readyDialTimeout}
	for _, port := range ports {
		addr := net.JoinHostPort(dialHost(bindIP), strconv.Itoa(port))
		conn, err := dialer.DialContext(ctx, "tcp", addr)
		if err == nil {
			_ = conn.Close()
			return fmt.Errorf(
				"embedded temporal port %d is already in use on %s; adjust configuration or stop the conflicting service",
				port,
				bindIP,
			)
		}
		if !isConnRefused(err) {
			return fmt.Errorf("verify port %d on %s: %w", port, bindIP, err)
		}
	}
	return nil
}

func isConnRefused(err error) bool {
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		if errors.Is(opErr.Err, syscall.ECONNREFUSED) {
			return true
		}
		var sysErr *os.SyscallError
		if errors.As(opErr.Err, &sysErr) {
			return errors.Is(sysErr.Err, syscall.ECONNREFUSED)
		}
	}
	return false
}

func servicePorts(cfg *Config) []int {
	ports := []int{
		cfg.FrontendPort,
		cfg.FrontendPort + 1,
		cfg.FrontendPort + 2,
		cfg.FrontendPort + 3,
	}
	if cfg.EnableUI {
		ports = append(ports, cfg.UIPort)
	}
	return ports
}

func dialHost(bindIP string) string {
	switch bindIP {
	case "", "0.0.0.0":
		return "127.0.0.1"
	case "::", "[::]":
		return "::1"
	default:
		return bindIP
	}
}

func (s *Server) setStarted(started bool) {
	s.mu.Lock()
	s.started = started
	s.mu.Unlock()
}

func (s *Server) startCore(ctx context.Context, log logger.Logger) (time.Duration, error) {
	startCtx, cancel := context.WithTimeout(ctx, s.config.StartTimeout)
	defer cancel()

	startTime := time.Now()
	if err := s.server.Start(); err != nil {
		return 0, fmt.Errorf("start temporal server: %w", err)
	}

	if err := s.waitForReady(startCtx); err != nil {
		stopErr := s.server.Stop()
		if stopErr != nil {
			log.Error("Failed to stop Temporal server after startup error", "error", stopErr)
		}
		return 0, fmt.Errorf("wait for ready: %w", err)
	}

	return time.Since(startTime), nil
}

func (s *Server) startUIServer(ctx context.Context, log logger.Logger) error {
	if s.uiServer == nil {
		return nil
	}

	if err := s.uiServer.Start(ctx); err != nil {
		if s.config.RequireUI {
			stopErr := s.server.Stop()
			if stopErr != nil {
				log.Error("Failed to stop Temporal server after UI startup error", "error", stopErr)
			}
			return fmt.Errorf("start temporal ui server: %w", err)
		}
		log.Warn("Failed to start Temporal UI server", "error", err)
	}

	return nil
}
