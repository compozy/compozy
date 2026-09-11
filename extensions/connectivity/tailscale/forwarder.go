package tailscale

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
	"time"
)

const (
	maxForwardConnections = 128
	forwardDialTimeout    = 5 * time.Second
	forwardIdleTimeout    = 2 * time.Minute
	// forwardReadTimeout bounds a whole request read, headers and body: a
	// client that stalls mid-body must not pin one of the 128 forward slots
	// forever, or enough stalled bodies turn every later request into a 503
	// (CWE-400). Two minutes matches the idle scale above and comfortably
	// covers JSON and multipart uploads over a tailscale link; SSE responses
	// are writes, so they stay unaffected (no WriteTimeout is set).
	forwardReadTimeout = 2 * time.Minute
)

// forwardedProto is the scheme the provider terminates TLS for on behalf of
// the daemon tier listener. The daemon listener is plaintext loopback HTTP,
// so the forwarder must declare the browser-facing scheme per request; the
// httpapi origin allowance compares canonical origins including scheme.
const forwardedProto = "https"

// X-Forwarded-Proto and X-Forwarded-Host are the forwarded-target headers the
// httpapi middleware consumes (requestScheme) and that document the original
// browser request. ReverseProxy's Rewrite mode strips client-supplied
// Forwarded/X-Forwarded-* headers before rewriting, so remote clients cannot
// spoof them.
const (
	forwardedProtoHeader = "X-Forwarded-Proto"
	forwardedHostHeader  = "X-Forwarded-Host"
)

type tierForwarder struct {
	listener net.Listener
	backend  *url.URL
	logger   *slog.Logger
	dial     func(context.Context, string, string) (net.Conn, error)

	proxy  *httputil.ReverseProxy
	server *http.Server

	stopLifecycle context.CancelFunc
	done          chan struct{}
	wg            sync.WaitGroup
	start         sync.Once
	stop          sync.Once
	closed        chan struct{}
	slots         chan struct{}

	mu         sync.RWMutex
	serveErr   error
	forwardErr error
	closing    bool
	closeErr   error
}

func newTierForwarder(listener net.Listener, target string, logger *slog.Logger) (*tierForwarder, error) {
	if listener == nil {
		return nil, errors.New("tailscale: listener is required")
	}
	if target == "" {
		return nil, errors.New("tailscale: forward target is required")
	}
	if logger == nil {
		logger = slog.Default()
	}
	forwarder := &tierForwarder{
		listener: listener,
		backend:  &url.URL{Scheme: "http", Host: target},
		logger:   logger,
		dial:     (&net.Dialer{Timeout: forwardDialTimeout}).DialContext,
		done:     make(chan struct{}),
		closed:   make(chan struct{}),
		slots:    make(chan struct{}, maxForwardConnections),
	}
	lifecycleCtx, cancel := context.WithCancel(context.Background())
	forwarder.stopLifecycle = cancel
	forwarder.proxy = forwarder.newProxy()
	forwarder.server = forwarder.newServer(lifecycleCtx)
	return forwarder, nil
}

// Start begins accepting tier connections and serving them to the daemon.
func (f *tierForwarder) Start() {
	f.start.Do(func() {
		f.mu.Lock()
		closing := f.closing
		f.mu.Unlock()
		if closing {
			f.stopLifecycle()
		}
		go f.serve()
	})
}

func (f *tierForwarder) Health() (string, string) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	if f.serveErr != nil {
		return providerHealthDegraded, "listener stopped accepting connections"
	}
	if f.forwardErr != nil {
		return providerHealthDegraded, "forward target is unavailable"
	}
	return providerHealthHealthy, ""
}

