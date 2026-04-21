package daemon

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"sync/atomic"
	"testing"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
	corepkg "github.com/compozy/compozy/internal/core"
	"github.com/fsnotify/fsnotify"
)

func TestWorkflowWatcherDebouncesBurstyWritesAndPersistsCheckpoint(t *testing.T) {
	workspaceRoot := t.TempDir()
	t.Setenv("HOME", t.TempDir())

	workflowDir := filepath.Join(workspaceRoot, ".compozy", "tasks", "watch-demo")
	if err := os.MkdirAll(workflowDir, 0o755); err != nil {
		t.Fatalf("mkdir workflow dir: %v", err)
	}

	taskPath := filepath.Join(workflowDir, "task_01.md")
	if err := os.WriteFile(taskPath, []byte(daemonTaskBody("pending", "watch demo")), 0o600); err != nil {
		t.Fatalf("write task file: %v", err)
	}
	if _, err := corepkg.SyncDirect(context.Background(), corepkg.SyncConfig{TasksDir: workflowDir}); err != nil {
		t.Fatalf("SyncDirect(initial): %v", err)
	}

	var (
		syncCount atomic.Int64
		emitCount atomic.Int64
	)
	watcher, err := startWorkflowWatcher(context.Background(), workflowWatcherConfig{
		WorkflowRoot: workflowDir,
		Debounce:     40 * time.Millisecond,
		Sync: func(ctx context.Context, workflowRoot string) error {
			syncCount.Add(1)
			_, err := corepkg.SyncDirect(ctx, corepkg.SyncConfig{TasksDir: workflowRoot})
			return err
		},
		Emit: func(context.Context, artifactSyncEvent) error {
			emitCount.Add(1)
			return nil
		},
	})
	if err != nil {
		t.Fatalf("startWorkflowWatcher(): %v", err)
	}
	defer func() {
		if stopErr := watcher.Stop(); stopErr != nil {
			t.Fatalf("watcher.Stop(): %v", stopErr)
		}
	}()

	checkpointBefore := queryWorkflowCheckpointChecksum(t, globalCatalogPath(t), filepath.Base(workflowDir))
	for _, title := range []string{"watch demo one", "watch demo two", "watch demo final"} {
		if err := os.WriteFile(taskPath, []byte(daemonTaskBody("completed", title)), 0o600); err != nil {
			t.Fatalf("rewrite task file: %v", err)
		}
	}

	waitForCondition(t, 5*time.Second, "watcher debounce sync", func() bool {
		title, status, ok := queryTaskItem(t, globalCatalogPath(t), filepath.Base(workflowDir), 1)
		return ok &&
			title == "watch demo final" &&
			status == "completed" &&
			queryWorkflowCheckpointChecksum(t, globalCatalogPath(t), filepath.Base(workflowDir)) != checkpointBefore &&
			syncCount.Load() == 1 &&
			emitCount.Load() == 1
	})

	time.Sleep(120 * time.Millisecond)
	if got := syncCount.Load(); got != 1 {
		t.Fatalf("sync count after debounce window = %d, want 1", got)
	}
	if got := emitCount.Load(); got != 1 {
		t.Fatalf("emit count after debounce window = %d, want 1", got)
	}
}

