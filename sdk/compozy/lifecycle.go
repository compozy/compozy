package compozy

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/compozy/compozy/engine/resources"
	"github.com/compozy/compozy/engine/tool/inline"
	appconfig "github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
	sdkclient "github.com/compozy/compozy/sdk/v2/client"
)

const (
	defaultHTTPReadHeaderTimeout = 5 * time.Second
	defaultHTTPShutdownTimeout   = 5 * time.Second
)

// Start boots the engine lifecycle by initializing the resource store, HTTP server, and SDK client.
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
	store, err := e.buildResourceStore(ctx, cfg)
	if err != nil {
		e.recordStartError(err)
		return err
	}
	httpState, err := e.startHTTPComponents(ctx, cfg)
	if err != nil {
		e.cleanupStore(ctx, store)
		cleanupErr := e.cleanupModeResources(ctx)
		if cleanupErr != nil {
			err = errors.Join(err, fmt.Errorf("cleanup mode resources: %w", cleanupErr))
		}
		e.recordStartError(err)
		return err
	}
	inlineManager, err := e.initInlineManager(ctx, cfg, store)
	if err != nil {
		e.cleanupHTTPState(ctx, httpState)
		e.cleanupStore(ctx, store)
		cleanupErr := e.cleanupModeResources(ctx)
		if cleanupErr != nil {
			err = errors.Join(err, fmt.Errorf("cleanup mode resources: %w", cleanupErr))
		}
		e.recordStartError(err)
		return err
	}
	e.applyStartState(cfg, store, httpState)
	if inlineManager != nil {
		e.stateMu.Lock()
		e.inlineManager = inlineManager
		e.stateMu.Unlock()
	}
	if log := logger.FromContext(ctx); log != nil {
		log.Info("engine started", "mode", string(e.mode), "base_url", httpState.baseURL)
	}
	return nil
}

type httpState struct {
	router   *chi.Mux
	server   *http.Server
	listener net.Listener
	cancel   context.CancelFunc
	client   *sdkclient.Client
	baseURL  string
	port     int
}

type stopState struct {
	server   *http.Server
	listener net.Listener
	store    resources.ResourceStore
	cancel   context.CancelFunc
	cfg      *appconfig.Config
}

func (e *Engine) startHTTPComponents(ctx context.Context, cfg *appconfig.Config) (*httpState, error) {
	router := chi.NewRouter()
	listenHost, listenPort := e.resolveListenAddress(cfg)
	listener, actualPort, err := e.listen(ctx, listenHost, listenPort)
	if err != nil {
		return nil, err
	}
	serverCtx, cancel := context.WithCancel(ctx)
	server := e.newHTTPServer(serverCtx, router, cfg, listener.Addr().String())
	client, baseURL, err := e.newClient(ctx, listenHost, actualPort)
	if err != nil {
		cancel()
		_ = listener.Close()
		return nil, err
	}
	e.serverWG = sync.WaitGroup{}
	e.launchServer(ctx, server, listener)
	return &httpState{
		router:   router,
		server:   server,
		listener: listener,
		cancel:   cancel,
		client:   client,
		baseURL:  baseURL,
		port:     actualPort,
	}, nil
}

func (e *Engine) initInlineManager(
	ctx context.Context,
	cfg *appconfig.Config,
	store resources.ResourceStore,
) (*inline.Manager, error) {
	if store == nil {
		return nil, fmt.Errorf("resource store is required for inline manager")
	}
	projectName := projectNameOf(e.project)
	if projectName == "" {
		return nil, nil
	}
	root := strings.TrimSpace(cfg.CLI.CWD)
	if root == "" {
		wd, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("resolve project root: %w", err)
		}
		root = wd
	}
	manager, err := inline.NewManager(ctx, inline.Options{
		ProjectRoot:    root,
		ProjectName:    projectName,
		Store:          store,
		UserEntrypoint: strings.TrimSpace(cfg.Runtime.EntrypointPath),
	})
	if err != nil {
		return nil, err
	}
	if err := manager.Start(ctx); err != nil {
		_ = manager.Close(ctx)
		return nil, err
	}
	cfg.Runtime.EntrypointPath = manager.EntrypointPath()
	return manager, nil
}

func (e *Engine) cleanupHTTPState(ctx context.Context, state *httpState) {
	if state == nil {
		return
	}
	if state.cancel != nil {
		state.cancel()
	}
	if state.server != nil {
		shutdownCtx, cancel := context.WithTimeout(ctx, defaultHTTPShutdownTimeout)
		if cancel != nil {
			defer cancel()
		}
		if err := state.server.Shutdown(shutdownCtx); err != nil && !errors.Is(err, http.ErrServerClosed) {
			if log := logger.FromContext(ctx); log != nil {
				log.Warn("failed to shutdown http server", "error", err)
			}
		}
	}
	if state.listener != nil {
		if err := state.listener.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			if log := logger.FromContext(ctx); log != nil {
				log.Warn("failed to close listener", "error", err)
			}
		}
	}
}

func (e *Engine) applyStartState(cfg *appconfig.Config, store resources.ResourceStore, httpState *httpState) {
	e.stateMu.Lock()
	e.resourceStore = store
	e.router = httpState.router
	e.server = httpState.server
	e.listener = httpState.listener
	e.client = httpState.client
	e.configSnapshot = cfg
	e.serverCancel = httpState.cancel
	e.started = true
	e.baseURL = httpState.baseURL
	e.port = httpState.port
	e.stopErr = nil
	e.stateMu.Unlock()
	e.errMu.Lock()
	e.startErr = nil
	e.errMu.Unlock()
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
	inlineManager := e.takeInlineManager()
	if inlineManager != nil {
		if err := inlineManager.Close(ctx); err != nil && log != nil {
			log.Warn("failed to close inline manager", "error", err)
		}
	}
	state := e.detachStopState()
	if state.cancel != nil {
		state.cancel()
	}
	shutdownCtx, cancel := deriveShutdownContext(ctx, state.cfg)
	if cancel != nil {
		defer cancel()
	}
	errs := e.shutdownResources(shutdownCtx, ctx, state)
	return e.finalizeStop(ctx, errs)
}

