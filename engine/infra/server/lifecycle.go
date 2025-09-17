package server

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
)

func (s *Server) Run() error {
	state, cleanupFuncs, err := s.setupDependencies()
	if err != nil {
		s.cleanup(cleanupFuncs)
		return err
	}
	if err := s.buildRouter(state); err != nil {
		s.cleanup(cleanupFuncs)
		return fmt.Errorf("failed to build router: %w", err)
	}
	return s.startAndRunServer(cleanupFuncs)
}

func (s *Server) startAndRunServer(cleanupFuncs []func()) error {
	srv := s.createHTTPServer()
	s.httpServer = srv
	errChan := make(chan error, 1)
	go s.startServer(srv, errChan)
	select {
	case err := <-errChan:
		if err != nil {
			s.cleanup(cleanupFuncs)
			return err
		}
	case <-time.After(config.FromContext(s.ctx).Server.Timeouts.StartProbeDelay):
		s.logStartupBanner()
	}
	return s.handleGracefulShutdown(srv, cleanupFuncs, errChan)
}

func (s *Server) createHTTPServer() *http.Server {
	cfg := config.FromContext(s.ctx)
	host := s.serverConfig.Host
	port := s.serverConfig.Port
	if cfg != nil {
		host = cfg.Server.Host
		port = cfg.Server.Port
	}
	addr := fmt.Sprintf("%s:%d", host, port)
	log := logger.FromContext(s.ctx)
	log.Info("Starting HTTP server", "address", fmt.Sprintf("http://%s", addr))
	return &http.Server{
		Addr:         addr,
		Handler:      s.router,
		BaseContext:  func(net.Listener) context.Context { return s.ctx },
		ReadTimeout:  cfg.Server.Timeouts.HTTPRead,
		WriteTimeout: cfg.Server.Timeouts.HTTPWrite,
		IdleTimeout:  cfg.Server.Timeouts.HTTPIdle,
	}
}

func (s *Server) startServer(srv *http.Server, errChan chan<- error) {
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log := logger.FromContext(s.ctx)
		log.Error("HTTP server failed", "error", err)
		errChan <- fmt.Errorf("HTTP server failed: %w", err)
		return
	}
}

func (s *Server) handleGracefulShutdown(srv *http.Server, cleanupFuncs []func(), errChan <-chan error) error {
	log := logger.FromContext(s.ctx)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(quit)
	select {
	case <-quit:
		log.Debug("Received shutdown signal, initiating graceful shutdown")
	case <-s.shutdownChan:
		log.Debug("Received programmatic shutdown signal, initiating graceful shutdown")
	case err := <-errChan:
		if err != nil {
			log.Error("Server reported failure, shutting down", "error", err)
			s.cleanup(cleanupFuncs)
			s.cancel()
			return err
		}
		log.Debug("HTTP server closed, proceeding with shutdown")
	}
	s.cleanup(cleanupFuncs)
	s.cancel()
	shutdownCtx, shutdownCancel := context.WithTimeout(
		context.WithoutCancel(s.ctx),
		config.FromContext(s.ctx).Server.Timeouts.ServerShutdown,
	)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("server shutdown failed: %w", err)
	}
	log.Info("Server shutdown completed successfully")
	return nil
}

func (s *Server) Shutdown() {
	s.shutdownOnce.Do(func() {
		select {
		case s.shutdownChan <- struct{}{}:
		default:
		}
	})
}

func (s *Server) RegisterCleanup(fn func()) {
	if fn == nil {
		return
	}
	s.cleanupMu.Lock()
	s.extraCleanups = append(s.extraCleanups, fn)
	s.cleanupMu.Unlock()
}
