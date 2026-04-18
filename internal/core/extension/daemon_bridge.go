package extensions

import "context"

type daemonHostBridgeContextKey struct{}

// WithDaemonHostBridge binds one daemon-owned host bridge to the run-scope
// bootstrap context so the extension runtime can preserve daemon ownership for
// selected Host API callbacks.
func WithDaemonHostBridge(ctx context.Context, bridge DaemonHostBridge) context.Context {
	if ctx == nil || bridge == nil {
		return ctx
	}
	return context.WithValue(ctx, daemonHostBridgeContextKey{}, bridge)
}

// DaemonHostBridgeFromContext returns the daemon-owned host bridge attached to
// a run-scope bootstrap context, when one has been bound.
func DaemonHostBridgeFromContext(ctx context.Context) DaemonHostBridge {
	if ctx == nil {
		return nil
	}
	bridge, ok := ctx.Value(daemonHostBridgeContextKey{}).(DaemonHostBridge)
	if !ok {
		return nil
	}
	return bridge
}

func daemonHostBridgeFromContext(ctx context.Context) DaemonHostBridge {
	return DaemonHostBridgeFromContext(ctx)
}
