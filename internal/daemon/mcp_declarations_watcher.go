package daemon

import (
	"context"
	"path/filepath"
	"sync"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/filesnap"
)

const mcpDeclarationWatcherLabel = "MCP declaration watcher"

// mcpDeclarationWatcher republishes authored config layers, including removed files.
// Snapshot advancement follows publication so a failed sync is retried.
type mcpDeclarationWatcher struct {
	mu        sync.Mutex
	paths     func(context.Context) ([]string, error)
	sync      func(context.Context) error
	snapshots map[string]filesnap.Snapshot
}

func (w *mcpDeclarationWatcher) poll(ctx context.Context) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	current, err := w.capture(ctx)
	if err != nil {
		return err
	}
	if w.snapshots != nil && filesnap.Equal(w.snapshots, current) {
		return nil
	}
	if err := w.sync(ctx); err != nil {
		return err
	}
	w.snapshots = current
	return nil
}

func (w *mcpDeclarationWatcher) capture(ctx context.Context) (map[string]filesnap.Snapshot, error) {
	paths, err := w.paths(ctx)
	if err != nil {
		return nil, err
	}
	current := make(map[string]filesnap.Snapshot, len(paths))
	for _, path := range paths {
		snapshot, err := filesnap.FromPath(path)
		if err != nil {
			// Declaration loading retains unreadable scopes. Continue observing healthy
			// layers; recovery of this path will trigger its next reconciliation.
			continue
		}
		current[path] = snapshot
	}
	return current, nil
}

func (d *Daemon) bootMCPDeclarationWatcher(ctx context.Context, state *bootState, cleanup *bootCleanup) {
	if state.toolMCPResources == nil || state.workspaceResolver == nil {
		return
	}
	workerCtx, cancel := context.WithCancel(ctx)
	worker := newOwnedWorkerGroup(cancel)
	complete, ok := worker.Begin()
	if !ok {
		cancel()
		return
	}
	state.runtimeWorkers.mcpDeclarations = worker
	cleanup.add(func(cleanupCtx context.Context) error {
		return stopOwnedWorker(cleanupCtx, mcpDeclarationWatcherLabel, worker)
	})
	watcher := mcpDeclarationWatcher{
		paths: mcpDeclarationPaths(d.homePaths, state.registry, bootProfileCatalog{state: state}),
		sync: func(refreshCtx context.Context) error {
			state.workspaceResolver.InvalidateAll()
			return state.toolMCPResources.Sync(refreshCtx)
		},
	}
	if syncer, ok := state.toolMCPResources.(*toolMCPSourceSyncer); ok {
		syncer.declarationWatch = &watcher
	}
	interval := state.cfg.Skills.PollInterval
	if interval <= 0 {
		interval = 3 * time.Second
	}
	go func() {
		defer complete()
		defer cancel()
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		delay := interval
		for {
			if err := watcher.poll(workerCtx); err != nil && workerCtx.Err() == nil && state.logger != nil {
				// Parsing errors can contain config values; keep the warning secret-free.
				state.logger.WarnContext(workerCtx, "daemon: MCP declaration refresh deferred")
				delay = min(max(delay*2, interval), time.Minute)
			} else {
				delay = interval
			}
			ticker.Reset(delay)
			select {
			case <-workerCtx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
}

func mcpDeclarationPaths(
	home compozyconfig.HomePaths,
	registry workspaceRegistryReader,
	profiles extensionProfileCatalog,
) func(context.Context) ([]string, error) {
	return func(ctx context.Context) ([]string, error) {
		names, err := activeResourceProfileNames(ctx, profiles)
		if err != nil {
			return nil, err
		}
		roots := []string{home.HomeDir}
		if registry != nil {
			workspaces, err := registry.ListWorkspaces(ctx)
			if err != nil {
				return nil, err
			}
			for idx := range workspaces {
				roots = append(roots, filepath.Join(workspaces[idx].RootDir, compozyconfig.DirName))
			}
		}
		paths := []string{home.ConfigFile}
		for _, root := range roots {
			paths = append(
				paths,
				filepath.Join(root, compozyconfig.ConfigName),
				filepath.Join(root, compozyconfig.MCPJSONName),
			)
			for _, name := range names {
				profileRoot := filepath.Join(root, compozyconfig.ProfilesDirName, name)
				paths = append(
					paths,
					filepath.Join(profileRoot, compozyconfig.ConfigName),
					filepath.Join(profileRoot, compozyconfig.MCPJSONName),
				)
			}
		}
		return paths, nil
	}
}
