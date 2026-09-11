package tailscale

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

// startVerificationRelay exposes the loopback transport the daemon proves its
// private endpoint through. The relay is a transparent byte pipe: the prover's
// TLS session must terminate at the node's private listener, so the relay
// forwards raw bytes and never interprets them.
func startVerificationRelay(
	ctx context.Context,
	node tailscaleNode,
	tier, domain string,
	logger *slog.Logger,
) (*verificationRelay, error) {
	if tier != tierPrivate {
		return nil, nil
	}
	listener, err := (&net.ListenConfig{}).Listen(ctx, "tcp", "127.0.0.1:0")
	if err != nil {
		return nil, errors.New("tailscale: cannot bind private verification transport")
	}
	relay := newVerificationRelay(listener, net.JoinHostPort(domain, privateListenerPort), logger)
	relay.dial = node.Dial
	relay.Start()
	return relay, nil
}

// verificationRelay pipes raw bytes between a loopback prover connection and
// the node's private listener. It is deliberately transport-transparent, so it
// shares none of the tier forwarder's HTTP-aware semantics.
type verificationRelay struct {
	listener net.Listener
	target   string
	logger   *slog.Logger
	dial     func(context.Context, string, string) (net.Conn, error)

	stopLifecycle context.CancelFunc
	done          chan struct{}
	closed        chan struct{}
	start         sync.Once
	stop          sync.Once

	mu          sync.Mutex
	connections map[net.Conn]struct{}
	closing     bool
	closeErr    error

	wg    sync.WaitGroup
	slots chan struct{}
}

func newVerificationRelay(listener net.Listener, target string, logger *slog.Logger) *verificationRelay {
	if logger == nil {
		logger = slog.Default()
	}
	return &verificationRelay{
		listener:    listener,
		target:      target,
		logger:      logger,
		done:        make(chan struct{}),
		closed:      make(chan struct{}),
		connections: make(map[net.Conn]struct{}),
		slots:       make(chan struct{}, maxForwardConnections),
	}
}

// Start begins accepting prover connections and piping them to the node.
func (r *verificationRelay) Start() {
	r.start.Do(func() {
		ctx, cancel := context.WithCancel(context.Background())
		r.stopLifecycle = cancel
		r.mu.Lock()
		closing := r.closing
		r.mu.Unlock()
		if closing {
			cancel()
		}
		go r.serve(ctx)
	})
}

// Close stops accepting prover connections, tears down in-flight pipes, and
// waits for them to drain.
func (r *verificationRelay) Close(ctx context.Context) error {
	if r == nil {
		return nil
	}
	r.Start()
	r.stop.Do(func() {
		r.mu.Lock()
		r.closing = true
		r.mu.Unlock()
		if r.stopLifecycle != nil {
			r.stopLifecycle()
		}
		go r.shutdown()
	})
	select {
	case <-r.closed:
		r.mu.Lock()
		err := r.closeErr
		r.mu.Unlock()
		return err
	case <-ctx.Done():
		return fmt.Errorf("tailscale: wait for verification relay: %w", ctx.Err())
	}
}

func (r *verificationRelay) serve(ctx context.Context) {
	defer close(r.done)
	defer r.wg.Wait()
	for {
		connection, err := r.listener.Accept()
		if err != nil {
			r.mu.Lock()
			closing := r.closing
			r.mu.Unlock()
			if !closing && ctx.Err() == nil {
				r.logger.Warn("verification relay stopped accepting connections", "error", err)
			}
			return
		}
		select {
		case r.slots <- struct{}{}:
		default:
			if err := connection.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
				r.logger.Warn("close excess verification connection", "error", err)
			}
			continue
		}
		r.track(connection)
		r.wg.Add(1)
		go r.pipe(ctx, connection)
	}
}

func (r *verificationRelay) pipe(ctx context.Context, inbound net.Conn) {
	defer r.wg.Done()
	defer func() { <-r.slots }()
	defer r.closeTracked(inbound)
	dialCtx, cancel := context.WithTimeout(ctx, forwardDialTimeout)
	defer cancel()
	upstream, err := r.dial(dialCtx, "tcp", r.target)
	if err != nil {
		if ctx.Err() == nil {
			r.logger.Warn("connectivity verification relay target unavailable")
		}
		return
	}
	r.track(upstream)
	defer r.closeTracked(upstream)
	inboundIdle := idleDeadlineConn{Conn: inbound, timeout: forwardIdleTimeout}
	upstreamIdle := idleDeadlineConn{Conn: upstream, timeout: forwardIdleTimeout}
	results := make(chan error, 2)
	go copyConnection(results, upstreamIdle, inboundIdle)
	go copyConnection(results, inboundIdle, upstreamIdle)
	firstErr := <-results
	if err := inbound.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
		r.logger.Debug("close relayed inbound connection", "error", err)
	}
	if err := upstream.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
		r.logger.Debug("close relayed upstream connection", "error", err)
	}
	secondErr := <-results
	for _, copyErr := range []error{firstErr, secondErr} {
		if copyErr != nil && !errors.Is(copyErr, net.ErrClosed) && ctx.Err() == nil {
			r.logger.Debug("copy relayed connection", "error", copyErr)
		}
	}
}

func (r *verificationRelay) shutdown() {
	var errs []error
	r.mu.Lock()
	connections := make([]net.Conn, 0, len(r.connections))
	for connection := range r.connections {
		connections = append(connections, connection)
	}
	r.mu.Unlock()
	for _, connection := range connections {
		if err := connection.Close(); err != nil && !onlyClosedNetworkError(err) {
			errs = append(errs, fmt.Errorf("tailscale: close relayed connection: %w", err))
		}
	}
	if err := r.listener.Close(); err != nil && !onlyClosedNetworkError(err) {
		errs = append(errs, fmt.Errorf("tailscale: close verification relay listener: %w", err))
	}
	// r.done closes only after every in-flight relayed connection returns.
	<-r.done
	r.mu.Lock()
	r.closeErr = errors.Join(errs...)
	r.mu.Unlock()
	close(r.closed)
}

func (r *verificationRelay) track(connection net.Conn) {
	r.mu.Lock()
	r.connections[connection] = struct{}{}
	r.mu.Unlock()
}

func (r *verificationRelay) closeTracked(connection net.Conn) {
	r.mu.Lock()
	delete(r.connections, connection)
	r.mu.Unlock()
	if err := connection.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
		r.logger.Debug("close tracked relayed connection", "error", err)
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

func copyConnection(result chan<- error, destination io.Writer, source io.Reader) {
	_, err := io.Copy(destination, source)
	result <- err
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
