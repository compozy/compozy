package daemon

import (
	"context"

	"fmt"

	bridgepkg "github.com/compozy/agh/internal/bridges"

	"github.com/compozy/agh/internal/notifications"
	presetspkg "github.com/compozy/agh/internal/notifications/presets"
)

func (d *Daemon) bootNotificationPresets(ctx context.Context, state *bootState) error {
	if state == nil || state.registry == nil {
		return nil
	}
	store, ok := state.registry.(presetspkg.Store)
	if !ok {
		if state.logger != nil {
			state.logger.Debug(
				"daemon: skipping notification presets because registry does not expose preset persistence",
			)
		}
		return nil
	}
	cursors, ok := state.registry.(notifications.CursorStore)
	if !ok {
		if state.logger != nil {
			state.logger.Debug(
				"daemon: skipping notification presets because registry does not expose cursor persistence",
			)
		}
		return nil
	}
	service := presetspkg.NewService(presetspkg.Config{
		Store:   store,
		Cursors: cursors,
		Bridges: state.bridges,
		Events:  extensionEventSummaryStore(state.registry),
		Logger:  state.logger,
		Now:     d.now,
		Timeout: state.cfg.Task.Orchestration.BridgeNotificationTimeout,
	})
	if err := service.EnsureBuiltIns(ctx); err != nil {
		return fmt.Errorf("daemon: seed notification preset defaults: %w", err)
	}
	state.notificationPresets = service
	return nil
}

func bridgeRuntimeDedupStore(runtime *bridgeRuntime) bridgeDedupStore {
	if runtime == nil {
		return nil
	}
	return runtime.store
}

func bridgeRuntimeBroker(runtime *bridgeRuntime) *bridgepkg.Broker {
	if runtime == nil {
		return nil
	}
	return runtime.Broker()
}
