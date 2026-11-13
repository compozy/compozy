package mcp

import (
	"context"
	"errors"
	"net"
	"time"
)

const (
	// hostedBindDialRetryWindow bounds bind retries while the daemon's socket is not accepting
	// connections yet. It stays below the daemon's bind nonce TTL so a late bind still succeeds.
	hostedBindDialRetryWindow = 20 * time.Second
	// hostedBindDialRetryInterval is the pause between dial attempts.
	hostedBindDialRetryInterval = 250 * time.Millisecond
)

// bindHostedMCPWithDialRetry binds the proxy, retrying only while the daemon cannot be dialed.
// A provider can start before the daemon listens (boot recovery); failing that first dial made
// the daemon wait out the whole bind deadline. Any answer from the daemon, including a refusal,
// is returned at once.
func bindHostedMCPWithDialRetry(
	ctx context.Context,
	client HostedProxyClient,
	req HostedBindRequest,
) (HostedBindResponse, error) {
	// The window bounds in-flight attempts too, not only the gaps between them.
	windowCtx, cancel := context.WithTimeout(ctx, hostedBindDialRetryWindow)
	defer cancel()
	deadline := time.NewTimer(hostedBindDialRetryWindow)
	defer deadline.Stop()
	for {
		bind, err := client.BindHostedMCP(windowCtx, req)
		if err == nil || !isHostedDialError(err) {
			return bind, err
		}
		retry := time.NewTimer(hostedBindDialRetryInterval)
		select {
		case <-ctx.Done():
			retry.Stop()
			return HostedBindResponse{}, errors.Join(err, ctx.Err())
		case <-deadline.C:
			retry.Stop()
			return HostedBindResponse{}, err
		case <-retry.C:
		}
	}
}

// isHostedDialError reports that no connection to the daemon could be opened.
func isHostedDialError(err error) bool {
	opErr, ok := errors.AsType[*net.OpError](err)
	return ok && opErr.Op == "dial"
}
