package daemon

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"

	aghconfig "github.com/compozy/agh/internal/config"

	looppkg "github.com/compozy/agh/internal/loop"

	"github.com/compozy/agh/internal/resources"
	"github.com/compozy/agh/internal/windowmanager"

	workspacepkg "github.com/compozy/agh/internal/workspace"
)

func (d *Daemon) bootRuntime(ctx context.Context, state *bootState, cleanup *bootCleanup) error {
	if err := d.bootLockAndSocket(ctx, state, cleanup); err != nil {
		return err
	}
	if err := d.bootRegistryState(ctx, state, cleanup); err != nil {
		return err
	}
	if err := d.bootRuntimeServices(ctx, state, cleanup); err != nil {
		return err
	}
	if err := d.attachRuntimeObserver(ctx, state); err != nil {
		return err
	}
	return nil
}

func (d *Daemon) bootLockAndSocket(
	ctx context.Context,
	state *bootState,
	cleanup *bootCleanup,
) error {
	pid := d.pid()
	lock, err := d.acquireLock(d.homePaths.DaemonLock, pid)
	if err != nil {
		return err
	}
	cleanup.add(func(context.Context) error {
		return lock.Release()
	})
	state.lock = lock

	if stalePID := d.resolveStaleDaemonPID(lock, pid, state.logger); stalePID > 0 {
		if cleanupErr := d.cleanupOrphans(ctx, stalePID); cleanupErr != nil {
			state.logger.Warn(
				"daemon: cleanup orphan processes failed",
				"stale_pid",
				stalePID,
				"error",
				cleanupErr,
			)
		}
	}

	if err := removeStaleSocket(state.cfg.Daemon.Socket); err != nil {
		return err
	}
	return nil
}

func (d *Daemon) resolveStaleDaemonPID(lock *Lock, pid int, logger *slog.Logger) int {
	if lock == nil {
		return 0
	}
	if stalePID := lock.StalePID(); stalePID > 0 {
		return stalePID
	}

	existingInfo, err := ReadInfo(d.homePaths.DaemonInfo)
	switch {
	case err == nil && existingInfo.PID > 0 && existingInfo.PID != pid && !d.processAlive(existingInfo.PID):
		return existingInfo.PID
	case err != nil && !errors.Is(err, os.ErrNotExist) && logger != nil:
		logger.Warn(
			"daemon: read stale daemon info failed",
			"path",
			d.homePaths.DaemonInfo,
			"error",
			err,
		)
	}
	return 0
}

func (d *Daemon) bootRegistryState(
	ctx context.Context,
	state *bootState,
	cleanup *bootCleanup,
) error {
	registry, err := d.openRegistry(ctx, d.homePaths.DatabaseFile)
	if err != nil {
		return fmt.Errorf("daemon: open global database %q: %w", d.homePaths.DatabaseFile, err)
	}
	cleanup.add(func(ctx context.Context) error {
		return registry.Close(ctx)
	})
	if err := d.bootMemoryCatalog(ctx, state, cleanup); err != nil {
		return err
	}

	workspaceResolver, err := workspacepkg.NewResolver(
		registry,
		workspacepkg.WithHomePaths(d.homePaths),
		workspacepkg.WithLogger(state.logger),
		workspacepkg.WithConfigLoader(func(rootDir string) (aghconfig.Config, error) {
			return aghconfig.LoadForHome(d.homePaths, aghconfig.WithWorkspaceRoot(rootDir))
		}),
		workspacepkg.WithChangeHook(func(changeCtx context.Context) error {
			return syncWorkspaceDerivedResources(changeCtx, state)
		}),
	)
	if err != nil {
		return fmt.Errorf("daemon: create workspace resolver: %w", err)
	}
	state.registry = registry
	if err := d.bootDeadEntityRegistry(state); err != nil {
		return err
	}
	if err := reconcileBootNetworkAvailability(ctx, state); err != nil {
		return err
	}
	if state.skillsRegistry != nil {
		state.skillsRegistry.SetEventSummaryStore(registry)
	}
	state.workspaceResolver = workspaceResolver
	if state.harnessRecorder != nil {
		state.harnessRecorder.SetStore(registry)
	}
	if err := d.bootDefaultWorkspaceAndWindowManager(ctx, state, cleanup); err != nil {
		return err
	}
	memoryProviders, err := newDaemonMemoryProviderRegistry(ctx, state)
	if err != nil {
		return fmt.Errorf("daemon: create memory provider registry: %w", err)
	}
	state.memoryProviderRegistry = memoryProviders
	return nil
}

func (d *Daemon) ensureDefaultWorkspace(ctx context.Context, state *bootState) error {
	if state == nil || state.workspaceResolver == nil {
		return errors.New("daemon: workspace resolver is required before default workspace registration")
	}
	operatorHome, err := d.operatorHomeDir()
	if err != nil {
		return fmt.Errorf("daemon: resolve default workspace root: %w", err)
	}
	resolved, err := state.workspaceResolver.ResolveOrRegister(ctx, operatorHome)
	if err != nil {
		return fmt.Errorf("daemon: register default workspace %q: %w", operatorHome, err)
	}
	if state.logger != nil {
		state.logger.Info(
			"daemon: default workspace ready",
			"workspace_id",
			resolved.ID,
			"root_dir",
			resolved.RootDir,
			"name",
			resolved.Name,
		)
	}
	return nil
}

func (d *Daemon) operatorHomeDir() (string, error) {
	getenv := d.getenv
	if getenv == nil {
		getenv = os.Getenv
	}
	return aghconfig.ResolveOperatorHomeDirWithLookup(d.homePaths, func(key string) (string, bool) {
		value := getenv(key)
		return value, strings.TrimSpace(value) != ""
	})
}

func (d *Daemon) bootRuntimeResourceGraph(state *bootState) error {
	resourceKernel, err := d.buildResourceKernel(state.registry)
	if err != nil {
		return err
	}
	state.resourceKernel = resourceKernel
	state.resourceCodecs, err = d.buildResourceCodecs(state.bridges)
	if err != nil {
		return err
	}
	bridgeResources, err := bridgeInstanceResourceStore(
		resourceRawStore(resourceKernel),
		state.resourceCodecs,
	)
	if err != nil {
		return err
	}
	if state.bridges != nil && bridgeResources != nil {
		state.bridges.setResourceDefinitions(
			bridgeResources,
			resourceReconcileActor(),
			func(ctx context.Context, kind resources.ResourceKind, reason resources.ReconcileReason) error {
				if state.resourceReconcile == nil {
					return nil
				}
				return state.resourceReconcile.Trigger(ctx, kind, reason)
			},
		)
	}
	state.agentCatalog = newResourceCatalog(cloneAgentDef)
	state.soulCatalog = newResourceCatalog(cloneSoulResourceSpec)
	state.heartbeatCatalog = newResourceCatalog(cloneHeartbeatResourceSpec)
	state.loopCatalog = newResourceCatalog(looppkg.CloneResourceSpec)
	if state.windowLayoutCatalog == nil {
		state.windowLayoutCatalog = newResourceCatalog(windowmanager.CloneLayoutResource)
	}
	return nil
}
