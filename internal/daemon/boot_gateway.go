package daemon

import (
	"context"
	"errors"
	"fmt"

	"github.com/compozy/compozy/internal/gateway"
)

func (d *Daemon) bootGateway(ctx context.Context, state *bootState, cleanup *bootCleanup) error {
	if state == nil {
		return errors.New("daemon: boot gateway state is required")
	}
	store, ok := state.registry.(gateway.Store)
	if !ok {
		return errors.New("daemon: registry does not implement the gateway store")
	}
	policy, err := gateway.NewPolicy(store, nil, state.cfg.Gateway.Enabled)
	if err != nil {
		return fmt.Errorf("daemon: create gateway policy: %w", err)
	}
	cleanup.add(policy.Close)
	state.gateway = policy
	if _, err := policy.Reconcile(ctx); err != nil {
		if errors.Is(err, gateway.ErrExposureRefused) {
			state.logger.Warn("daemon: gateway exposure refused; continuing local-only", "error", err)
			return nil
		}
		return fmt.Errorf("daemon: reconcile gateway before server advertisement: %w", err)
	}
	return nil
}
