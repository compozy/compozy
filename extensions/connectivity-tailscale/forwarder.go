package connectivitytailscale

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"sync"
	"time"
)

const (
	maxForwardConnections = 128
	forwardDialTimeout    = 5 * time.Second
	forwardIdleTimeout    = 2 * time.Minute
)

type tierForwarder struct {
	listener net.Listener
	target   string
	logger   *slog.Logger

	ctx    context.Context
	cancel context.CancelFunc
	done   chan struct{}
	wg     sync.WaitGroup
	start  sync.Once
	stop   sync.Once
	closed chan struct{}
	slots  chan struct{}

	mu          sync.Mutex
	connections map[net.Conn]struct{}
	serveErr    error
	forwardErr  error
	closing     bool
	closeErr    error
}

func newTierForwarder(listener net.Listener, target string, logger *slog.Logger) (*tierForwarder, error) {
	if listener == nil {
		return nil, errors.New("connectivity-tailscale: listener is required")
	}
	if target == "" {
		return nil, errors.New("connectivity-tailscale: forward target is required")
	}
	if logger == nil {
		logger = slog.Default()
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &tierForwarder{
		listener:    listener,
		target:      target,
		logger:      logger,
		ctx:         ctx,
		cancel:      cancel,
		done:        make(chan struct{}),
		closed:      make(chan struct{}),
		slots:       make(chan struct{}, maxForwardConnections),
		connections: make(map[net.Conn]struct{}),
	}, nil
}

func (f *tierForwarder) Start() {
	f.start.Do(func() { go f.serve() })
}

func (f *tierForwarder) Health() (string, string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.serveErr != nil || f.forwardErr != nil {
		return providerHealthDegraded, "listener stopped accepting connections"
	}
	return providerHealthHealthy, ""
}

func (f *tierForwarder) Close(ctx context.Context) error {
	if f == nil {
		return nil
	}
	f.stop.Do(func() {
		f.mu.Lock()
		f.closing = true
		f.mu.Unlock()
		f.cancel()
		go f.shutdown()
	})
	select {
	case <-f.closed:
		f.mu.Lock()
		err := f.closeErr
		f.mu.Unlock()
		return err
	case <-ctx.Done():
		return fmt.Errorf("connectivity-tailscale: wait for forwarder: %w", ctx.Err())
	}
}

func (f *tierForwarder) serve() {
	defer close(f.done)
	defer f.wg.Wait()
	for {
		connection, err := f.listener.Accept()
		if err != nil {
			f.mu.Lock()
			closing := f.closing
			if !closing {
				f.serveErr = fmt.Errorf("accept connection: %w", err)
			}
			f.mu.Unlock()
			if closing {
				return
			}
			return
		}
		select {
		case f.slots <- struct{}{}:
		default:
			if err := connection.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
				f.logger.Warn("close excess connectivity connection", "error", err)
			}
			continue
		}
		f.track(connection)
		f.wg.Add(1)
		go f.forward(connection)
	}
}

func (f *tierForwarder) forward(inbound net.Conn) {
	defer f.wg.Done()
	defer func() { <-f.slots }()
	defer f.closeTracked(inbound)
	upstream, err := (&net.Dialer{Timeout: forwardDialTimeout}).DialContext(f.ctx, "tcp", f.target)
	if err != nil {
		if f.ctx.Err() == nil {
			f.recordForwardFailure()
			f.logger.Warn("connectivity forward target unavailable")
		}
		return
	}
	f.track(upstream)
	defer f.closeTracked(upstream)
	inboundIdle := idleDeadlineConn{Conn: inbound, timeout: forwardIdleTimeout}
	upstreamIdle := idleDeadlineConn{Conn: upstream, timeout: forwardIdleTimeout}
	results := make(chan error, 2)
	go copyConnection(results, upstreamIdle, inboundIdle)
	go copyConnection(results, inboundIdle, upstreamIdle)
	firstErr := <-results
	if err := inbound.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
		f.logger.Debug("close forwarded inbound connection", "error", err)
	}
	if err := upstream.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
		f.logger.Debug("close forwarded upstream connection", "error", err)
	}
	secondErr := <-results
	for _, copyErr := range []error{firstErr, secondErr} {
		if copyErr != nil && !errors.Is(copyErr, net.ErrClosed) && f.ctx.Err() == nil {
			f.logger.Debug("copy forwarded connection", "error", copyErr)
		}
	}
}

type idleDeadlineConn struct {
	net.Conn
	timeout time.Duration
}

func (c idleDeadlineConn) Read(payload []byte) (int, error) {
	if err := c.SetReadDeadline(time.Now().Add(c.timeout)); err != nil {
		return 0, err
	}
	return c.Conn.Read(payload)
}

func (c idleDeadlineConn) Write(payload []byte) (int, error) {
	if err := c.SetWriteDeadline(time.Now().Add(c.timeout)); err != nil {
		return 0, err
	}
	return c.Conn.Write(payload)
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
	f.mu.Lock()
	connections := make([]net.Conn, 0, len(f.connections))
	for connection := range f.connections {
		connections = append(connections, connection)
	}
	f.mu.Unlock()
	for _, connection := range connections {
		if err := connection.Close(); err != nil && !onlyClosedNetworkError(err) {
			errs = append(errs, fmt.Errorf("connectivity-tailscale: close forwarded connection: %w", err))
		}
	}
	if err := f.listener.Close(); err != nil && !onlyClosedNetworkError(err) {
		errs = append(errs, fmt.Errorf("connectivity-tailscale: close listener: %w", err))
	}
	<-f.done
	f.mu.Lock()
	f.closeErr = errors.Join(errs...)
	f.mu.Unlock()
	close(f.closed)
}

func onlyClosedNetworkError(err error) bool {
	if err == nil {
		return false
	}
	if joined, ok := err.(interface{ Unwrap() []error }); ok {
		children := joined.Unwrap()
		if len(children) == 0 {
			return false
		}
		for _, child := range children {
			if !onlyClosedNetworkError(child) {
				return false
			}
		}
		return true
	}
	return errors.Is(err, net.ErrClosed)
}

func copyConnection(result chan<- error, destination io.Writer, source io.Reader) {
	_, err := io.Copy(destination, source)
	result <- err
}

func (f *tierForwarder) track(connection net.Conn) {
	f.mu.Lock()
	f.connections[connection] = struct{}{}
	f.mu.Unlock()
}

func (f *tierForwarder) closeTracked(connection net.Conn) {
	f.mu.Lock()
	delete(f.connections, connection)
	f.mu.Unlock()
	if err := connection.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
		f.logger.Debug("close tracked forwarded connection", "error", err)
	}
}
