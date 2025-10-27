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
	mu           sync.Mutex
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

	return server, fmt.Sprintf("%s:%d", cfg.BindIP, cfg.FrontendPort), nil
}

// Start boots the embedded Temporal server and waits for readiness.
func (s *Server) Start(ctx context.Context) error {
	if ctx == nil {
		return errNilContext
	}

	s.mu.Lock()
	if s.started {
		s.mu.Unlock()
		return errAlreadyStarted
	}
	s.mu.Unlock()

	startCtx, cancel := context.WithTimeout(ctx, s.config.StartTimeout)
	defer cancel()

	startTime := time.Now()
	if err := s.server.Start(); err != nil {
		return fmt.Errorf("start temporal server: %w", err)
	}

	if err := s.waitForReady(startCtx); err != nil {
		stopErr := s.server.Stop()
		if stopErr != nil {
			logger.FromContext(ctx).Error("Failed to stop Temporal server after startup error", "error", stopErr)
		}
		return fmt.Errorf("wait for ready: %w", err)
	}

	s.mu.Lock()
	s.started = true
	s.mu.Unlock()

	logger.FromContext(ctx).Info(
		"Embedded Temporal server started",
		"frontend_addr", s.frontendAddr,
		"duration", time.Since(startTime),
	)

	if s.uiServer != nil {
		if err := s.uiServer.Start(ctx); err != nil {
			logger.FromContext(ctx).Warn("Failed to start Temporal UI server", "error", err)
		}
	}

	return nil
}

// Stop gracefully shuts down the embedded Temporal server.
func (s *Server) Stop(ctx context.Context) error {
	if ctx == nil {
		return errNilContext
	}

	s.mu.Lock()
	if !s.started {
		s.mu.Unlock()
		return nil
	}
	s.started = false
	s.mu.Unlock()

	stopStart := time.Now()
	logger.FromContext(ctx).Info("Stopping embedded Temporal server", "frontend_addr", s.frontendAddr)

	if s.uiServer != nil {
		if err := s.uiServer.Stop(ctx); err != nil {
			logger.FromContext(ctx).Warn("Failed to stop Temporal UI server", "error", err)
		}
	}

	if err := s.server.Stop(); err != nil {
		return fmt.Errorf("stop temporal server: %w", err)
	}

	logger.FromContext(ctx).Info(
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

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			conn, err := dialer.DialContext(ctx, "tcp", s.frontendAddr)
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
		addr := net.JoinHostPort(bindIP, strconv.Itoa(port))
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
		if opErr.Err == syscall.ECONNREFUSED {
			return true
		}
		var sysErr *os.SyscallError
		if errors.As(opErr.Err, &sysErr) {
			return sysErr.Err == syscall.ECONNREFUSED
		}
	}
	return false
}

func servicePorts(cfg *Config) []int {
	return []int{
		cfg.FrontendPort,
		cfg.FrontendPort + 1,
		cfg.FrontendPort + 2,
		cfg.FrontendPort + 3,
	}
}
