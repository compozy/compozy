package tailscale

import (
	"context"
	"errors"
	"log/slog"
	"net"
)

func startVerificationRelay(
	ctx context.Context,
	node tailscaleNode,
	tier, domain string,
	logger *slog.Logger,
) (*tierForwarder, error) {
	if tier != tierPrivate {
		return nil, nil
	}
	listener, err := (&net.ListenConfig{}).Listen(ctx, "tcp", "127.0.0.1:0")
	if err != nil {
		return nil, errors.New("tailscale: cannot bind private verification transport")
	}
	relay, err := newTierForwarder(listener, net.JoinHostPort(domain, privateListenerPort), logger)
	if err != nil {
		return nil, errors.Join(err, closeListener(listener))
	}
	relay.dial = node.Dial
	relay.Start()
	return relay, nil
}
