package compozy

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync"

	"github.com/go-chi/chi/v5"

	"github.com/compozy/compozy/engine/resources"
	appconfig "github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
	sdkclient "github.com/compozy/compozy/sdk/v2/client"
)

// Start boots the engine lifecycle by initialising the resource store, HTTP server, and SDK client.
func (e *Engine) Start(ctx context.Context) error {
	if ctx == nil {
		return fmt.Errorf("context is required")
	}
	e.startMu.Lock()
	defer e.startMu.Unlock()
	if e.IsStarted() {
		return ErrAlreadyStarted
	}
	cfg := appconfig.FromContext(ctx)
	if cfg == nil {
		return ErrConfigUnavailable
	}
	log := logger.FromContext(ctx)
	store, err := e.buildResourceStore(ctx)
	if err != nil {
		e.recordStartError(err)
		return err
	}
	router := chi.NewRouter()
	listenHost, listenPort := e.resolveListenAddress(cfg)
	listener, actualPort, err := e.listen(listenHost, listenPort)
	if err != nil {
		e.cleanupStore(ctx, store)
		e.recordStartError(err)
		return err
	}
	serverCtx, cancel := context.WithCancel(ctx)
	server := e.newHTTPServer(serverCtx, router, cfg, listener.Addr().String())
	client, baseURL, err := e.newClient(ctx, listenHost, actualPort)
	if err != nil {
		cancel()
		_ = listener.Close()
		e.cleanupStore(ctx, store)
		e.recordStartError(err)
		return err
	}
	e.serverWG = sync.WaitGroup{}
	e.launchServer(log, server, listener)
	e.stateMu.Lock()
	e.resourceStore = store
	e.router = router
	e.server = server
	e.listener = listener
	e.client = client
	e.configSnapshot = cfg
	e.serverCancel = cancel
	e.started = true
	e.baseURL = baseURL
	e.port = actualPort
	e.stopErr = nil
	e.stateMu.Unlock()
	e.errMu.Lock()
	e.startErr = nil
	e.errMu.Unlock()
	log.Info("engine started", "mode", string(e.mode), "base_url", baseURL)
	return nil
}

// Stop gracefully shuts down the engine and all managed resources.
func (e *Engine) Stop(ctx context.Context) error {
	if ctx == nil {
		return fmt.Errorf("context is required")
	}
	e.stopMu.Lock()
	defer e.stopMu.Unlock()
	if !e.IsStarted() && e.server == nil && e.resourceStore == nil {
		return e.stopErr
	}
	log := logger.FromContext(ctx)
	e.stateMu.Lock()
	server := e.server
	listener := e.listener
	store := e.resourceStore
	cancel := e.serverCancel
	e.router = nil
	cfg := e.configSnapshot
	e.server = nil
	e.listener = nil
	e.resourceStore = nil
	e.serverCancel = nil
	e.started = false
	e.stateMu.Unlock()
	if cancel != nil {
		cancel()
	}
	shutdownCtx := ctx
	if cfg != nil && cfg.Server.Timeouts.ServerShutdown > 0 {
		var cancelShutdown context.CancelFunc
		shutdownCtx, cancelShutdown = context.WithTimeout(ctx, cfg.Server.Timeouts.ServerShutdown)
		defer cancelShutdown()
	}
	var errs []error
	if server != nil {
		if err := server.Shutdown(shutdownCtx); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errs = append(errs, fmt.Errorf("shutdown http server: %w", err))
		}
	}
	if listener != nil {
		if err := listener.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			errs = append(errs, fmt.Errorf("close listener: %w", err))
		}
	}
	e.serverWG.Wait()
	if store != nil {
		if err := store.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close resource store: %w", err))
		}
	}
	if serverErr := e.serverFailure(); serverErr != nil {
		errs = append(errs, serverErr)
	}
	if len(errs) > 0 {
		e.errMu.Lock()
		e.stopErr = errors.Join(errs...)
		e.errMu.Unlock()
		if log != nil {
			log.Error("engine stopped with errors", "error", e.stopErr)
		}
		return e.stopErr
	}
	e.errMu.Lock()
	e.serverErr = nil
	e.stopErr = nil
	e.errMu.Unlock()
	if log != nil {
		log.Info("engine stopped")
	}
	return nil
}

// Wait blocks until the engine HTTP server goroutine completes.
func (e *Engine) Wait() {
	e.serverWG.Wait()
}

// Server returns the active HTTP server instance.
func (e *Engine) Server() *http.Server {
	e.stateMu.RLock()
	defer e.stateMu.RUnlock()
	return e.server
}

// Router returns the current HTTP router instance.
func (e *Engine) Router() *chi.Mux {
	e.stateMu.RLock()
	defer e.stateMu.RUnlock()
	return e.router
}

