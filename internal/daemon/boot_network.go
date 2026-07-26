package daemon

import (
	"context"
	"errors"
	"fmt"

	"github.com/compozy/agh/internal/network"
)

func (d *Daemon) bootNetwork(ctx context.Context, state *bootState, cleanup *bootCleanup) error {
	if state == nil {
		return errors.New("daemon: boot network state is required")
	}
	if state.sessions == nil {
		return errors.New("daemon: session manager is required before booting network")
	}

	bindable, ok := state.sessions.(networkBindableSessionManager)
	if !ok {
		return errMissingNetworkBindingSurface
	}
	wakeStore, ok := state.registry.(networkWakeStore)
	if !ok {
		return errors.New("daemon: registry does not implement the network wake store")
	}
	runner, err := newNetworkWakeRunner(
		state.deps.Tasks,
		bindable,
		wakeStore,
		state.logger,
		state.cfg.Limits.MaxConcurrentAgents,
	)
	if err != nil {
		return err
	}
	if err := runner.SetEnabled(ctx, state.cfg.Network.Enabled); err != nil {
		return fmt.Errorf("daemon: configure network wake runner availability: %w", err)
	}

	manager, err := network.NewManager(
		ctx,
		state.cfg.Network,
		d.homePaths.NetworkAuditFile,
		state.registry,
		network.WithManagerLogger(state.logger),
		network.WithManagerHookDispatcher(state.notifier),
		network.WithManagerWakeNotifier(runner),
	)
	if err != nil {
		return fmt.Errorf("daemon: create network manager: %w", err)
	}

	bindable.SetNetworkPeerLifecycle(manager)
	bindable.SetTurnEndNotifier(manager.OnTurnEnd)
	cleanup.add(func(ctx context.Context) error {
		return manager.Shutdown(ctx)
	})
	state.networkWakeRunner = runner
	state.network = manager
	state.deps.Network = manager
	return nil
}

func (d *Daemon) startNetworkWakeRunner(
	ctx context.Context,
	state *bootState,
	cleanup *bootCleanup,
) error {
	if state == nil || state.networkWakeRunner == nil {
		return errors.New("daemon: network wake runner is required before start")
	}
	if err := state.networkWakeRunner.Start(ctx); err != nil {
		return fmt.Errorf("daemon: start network wake runner: %w", err)
	}
	cleanup.add(func(ctx context.Context) error {
		return state.networkWakeRunner.Drain(ctx)
	})
	return nil
}
