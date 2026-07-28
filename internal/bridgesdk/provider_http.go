package bridgesdk

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// ProviderHTTPConfig configures one shared provider webhook server.
type ProviderHTTPConfig struct {
	ReadHeaderTimeout time.Duration
	IdleTimeout       time.Duration
	Handler           http.Handler
	Go                func(func()) bool
	OnError           func(error)
}

// ProviderHTTPServer owns one provider listener/server lifecycle.
type ProviderHTTPServer struct {
	config ProviderHTTPConfig

	mu       sync.Mutex
	server   *http.Server
	listener net.Listener
	listenAt string
	addr     string
}

// NewProviderHTTPServer creates one provider webhook server owner.
func NewProviderHTTPServer(config ProviderHTTPConfig) (*ProviderHTTPServer, error) {
	if config.Handler == nil {
		return nil, errors.New("bridgesdk: provider http handler is required")
	}
	if config.Go == nil {
		return nil, errors.New("bridgesdk: provider http goroutine owner is required")
	}
	return &ProviderHTTPServer{config: config}, nil
}

// Start listens once at the requested address and serves the provider handler.
func (s *ProviderHTTPServer) Start(listenAddr string) error {
	if s == nil {
		return errors.New("bridgesdk: provider http server is required")
	}
	listenAddr = strings.TrimSpace(listenAddr)
	if listenAddr == "" {
		return errors.New("bridgesdk: provider http listen address is required")
	}
	s.mu.Lock()
	if s.server != nil {
		currentAddr := s.listenAt
		s.mu.Unlock()
		if currentAddr == listenAddr {
			return nil
		}
		return fmt.Errorf("bridgesdk: provider http server already listens on %q", currentAddr)
	}
	var listenConfig net.ListenConfig
	listener, err := listenConfig.Listen(context.Background(), "tcp", listenAddr)
	if err != nil {
		s.mu.Unlock()
		return fmt.Errorf("bridgesdk: listen for provider webhook: %w", err)
	}
	server := &http.Server{
		Handler:           s.config.Handler,
		ReadHeaderTimeout: s.config.ReadHeaderTimeout,
		IdleTimeout:       s.config.IdleTimeout,
	}
	s.listener = listener
	s.server = server
	s.listenAt = listenAddr
	s.addr = listener.Addr().String()
	s.mu.Unlock()

	if !s.config.Go(func() {
		if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) &&
			!errors.Is(err, net.ErrClosed) && s.config.OnError != nil {
			s.config.OnError(err)
		}
	}) {
		return errors.Join(ErrProviderStopped, s.Shutdown(context.Background()))
	}
	return nil
}

// Address returns the configured listen address while the server is active.
func (s *ProviderHTTPServer) Address() string {
	if s == nil {
		return ""
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.addr
}

// Shutdown detaches and closes the provider listener/server.
func (s *ProviderHTTPServer) Shutdown(ctx context.Context) error {
	if s == nil {
		return nil
	}
	if ctx == nil {
		return errors.New("bridgesdk: provider http shutdown context is required")
	}
	s.mu.Lock()
	server := s.server
	listener := s.listener
	s.server = nil
	s.listener = nil
	s.listenAt = ""
	s.addr = ""
	s.mu.Unlock()

	var shutdownErr error
	if server != nil {
		if err := server.Shutdown(ctx); err != nil && !errors.Is(err, http.ErrServerClosed) {
			shutdownErr = errors.Join(shutdownErr, err)
		}
		if err := server.Close(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			shutdownErr = errors.Join(shutdownErr, err)
		}
	}
	if listener != nil {
		if err := listener.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			shutdownErr = errors.Join(shutdownErr, err)
		}
	}
	if shutdownErr != nil {
		return fmt.Errorf("bridgesdk: shutdown provider http server: %w", shutdownErr)
	}
	return nil
}
