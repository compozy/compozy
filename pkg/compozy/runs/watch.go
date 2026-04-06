package runs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
)

// RunEventKind identifies the type of workspace run event.
type RunEventKind string

const (
	// RunEventCreated reports a newly observed run.
	RunEventCreated RunEventKind = "created"
	// RunEventStatusChanged reports a run whose status changed.
	RunEventStatusChanged RunEventKind = "status_changed"
	// RunEventRemoved reports a removed run directory.
	RunEventRemoved RunEventKind = "removed"
)

const (
	runMetadataPollInterval = 10 * time.Millisecond
	runMetadataReadyTimeout = 250 * time.Millisecond
)

// RunEvent reports workspace-level run lifecycle changes.
type RunEvent struct {
	Kind    RunEventKind
	RunID   string
	Summary *RunSummary
}

// WatchWorkspace emits RunEvent notifications for runs under workspaceRoot.
func WatchWorkspace(ctx context.Context, workspaceRoot string) (<-chan RunEvent, <-chan error) {
	out := make(chan RunEvent)
	errs := make(chan error, 4)
	cleanRoot := cleanWorkspaceRoot(workspaceRoot)
	runsDir := runsDirForWorkspace(cleanRoot)

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		sendSetupError(errs, fmt.Errorf("create workspace watcher: %w", err))
		close(out)
		close(errs)
		return out, errs
	}

	if err := watcher.Add(runsDir); err != nil {
		_ = watcher.Close()
		sendSetupError(errs, fmt.Errorf("watch runs directory: %w", err))
		close(out)
		close(errs)
		return out, errs
	}

	known := make(map[string]RunSummary)
	watchedRunDirs := make(map[string]string)
	if err := seedWorkspaceWatcher(cleanRoot, runsDir, watcher, known, watchedRunDirs); err != nil {
		_ = watcher.Close()
		sendSetupError(errs, err)
		close(out)
		close(errs)
		return out, errs
	}

	go func() {
		defer close(out)
		defer close(errs)
		defer watcher.Close()

		for {
			select {
			case <-ctx.Done():
				return
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				if !sendRunError(ctx, errs, err) {
					return
				}
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if err := handleWorkspaceEvent(
					ctx,
					cleanRoot,
					runsDir,
					watcher,
					known,
					watchedRunDirs,
					out,
					event,
				); err != nil {
					if !sendRunError(ctx, errs, err) {
						return
					}
				}
			}
		}
	}()

	return out, errs
}

func sendSetupError(dst chan<- error, err error) {
	if err == nil {
		return
	}
	select {
	case dst <- err:
	default:
	}
}

func seedWorkspaceWatcher(
	workspaceRoot string,
	runsDir string,
	watcher *fsnotify.Watcher,
	known map[string]RunSummary,
	watchedRunDirs map[string]string,
) error {
	entries, err := os.ReadDir(runsDir)
	if err != nil {
		return fmt.Errorf("read runs directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		runID := entry.Name()
		runDir := filepath.Join(runsDir, runID)
		if err := addRunDirWatch(watcher, watchedRunDirs, runID, runDir); err != nil {
			return err
		}
		run, err := loadRun(workspaceRoot, runID)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return err
		}
		known[runID] = run.Summary()
	}
	return nil
}

func handleWorkspaceEvent(
	ctx context.Context,
	workspaceRoot string,
	runsDir string,
	watcher *fsnotify.Watcher,
	known map[string]RunSummary,
	watchedRunDirs map[string]string,
	out chan<- RunEvent,
	event fsnotify.Event,
) error {
	runID, pathKind := classifyWorkspacePath(runsDir, event.Name)
	if runID == "" {
		return nil
	}

	switch pathKind {
	case workspacePathRunDir:
		runDir := filepath.Join(runsDir, runID)
		if event.Has(fsnotify.Create) {
			if err := addRunDirWatch(watcher, watchedRunDirs, runID, runDir); err != nil {
				return err
			}
			return refreshWorkspaceRunEventually(ctx, workspaceRoot, runID, known, out)
		}
		if event.Has(fsnotify.Remove) || event.Has(fsnotify.Rename) {
			return removeWorkspaceRun(ctx, watcher, runID, known, watchedRunDirs, out)
		}
	case workspacePathRunMeta:
		if event.Has(fsnotify.Create) || event.Has(fsnotify.Write) || event.Has(fsnotify.Chmod) {
			return refreshWorkspaceRun(ctx, workspaceRoot, runID, known, out)
		}
		if event.Has(fsnotify.Remove) || event.Has(fsnotify.Rename) {
			runDir := filepath.Join(runsDir, runID)
			if _, err := os.Stat(runDir); errors.Is(err, os.ErrNotExist) {
				return removeWorkspaceRun(ctx, watcher, runID, known, watchedRunDirs, out)
			}
		}
	}

	return nil
}

