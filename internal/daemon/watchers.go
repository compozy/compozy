package daemon

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/compozy/compozy/internal/core/reviews"
	"github.com/compozy/compozy/internal/core/tasks"
	"github.com/fsnotify/fsnotify"
)

const defaultWatcherDebounce = 500 * time.Millisecond

type artifactSyncEvent struct {
	RelativePath string
	ChangeKind   string
	Checksum     string
}

type workflowWatcherConfig struct {
	WorkflowRoot string
	Debounce     time.Duration
	Sync         func(context.Context, string) error
	Emit         func(context.Context, artifactSyncEvent) error
	Logger       *slog.Logger
}

type workflowWatcher struct {
	workflowRoot string
	debounce     time.Duration
	syncFn       func(context.Context, string) error
	emitFn       func(context.Context, artifactSyncEvent) error
	logger       *slog.Logger

	stopOnce sync.Once
	stopCh   chan struct{}
	done     chan struct{}

	errMu sync.Mutex
	err   error
}

type workflowWatchState struct {
	pending        map[string]artifactSyncEvent
	refreshWatches bool
	watchedDirs    map[string]struct{}
}

type watcherDebounce struct {
	timer  *time.Timer
	active bool
}

func startWorkflowWatcher(ctx context.Context, cfg workflowWatcherConfig) (*workflowWatcher, error) {
	workflowRoot := filepath.Clean(strings.TrimSpace(cfg.WorkflowRoot))
	if workflowRoot == "" {
		return nil, errors.New("daemon: workflow watcher root is required")
	}
	if cfg.Sync == nil {
		return nil, errors.New("daemon: workflow watcher sync function is required")
	}

	debounce := cfg.Debounce
	if debounce <= 0 {
		debounce = defaultWatcherDebounce
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("daemon: create workflow watcher: %w", err)
	}

	watchedDirs, err := reconcileWorkflowWatches(watcher, workflowRoot, nil)
	if err != nil {
		_ = watcher.Close()
		return nil, err
	}

	runner := &workflowWatcher{
		workflowRoot: workflowRoot,
		debounce:     debounce,
		syncFn:       cfg.Sync,
		emitFn:       cfg.Emit,
		logger:       cfg.Logger,
		stopCh:       make(chan struct{}),
		done:         make(chan struct{}),
	}
	if runner.logger == nil {
		runner.logger = slog.Default()
	}

	go runner.run(ctx, watcher, watchedDirs)
	return runner, nil
}

func (w *workflowWatcher) Stop() error {
	if w == nil {
		return nil
	}
	w.stopOnce.Do(func() {
		close(w.stopCh)
	})
	<-w.done
	return w.stopError()
}

func (w *workflowWatcher) run(ctx context.Context, watcher *fsnotify.Watcher, watchedDirs map[string]struct{}) {
	defer close(w.done)
	defer func() {
		if err := watcher.Close(); err != nil {
			w.recordError(fmt.Errorf("daemon: close workflow watcher: %w", err))
		}
	}()

	state := &workflowWatchState{
		pending:     make(map[string]artifactSyncEvent),
		watchedDirs: watchedDirs,
	}
	debounce := newWatcherDebounce(w.debounce)
	defer debounce.stop()

	for {
		select {
		case <-ctx.Done():
			w.stopAndFlush(ctx, watcher, debounce, state)
			return
		case <-w.stopCh:
			w.stopAndFlush(ctx, watcher, debounce, state)
			return
		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			w.handleBackendError(err)
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			if w.queueWatchEvent(state, event) {
				debounce.reset(w.debounce)
			}
		case <-debounce.channel():
			debounce.deactivate()
			w.flushPendingChanges(detachContext(ctx), watcher, state)
		}
	}
}

func newWatcherDebounce(duration time.Duration) *watcherDebounce {
	timer := time.NewTimer(duration)
	if !timer.Stop() {
		<-timer.C
	}
	return &watcherDebounce{timer: timer}
}

func (d *watcherDebounce) channel() <-chan time.Time {
	if d == nil || !d.active {
		return nil
	}
	return d.timer.C
}

func (d *watcherDebounce) reset(duration time.Duration) {
	if d == nil {
		return
	}
	d.stop()
	d.timer.Reset(duration)
	d.active = true
}

func (d *watcherDebounce) deactivate() {
	if d == nil {
		return
	}
	d.active = false
}

func (d *watcherDebounce) stop() {
	if d == nil || !d.active {
		return
	}
	if !d.timer.Stop() {
		select {
		case <-d.timer.C:
		default:
		}
	}
	d.active = false
}

func (w *workflowWatcher) stopAndFlush(
	ctx context.Context,
	watcher *fsnotify.Watcher,
	debounce *watcherDebounce,
	state *workflowWatchState,
) {
	debounce.stop()
	w.flushPendingChanges(detachContext(ctx), watcher, state)
}

