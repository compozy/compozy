package daemon

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/filesnap"
	looppkg "github.com/compozy/agh/internal/loop"
)

func startLoopWatcher(
	ctx context.Context,
	interval time.Duration,
	roots []string,
	rootsProvider func(context.Context) ([]string, error),
	publisher loopResourcePublisher,
	logger *slog.Logger,
) (context.CancelFunc, chan struct{}) {
	if publisher == nil {
		return nil, nil
	}
	if interval <= 0 {
		interval = 3 * time.Second
	}
	if logger == nil {
		logger = slog.Default()
	}
	watcherCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	watcher := &loopWatcher{
		interval:      interval,
		roots:         normalizeLoopWatcherRoots(roots),
		rootsProvider: rootsProvider,
		publisher:     publisher,
		logger:        logger,
		snapshots:     map[string]filesnap.Snapshot{},
	}
	go func() {
		defer close(done)
		watcher.start(watcherCtx)
	}()
	return cancel, done
}

func stopLoopWatcher(ctx context.Context, cancel context.CancelFunc, done <-chan struct{}) error {
	if cancel != nil {
		cancel()
	}
	if done == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func workspaceLoopWatcherRoots(
	homePaths aghconfig.HomePaths,
	registry Registry,
) func(context.Context) ([]string, error) {
	if registry == nil {
		return nil
	}
	return func(ctx context.Context) ([]string, error) {
		workspaces, err := registry.ListWorkspaces(ctx)
		if err != nil {
			return nil, fmt.Errorf("daemon: list workspaces for loop watcher: %w", err)
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
				roots = append(roots, root.LoopsDir())
			}
		}
		return roots, nil
	}
}

type loopWatcher struct {
	interval      time.Duration
	roots         []string
	rootsProvider func(context.Context) ([]string, error)
	publisher     loopResourcePublisher
	logger        *slog.Logger
	initialized   bool
	snapshots     map[string]filesnap.Snapshot
}

func (w *loopWatcher) start(ctx context.Context) {
	if ctx == nil {
		return
	}
	w.logger.Info("loops: watcher started", "roots", w.roots, "interval", w.interval)
	if err := w.pollOnce(ctx); err != nil && !errors.Is(err, context.Canceled) &&
		!errors.Is(err, context.DeadlineExceeded) {
		w.logger.Warn("loops: watcher poll failed", "error", err)
	}
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := w.pollOnce(ctx); err != nil {
				if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
					return
				}
				w.logger.Warn("loops: watcher poll failed", "error", err)
			}
		}
	}
}

func (w *loopWatcher) pollOnce(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	current, err := w.snapshotRoots(ctx)
	if err != nil {
		return err
	}
	if !w.initialized {
		w.snapshots = current
		w.initialized = true
		return nil
	}
	if !loopSnapshotsChanged(w.snapshots, current) {
		return nil
	}
	if err := w.publisher.Sync(ctx); err != nil {
		return err
	}
	w.snapshots = current
	return nil
}

func (w *loopWatcher) snapshotRoots(ctx context.Context) (map[string]filesnap.Snapshot, error) {
	roots, err := w.currentRoots(ctx)
	if err != nil {
		return nil, err
	}
	snapshots := map[string]filesnap.Snapshot{}
	for _, root := range roots {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		paths, err := scanLoopWatcherRoot(ctx, root)
		if err != nil {
			return nil, fmt.Errorf("loops: scan watcher root %q: %w", root, err)
		}
		for _, watchedPath := range paths {
			snapshot, err := filesnap.FromPath(watchedPath)
			if err != nil {
				if errors.Is(err, fs.ErrNotExist) {
					continue
				}
				return nil, fmt.Errorf("loops: snapshot watcher file %q: %w", watchedPath, err)
			}
			snapshots[watchedPath] = snapshot
		}
	}
	return snapshots, nil
}

func (w *loopWatcher) currentRoots(ctx context.Context) ([]string, error) {
	if w.rootsProvider == nil {
		return w.roots, nil
	}
	additionalRoots, err := w.rootsProvider(ctx)
	if err != nil {
		return nil, err
	}
	roots := make([]string, 0, len(w.roots)+len(additionalRoots))
	roots = append(roots, w.roots...)
	roots = append(roots, additionalRoots...)
	return normalizeLoopWatcherRoots(roots), nil
}

func scanLoopWatcherRoot(ctx context.Context, root string) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	trimmed := strings.TrimSpace(root)
	if trimmed == "" {
		return nil, nil
	}
	if _, err := os.Stat(trimmed); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	paths := make([]string, 0)
	if err := filepath.WalkDir(trimmed, func(path string, entry fs.DirEntry, walkErr error) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		if looppkg.IsDefinitionFileName(entry.Name()) {
			paths = append(paths, path)
		}
		return nil
	}); err != nil {
		return nil, err
	}
	slices.Sort(paths)
	return paths, nil
}

func loopSnapshotsChanged(previous, current map[string]filesnap.Snapshot) bool {
	if len(previous) != len(current) {
		return true
	}
	for path, currentSnapshot := range current {
		previousSnapshot, ok := previous[path]
		if !ok {
			return true
		}
		if previousSnapshot.Size != currentSnapshot.Size || !previousSnapshot.ModTime.Equal(currentSnapshot.ModTime) {
			return true
		}
	}
	return false
}

func normalizeLoopWatcherRoots(roots []string) []string {
	normalized := make([]string, 0, len(roots))
	seen := map[string]struct{}{}
	for _, root := range roots {
		trimmed := strings.TrimSpace(root)
		if trimmed == "" {
			continue
		}
		cleaned := filepath.Clean(trimmed)
		if _, exists := seen[cleaned]; exists {
			continue
		}
		seen[cleaned] = struct{}{}
		normalized = append(normalized, cleaned)
	}
	slices.Sort(normalized)
	return normalized
}
