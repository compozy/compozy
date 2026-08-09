package tailscale

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
	ProvisionCertificate(context.Context, string) error
	Health(context.Context) error
	Close(context.Context) error
}

type tailscaleNodeFactory func(string, *slog.Logger) (tailscaleNode, error)

var errAuthKeyBindingRequired = errors.New("tailscale: TS_AUTHKEY binding is required")

type tsnetNode struct {
	server *tsnet.Server
}

func newTSNetNode(stateDir string, logger *slog.Logger) (tailscaleNode, error) {
	if strings.TrimSpace(stateDir) == "" {
		return nil, errors.New("tailscale: state directory is required")
	}
	if err := os.MkdirAll(stateDir, 0o700); err != nil {
		return nil, fmt.Errorf("tailscale: create state directory: %w", err)
	}
	if logger == nil {
		logger = slog.Default()
	}
	authKey := strings.TrimSpace(os.Getenv("TS_AUTHKEY"))
	if authKey == "" {
		return nil, errAuthKeyBindingRequired
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
	return &tsnetNode{server: server}, nil
}

func (n *tsnetNode) Up(ctx context.Context) error {
	if n == nil || n.server == nil {
		return errors.New("tailscale: node is required")
	}
	if _, err := n.server.Up(ctx); err != nil {
		return fmt.Errorf("tailscale: connect operator tailnet: %w", err)
	}
	return nil
}

func (n *tsnetNode) ListenPrivate(ctx context.Context) (net.Listener, error) {
	return listenWithContext(ctx, func() (net.Listener, error) {
		return n.server.ListenTLS("tcp", ":"+privateListenerPort)
	})
}

func (n *tsnetNode) ListenPublic(ctx context.Context) (net.Listener, error) {
	return listenWithContext(ctx, func() (net.Listener, error) {
		return n.server.ListenFunnel("tcp", ":"+publicListenerPort, tsnet.FunnelOnly())
	})
}

func (n *tsnetNode) CertificateDomains() []string {
	return n.server.CertDomains()
}

func (n *tsnetNode) ProvisionCertificate(ctx context.Context, domain string) error {
	if n == nil || n.server == nil {
		return errors.New("tailscale: node is required")
	}
	client, err := n.server.LocalClient()
	if err != nil {
		return fmt.Errorf("tailscale: access local certificate service: %w", err)
	}
	certificatePEM, privateKeyPEM, err := client.CertPair(ctx, domain)
	clear(certificatePEM)
	clear(privateKeyPEM)
	if err != nil {
		return fmt.Errorf("tailscale: provision HTTPS certificate: %w", err)
	}
	return nil
}

func (n *tsnetNode) Health(ctx context.Context) error {
	if n == nil || n.server == nil {
		return errors.New("tailscale: node is required")
	}
	client, err := n.server.LocalClient()
	if err != nil {
		return fmt.Errorf("tailscale: inspect node health: %w", err)
	}
	status, err := client.StatusWithoutPeers(ctx)
	if err != nil {
		return fmt.Errorf("tailscale: inspect node status: %w", err)
	}
	if status.BackendState != "Running" || len(status.Health) > 0 {
		return errors.New("tailscale: node is not healthy")
	}
	return nil
}

func (n *tsnetNode) Close(ctx context.Context) error {
	if n == nil || n.server == nil {
		return nil
	}
	if err := runContextBoundOperation(ctx, n.server.Close); err != nil {
		return fmt.Errorf("tailscale: close node: %w", err)
	}
	return nil
}

func listenWithContext(
	ctx context.Context,
	listen func() (net.Listener, error),
) (net.Listener, error) {
	if ctx == nil || listen == nil {
		return nil, errors.New("tailscale: listener dependencies are required")
	}
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("tailscale: wait for listener: %w", err)
	}
	listener, err := listen()
	if err != nil {
		return nil, err
	}
	if listener == nil {
		return nil, errors.New("tailscale: listener is required")
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		closeErr := listener.Close()
		if errors.Is(closeErr, net.ErrClosed) {
			closeErr = nil
		}
		return nil, errors.Join(
			fmt.Errorf("tailscale: wait for listener: %w", ctxErr),
			closeErr,
		)
	}
	return listener, nil
}

func runContextBoundOperation(ctx context.Context, operation func() error) error {
	if ctx == nil || operation == nil {
		return errors.New("tailscale: operation dependencies are required")
	}
	operationErr := operation()
	return errors.Join(operationErr, ctx.Err())
}
