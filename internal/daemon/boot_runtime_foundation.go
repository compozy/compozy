package daemon

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
	extensionpkg "github.com/compozy/compozy/internal/extension"

	looppkg "github.com/compozy/compozy/internal/loop"
	mcppkg "github.com/compozy/compozy/internal/mcp"

	"github.com/compozy/compozy/internal/resources"
	"github.com/compozy/compozy/internal/store/globaldb"
	"github.com/compozy/compozy/internal/windowmanager"

	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

func (d *Daemon) bootRuntime(ctx context.Context, state *bootState, cleanup *bootCleanup) error {
	if err := d.bootRuntimeServices(ctx, state, cleanup); err != nil {
		return err
	}
	if err := d.attachRuntimeObserver(ctx, state); err != nil {
		return err
	}
	return nil
}

func (d *Daemon) bootRuntimeFoundation(ctx context.Context, state *bootState, cleanup *bootCleanup) error {
	if state.mcpRuntimeHealth == nil {
		state.mcpRuntimeHealth = mcppkg.NewRuntimeHealthRegistry()
	}
	if err := d.bootLockAndSocket(ctx, state, cleanup); err != nil {
		return err
	}
	if err := d.bootRegistryState(ctx, state, cleanup); err != nil {
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
	startedAt, err := d.processStartedAt(pid)
	if err != nil {
		return fmt.Errorf("daemon: read process %d start time: %w", pid, err)
	}
	state.startedAt = startedAt.UTC()

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
	if err := registry.VerifyDefaultProfile(ctx); err != nil {
		return fmt.Errorf("daemon: verify default profile: %w", err)
	}
	if err := d.bootMemoryCatalog(ctx, state, cleanup); err != nil {
		return err
	}
	if profileStore, ok := registry.(*globaldb.GlobalDB); ok {
		profiles, err := d.bootProfiles(state, profileStore)
		if err != nil {
			return err
		}
		state.profiles = profiles
		state.deps.Profiles = profiles
		d.providerPreStarter.SetProfileAvailabilityChecker(profiles)
	}
	workspaceResolver, err := d.newBootWorkspaceResolver(state, registry)
	if err != nil {
		return err
	}
	state.registry = registry
	if bindings, ok := any(registry).(extensionpkg.EnvBindingStore); ok {
		state.extensionEnvBindings = bindings
	}
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
	if err := d.bootWindowManager(ctx, state, cleanup); err != nil {
		return err
	}
	if err := recoverProfiles(ctx, state); err != nil {
		return err
	}
	memoryProviders, err := newDaemonMemoryProviderRegistry(ctx, state)
	if err != nil {
		return fmt.Errorf("daemon: create memory provider registry: %w", err)
	}
	state.memoryProviderRegistry = memoryProviders
	return nil
}

func (d *Daemon) newBootWorkspaceResolver(state *bootState, registry Registry) (*workspacepkg.Resolver, error) {
	getenv := d.getenv
	if getenv == nil {
		getenv = os.Getenv
	}
	operatorHome, err := compozyconfig.ResolveOperatorHomeDirWithLookup(
		d.homePaths,
		func(key string) (string, bool) {
			value := getenv(key)
			return value, strings.TrimSpace(value) != ""
		},
	)
	if err != nil {
		return nil, fmt.Errorf("daemon: resolve operator home: %w", err)
	}
	resolver, err := workspacepkg.NewResolver(
		registry,
		workspacepkg.WithHomePaths(d.homePaths),
		workspacepkg.WithOperatorHomeDir(operatorHome),
		workspacepkg.WithLogger(state.logger),
		workspacepkg.WithConfigLoader(func(rootDir string) (compozyconfig.Config, error) {
			return compozyconfig.LoadForHome(d.homePaths, compozyconfig.WithWorkspaceRoot(rootDir))
		}),
		workspacepkg.WithProfileConfigLoader(func(rootDir string, profileName string) (compozyconfig.Config, error) {
			return compozyconfig.LoadForHome(
				d.homePaths, compozyconfig.WithWorkspaceRoot(rootDir), compozyconfig.WithProfile(profileName),
			)
		}),
		workspacepkg.WithProfileAvailabilityChecker(state.profiles),
		workspacepkg.WithChangeHook(func(changeCtx context.Context) error {
			return syncWorkspaceDerivedResources(changeCtx, state)
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("daemon: create workspace resolver: %w", err)
	}
	return resolver, nil
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
				_, err := state.resourceReconcile.Trigger(ctx, kind, reason)
				return err
			},
		)
	}
	state.agentCatalog = newResourceCatalog(cloneAgentDef)
	state.soulCatalog = newResourceCatalog(cloneSoulResourceSpec)
	state.heartbeatCatalog = newResourceCatalog(cloneHeartbeatResourceSpec)
	state.loopCatalog = newResourceCatalog(looppkg.CloneResourceSpec)
	bindToolProjectionCatalogInvalidation(state)
	if state.windowLayoutCatalog == nil {
		state.windowLayoutCatalog = newResourceCatalog(windowmanager.CloneLayoutResource)
	}
	return nil
}
