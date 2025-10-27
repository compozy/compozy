package embedded

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/compozy/compozy/pkg/logger"
	uiserver "github.com/temporalio/ui-server/v2/server"
	uiconfig "github.com/temporalio/ui-server/v2/server/config"
	"github.com/temporalio/ui-server/v2/server/server_options"
)

// UIServer manages the Temporal Web UI lifecycle for the embedded server.
type UIServer struct {
	mu           sync.Mutex
	server       *uiserver.Server
	config       *Config
	address      string
	temporalAddr string
	runErrCh     chan error
	started      bool
}

// newUIServer constructs a UIServer when UI support is enabled.
func newUIServer(cfg *Config) *UIServer {
	if cfg == nil || !cfg.EnableUI || cfg.UIPort <= 0 {
		return nil
	}

	uiCfg := &uiconfig.Config{
		TemporalGRPCAddress: fmt.Sprintf("%s:%d", cfg.BindIP, cfg.FrontendPort),
		Host:                cfg.BindIP,
		Port:                cfg.UIPort,
		EnableUI:            true,
		DefaultNamespace:    cfg.Namespace,
		HideLogs:            true,
	}

	srv := uiserver.NewServer(server_options.WithConfigProvider(uiCfg))

	return &UIServer{
		server:       srv,
		config:       cfg,
		address:      net.JoinHostPort(cfg.BindIP, strconv.Itoa(cfg.UIPort)),
		temporalAddr: uiCfg.TemporalGRPCAddress,
	}
}

// Start launches the Temporal Web UI and waits until it becomes reachable.
func (s *UIServer) Start(ctx context.Context) error {
	if ctx == nil {
		return errNilContext
	}

	s.mu.Lock()
	if s.started {
		s.mu.Unlock()
		return errAlreadyStarted
	}
	if s.runErrCh != nil {
		s.mu.Unlock()
		return fmt.Errorf("temporal ui server is already running")
	}
	s.mu.Unlock()

	if err := ensureUIPortAvailable(ctx, s.config.BindIP, s.config.UIPort); err != nil {
		return err
	}

	log := logger.FromContext(ctx)
	log.Info(
		"Starting Temporal UI server",
		"address", s.address,
		"frontend_addr", s.temporalAddr,
	)

	runErrCh := make(chan error, 1)
	go func(ch chan<- error) {
		if err := s.server.Start(); err != nil {
			ch <- fmt.Errorf("ui server exited: %w", err)
		}
		close(ch)
	}(runErrCh)

	if err := waitForHTTPReady(ctx, s.address, runErrCh); err != nil {
		s.server.Stop()
		return fmt.Errorf("wait for ui ready: %w", err)
	}

	s.mu.Lock()
	s.started = true
	s.runErrCh = runErrCh
	s.mu.Unlock()

	log.Info("Temporal UI server started", "address", s.address)
	return nil
}

// Stop gracefully shuts down the Temporal Web UI server.
func (s *UIServer) Stop(ctx context.Context) error {
	if ctx == nil {
		return errNilContext
	}

	s.mu.Lock()
	if !s.started {
		s.mu.Unlock()
		return nil
	}
	runErrCh := s.runErrCh
	s.runErrCh = nil
	s.started = false
	s.mu.Unlock()

	logger.FromContext(ctx).Info("Stopping Temporal UI server", "address", s.address)

	done := make(chan struct{})
	go func() {
		s.server.Stop()
		close(done)
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-done:
		if runErrCh != nil {
			<-runErrCh
		}
		logger.FromContext(ctx).Info("Temporal UI server stopped", "address", s.address)
		return nil
	}
}

func ensureUIPortAvailable(ctx context.Context, bindIP string, port int) error {
	dialer := &net.Dialer{Timeout: readyDialTimeout}
	address := net.JoinHostPort(bindIP, strconv.Itoa(port))

	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err == nil {
		_ = conn.Close()
		return fmt.Errorf(
			"temporal ui port %d is already in use on %s; adjust configuration or stop the conflicting service",
			port,
			bindIP,
		)
	}
	if !isConnRefused(err) {
		return fmt.Errorf("verify temporal ui port %d on %s: %w", port, bindIP, err)
	}
	return nil
}

func waitForHTTPReady(ctx context.Context, address string, runErrCh <-chan error) error {
	dialer := &net.Dialer{Timeout: readyDialTimeout}
	ticker := time.NewTicker(readyPollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err, ok := <-runErrCh:
			if ok && err != nil {
				return err
			}
			if !ok {
				return fmt.Errorf("ui server stopped before accepting connections")
			}
		case <-ticker.C:
			conn, err := dialer.DialContext(ctx, "tcp", address)
			if err == nil {
				_ = conn.Close()
				return nil
			}
			if !isConnRefused(err) && err != nil {
				return fmt.Errorf("dial temporal ui address %s: %w", address, err)
			}
		}
	}
}
