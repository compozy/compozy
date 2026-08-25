package skills

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/compozy/compozy/internal/filesnap"
)

const (
	watcherAddedKey    = "added"
	watcherModifiedKey = "modified"
)

const defaultWatcherInterval = 3 * time.Second

type globalRefresher interface {
	RefreshGlobal(ctx context.Context) error
}

type fileChange struct {
	path   string
	action string
}

// Watcher polls global skill directories and refreshes the registry when their
// SKILL.md snapshots change.
type Watcher struct {
	registry globalRefresher
	interval time.Duration
	roots    []string
	logger   *slog.Logger

	afterRefresh       func(context.Context) error
	rootsProvider      func(context.Context) ([]string, error)
	agentRoots         []string
	agentRootsProvider func(context.Context) ([]string, error)
	configuredRoots    func() ([]string, []string)

	mu          sync.Mutex
	initialized bool
	snapshots   map[string]filesnap.Snapshot
}

// NewWatcher constructs a watcher that polls the registry's global skill
// directories. A non-positive interval falls back to the default poll interval.
func NewWatcher(registry *Registry, interval time.Duration) *Watcher {
	var roots []string
	var agentRoots []string
	snapshots := make(map[string]filesnap.Snapshot)
	initialized := false
	if registry != nil {
		roots, agentRoots = registry.watcherRootsSnapshot()
		snapshots, initialized = registry.globalSnapshotState()
	}

	watcher := &Watcher{
		registry:    registry,
		interval:    watcherInterval(interval),
		roots:       roots,
		agentRoots:  agentRoots,
		logger:      slog.Default(),
		initialized: initialized,
		snapshots:   snapshots,
	}
	if registry != nil {
		watcher.configuredRoots = registry.watcherRootsSnapshot
	}
	return watcher
}

// SetAgentRootsProvider installs the explicit agent-definition roots watched on each poll.
func (w *Watcher) SetAgentRootsProvider(provider func(context.Context) ([]string, error)) {
	if w == nil {
		return
	}
	w.agentRootsProvider = provider
}

func newWatcher(registry globalRefresher, interval time.Duration, roots []string) *Watcher {
	return &Watcher{
		registry:  registry,
		interval:  watcherInterval(interval),
		roots:     watcherRoots(roots...),
		logger:    slog.Default(),
		snapshots: make(map[string]filesnap.Snapshot),
	}
}

// SetAfterRefresh installs an optional callback that runs after a successful
// registry refresh and before watcher snapshots are committed.
func (w *Watcher) SetAfterRefresh(callback func(context.Context) error) {
	if w == nil {
		return
	}
	w.afterRefresh = callback
}

// SetRootsProvider installs an optional callback that contributes additional
// directories to watch on each poll, such as workspace-local skill roots.
func (w *Watcher) SetRootsProvider(provider func(context.Context) ([]string, error)) {
	if w == nil {
		return
	}
	w.rootsProvider = provider
}

// Start runs the polling loop until the provided context is canceled.
func (w *Watcher) Start(ctx context.Context) {
	if ctx == nil {
		return
	}

	if w.logger == nil {
		w.logger = slog.Default()
	}

	w.logger.Info("skills: watcher started", "roots", w.roots, "interval", w.interval)
	if err := w.pollOnce(
		ctx,
	); err != nil && !errors.Is(err, context.Canceled) &&
		!errors.Is(err, context.DeadlineExceeded) {
		w.logger.Warn("skills: watcher poll failed", "error", err)
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
				w.logger.Warn("skills: watcher poll failed", "error", err)
			}
		}
	}
}