func (f *tierForwarder) Close(ctx context.Context) error {
	if f == nil {
		return nil
	}
	f.Start()
	f.stop.Do(func() {
		f.mu.Lock()
		f.closing = true
		f.mu.Unlock()
		if f.stopLifecycle != nil {
			f.stopLifecycle()
		}
		go f.shutdown()
	})
	select {
	case <-f.closed:
		f.mu.Lock()
		err := f.closeErr
		f.mu.Unlock()
		return err
	case <-ctx.Done():
		return fmt.Errorf("tailscale: wait for forwarder: %w", ctx.Err())
	}
}

func (f *tierForwarder) serve() {
	defer close(f.done)
	defer f.wg.Wait()
	if err := f.server.Serve(f.listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
		f.mu.Lock()
		if !f.closing {
			f.serveErr = fmt.Errorf("accept connection: %w", err)
		}
		f.mu.Unlock()
	}
}

// ServeHTTP proxies one browser request to the daemon tier listener as an
// HTTP-aware reverse proxy so the daemon observes the forwarded scheme and
// host of the original TLS request.
func (f *tierForwarder) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	select {
	case f.slots <- struct{}{}:
		defer func() { <-f.slots }()
	default:
		http.Error(writer, "tailscale: forward concurrency limit reached", http.StatusServiceUnavailable)
		return
	}
	f.mu.Lock()
	if f.closing {
		f.mu.Unlock()
		http.Error(writer, "tailscale: forwarder is shutting down", http.StatusServiceUnavailable)
		return
	}
	f.wg.Add(1)
	f.mu.Unlock()
	defer f.wg.Done()
	f.proxy.ServeHTTP(writer, request)
}

func (f *tierForwarder) newProxy() *httputil.ReverseProxy {
	return &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.SetURL(f.backend)
			// SetURL retargets the outbound Host header at the backend; the
			// daemon tier listener needs the original browser-facing host to
			// evaluate its same-origin allowance.
			pr.Out.Host = pr.In.Host
			pr.Out.Header.Set(forwardedProtoHeader, forwardedProto)
			if host := strings.TrimSpace(pr.In.Host); host != "" {
				pr.Out.Header.Set(forwardedHostHeader, host)
			}
		},
		Transport: &http.Transport{
			DialContext:         f.dial,
			MaxIdleConns:        maxForwardConnections,
			MaxIdleConnsPerHost: maxForwardConnections,
			IdleConnTimeout:     forwardIdleTimeout,
			DisableCompression:  true,
		},
		// SSE and task streams must not buffer behind the proxy.
		FlushInterval: -1,
		ErrorHandler: func(writer http.ResponseWriter, request *http.Request, _ error) {
			// The proxy error itself is intentionally discarded: the dial
			// failure is already recorded by recordForwardFailure as the
			// degraded health reason surfaced through `gateway status`.
			if request.Context().Err() == nil {
				f.recordForwardFailure()
				f.logger.Warn("connectivity forward target unavailable")
			}
			http.Error(writer, "tailscale: forward target unavailable", http.StatusBadGateway)
		},
	}
}

func (f *tierForwarder) newServer(lifecycleCtx context.Context) *http.Server {
	return &http.Server{
		Handler:           f,
		BaseContext:       func(net.Listener) context.Context { return lifecycleCtx },
		ReadHeaderTimeout: forwardIdleTimeout,
		ReadTimeout:       forwardReadTimeout,
		IdleTimeout:       forwardIdleTimeout,
	}
}

func (f *tierForwarder) recordForwardFailure() {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.forwardErr == nil {
		f.forwardErr = errors.New("forward target unavailable")
	}
}

func (f *tierForwarder) shutdown() {
	var errs []error
	if err := f.server.Close(); err != nil &&
		!errors.Is(err, http.ErrServerClosed) && !errors.Is(err, net.ErrClosed) {
		errs = append(errs, fmt.Errorf("tailscale: stop forward server: %w", err))
	}
	// f.done closes only after every in-flight proxied request returns.
	<-f.done
	f.mu.Lock()
	f.closeErr = errors.Join(errs...)
	f.mu.Unlock()
	close(f.closed)
}