func (w *workflowWatcher) handleBackendError(err error) {
	if err == nil {
		return
	}
	w.recordError(fmt.Errorf("daemon: workflow watcher error: %w", err))
	w.logWarn("daemon: workflow watcher backend error", "root", w.workflowRoot, "error", err)
}

func (w *workflowWatcher) queueWatchEvent(state *workflowWatchState, event fsnotify.Event) bool {
	if state == nil {
		return false
	}
	change, relevant, dirChanged := classifyWatchEvent(w.workflowRoot, state.watchedDirs, event)
	if !relevant && !dirChanged {
		return false
	}
	if dirChanged {
		state.refreshWatches = true
	}
	if relevant {
		state.pending[change.RelativePath] = change
	}
	return true
}

func (w *workflowWatcher) flushPendingChanges(
	flushCtx context.Context,
	watcher *fsnotify.Watcher,
	state *workflowWatchState,
) {
	if state == nil || (len(state.pending) == 0 && !state.refreshWatches) {
		return
	}

	changes := sortArtifactSyncEvents(state.pending)
	state.pending = make(map[string]artifactSyncEvent)
	refreshNeeded := state.refreshWatches
	state.refreshWatches = false

	// When a directory moves, newly written files inside the renamed tree can race
	// with the next sync if the backend watch list is refreshed only afterward.
	// Refresh the watch set before syncing so follow-up writes land on a watched
	// path, then reconcile once more after sync to converge with the final tree.
	if refreshNeeded {
		if !w.reconcileWatchState(watcher, state, "daemon: refresh workflow watch list before sync") {
			return
		}
	}

	if err := w.syncFn(flushCtx, w.workflowRoot); err != nil {
		w.logWarn("daemon: workflow watcher sync failed", "root", w.workflowRoot, "error", err)
		if refreshNeeded {
			w.reconcileWatchState(watcher, state, "daemon: reconcile workflow watch list after sync failure")
		}
		return
	}

	if !w.reconcileWatchState(watcher, state, "daemon: reconcile workflow watch list") {
		return
	}
	w.emitPendingChanges(flushCtx, changes)
}

func (w *workflowWatcher) reconcileWatchState(
	watcher *fsnotify.Watcher,
	state *workflowWatchState,
	warnMessage string,
) bool {
	nextWatched, err := reconcileWorkflowWatches(watcher, w.workflowRoot, state.watchedDirs)
	if err != nil {
		w.recordError(err)
		w.logWarn(warnMessage, "root", w.workflowRoot, "error", err)
		return false
	}
	state.watchedDirs = nextWatched
	return true
}

func (w *workflowWatcher) emitPendingChanges(
	ctx context.Context,
	changes []artifactSyncEvent,
) {
	if w.emitFn == nil {
		return
	}
	for _, change := range changes {
		change.Checksum = artifactChecksum(w.workflowRoot, change.RelativePath)
		if err := w.emitFn(ctx, change); err != nil {
			w.recordError(err)
			w.logWarn(
				"daemon: emit workflow artifact sync event",
				"root",
				w.workflowRoot,
				"path",
				change.RelativePath,
				"error",
				err,
			)
		}
	}
}

func (w *workflowWatcher) recordError(err error) {
	if w == nil || err == nil {
		return
	}
	w.errMu.Lock()
	defer w.errMu.Unlock()
	w.err = errors.Join(w.err, err)
}

func (w *workflowWatcher) stopError() error {
	if w == nil {
		return nil
	}
	w.errMu.Lock()
	defer w.errMu.Unlock()
	return w.err
}

func (w *workflowWatcher) logWarn(msg string, args ...any) {
	if w == nil || w.logger == nil {
		return
	}
	w.logger.Warn(msg, args...)
}

func classifyWatchEvent(
	workflowRoot string,
	watchedDirs map[string]struct{},
	event fsnotify.Event,
) (artifactSyncEvent, bool, bool) {
	path := filepath.Clean(strings.TrimSpace(event.Name))
	if path == "" {
		return artifactSyncEvent{}, false, false
	}

	relativePath, ok := workflowRelativePath(workflowRoot, path)
	if !ok {
		return artifactSyncEvent{}, false, false
	}

	isDirectory := watchEventTargetsDirectory(path, watchedDirs, event)
	if isDirectory {
		return artifactSyncEvent{}, false, true
	}
	if !isRelevantWorkflowArtifact(relativePath) {
		return artifactSyncEvent{}, false, false
	}
	return artifactSyncEvent{
		RelativePath: relativePath,
		ChangeKind:   classifyWatchChangeKind(event),
	}, true, false
}

