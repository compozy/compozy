package runs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"

	"github.com/compozy/compozy/pkg/compozy/events"
)

func TestSchemaVersionErrorFormattingAndUnwrap(t *testing.T) {
	var nilErr *SchemaVersionError
	if got := nilErr.Error(); got != ErrIncompatibleSchemaVersion.Error() {
		t.Fatalf("nil SchemaVersionError.Error() = %q, want %q", got, ErrIncompatibleSchemaVersion.Error())
	}

	err := &SchemaVersionError{Version: "99.0"}
	if got := err.Error(); !strings.Contains(got, "99.0") {
		t.Fatalf("SchemaVersionError.Error() = %q, want version", got)
	}
	if !errors.Is(err, ErrIncompatibleSchemaVersion) {
		t.Fatalf("errors.Is(%v, ErrIncompatibleSchemaVersion) = false, want true", err)
	}
}

func TestSummaryHandlesNilRun(t *testing.T) {
	var run *Run
	if got := run.Summary(); got != (RunSummary{}) {
		t.Fatalf("Summary() = %#v, want zero summary", got)
	}
}

func TestResolveRunArtifactPathVariants(t *testing.T) {
	workspaceRoot := filepath.Join(string(filepath.Separator), "workspace")
	fallback := filepath.Join(workspaceRoot, ".compozy", "runs", "run", "events.jsonl")

	if got := resolveRunArtifactPath(workspaceRoot, fallback, ""); got != fallback {
		t.Fatalf("resolveRunArtifactPath(empty) = %q, want %q", got, fallback)
	}
	if got := resolveRunArtifactPath(workspaceRoot, fallback, "/tmp/events.jsonl"); got != "/tmp/events.jsonl" {
		t.Fatalf("resolveRunArtifactPath(abs) = %q, want /tmp/events.jsonl", got)
	}
	if got := resolveRunArtifactPath(
		workspaceRoot,
		fallback,
		filepath.Join("relative", "events.jsonl"),
	); got != filepath.Join(
		workspaceRoot,
		"relative",
		"events.jsonl",
	) {
		t.Fatalf("resolveRunArtifactPath(rel) = %q, want joined path", got)
	}
}

func TestLoadResultStatusHandlesMissingAndMalformedFiles(t *testing.T) {
	workspaceRoot := t.TempDir()
	if got, err := loadResultStatus(filepath.Join(workspaceRoot, "missing.json")); err != nil || got != "" {
		t.Fatalf("loadResultStatus(missing) = (%q, %v), want empty nil", got, err)
	}

	path := filepath.Join(workspaceRoot, "result.json")
	if err := os.WriteFile(path, []byte("{"), 0o600); err != nil {
		t.Fatalf("write malformed result: %v", err)
	}
	if _, err := loadResultStatus(path); err == nil {
		t.Fatalf("loadResultStatus(malformed) error = nil, want error")
	}
}

