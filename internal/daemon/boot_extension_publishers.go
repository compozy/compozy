package daemon

import (
	"context"
	"errors"
	"fmt"

	aghconfig "github.com/compozy/agh/internal/config"

	extensionpkg "github.com/compozy/agh/internal/extension"

	"github.com/compozy/agh/internal/session"

	"github.com/compozy/agh/internal/support"
)

func extensionRuntimeHasRegisteredEntries(
	ctx context.Context,
	registry *extensionpkg.Registry,
	runtime extensionRuntime,
) bool {
	if ctx == nil || registry == nil || runtime == nil {
		return false
	}
	if err := ctx.Err(); err != nil {
		return false
	}

	infos, err := registry.List()
	if err != nil {
		return false
	}

	for _, info := range infos {
		if !info.Enabled {
			continue
		}

		ext, err := runtime.Get(info.Name)
		if err != nil || ext == nil {
			continue
		}
		if ext.Status.Registered {
			return true
		}
	}

	return false
}

func (d *Daemon) bootServers(ctx context.Context, state *bootState, cleanup *bootCleanup) error {
	loopAPI, err := newDaemonLoopAPIService(state, d.homePaths, d.now)
	if err != nil {
		return err
	}
	state.deps.Loops = loopAPI
	if installer, ok := state.sessions.(goalCommandHandlerInstaller); ok {
		if handler, handlerOK := loopAPI.(session.GoalCommandHandler); handlerOK {
			installer.SetGoalCommandHandler(handler)
		}
	}

	httpServer, err := d.httpFactory(ctx, state.deps)
	if err != nil {
		return fmt.Errorf("daemon: create http server: %w", err)
	}
	if err := httpServer.Start(ctx); err != nil {
		return fmt.Errorf("daemon: start http server: %w", err)
	}
	cleanup.add(func(ctx context.Context) error {
		return httpServer.Shutdown(ctx)
	})

	udsServer, err := d.udsFactory(ctx, state.deps)
	if err != nil {
		return fmt.Errorf("daemon: create uds server: %w", err)
	}
	if err := udsServer.Start(ctx); err != nil {
		return fmt.Errorf("daemon: start uds server: %w", err)
	}
	cleanup.add(func(ctx context.Context) error {
		return udsServer.Shutdown(ctx)
	})

	networkInfo, err := daemonNetworkInfo(ctx, state.cfg.Network, state.deps.Network)
	if err != nil {
		return err
	}
	info := Info{
		PID:       d.pid(),
		Port:      resolveDaemonPort(state.cfg.HTTP.Port, httpServer),
		StartedAt: state.startedAt,
		Network:   networkInfo,
	}
	if err := WriteInfo(d.homePaths.DaemonInfo, info); err != nil {
		return err
	}
	cleanup.add(func(context.Context) error {
		return RemoveInfo(d.homePaths.DaemonInfo)
	})

	state.httpServer = httpServer
	state.udsServer = udsServer
	state.info = info
	return nil
}

func (d *Daemon) bootSupportBundles(state *bootState) error {
	if state == nil {
		return errors.New("daemon: boot support bundles state is required")
	}
	configSnapshot := support.ConfigSnapshotFunc(nil)
	if activeSettings, ok := state.deps.Settings.(interface {
		ActiveConfig(context.Context) (aghconfig.Config, error)
	}); ok {
		configSnapshot = func(ctx context.Context) (aghconfig.Config, error) {
			return activeSettings.ActiveConfig(ctx)
		}
	}
	snapshots := d.supportBundleSnapshotHandlers(state)
	state.deps.SupportBundles = support.NewService(&support.Builder{
		HomePaths:      d.homePaths,
		Config:         state.cfg,
		ConfigSnapshot: configSnapshot,
		Now:            d.now,
		Sources: support.Sources{
			Status: func(ctx context.Context) (any, error) {
				return snapshots.StatusSnapshot(ctx)
			},
			Doctor: func(ctx context.Context) (any, error) {
				return snapshots.DoctorSnapshot(ctx)
			},
			Providers: func(ctx context.Context) (any, error) {
				return snapshots.ProviderListSnapshot(ctx)
			},
			ConfigApplyRecords: func(ctx context.Context) (any, error) {
				return snapshots.ConfigApplyRecordsSnapshot(ctx)
			},
			EventSummaries: func(ctx context.Context) (any, error) {
				return snapshots.EventSummariesSnapshot(ctx)
			},
			Sessions: func(ctx context.Context) (any, error) {
				return snapshots.SessionsSnapshot(ctx)
			},
		},
	})
	return nil
}
