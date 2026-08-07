package connectivitytailscale

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"strings"

	"tailscale.com/tsnet"
)

type tailscaleNode interface {
	Up(context.Context) error
	ListenPrivate(context.Context) (net.Listener, error)
	ListenPublic(context.Context) (net.Listener, error)
	CertificateDomains() []string
	Health(context.Context) error
	Close(context.Context) error
}

type tailscaleNodeFactory func(string, *slog.Logger) (tailscaleNode, error)

type tsnetNode struct {
	server *tsnet.Server
	logger *slog.Logger
}

func newTSNetNode(stateDir string, logger *slog.Logger) (tailscaleNode, error) {
	if strings.TrimSpace(stateDir) == "" {
		return nil, errors.New("connectivity-tailscale: state directory is required")
	}
	if err := os.MkdirAll(stateDir, 0o700); err != nil {
		return nil, fmt.Errorf("connectivity-tailscale: create state directory: %w", err)
	}
	if logger == nil {
		logger = slog.Default()
	}
	authKey := strings.TrimSpace(os.Getenv("TS_AUTHKEY"))
	if authKey == "" {
		return nil, errors.New("connectivity-tailscale: TS_AUTHKEY binding is required")
	}
	server := &tsnet.Server{
		Dir:      stateDir,
		Hostname: "compozy-gateway",
		AuthKey:  authKey,
		UserLogf: func(string, ...any) {
			logger.Info("tailscale node status changed")
		},
		Logf: func(string, ...any) {
			logger.Debug("tailscale node diagnostic")
		},
	}
	return &tsnetNode{server: server, logger: logger}, nil
}

func (n *tsnetNode) Up(ctx context.Context) error {
	if n == nil || n.server == nil {
		return errors.New("connectivity-tailscale: node is required")
	}
	if _, err := n.server.Up(ctx); err != nil {
		return fmt.Errorf("connectivity-tailscale: connect operator tailnet: %w", err)
	}
	return nil
}

func (n *tsnetNode) ListenPrivate(ctx context.Context) (net.Listener, error) {
	return listenWithContext(ctx, n.logger, func() (net.Listener, error) {
		return n.server.ListenTLS("tcp", ":8443")
	})
}

func (n *tsnetNode) ListenPublic(ctx context.Context) (net.Listener, error) {
	return listenWithContext(ctx, n.logger, func() (net.Listener, error) {
		return n.server.ListenFunnel("tcp", ":443", tsnet.FunnelOnly())
	})
}

func (n *tsnetNode) CertificateDomains() []string {
	return n.server.CertDomains()
}

func (n *tsnetNode) Health(ctx context.Context) error {
	if n == nil || n.server == nil {
		return errors.New("connectivity-tailscale: node is required")
	}
	client, err := n.server.LocalClient()
	if err != nil {
		return fmt.Errorf("connectivity-tailscale: inspect node health: %w", err)
	}
	status, err := client.StatusWithoutPeers(ctx)
	if err != nil {
		return fmt.Errorf("connectivity-tailscale: inspect node status: %w", err)
	}
	if status.BackendState != "Running" || len(status.Health) > 0 {
		return errors.New("connectivity-tailscale: node is not healthy")
	}
	return nil
}

func (n *tsnetNode) Close(ctx context.Context) error {
	if n == nil || n.server == nil {
		return nil
	}
	result := make(chan error, 1)
	go func() {
		result <- n.server.Close()
	}()
	select {
	case err := <-result:
		return err
	case <-ctx.Done():
		return fmt.Errorf("connectivity-tailscale: close node: %w", ctx.Err())
	}
}

type listenerResult struct {
	listener net.Listener
	err      error
}

func listenWithContext(
	ctx context.Context,
	logger *slog.Logger,
	listen func() (net.Listener, error),
) (net.Listener, error) {
	result := make(chan listenerResult, 1)
	go func() {
		listener, err := listen()
		result <- listenerResult{listener: listener, err: err}
	}()
	select {
	case outcome := <-result:
		return outcome.listener, outcome.err
	case <-ctx.Done():
		go closeLateListener(result, logger)
		return nil, fmt.Errorf("connectivity-tailscale: wait for listener: %w", ctx.Err())
	}
}

func closeLateListener(result <-chan listenerResult, logger *slog.Logger) {
	outcome := <-result
	if outcome.listener != nil {
		if err := outcome.listener.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			logger.Error("close listener opened after deadline", "error", err)
		}
	}
}