func TestFileHasPartialFinalLineDetectsStates(t *testing.T) {
	workspaceRoot := t.TempDir()
	path := filepath.Join(workspaceRoot, "events.jsonl")

	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	defer func() {
		_ = file.Close()
	}()

	if partial, err := fileHasPartialFinalLine(file); err != nil || partial {
		t.Fatalf("fileHasPartialFinalLine(empty) = (%v, %v), want false nil", partial, err)
	}

	if err := os.WriteFile(path, []byte("{\"line\":1}\n"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if partial, err := fileHasPartialFinalLine(file); err != nil || partial {
		t.Fatalf("fileHasPartialFinalLine(complete) = (%v, %v), want false nil", partial, err)
	}

	if err := os.WriteFile(path, []byte("{\"line\":1}"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if partial, err := fileHasPartialFinalLine(file); err != nil || !partial {
		t.Fatalf("fileHasPartialFinalLine(partial) = (%v, %v), want true nil", partial, err)
	}
}

func TestLiveTailOffsetSnapshotHandlesMissingCompleteAndLongPartialFiles(t *testing.T) {
	workspaceRoot := t.TempDir()

	if offset, err := liveTailOffsetSnapshot(filepath.Join(workspaceRoot, "missing.jsonl")); err != nil || offset != 0 {
		t.Fatalf("liveTailOffsetSnapshot(missing) = (%d, %v), want 0 nil", offset, err)
	}

	completePath := filepath.Join(workspaceRoot, "complete.jsonl")
	if err := os.WriteFile(completePath, []byte("one\n"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if offset, err := liveTailOffsetSnapshot(completePath); err != nil || offset != int64(len("one\n")) {
		t.Fatalf("liveTailOffsetSnapshot(complete) = (%d, %v), want %d nil", offset, err, len("one\n"))
	}

	partialPath := filepath.Join(workspaceRoot, "partial.jsonl")
	content := "prefix\n" + strings.Repeat("x", tailOffsetChunkSize+10)
	if err := os.WriteFile(partialPath, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if offset, err := liveTailOffsetSnapshot(partialPath); err != nil || offset != int64(len("prefix\n")) {
		t.Fatalf("liveTailOffsetSnapshot(partial) = (%d, %v), want %d nil", offset, err, len("prefix\n"))
	}
}

func TestTailNilRunReturnsErrorAndClosedChannels(t *testing.T) {
	var run *Run
	eventsCh, errsCh := run.Tail(context.Background(), 0)

	select {
	case err, ok := <-errsCh:
		if !ok || err == nil || !strings.Contains(err.Error(), "nil run") {
			t.Fatalf("Tail() error = %v, want nil run error", err)
		}
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for nil-run tail error")
	}

	waitForEventChannelClose(t, eventsCh, "events", time.Second)
	waitForEventChannelClose(t, errsCh, "errors", time.Second)
}

func TestTailReportsLiveDecodeErrors(t *testing.T) {
	workspaceRoot := t.TempDir()
	runID := "run-tail-errors"
	runDir := writeRunFixture(t, workspaceRoot, runID, runFixture{
		runJSON: map[string]any{
			"run_id":         runID,
			"mode":           "exec",
			"workspace_root": workspaceRoot,
			"created_at":     time.Date(2026, 4, 6, 12, 0, 0, 0, time.UTC),
		},
		events: []events.Event{
			testEvent(runID, 1, events.EventKindRunStarted),
		},
	})

	run, err := Open(workspaceRoot, runID)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	eventsCh, errsCh := run.Tail(ctx, 2)

	file, err := os.OpenFile(filepath.Join(runDir, "events.jsonl"), os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatalf("OpenFile() error = %v", err)
	}
	if _, err := file.WriteString("{bad json}\n"); err != nil {
		t.Fatalf("WriteString() error = %v", err)
	}
	_ = file.Close()

	select {
	case err := <-errsCh:
		if err == nil || !strings.Contains(err.Error(), "decode run event line") {
			t.Fatalf("Tail() error = %v, want decode error", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for live decode error")
	}

	cancelAndAwaitClose(t, cancel, eventsCh, errsCh)
}

func TestWatchWorkspaceHandlesMissingRunsDirUntilFirstRunAppears(t *testing.T) {
	workspaceRoot := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	eventsCh, errsCh := WatchWorkspace(ctx, workspaceRoot)
	writeRunFixture(t, workspaceRoot, "run-created-late", runFixture{
		runJSON: map[string]any{
			"run_id":         "run-created-late",
			"mode":           "exec",
			"status":         "running",
			"workspace_root": workspaceRoot,
			"created_at":     time.Date(2026, 4, 6, 12, 0, 0, 0, time.UTC),
		},
	})

	event := awaitRunEvent(t, eventsCh, errsCh, time.Second)
	if event.Kind != RunEventCreated || event.RunID != "run-created-late" {
		t.Fatalf("RunEvent = %#v, want created for run-created-late", event)
	}
	if event.Summary == nil || event.Summary.Status != "running" {
		t.Fatalf("RunEvent summary = %#v, want running summary", event.Summary)
	}

	cancelAndAwaitClose(t, cancel, eventsCh, errsCh)
}

func TestTransientRunLoadErrorsAndPathClassification(t *testing.T) {
	var syntaxErr error
	if err := json.Unmarshal([]byte("{"), &map[string]any{}); err != nil {
		syntaxErr = err
	}
	if !isTransientRunLoadError(syntaxErr) {
		t.Fatalf("isTransientRunLoadError(%v) = false, want true", syntaxErr)
	}
	if isTransientRunLoadError(errors.New("boom")) {
		t.Fatalf("isTransientRunLoadError(non-transient) = true, want false")
	}

	runsDir := "/workspace/.compozy/runs"
	if runID, kind := classifyWorkspacePath(
		runsDir,
		filepath.Join(runsDir, "run-1"),
	); runID != "run-1" ||
		kind != workspacePathRunDir {
		t.Fatalf("classifyWorkspacePath(run dir) = (%q, %v), want (run-1, runDir)", runID, kind)
	}
	if runID, kind := classifyWorkspacePath(
		runsDir,
		filepath.Join(runsDir, "run-1", "run.json"),
	); runID != "run-1" ||
		kind != workspacePathRunMeta {
		t.Fatalf("classifyWorkspacePath(run meta) = (%q, %v), want (run-1, runMeta)", runID, kind)
	}
	if runID, kind := classifyWorkspacePath(
		runsDir,
		filepath.Join(runsDir, "run-1", "events.jsonl"),
	); runID != "" ||
		kind != workspacePathUnknown {
		t.Fatalf("classifyWorkspacePath(other file) = (%q, %v), want unknown", runID, kind)
	}
}

func TestListReturnsEmptyWhenRunsDirMissing(t *testing.T) {
	got, err := List(t.TempDir(), ListOptions{})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if got != nil {
		t.Fatalf("List() = %#v, want nil slice for missing runs dir", got)
	}
}

func TestCleanWorkspaceRootAndRunsDirForWorkspace(t *testing.T) {
	if got := cleanWorkspaceRoot(" . "); got != "" {
		t.Fatalf("cleanWorkspaceRoot('.') = %q, want empty", got)
	}
	root := filepath.Join(string(filepath.Separator), "tmp", "workspace")
	if got := cleanWorkspaceRoot(filepath.Join(root, "..", "workspace")); got != root {
		t.Fatalf("cleanWorkspaceRoot(clean) = %q, want %s", got, root)
	}
	if got := runsDirForWorkspace(root); got != filepath.Join(root, ".compozy", "runs") {
		t.Fatalf("runsDirForWorkspace() = %q, want joined runs dir", got)
	}
}

func TestAddRunDirWatchAndRemoveWorkspaceRunHelpers(t *testing.T) {
	workspaceRoot := t.TempDir()
	runDir := filepath.Join(workspaceRoot, ".compozy", "runs", "run-helper")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatalf("mkdir run dir: %v", err)
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		t.Fatalf("NewWatcher() error = %v", err)
	}
	defer watcher.Close()

	watchedRunDirs := make(map[string]string)
	if err := addRunDirWatch(watcher, watchedRunDirs, "missing", filepath.Join(workspaceRoot, "missing")); err != nil {
		t.Fatalf("addRunDirWatch(missing) error = %v", err)
	}

	filePath := filepath.Join(workspaceRoot, "plain-file")
	if err := os.WriteFile(filePath, []byte("x"), 0o600); err != nil {
		t.Fatalf("write plain file: %v", err)
	}
	if err := addRunDirWatch(watcher, watchedRunDirs, "plain-file", filePath); err != nil {
		t.Fatalf("addRunDirWatch(file) error = %v", err)
	}

	if err := addRunDirWatch(watcher, watchedRunDirs, "run-helper", runDir); err != nil {
		t.Fatalf("addRunDirWatch(dir) error = %v", err)
	}
	if err := addRunDirWatch(watcher, watchedRunDirs, "run-helper", runDir); err != nil {
		t.Fatalf("addRunDirWatch(duplicate) error = %v", err)
	}

	known := map[string]RunSummary{"run-helper": {RunID: "run-helper"}}
	out := make(chan RunEvent, 1)
	if err := removeWorkspaceRun(context.Background(), watcher, "run-helper", known, watchedRunDirs, out); err != nil {
		t.Fatalf("removeWorkspaceRun() error = %v", err)
	}
	event := <-out
	if event.Kind != RunEventRemoved || event.RunID != "run-helper" {
		t.Fatalf("removeWorkspaceRun() event = %#v, want removed run-helper", event)
	}

	if err := removeWorkspaceRun(context.Background(), watcher, "unknown", known, watchedRunDirs, out); err != nil {
		t.Fatalf("removeWorkspaceRun(unknown) error = %v", err)
	}
}

func TestRefreshWorkspaceRunEventuallyTimesOutWithoutMetadata(t *testing.T) {
	workspaceRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(workspaceRoot, ".compozy", "runs", "run-empty"), 0o755); err != nil {
		t.Fatalf("mkdir run dir: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), runMetadataReadyTimeout+50*time.Millisecond)
	defer cancel()

	known := make(map[string]RunSummary)
	out := make(chan RunEvent, 1)
	if err := refreshWorkspaceRunEventually(ctx, workspaceRoot, "run-empty", known, out); err != nil {
		t.Fatalf("refreshWorkspaceRunEventually() error = %v", err)
	}
	select {
	case event := <-out:
		t.Fatalf("refreshWorkspaceRunEventually() event = %#v, want none", event)
	default:
	}
}

func TestRunAndWorkspaceSendHelpersRespectContext(t *testing.T) {
	runEvents := make(chan events.Event)
	runErrs := make(chan error)
	workspaceEvents := make(chan RunEvent)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if sendRunEvent(ctx, runEvents, events.Event{}) {
		t.Fatalf("sendRunEvent(canceled) = true, want false")
	}
	if sendRunError(ctx, runErrs, errors.New("boom")) {
		t.Fatalf("sendRunError(canceled) = true, want false")
	}
	if !sendRunError(context.Background(), runErrs, nil) {
		t.Fatalf("sendRunError(nil error) = false, want true")
	}
	if err := sendWorkspaceEvent(ctx, workspaceEvents, RunEvent{}); err != nil {
		t.Fatalf("sendWorkspaceEvent(canceled) error = %v, want nil", err)
	}

	setupErrs := make(chan error, 1)
	sendSetupError(setupErrs, nil)
	sendSetupError(setupErrs, errors.New("setup failed"))
	select {
	case err := <-setupErrs:
		if err == nil || err.Error() != "setup failed" {
			t.Fatalf("sendSetupError() err = %v, want setup failed", err)
		}
	default:
		t.Fatalf("sendSetupError() did not enqueue error")
	}
}

func TestSeedWorkspaceWatcherSkipsMissingRunMetadataAndLoadSummaryHardErrors(t *testing.T) {
	workspaceRoot := t.TempDir()
	runsDir := filepath.Join(workspaceRoot, ".compozy", "runs")
	if err := os.MkdirAll(filepath.Join(runsDir, "missing-meta"), 0o755); err != nil {
		t.Fatalf("mkdir missing run dir: %v", err)
	}
	writeRunFixture(t, workspaceRoot, "bad-result", runFixture{
		runJSON: map[string]any{
			"run_id":         "bad-result",
			"mode":           "exec",
			"workspace_root": workspaceRoot,
			"result_path":    filepath.Join(runsDir, "bad-result", "result.json"),
			"created_at":     time.Date(2026, 4, 6, 12, 0, 0, 0, time.UTC),
		},
	})
	if err := os.WriteFile(filepath.Join(runsDir, "bad-result", "result.json"), []byte("{"), 0o600); err != nil {
		t.Fatalf("write malformed result: %v", err)
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		t.Fatalf("NewWatcher() error = %v", err)
	}
	defer watcher.Close()

	known := make(map[string]RunSummary)
	watchedRunDirs := make(map[string]string)
	if err := seedWorkspaceWatcher(workspaceRoot, runsDir, watcher, known, watchedRunDirs); err == nil {
		t.Fatalf("seedWorkspaceWatcher() error = nil, want malformed result error")
	}

	if _, ok, err := loadRunSummaryIfReady(workspaceRoot, "missing-meta"); err != nil || ok {
		t.Fatalf("loadRunSummaryIfReady(missing-meta) = (%v, %v), want false nil", ok, err)
	}
	if _, ok, err := loadRunSummaryIfReady(workspaceRoot, "bad-result"); err != nil || ok {
		t.Fatalf("loadRunSummaryIfReady(bad-result) = (%v, %v), want retryable nil error", ok, err)
	}
}

func TestHandleWorkspaceEventRemovesRunOnRunMetaRemovalWhenDirectoryDisappears(t *testing.T) {
	workspaceRoot := t.TempDir()
	runDir := writeRunFixture(t, workspaceRoot, "run-meta-remove", runFixture{
		runJSON: map[string]any{
			"run_id":         "run-meta-remove",
			"mode":           "exec",
			"status":         "running",
			"workspace_root": workspaceRoot,
			"created_at":     time.Date(2026, 4, 6, 12, 0, 0, 0, time.UTC),
		},
	})
	runsDir := filepath.Join(workspaceRoot, ".compozy", "runs")

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		t.Fatalf("NewWatcher() error = %v", err)
	}
	defer watcher.Close()
	if err := watcher.Add(runsDir); err != nil {
		t.Fatalf("watcher.Add(runsDir) error = %v", err)
	}
	if err := watcher.Add(runDir); err != nil {
		t.Fatalf("watcher.Add(runDir) error = %v", err)
	}

	known := map[string]RunSummary{"run-meta-remove": {RunID: "run-meta-remove", Status: "running"}}
	watchedRunDirs := map[string]string{"run-meta-remove": runDir}
	out := make(chan RunEvent, 1)

	if err := os.RemoveAll(runDir); err != nil {
		t.Fatalf("RemoveAll() error = %v", err)
	}
	if err := handleWorkspaceEvent(
		context.Background(),
		workspaceRoot,
		runsDir,
		watcher,
		known,
		watchedRunDirs,
		out,
		fsnotify.Event{Name: filepath.Join(runDir, "run.json"), Op: fsnotify.Remove},
	); err != nil {
		t.Fatalf("handleWorkspaceEvent() error = %v", err)
	}

	event := <-out
	if event.Kind != RunEventRemoved || event.RunID != "run-meta-remove" {
		t.Fatalf("handleWorkspaceEvent() event = %#v, want removed run-meta-remove", event)
	}
}

func TestIsIgnorableWatchRemoveError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "non-existent watch",
			err:  fsnotify.ErrNonExistentWatch,
			want: true,
		},
		{
			name: "closed watcher",
			err:  fsnotify.ErrClosed,
			want: true,
		},
		{
			name: "path missing",
			err:  os.ErrNotExist,
			want: true,
		},
		{
			name: "linux invalid argument after delete",
			err:  syscall.EINVAL,
			want: true,
		},
		{
			name: "wrapped linux invalid argument after delete",
			err:  fmt.Errorf("remove watch: %w", syscall.EINVAL),
			want: true,
		},
		{
			name: "unrelated error",
			err:  syscall.EPERM,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := isIgnorableWatchRemoveError(tt.err); got != tt.want {
				t.Fatalf("isIgnorableWatchRemoveError(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}
