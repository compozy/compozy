package daemon

import (
	"context"
	"errors"
	"fmt"

	"strings"

	"time"

	core "github.com/compozy/agh/internal/api/core"

	aghconfig "github.com/compozy/agh/internal/config"

	"github.com/compozy/agh/internal/network"

	"github.com/compozy/agh/internal/skills"

	skillbundled "github.com/compozy/agh/skills"
)

func daemonNetworkInfo(
	ctx context.Context,
	cfg aghconfig.NetworkConfig,
	service core.NetworkService,
) (*NetworkInfo, error) {
	if !cfg.Enabled {
		return &NetworkInfo{
			Enabled: false,
			Status:  network.StatusDisabled,
		}, nil
	}
	if service == nil {
		return nil, errors.New("daemon: network service is required when network is enabled")
	}

	status, err := service.Status(ctx)
	if err != nil {
		return nil, fmt.Errorf("daemon: read network status: %w", err)
	}
	if status == nil {
		return nil, errors.New("daemon: network status is required")
	}

	return &NetworkInfo{
		Enabled: status.Enabled,
		Status:  strings.TrimSpace(status.Status),
	}, nil
}

func (d *Daemon) bootFinalize(ctx context.Context, state *bootState) error {
	if state.resourceReconcile != nil {
		if err := state.resourceReconcile.RunBoot(ctx); err != nil {
			return fmt.Errorf("daemon: boot resource reconcile: %w", err)
		}
	}

	d.reconcileDaemonSandboxes(ctx, state)

	reconcileResult, err := state.observer.Reconcile(ctx)
	if err != nil {
		return fmt.Errorf("daemon: reconcile sessions: %w", err)
	}
	state.logger.Info(
		"daemon: boot reconciliation complete",
		"indexed_sessions", len(reconcileResult.Indexed),
		"orphaned_sessions", len(reconcileResult.Orphaned),
	)

	if d.shouldVerifyBoundaries() {
		if boundaryErr := d.Boundaries(ctx); boundaryErr != nil {
			state.logger.Warn("daemon: boundary verification warning", "error", boundaryErr)
		}
	}
	return nil
}

func (d *Daemon) skillsRegistryConfig(cfg *aghconfig.Config) skills.RegistryConfig {
	if cfg == nil {
		return skills.RegistryConfig{
			BundledFS:     skillbundled.FS(),
			UserSkillsDir: d.homePaths.SkillsDir,
		}
	}

	return skills.RegistryConfig{
		BundledFS:      skillbundled.FS(),
		UserSkillsDir:  d.homePaths.SkillsDir,
		UserAgentsDir:  d.homePaths.AgentsDir,
		DisabledSkills: append([]string(nil), cfg.Skills.DisabledSkills...),
	}
}

func startSkillsWatcher(
	ctx context.Context,
	registry *skills.Registry,
	interval time.Duration,
	rootsProvider func(context.Context) ([]string, error),
	afterRefresh func(context.Context) error,
) (context.CancelFunc, chan struct{}) {
	if registry == nil {
		return nil, nil
	}

	watcherCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	watcher := skills.NewWatcher(registry, interval)
	watcher.SetRootsProvider(rootsProvider)
	watcher.SetAfterRefresh(afterRefresh)
	go func() {
		defer close(done)
		watcher.Start(watcherCtx)
	}()
	return cancel, done
}

func workspaceSkillWatcherRoots(
	homePaths aghconfig.HomePaths,
	registry Registry,
) func(context.Context) ([]string, error) {
	if registry == nil {
		return nil
	}

	return func(ctx context.Context) ([]string, error) {
		workspaces, err := registry.ListWorkspaces(ctx)
		if err != nil {
			return nil, fmt.Errorf("daemon: list workspaces for skill watcher: %w", err)
		}

		roots := make([]string, 0, len(workspaces)*2)
		for _, workspace := range workspaces {
			for _, root := range aghconfig.WorkspaceDiscoveryRoots(
				workspace.RootDir,
				workspace.AdditionalDirs,
				homePaths,
			) {
				if root.Source == aghconfig.WorkspaceDiscoverySourceGlobal {
					continue
				}
				roots = append(roots, root.SkillsDir(), root.AgentsDir())
			}
		}

		return roots, nil
	}
}

func stopSkillsWatcher(ctx context.Context, cancel context.CancelFunc, done <-chan struct{}) error {
	if cancel != nil {
		cancel()
	}
	if done == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.TODO()
	}
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func resolveDaemonPort(defaultPort int, server Server) int {
	type portReporter interface {
		Port() int
	}

	if reporter, ok := server.(portReporter); ok && reporter.Port() >= 0 {
		return reporter.Port()
	}
	return defaultPort
}