func (e *Engine) detachStopState() stopState {
	e.stateMu.Lock()
	defer e.stateMu.Unlock()
	state := stopState{
		server:   e.server,
		listener: e.listener,
		store:    e.resourceStore,
		cancel:   e.serverCancel,
		cfg:      e.configSnapshot,
	}
	e.router = nil
	e.server = nil
	e.listener = nil
	e.resourceStore = nil
	e.serverCancel = nil
	e.client = nil
	e.configSnapshot = nil
	e.baseURL = ""
	e.port = 0
	e.started = false
	return state
}

func (e *Engine) takeInlineManager() *inline.Manager {
	e.stateMu.Lock()
	defer e.stateMu.Unlock()
	manager := e.inlineManager
	e.inlineManager = nil
	return manager
}

func deriveShutdownContext(ctx context.Context, cfg *appconfig.Config) (context.Context, context.CancelFunc) {
	if cfg == nil {
		return ctx, nil
	}
	timeout := cfg.Server.Timeouts.ServerShutdown
	if timeout <= 0 {
		return ctx, nil
	}
	return context.WithTimeout(ctx, timeout)
}

func (e *Engine) shutdownResources(shutdownCtx context.Context, baseCtx context.Context, state stopState) []error {
	var errs []error
	if state.server != nil {
		if err := state.server.Shutdown(shutdownCtx); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errs = append(errs, fmt.Errorf("shutdown http server: %w", err))
		}
	}
	if state.listener != nil {
		if err := state.listener.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			errs = append(errs, fmt.Errorf("close listener: %w", err))
		}
	}
	e.serverWG.Wait()
	if state.store != nil {
		if err := state.store.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close resource store: %w", err))
		}
	}
	if cleanupErr := e.cleanupModeResources(context.WithoutCancel(baseCtx)); cleanupErr != nil {
		errs = append(errs, fmt.Errorf("cleanup mode resources: %w", cleanupErr))
	}
	if serverErr := e.serverFailure(); serverErr != nil {
		errs = append(errs, serverErr)
	}
	return errs
}

func (e *Engine) finalizeStop(ctx context.Context, errs []error) error {
	log := logger.FromContext(ctx)
	if len(errs) > 0 {
		err := errors.Join(errs...)
		e.errMu.Lock()
		e.stopErr = err
		e.errMu.Unlock()
		if log != nil {
			log.Error("engine stopped with errors", "error", err)
		}
		return err
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

func (e *Engine) buildResourceStore(ctx context.Context, cfg *appconfig.Config) (resources.ResourceStore, error) {
	state, err := e.bootstrapMode(ctx, cfg)
	if err != nil {
		return nil, err
	}
	if state == nil || state.resourceStore == nil {
		if state != nil {
			state.cleanupOnError(context.WithoutCancel(ctx))
		}
		return nil, fmt.Errorf("mode runtime did not provide resource store")
	}
	e.stateMu.Lock()
	e.modeCleanups = state.cleanups
	e.stateMu.Unlock()
	return state.resourceStore, nil
}

func (e *Engine) newHTTPServer(
	ctx context.Context,
	router *chi.Mux,
	cfg *appconfig.Config,
	addr string,
) *http.Server {
	server := &http.Server{
		Addr:              addr,
		Handler:           router,
		BaseContext:       func(net.Listener) context.Context { return ctx },
		ReadHeaderTimeout: defaultHTTPReadHeaderTimeout,
	}
	if cfg != nil {
		timeouts := cfg.Server.Timeouts
		server.ReadTimeout = timeouts.HTTPRead
		server.WriteTimeout = timeouts.HTTPWrite
		server.IdleTimeout = timeouts.HTTPIdle
		if timeouts.HTTPReadHeader > 0 {
			server.ReadHeaderTimeout = timeouts.HTTPReadHeader
		}
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

func (e *Engine) listen(ctx context.Context, host string, port int) (net.Listener, int, error) {
	if ctx == nil {
		return nil, 0, fmt.Errorf("context is required")
	}
	if host == "" {
		host = loopbackHostname
	}
	address := fmt.Sprintf("%s:%d", host, port)
	if port <= 0 {
		address = fmt.Sprintf("%s:0", host)
	}
	lc := net.ListenConfig{}
	listener, err := lc.Listen(ctx, "tcp", address)
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

func (e *Engine) launchServer(ctx context.Context, srv *http.Server, ln net.Listener) {
	log := logger.FromContext(ctx)
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

func (e *Engine) cleanupModeResources(ctx context.Context) error {
	cleanups := e.extractModeCleanups()
	if len(cleanups) == 0 {
		return nil
	}
	var errs []error
	for i := len(cleanups) - 1; i >= 0; i-- {
		fn := cleanups[i]
		if fn == nil {
			continue
		}
		if err := fn(ctx); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (e *Engine) extractModeCleanups() []modeCleanup {
	e.stateMu.Lock()
	defer e.stateMu.Unlock()
	cleanups := e.modeCleanups
	e.modeCleanups = nil
	return cleanups
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
