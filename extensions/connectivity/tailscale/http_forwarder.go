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

// Session waits allow thirty minutes; stalled headers get one extra minute to settle.
const forwardResponseHeaderTimeout = 31 * time.Minute

type httpTierForwarder struct {
	listener    net.Listener
	server      *http.Server
	transport   *http.Transport
	cancel      context.CancelFunc
	logger      *slog.Logger
	start       sync.Once
	stop        sync.Once
	done        chan struct{}
	closed      chan struct{}
	slots       chan struct{}
	workers     sync.WaitGroup
	requests    sync.WaitGroup
	mu          sync.RWMutex
	connections map[*httpForwardConn]struct{}
	closing     bool
	serveErr    error
	forwardErr  error
	closeErr    error
}

func newHTTPTierForwarder(
	listener net.Listener,
	target, advertisedURL string,
	logger *slog.Logger,
) (*httpTierForwarder, error) {
	endpoint, err := url.Parse(advertisedURL)
	if err != nil || endpoint.Scheme != "https" || endpoint.Host == "" || listener == nil || target == "" {
		return nil, errors.New("tailscale: HTTPS endpoint, listener and forward target are required")
	}
	if logger == nil {
		logger = slog.Default()
	}
	ctx, cancel := context.WithCancel(context.Background())
	f := &httpTierForwarder{
		listener: listener, cancel: cancel, logger: logger,
		done: make(chan struct{}), closed: make(chan struct{}),
		slots: make(chan struct{}, maxForwardConnections), connections: make(map[*httpForwardConn]struct{}),
	}
	f.transport = &http.Transport{
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			connection, err := (&net.Dialer{Timeout: forwardDialTimeout}).DialContext(ctx, network, address)
			if err != nil && ctx.Err() == nil {
				f.mu.Lock()
				if !f.closing {
					f.forwardErr = errors.New("forward target unavailable")
				}
				f.mu.Unlock()
			}
			return connection, err
		},
		MaxConnsPerHost: maxForwardConnections, MaxIdleConnsPerHost: maxForwardConnections,
		IdleConnTimeout: forwardIdleTimeout, ResponseHeaderTimeout: forwardResponseHeaderTimeout,
	}
	proxy := &httputil.ReverseProxy{
		Rewrite: func(r *httputil.ProxyRequest) {
			r.SetURL(&url.URL{Scheme: "http", Host: target})
			r.Out.Host = r.In.Host
			// Rewrite strips client forwarding headers before establishing the TLS-facing target.
			r.Out.Header.Set("X-Forwarded-Proto", "https")
			r.Out.Header.Set("X-Forwarded-Host", r.In.Host)
		},
		Transport: f.transport,
		ErrorHandler: func(w http.ResponseWriter, _ *http.Request, _ error) {
			http.Error(w, "gateway forward target unavailable", http.StatusBadGateway)
		},
	}
	f.server = &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			f.mu.Lock()
			if f.closing {
				f.mu.Unlock()
				http.Error(w, "gateway forwarder closed", http.StatusServiceUnavailable)
				return
			}
			f.requests.Add(1)
			f.mu.Unlock()
			defer f.requests.Done()
			if !strings.EqualFold(r.Host, endpoint.Host) &&
				(endpoint.Port() != "" || !strings.EqualFold(r.Host, endpoint.Host+":443")) {
				http.Error(w, "gateway request host not allowed", http.StatusForbidden)
				return
			}
			proxy.ServeHTTP(w, r)
		}),
		BaseContext:       func(net.Listener) context.Context { return ctx },
		ReadHeaderTimeout: forwardDialTimeout, ReadTimeout: forwardIdleTimeout, IdleTimeout: forwardIdleTimeout,
		ErrorLog: slog.NewLogLogger(logger.Handler(), slog.LevelDebug),
	}
	return f, nil
}

func (f *httpTierForwarder) Start() {
	f.start.Do(func() {
		go func() {
			defer close(f.done)
			err := f.server.Serve(&httpForwardListener{Listener: f.listener, owner: f})
			f.mu.Lock()
			if !f.closing {
				f.serveErr = err
			}
			f.mu.Unlock()
			f.workers.Wait()
		}()
	})
}

func (f *httpTierForwarder) Health() (string, string) {
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

func (f *httpTierForwarder) Close(ctx context.Context) error {
	if f == nil {
		return nil
	}
	f.Start()
	f.stop.Do(func() {
		f.mu.Lock()
		f.closing = true
		f.mu.Unlock()
		f.cancel()
		go f.shutdown()
	})
	select {
	case <-f.closed:
		return f.closeErr
	case <-ctx.Done():
		return fmt.Errorf("tailscale: wait for HTTP forwarder: %w", ctx.Err())
	}
}

func (f *httpTierForwarder) shutdown() {
	var errs []error
	if err := f.server.Close(); err != nil && !onlyClosedNetworkError(err) {
		errs = append(errs, err)
	}
	f.mu.RLock()
	connections := make([]*httpForwardConn, 0, len(f.connections))
	for connection := range f.connections {
		connections = append(connections, connection)
	}
	f.mu.RUnlock()
	// Server.Close excludes hijacked WebSockets, which remain owned by this forwarder.
	for _, connection := range connections {
		if err := connection.Close(); err != nil && !onlyClosedNetworkError(err) {
			errs = append(errs, err)
		}
	}
	<-f.done
	f.requests.Wait()
	f.transport.CloseIdleConnections()
	f.closeErr = errors.Join(errs...)
	close(f.closed)
}

type httpForwardListener struct {
	net.Listener
	owner *httpTierForwarder
}

func (l *httpForwardListener) Accept() (net.Conn, error) {
	for {
		connection, err := l.Listener.Accept()
		if err != nil {
			return nil, err
		}
		f := l.owner
		f.mu.Lock()
		select {
		case f.slots <- struct{}{}:
			if !f.closing {
				wrapped := &httpForwardConn{Conn: connection, owner: f}
				f.connections[wrapped] = struct{}{}
				f.workers.Add(1)
				f.mu.Unlock()
				return wrapped, nil
			}
			<-f.slots
		default:
		}
		f.mu.Unlock()
		if err := connection.Close(); err != nil && !onlyClosedNetworkError(err) {
			f.logger.Debug("close excess connectivity connection", "error", err)
		}
	}
}

type httpForwardConn struct {
	net.Conn
	owner *httpTierForwarder
	once  sync.Once
	err   error
}

func (c *httpForwardConn) Close() error {
	c.once.Do(func() {
		c.err = c.Conn.Close()
		c.owner.mu.Lock()
		delete(c.owner.connections, c)
		c.owner.mu.Unlock()
		<-c.owner.slots
		c.owner.workers.Done()
	})
	return c.err
}

func (c *httpForwardConn) Write(payload []byte) (int, error) {
	return (idleDeadlineConn{Conn: c.Conn, timeout: forwardIdleTimeout}).Write(payload)
}
