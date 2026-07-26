package daemon

import (
	"context"
	"fmt"

	"github.com/compozy/agh/internal/store"
)

func networkAvailabilityStoreDependency(registry Registry) store.NetworkAvailabilityStore {
	availability, ok := registry.(store.NetworkAvailabilityStore)
	if !ok {
		return nil
	}
	return availability
}

func reconcileBootNetworkAvailability(ctx context.Context, state *bootState) error {
	if state == nil {
		return fmt.Errorf("daemon: boot network availability state is required")
	}
	availability := networkAvailabilityStoreDependency(state.registry)
	if availability == nil {
		return fmt.Errorf("daemon: boot requires network availability store")
	}
	if _, err := availability.SetNetworkAvailability(
		ctx,
		state.cfg.Network.Enabled,
		"config.reload.boot",
	); err != nil {
		return fmt.Errorf("daemon: reconcile network availability: %w", err)
	}
	return nil
}