func (w *Watcher) pollOnce(ctx context.Context) error {
	changed, snapshots, changes, err := w.detectChanges(ctx)
	if err != nil {
		return err
	}
	if !changed {
		return nil
	}

	for _, change := range changes {
		w.logger.Debug("skills: watcher detected change", "path", change.path, "action", change.action)
	}

	if err := checkRegistryContext(ctx); err != nil {
		return err
	}
	if w.registry != nil {
		if err := w.registry.RefreshGlobal(ctx); err != nil {
			return fmt.Errorf("skills: refresh global registry: %w", err)
		}
	}
	if err := checkRegistryContext(ctx); err != nil {
		return err
	}
	if w.afterRefresh != nil {
		if err := w.afterRefresh(ctx); err != nil {
			return fmt.Errorf("skills: run watcher refresh callback: %w", err)
		}
	}
	if err := checkRegistryContext(ctx); err != nil {
		return err
	}

	w.commitSnapshots(snapshots)
	return nil
}

func (w *Watcher) detectChanges(ctx context.Context) (bool, map[string]filesnap.Snapshot, []fileChange, error) {
	if err := checkRegistryContext(ctx); err != nil {
		return false, nil, nil, err
	}

	current, err := w.snapshotRoots(ctx)
	if err != nil {
		return false, nil, nil, err
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.initialized {
		w.snapshots = current
		w.initialized = true
		return false, nil, nil, nil
	}

	changes := diffSnapshots(w.snapshots, current)
	if len(changes) == 0 {
		return false, nil, nil, nil
	}

	return true, current, changes, nil
}

func (w *Watcher) commitSnapshots(snapshots map[string]filesnap.Snapshot) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.snapshots = snapshots
	if w.snapshots == nil {
		w.snapshots = make(map[string]filesnap.Snapshot)
	}
	w.initialized = true
}

func (w *Watcher) snapshotRoots(ctx context.Context) (map[string]filesnap.Snapshot, error) {
	roots, agentRoots, err := w.currentRoots(ctx)
	if err != nil {
		return nil, err
	}

	snapshots := make(map[string]filesnap.Snapshot)
	for _, root := range roots {
		if err := checkRegistryContext(ctx); err != nil {
			return nil, err
		}

		_, isAgentRoot := agentRoots[root]
		rootSnapshots, err := watcherSnapshotRoot(root, isAgentRoot)
		if err != nil {
			return nil, fmt.Errorf("skills: scan watcher root %q: %w", root, err)
		}
		maps.Copy(snapshots, rootSnapshots)
	}

	return snapshots, nil
}

func watcherSnapshotRoot(root string, agentRoot bool) (map[string]filesnap.Snapshot, error) {
	if !agentRoot {
		paths, snapshots, err := scanDirectoryWithSnapshots(root)
		if err != nil {
			return nil, err
		}
		if err := recordSidecarSnapshots(paths, snapshots); err != nil {
			return nil, err
		}
		return snapshots, nil
	}

	paths, err := watcherScanRoot(root, true)
	if err != nil {
		return nil, err
	}

	snapshots := make(map[string]filesnap.Snapshot, len(paths))
	skillPaths := make([]string, 0, len(paths))
	for _, watchedPath := range paths {
		snapshot, err := filesnap.FromPath(watchedPath)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			return nil, fmt.Errorf("skills: snapshot watcher file %q: %w", watchedPath, err)
		}

		snapshots[watchedPath] = snapshot
		if filepath.Base(watchedPath) == skillFileName {
			skillPaths = append(skillPaths, watchedPath)
		}
	}

	if err := recordSidecarSnapshots(skillPaths, snapshots); err != nil {
		return nil, err
	}
	return snapshots, nil
}