func addRunDirWatch(
	watcher *fsnotify.Watcher,
	watchedRunDirs map[string]string,
	runID string,
	runDir string,
) error {
	info, err := os.Stat(runDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("stat run directory %q: %w", runID, err)
	}
	if !info.IsDir() {
		return nil
	}
	if _, ok := watchedRunDirs[runID]; ok {
		return nil
	}
	if err := watcher.Add(runDir); err != nil {
		return fmt.Errorf("watch run directory %q: %w", runID, err)
	}
	watchedRunDirs[runID] = runDir
	return nil
}

func refreshWorkspaceRun(
	ctx context.Context,
	workspaceRoot string,
	runID string,
	known map[string]RunSummary,
	out chan<- RunEvent,
) error {
	summary, ok, err := loadRunSummaryIfReady(workspaceRoot, runID)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}

	return applyWorkspaceRunSummary(ctx, runID, summary, known, out)
}

func refreshWorkspaceRunEventually(
	ctx context.Context,
	workspaceRoot string,
	runID string,
	known map[string]RunSummary,
	out chan<- RunEvent,
) error {
	summary, ok, err := loadRunSummaryIfReady(workspaceRoot, runID)
	if err != nil {
		return err
	}
	if ok {
		return applyWorkspaceRunSummary(ctx, runID, summary, known, out)
	}

	timer := time.NewTimer(runMetadataReadyTimeout)
	defer timer.Stop()
	ticker := time.NewTicker(runMetadataPollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-timer.C:
			return nil
		case <-ticker.C:
			summary, ok, err = loadRunSummaryIfReady(workspaceRoot, runID)
			if err != nil {
				return err
			}
			if ok {
				return applyWorkspaceRunSummary(ctx, runID, summary, known, out)
			}
		}
	}
}

func loadRunSummaryIfReady(workspaceRoot, runID string) (RunSummary, bool, error) {
	run, err := loadRun(workspaceRoot, runID)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) || isTransientRunLoadError(err) {
			return RunSummary{}, false, nil
		}
		return RunSummary{}, false, err
	}
	return run.Summary(), true, nil
}

func isTransientRunLoadError(err error) bool {
	if err == nil {
		return false
	}
	var syntaxErr *json.SyntaxError
	return errors.As(err, &syntaxErr) ||
		errors.Is(err, io.ErrUnexpectedEOF) ||
		strings.Contains(err.Error(), "unexpected end of JSON input")
}

func applyWorkspaceRunSummary(
	ctx context.Context,
	runID string,
	summary RunSummary,
	known map[string]RunSummary,
	out chan<- RunEvent,
) error {
	previous, exists := known[runID]
	known[runID] = summary

	switch {
	case !exists:
		return sendWorkspaceEvent(ctx, out, RunEvent{
			Kind:    RunEventCreated,
			RunID:   runID,
			Summary: summaryPointer(summary),
		})
	case previous.Status != summary.Status:
		return sendWorkspaceEvent(ctx, out, RunEvent{
			Kind:    RunEventStatusChanged,
			RunID:   runID,
			Summary: summaryPointer(summary),
		})
	default:
		return nil
	}
}

func removeWorkspaceRun(
	ctx context.Context,
	watcher *fsnotify.Watcher,
	runID string,
	known map[string]RunSummary,
	watchedRunDirs map[string]string,
	out chan<- RunEvent,
) error {
	if runDir, ok := watchedRunDirs[runID]; ok {
		delete(watchedRunDirs, runID)
		if err := watcher.Remove(runDir); err != nil &&
			!errors.Is(err, fsnotify.ErrNonExistentWatch) &&
			!errors.Is(err, fsnotify.ErrClosed) &&
			!errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("remove run watch %q: %w", runID, err)
		}
	}
	if _, ok := known[runID]; !ok {
		return nil
	}
	delete(known, runID)
	return sendWorkspaceEvent(ctx, out, RunEvent{
		Kind:  RunEventRemoved,
		RunID: runID,
	})
}

func sendWorkspaceEvent(ctx context.Context, dst chan<- RunEvent, event RunEvent) error {
	select {
	case <-ctx.Done():
		return nil
	case dst <- event:
		return nil
	}
}

func summaryPointer(summary RunSummary) *RunSummary {
	copyValue := summary
	return &copyValue
}

type workspacePathKind uint8

const (
	workspacePathUnknown workspacePathKind = iota
	workspacePathRunDir
	workspacePathRunMeta
)

func classifyWorkspacePath(runsDir, target string) (string, workspacePathKind) {
	rel, err := filepath.Rel(runsDir, target)
	if err != nil {
		return "", workspacePathUnknown
	}
	rel = filepath.Clean(rel)
	if rel == "." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", workspacePathUnknown
	}

	parts := strings.Split(rel, string(filepath.Separator))
	switch {
	case len(parts) == 1 && parts[0] != "":
		return parts[0], workspacePathRunDir
	case len(parts) == 2 && parts[0] != "" && parts[1] == "run.json":
		return parts[0], workspacePathRunMeta
	default:
		return "", workspacePathUnknown
	}
}