func TestWorkflowWatcherIgnoresWritesOutsideWorkflowRoot(t *testing.T) {
	workspaceRoot := t.TempDir()
	t.Setenv("HOME", t.TempDir())

	workflowDir := filepath.Join(workspaceRoot, ".compozy", "tasks", "owned-workflow")
	if err := os.MkdirAll(workflowDir, 0o755); err != nil {
		t.Fatalf("mkdir workflow dir: %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(workflowDir, "task_01.md"),
		[]byte(daemonTaskBody("pending", "owned workflow")),
		0o600,
	); err != nil {
		t.Fatalf("write task file: %v", err)
	}
	if _, err := corepkg.SyncDirect(context.Background(), corepkg.SyncConfig{TasksDir: workflowDir}); err != nil {
		t.Fatalf("SyncDirect(initial): %v", err)
	}

	var syncCount atomic.Int64
	watcher, err := startWorkflowWatcher(context.Background(), workflowWatcherConfig{
		WorkflowRoot: workflowDir,
		Debounce:     40 * time.Millisecond,
		Sync: func(ctx context.Context, workflowRoot string) error {
			syncCount.Add(1)
			_, err := corepkg.SyncDirect(ctx, corepkg.SyncConfig{TasksDir: workflowRoot})
			return err
		},
	})
	if err != nil {
		t.Fatalf("startWorkflowWatcher(): %v", err)
	}
	defer func() {
		if stopErr := watcher.Stop(); stopErr != nil {
			t.Fatalf("watcher.Stop(): %v", stopErr)
		}
	}()

	checkpointBefore := queryWorkflowCheckpointChecksum(t, globalCatalogPath(t), "owned-workflow")
	outsidePath := filepath.Join(workspaceRoot, ".compozy", "tasks", "other-workflow", "task_01.md")
	if err := os.MkdirAll(filepath.Dir(outsidePath), 0o755); err != nil {
		t.Fatalf("mkdir outside dir: %v", err)
	}
	if err := os.WriteFile(outsidePath, []byte(daemonTaskBody("completed", "outside workflow")), 0o600); err != nil {
		t.Fatalf("write outside task file: %v", err)
	}

	time.Sleep(150 * time.Millisecond)
	if got := syncCount.Load(); got != 0 {
		t.Fatalf("sync count for outside write = %d, want 0", got)
	}
	if got := queryWorkflowCheckpointChecksum(t, globalCatalogPath(t), "owned-workflow"); got != checkpointBefore {
		t.Fatalf("owned workflow checkpoint changed on outside write\nwant: %q\ngot:  %q", checkpointBefore, got)
	}
}

func TestWorkflowWatcherRefreshesWatchedDirectoriesAfterRenameAndDelete(t *testing.T) {
	workspaceRoot := t.TempDir()
	t.Setenv("HOME", t.TempDir())

	workflowDir := filepath.Join(workspaceRoot, ".compozy", "tasks", "rename-demo")
	if err := os.MkdirAll(filepath.Join(workflowDir, "qa", "reports"), 0o755); err != nil {
		t.Fatalf("mkdir workflow qa dir: %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(workflowDir, "task_01.md"),
		[]byte(daemonTaskBody("pending", "rename demo")),
		0o600,
	); err != nil {
		t.Fatalf("write task file: %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(workflowDir, "qa", "reports", "summary.md"),
		[]byte("# QA summary\n"),
		0o600,
	); err != nil {
		t.Fatalf("write qa summary: %v", err)
	}
	if _, err := corepkg.SyncDirect(context.Background(), corepkg.SyncConfig{TasksDir: workflowDir}); err != nil {
		t.Fatalf("SyncDirect(initial): %v", err)
	}

	var syncCount atomic.Int64
	watcher, err := startWorkflowWatcher(context.Background(), workflowWatcherConfig{
		WorkflowRoot: workflowDir,
		Debounce:     40 * time.Millisecond,
		Sync: func(ctx context.Context, workflowRoot string) error {
			syncCount.Add(1)
			_, err := corepkg.SyncDirect(ctx, corepkg.SyncConfig{TasksDir: workflowRoot})
			return err
		},
	})
	if err != nil {
		t.Fatalf("startWorkflowWatcher(): %v", err)
	}
	defer func() {
		if stopErr := watcher.Stop(); stopErr != nil {
			t.Fatalf("watcher.Stop(): %v", stopErr)
		}
	}()

	oldDir := filepath.Join(workflowDir, "qa", "reports")
	newDir := filepath.Join(workflowDir, "qa", "archive")
	if err := os.Rename(oldDir, newDir); err != nil {
		t.Fatalf("rename qa dir: %v", err)
	}
	waitForCondition(t, 5*time.Second, "watch refresh after rename", func() bool {
		_, exists := queryArtifactSnapshotBody(t, globalCatalogPath(t), "rename-demo", "qa/archive/summary.md")
		return exists && syncCount.Load() >= 1
	})

	newFile := filepath.Join(newDir, "followup.md")
	if err := os.WriteFile(newFile, []byte("# Followup\n"), 0o600); err != nil {
		t.Fatalf("write followup file: %v", err)
	}
	waitForCondition(t, 5*time.Second, "watch new dir after rename", func() bool {
		_, exists := queryArtifactSnapshotBody(t, globalCatalogPath(t), "rename-demo", "qa/archive/followup.md")
		return exists && syncCount.Load() >= 2
	})

	if err := os.Remove(newFile); err != nil {
		t.Fatalf("remove followup file: %v", err)
	}
	waitForCondition(t, 5*time.Second, "watch delete after rename", func() bool {
		_, exists := queryArtifactSnapshotBody(t, globalCatalogPath(t), "rename-demo", "qa/archive/followup.md")
		return !exists && syncCount.Load() >= 3
	})
}

func TestWorkflowWatcherFlushPendingChangesPreservesStateWhenPreSyncReconcileFails(t *testing.T) {
	t.Parallel()

	backendWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		t.Fatalf("fsnotify.NewWatcher() error = %v", err)
	}
	if err := backendWatcher.Close(); err != nil {
		t.Fatalf("backendWatcher.Close() error = %v", err)
	}

	var (
		syncCount atomic.Int64
		emitCount atomic.Int64
	)
	watcher := &workflowWatcher{
		workflowRoot: t.TempDir(),
		syncFn: func(context.Context, string) error {
			syncCount.Add(1)
			return nil
		},
		emitFn: func(context.Context, artifactSyncEvent) error {
			emitCount.Add(1)
			return nil
		},
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	state := &workflowWatchState{
		pending: map[string]artifactSyncEvent{
			"task_01.md": {
				RelativePath: "task_01.md",
				ChangeKind:   artifactChangeWrite,
			},
		},
		refreshWatches: true,
	}

	watcher.flushPendingChanges(context.Background(), backendWatcher, state)

	if got := syncCount.Load(); got != 0 {
		t.Fatalf("sync count = %d, want 0", got)
	}
	if got := emitCount.Load(); got != 0 {
		t.Fatalf("emit count = %d, want 0", got)
	}
	if !state.refreshWatches {
		t.Fatal("state.refreshWatches = false, want true after failed pre-sync reconcile")
	}
	change, ok := state.pending["task_01.md"]
	if !ok {
		t.Fatal("pending changes missing task_01.md after failed pre-sync reconcile")
	}
	if change.ChangeKind != artifactChangeWrite {
		t.Fatalf("pending change kind = %q, want %s", change.ChangeKind, artifactChangeWrite)
	}
	if watcher.stopError() == nil {
		t.Fatal("stopError() = nil, want reconcile failure recorded")
	}
}

func TestWorkflowWatcherValidatesConfigAndClassifiesArtifacts(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	if _, err := startWorkflowWatcher(context.Background(), workflowWatcherConfig{}); err == nil {
		t.Fatal("startWorkflowWatcher(missing root) error = nil, want non-nil")
	}

	workflowDir := filepath.Join(t.TempDir(), ".compozy", "tasks", "helper-demo")
	if err := os.MkdirAll(filepath.Join(workflowDir, "qa", "reports"), 0o755); err != nil {
		t.Fatalf("mkdir workflow dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(workflowDir, ".ignored"), 0o755); err != nil {
		t.Fatalf("mkdir hidden dir: %v", err)
	}
	if _, err := startWorkflowWatcher(context.Background(), workflowWatcherConfig{
		WorkflowRoot: workflowDir,
	}); err == nil {
		t.Fatal("startWorkflowWatcher(missing sync) error = nil, want non-nil")
	}

	directories, err := discoverWorkflowWatchDirs(workflowDir)
	if err != nil {
		t.Fatalf("discoverWorkflowWatchDirs(): %v", err)
	}
	if !slices.Contains(directories, workflowDir) {
		t.Fatalf("watch directories missing root %q: %#v", workflowDir, directories)
	}
	if !slices.Contains(directories, filepath.Join(workflowDir, "qa")) {
		t.Fatalf("watch directories missing qa dir: %#v", directories)
	}
	if slices.Contains(directories, filepath.Join(workflowDir, ".ignored")) {
		t.Fatalf("watch directories unexpectedly include hidden dir: %#v", directories)
	}

	relative, ok := workflowRelativePath(workflowDir, filepath.Join(workflowDir, "task_01.md"))
	if !ok || relative != "task_01.md" {
		t.Fatalf("workflowRelativePath(task) = %q, %v", relative, ok)
	}
	if _, ok := workflowRelativePath(workflowDir, filepath.Join(filepath.Dir(workflowDir), "elsewhere.md")); ok {
		t.Fatal("workflowRelativePath(outside root) = ok, want false")
	}

	for _, tc := range []struct {
		path string
		want bool
	}{
		{path: "task_01.md", want: true},
		{path: "_meta.md", want: true},
		{path: "reviews-001/issue_001.md", want: true},
		{path: "memory/MEMORY.md", want: true},
		{path: "notes.txt", want: false},
	} {
		if got := isRelevantWorkflowArtifact(tc.path); got != tc.want {
			t.Fatalf("isRelevantWorkflowArtifact(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}

	watchedDirs := map[string]struct{}{
		workflowDir:                      {},
		filepath.Join(workflowDir, "qa"): {},
	}
	fileChange, relevant, dirChanged := classifyWatchEvent(workflowDir, watchedDirs, fsnotify.Event{
		Name: filepath.Join(workflowDir, "task_01.md"),
		Op:   fsnotify.Write,
	})
	if !relevant || dirChanged {
		t.Fatalf("classifyWatchEvent(file) relevant=%v dirChanged=%v", relevant, dirChanged)
	}
	if fileChange.RelativePath != "task_01.md" || fileChange.ChangeKind != "write" {
		t.Fatalf("classifyWatchEvent(file) = %#v", fileChange)
	}

	_, relevant, dirChanged = classifyWatchEvent(workflowDir, watchedDirs, fsnotify.Event{
		Name: filepath.Join(workflowDir, "qa"),
		Op:   fsnotify.Rename,
	})
	if relevant || !dirChanged {
		t.Fatalf("classifyWatchEvent(dir) relevant=%v dirChanged=%v", relevant, dirChanged)
	}
}

func TestWorkflowWatcherErrorHelpers(t *testing.T) {
	errOne := errors.New("one")
	errTwo := errors.New("two")

	watcher := &workflowWatcher{
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	watcher.recordError(errOne)
	watcher.recordError(errTwo)
	watcher.logWarn("watcher warning", "path", "task_01.md")

	joined := watcher.stopError()
	if !errors.Is(joined, errOne) || !errors.Is(joined, errTwo) {
		t.Fatalf("stopError() = %v, want joined one/two", joined)
	}

	var nilWatcher *workflowWatcher
	nilWatcher.recordError(errOne)
	nilWatcher.logWarn("ignored")
	if err := nilWatcher.stopError(); err != nil {
		t.Fatalf("nil watcher stopError() = %v, want nil", err)
	}
}

func TestWorkflowWatcherHelpersDetectDirectoryCreatesAndChangeKinds(t *testing.T) {
	workflowDir := t.TempDir()
	createdDir := filepath.Join(workflowDir, "qa", "new")
	if err := os.MkdirAll(createdDir, 0o755); err != nil {
		t.Fatalf("mkdir created dir: %v", err)
	}

	if got := watchEventTargetsDirectory(createdDir, map[string]struct{}{}, fsnotify.Event{
		Name: createdDir,
		Op:   fsnotify.Create,
	}); !got {
		t.Fatal("watchEventTargetsDirectory(create dir) = false, want true")
	}

	for _, tc := range []struct {
		event fsnotify.Event
		want  string
	}{
		{event: fsnotify.Event{Op: fsnotify.Remove}, want: "remove"},
		{event: fsnotify.Event{Op: fsnotify.Rename}, want: "rename"},
		{event: fsnotify.Event{Op: fsnotify.Create}, want: "create"},
		{event: fsnotify.Event{Op: fsnotify.Write}, want: "write"},
		{event: fsnotify.Event{Op: fsnotify.Chmod}, want: "chmod"},
		{event: fsnotify.Event{Op: 0}, want: "update"},
	} {
		if got := classifyWatchChangeKind(tc.event); got != tc.want {
			t.Fatalf("classifyWatchChangeKind(%v) = %q, want %q", tc.event, got, tc.want)
		}
	}
}

func globalCatalogPath(t *testing.T) string {
	t.Helper()

	paths, err := compozyconfig.ResolveHomePaths()
	if err != nil {
		t.Fatalf("ResolveHomePaths(): %v", err)
	}
	return paths.GlobalDBPath
}