func workflowRelativePath(workflowRoot string, path string) (string, bool) {
	root := filepath.Clean(strings.TrimSpace(workflowRoot))
	cleanPath := filepath.Clean(strings.TrimSpace(path))
	if root == "" || cleanPath == "" {
		return "", false
	}

	relativePath, err := filepath.Rel(root, cleanPath)
	if err != nil {
		return "", false
	}
	relativePath = filepath.ToSlash(relativePath)
	if relativePath == "." || strings.HasPrefix(relativePath, "../") {
		return "", false
	}
	return relativePath, true
}

func watchEventTargetsDirectory(path string, watchedDirs map[string]struct{}, event fsnotify.Event) bool {
	cleanPath := filepath.Clean(strings.TrimSpace(path))
	if cleanPath == "" {
		return false
	}
	if _, ok := watchedDirs[cleanPath]; ok {
		return true
	}
	if event.Has(fsnotify.Create) {
		info, err := os.Stat(cleanPath)
		return err == nil && info.IsDir()
	}
	return false
}

func isRelevantWorkflowArtifact(relativePath string) bool {
	clean := filepath.ToSlash(strings.TrimSpace(relativePath))
	base := filepath.Base(clean)
	if !strings.HasSuffix(strings.ToLower(base), ".md") {
		return false
	}

	switch {
	case clean == "_meta.md", clean == "_tasks.md", clean == "_prd.md", clean == "_techspec.md":
		return true
	case tasks.ExtractTaskNumber(base) > 0 && !strings.Contains(clean, "/"):
		return true
	case strings.HasPrefix(clean, "adrs/"),
		strings.HasPrefix(clean, "memory/"),
		strings.HasPrefix(clean, "qa/"),
		strings.HasPrefix(clean, "prompt/"),
		strings.HasPrefix(clean, "prompts/"),
		strings.HasPrefix(clean, "protocol/"),
		strings.HasPrefix(clean, "protocols/"):
		return true
	default:
		topLevel := clean
		if idx := strings.IndexRune(clean, '/'); idx >= 0 {
			topLevel = clean[:idx]
		}
		if strings.HasPrefix(topLevel, "reviews-") {
			return filepath.Base(clean) == "_meta.md" || reviews.ExtractIssueNumber(filepath.Base(clean)) > 0
		}
		return false
	}
}

func classifyWatchChangeKind(event fsnotify.Event) string {
	switch {
	case event.Has(fsnotify.Remove):
		return "remove"
	case event.Has(fsnotify.Rename):
		return "rename"
	case event.Has(fsnotify.Create):
		return "create"
	case event.Has(fsnotify.Write):
		return "write"
	case event.Has(fsnotify.Chmod):
		return "chmod"
	default:
		return "update"
	}
}

func sortArtifactSyncEvents(pending map[string]artifactSyncEvent) []artifactSyncEvent {
	changes := make([]artifactSyncEvent, 0, len(pending))
	for _, change := range pending {
		changes = append(changes, change)
	}
	sort.Slice(changes, func(i, j int) bool {
		if changes[i].RelativePath == changes[j].RelativePath {
			return changes[i].ChangeKind < changes[j].ChangeKind
		}
		return changes[i].RelativePath < changes[j].RelativePath
	})
	return changes
}

func artifactChecksum(workflowRoot string, relativePath string) string {
	path := filepath.Join(workflowRoot, filepath.FromSlash(strings.TrimSpace(relativePath)))
	content, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}

func reconcileWorkflowWatches(
	watcher *fsnotify.Watcher,
	workflowRoot string,
	current map[string]struct{},
) (map[string]struct{}, error) {
	directories, err := discoverWorkflowWatchDirs(workflowRoot)
	if err != nil {
		return nil, err
	}

	next := make(map[string]struct{}, len(directories))
	for _, dir := range directories {
		next[dir] = struct{}{}
		if current != nil {
			if _, ok := current[dir]; ok {
				continue
			}
		}
		if err := watcher.Add(dir); err != nil {
			return nil, fmt.Errorf("daemon: watch workflow directory %s: %w", dir, err)
		}
	}

	for dir := range current {
		if _, ok := next[dir]; ok {
			continue
		}
		if err := watcher.Remove(dir); err != nil && !errors.Is(err, fsnotify.ErrNonExistentWatch) {
			return nil, fmt.Errorf("daemon: remove workflow watch %s: %w", dir, err)
		}
	}
	return next, nil
}

func discoverWorkflowWatchDirs(workflowRoot string) ([]string, error) {
	root := filepath.Clean(strings.TrimSpace(workflowRoot))
	if root == "" {
		return nil, errors.New("daemon: workflow watch root is required")
	}

	directories := []string{root}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !entry.IsDir() {
			return nil
		}
		if path != root && strings.HasPrefix(entry.Name(), ".") {
			return filepath.SkipDir
		}
		if path != root {
			directories = append(directories, filepath.Clean(path))
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("daemon: discover workflow watch directories: %w", err)
	}

	sort.Strings(directories)
	return directories, nil
}