// Config returns the configuration snapshot captured at startup.
func (e *Engine) Config() *appconfig.Config {
	e.stateMu.RLock()
	defer e.stateMu.RUnlock()
	return e.configSnapshot
}

// ResourceStore returns the active resource store.
func (e *Engine) ResourceStore() resources.ResourceStore {
	e.stateMu.RLock()
	defer e.stateMu.RUnlock()
	return e.resourceStore
}

// Mode returns the configured engine mode.
func (e *Engine) Mode() Mode {
	return e.mode
}

// IsStarted reports whether the engine lifecycle has been started.
func (e *Engine) IsStarted() bool {
	e.stateMu.RLock()
	defer e.stateMu.RUnlock()
	return e.started
}

func (e *Engine) buildResourceStore(ctx context.Context) (resources.ResourceStore, error) {
	switch e.mode {
	case ModeStandalone:
		log := logger.FromContext(ctx)
		if log != nil {
			log.Debug("initializing memory resource store for standalone mode")
		}
		return resources.NewMemoryResourceStore(), nil
	case ModeDistributed:
		return nil, ErrDistributedModeUnsupported
	default:
		return nil, fmt.Errorf("unsupported engine mode %q", e.mode)
	}
}

func (e *Engine) newHTTPServer(
	ctx context.Context,
	router *chi.Mux,
	cfg *appconfig.Config,
	addr string,
) *http.Server {
	server := &http.Server{
		Addr:        addr,
		Handler:     router,
		BaseContext: func(net.Listener) context.Context { return ctx },
	}
	if cfg != nil {
		timeouts := cfg.Server.Timeouts
		server.ReadTimeout = timeouts.HTTPRead
		server.WriteTimeout = timeouts.HTTPWrite
		server.IdleTimeout = timeouts.HTTPIdle
		server.ReadHeaderTimeout = timeouts.HTTPReadHeader
	}
	return server
}

func (e *Engine) resolveListenAddress(cfg *appconfig.Config) (string, int) {
	host := e.host
	port := e.port
	if port <= 0 && cfg != nil && cfg.Server.Port > 0 {
		port = cfg.Server.Port
	}
	if host == "" && cfg != nil && cfg.Server.Host != "" {
		host = cfg.Server.Host
	}
	if host == "" {
		host = loopbackHostname
	}
	return host, port
}

func (e *Engine) listen(host string, port int) (net.Listener, int, error) {
	if host == "" {
		host = loopbackHostname
	}
	address := fmt.Sprintf("%s:%d", host, port)
	if port <= 0 {
		address = fmt.Sprintf("%s:0", host)
	}
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return nil, 0, fmt.Errorf("listen on %s: %w", address, err)
	}
	tcpAddr, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		_ = listener.Close()
		return nil, 0, fmt.Errorf("expected tcp address, got %T", listener.Addr())
	}
	return listener, tcpAddr.Port, nil
}

func (e *Engine) newClient(ctx context.Context, host string, port int) (*sdkclient.Client, string, error) {
	clientHost := sanitizeHostForClient(host)
	baseURL := fmt.Sprintf("%s://%s:%d", httpScheme, clientHost, port)
	client, err := sdkclient.New(ctx, baseURL)
	if err != nil {
		return nil, "", fmt.Errorf("initialize sdk client: %w", err)
	}
	return client, baseURL, nil
}

func (e *Engine) launchServer(log logger.Logger, srv *http.Server, ln net.Listener) {
	if log != nil {
		log.Debug("starting http server", "address", ln.Addr().String())
	}
	e.serverWG.Go(func() {
		if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			if log != nil {
				log.Error("http server failed", "error", err)
			}
			e.recordServerError(fmt.Errorf("http server failure: %w", err))
			return
		}
	})
}

func sanitizeHostForClient(host string) string {
	if host == "" || host == "0.0.0.0" || host == "::" {
		return loopbackHostname
	}
	return host
}

func (e *Engine) recordServerError(err error) {
	if err == nil {
		return
	}
	e.errMu.Lock()
	e.serverErr = err
	e.errMu.Unlock()
}

func (e *Engine) serverFailure() error {
	e.errMu.Lock()
	defer e.errMu.Unlock()
	return e.serverErr
}

func (e *Engine) cleanupStore(ctx context.Context, store resources.ResourceStore) {
	if store == nil {
		return
	}
	if err := store.Close(); err != nil {
		log := logger.FromContext(ctx)
		if log != nil {
			log.Warn("failed to close resource store during cleanup", "error", err)
		}
	}
}

func (e *Engine) recordStartError(err error) {
	e.errMu.Lock()
	e.startErr = err
	e.errMu.Unlock()
}