func watcherScanRoot(root string, agentRoot bool) ([]string, error) {
	if !agentRoot {
		return scanDirectory(root)
	}

	trimmedRoot := strings.TrimSpace(root)
	if trimmedRoot == "" {
		return nil, nil
	}

	if _, err := os.Stat(trimmedRoot); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}

	paths := make([]string, 0)
	err := filepath.WalkDir(trimmedRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		base := entry.Name()
		if base == "AGENT.md" || base == skillFileName {
			paths = append(paths, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	slices.Sort(paths)
	return paths, nil
}

func (w *Watcher) currentRoots(ctx context.Context) ([]string, map[string]struct{}, error) {
	if w == nil {
		return nil, nil, nil
	}
	additionalRoots, err := resolveWatcherRootsProvider(ctx, w.rootsProvider, "skill")
	if err != nil {
		return nil, nil, err
	}
	additionalAgentRoots, err := resolveWatcherRootsProvider(ctx, w.agentRootsProvider, skillAgentKind)
	if err != nil {
		return nil, nil, err
	}
	baseRoots := append([]string(nil), w.roots...)
	baseAgentRoots := append([]string(nil), w.agentRoots...)
	if w.configuredRoots != nil {
		baseRoots, baseAgentRoots = w.configuredRoots()
	}
	agentRoots := watcherRoots(append(baseAgentRoots, additionalAgentRoots...)...)
	roots := make([]string, 0, len(baseRoots)+len(additionalRoots)+len(agentRoots))
	roots = append(roots, baseRoots...)
	roots = append(roots, additionalRoots...)
	roots = append(roots, agentRoots...)
	agentSet := make(map[string]struct{}, len(agentRoots))
	for _, root := range agentRoots {
		agentSet[root] = struct{}{}
	}
	return watcherRoots(roots...), agentSet, nil
}

func (r *Registry) watcherRootsSnapshot() ([]string, []string) {
	if r == nil {
		return nil, nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	skillRoots := make([]string, 0, len(r.cfg.GlobalSkillRoots)+1)
	for _, root := range r.cfg.GlobalSkillRoots {
		skillRoots = append(skillRoots, root.Dir)
	}
	agentRoots := watcherRoots(r.cfg.GlobalAgentsDir)
	skillRoots = append(skillRoots, agentRoots...)
	return watcherRoots(skillRoots...), agentRoots
}

func resolveWatcherRootsProvider(
	ctx context.Context,
	provider func(context.Context) ([]string, error),
	kind string,
) ([]string, error) {
	if provider == nil {
		return nil, nil
	}
	roots, err := provider(ctx)
	if err != nil {
		return nil, fmt.Errorf("skills: resolve %s watcher roots: %w", kind, err)
	}
	return roots, nil
}

func diffSnapshots(previous, current map[string]filesnap.Snapshot) []fileChange {
	changes := make([]fileChange, 0)

	for path, snapshot := range current {
		previousSnapshot, ok := previous[path]
		if !ok {
			changes = append(changes, fileChange{path: path, action: watcherAddedKey})
			continue
		}

		if snapshot.Size != previousSnapshot.Size || !snapshot.ModTime.Equal(previousSnapshot.ModTime) {
			changes = append(changes, fileChange{path: path, action: watcherModifiedKey})
		}
	}

	for path := range previous {
		if _, ok := current[path]; ok {
			continue
		}
		changes = append(changes, fileChange{path: path, action: "deleted"})
	}

	slices.SortFunc(changes, func(left, right fileChange) int {
		if cmp := strings.Compare(left.path, right.path); cmp != 0 {
			return cmp
		}
		return strings.Compare(left.action, right.action)
	})

	return changes
}

func watcherInterval(interval time.Duration) time.Duration {
	if interval <= 0 {
		return defaultWatcherInterval
	}

	return interval
}

func watcherRoots(roots ...string) []string {
	if len(roots) == 0 {
		return nil
	}

	normalized := make([]string, 0, len(roots))
	seen := make(map[string]struct{}, len(roots))
	for _, root := range roots {
		trimmed := strings.TrimSpace(root)
		if trimmed == "" {
			continue
		}

		absRoot, err := filepath.Abs(trimmed)
		if err != nil {
			absRoot = trimmed
		}
		if _, ok := seen[absRoot]; ok {
			continue
		}

		seen[absRoot] = struct{}{}
		normalized = append(normalized, absRoot)
	}

	slices.Sort(normalized)
	return normalized
}
