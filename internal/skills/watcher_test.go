package skills

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"testing"
	"time"
)

func TestWatcherDetectChangesAddedSkill(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	watcher := newTestWatcher(nil, time.Millisecond, root)

	if changed, _, _, err := watcher.detectChanges(context.Background()); err != nil {
		t.Fatalf("detectChanges() initial error = %v", err)
	} else if changed {
		t.Fatal("detectChanges() initial changed = true, want false")
	}

	skillPath := writeSkillFile(
		t,
		root,
		filepath.Join("added", skillFileName),
		skillWithDescription("added", "Added skill"),
	)

	changed, snapshots, changes, err := watcher.detectChanges(context.Background())
	if err != nil {
		t.Fatalf("detectChanges() error = %v", err)
	}
	if !changed {
		t.Fatal("detectChanges() changed = false, want true")
	}
	if len(changes) != 1 {
		t.Fatalf("detectChanges() len(changes) = %d, want 1", len(changes))
	}
	if changes[0].path != skillPath || changes[0].action != "added" {
		t.Fatalf("detectChanges() change = %#v, want added change for %q", changes[0], skillPath)
	}

	watcher.commitSnapshots(snapshots)

	if changed, _, _, err := watcher.detectChanges(context.Background()); err != nil {
		t.Fatalf("detectChanges() after commit error = %v", err)
	} else if changed {
		t.Fatal("detectChanges() after commit changed = true, want false")
	}
}

func TestWatcherDetectChangesModifiedSkillByMTime(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	skillPath := writeSkillFile(t, root, filepath.Join("mtime", skillFileName), skillWithDescription("mtime", "Alpha"))
	watcher := newTestWatcher(nil, time.Millisecond, root)

	if changed, _, _, err := watcher.detectChanges(context.Background()); err != nil {
		t.Fatalf("detectChanges() initial error = %v", err)
	} else if changed {
		t.Fatal("detectChanges() initial changed = true, want false")
	}

	modTime := time.Date(2026, 4, 6, 12, 0, 5, 0, time.UTC)
	rewriteSkillFile(t, skillPath, skillWithDescription("mtime", "Bravo"))
	setFileTimes(t, skillPath, modTime)

	changed, _, changes, err := watcher.detectChanges(context.Background())
	if err != nil {
		t.Fatalf("detectChanges() error = %v", err)
	}
	if !changed {
		t.Fatal("detectChanges() changed = false, want true")
	}
	if len(changes) != 1 {
		t.Fatalf("detectChanges() len(changes) = %d, want 1", len(changes))
	}
	if changes[0].path != skillPath || changes[0].action != "modified" {
		t.Fatalf("detectChanges() change = %#v, want modified change for %q", changes[0], skillPath)
	}
}

func TestWatcherDetectChangesDeletedSkill(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	skillPath := writeSkillFile(
		t,
		root,
		filepath.Join("deleted", skillFileName),
		skillWithDescription("deleted", "Deleted skill"),
	)
	watcher := newTestWatcher(nil, time.Millisecond, root)

	if changed, _, _, err := watcher.detectChanges(context.Background()); err != nil {
		t.Fatalf("detectChanges() initial error = %v", err)
	} else if changed {
		t.Fatal("detectChanges() initial changed = true, want false")
	}

	if err := os.Remove(skillPath); err != nil {
		t.Fatalf("Remove(%q) error = %v", skillPath, err)
	}

	changed, _, changes, err := watcher.detectChanges(context.Background())
	if err != nil {
		t.Fatalf("detectChanges() error = %v", err)
	}
	if !changed {
		t.Fatal("detectChanges() changed = false, want true")
	}
	if len(changes) != 1 {
		t.Fatalf("detectChanges() len(changes) = %d, want 1", len(changes))
	}
	if changes[0].path != skillPath || changes[0].action != "deleted" {
		t.Fatalf("detectChanges() change = %#v, want deleted change for %q", changes[0], skillPath)
	}
}

func TestWatcherDetectChangesNoFalsePositiveWhenUnchanged(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeSkillFile(t, root, filepath.Join("stable", skillFileName), skillWithDescription("stable", "Stable skill"))
	watcher := newTestWatcher(nil, time.Millisecond, root)

	if changed, _, _, err := watcher.detectChanges(context.Background()); err != nil {
		t.Fatalf("detectChanges() initial error = %v", err)
	} else if changed {
		t.Fatal("detectChanges() initial changed = true, want false")
	}

	if changed, _, _, err := watcher.detectChanges(context.Background()); err != nil {
		t.Fatalf("detectChanges() second error = %v", err)
	} else if changed {
		t.Fatal("detectChanges() second changed = true, want false")
	}
}

func TestWatcherDetectChangesUsesDynamicRootsProvider(t *testing.T) {
	t.Parallel()

	t.Run("Should detect workspace skill changes from dynamic roots", func(t *testing.T) {
		t.Parallel()

		globalRoot := t.TempDir()
		workspaceRoot := t.TempDir()
		watcher := newTestWatcher(nil, time.Millisecond, globalRoot)
		watcher.SetRootsProvider(func(context.Context) ([]string, error) {
			return []string{workspaceRoot}, nil
		})

		if changed, _, _, err := watcher.detectChanges(context.Background()); err != nil {
			t.Fatalf("detectChanges() initial error = %v", err)
		} else if changed {
			t.Fatal("detectChanges() initial changed = true, want false")
		}

		skillPath := writeSkillFile(
			t,
			workspaceRoot,
			filepath.Join("dynamic", skillFileName),
			skillWithDescription("dynamic", "Workspace dynamic skill"),
		)

		changed, _, changes, err := watcher.detectChanges(context.Background())
		if err != nil {
			t.Fatalf("detectChanges() error = %v", err)
		}
		if !changed {
			t.Fatal("detectChanges() changed = false, want true")
		}
		if len(changes) != 1 {
			t.Fatalf("detectChanges() len(changes) = %d, want 1", len(changes))
		}
		if changes[0].path != skillPath || changes[0].action != "added" {
			t.Fatalf("detectChanges() change = %#v, want added change for %q", changes[0], skillPath)
		}
	})

	t.Run("Should rederive the polled roots and refresh after a source replacement", func(t *testing.T) {
		t.Parallel()

		removedRoot := t.TempDir()
		replacementRoot := t.TempDir()
		removedPath := writeSkillFile(t, removedRoot, filepath.Join("removed", skillFileName), skillWithDescription("removed", "Removed"))
		replacementPath := writeSkillFile(t, replacementRoot, filepath.Join("replacement", skillFileName), skillWithDescription("replacement", "Replacement"))
		activeRoots := []string{removedRoot}
		spy := newRefreshSpy()
		watcher := newTestWatcher(spy, time.Millisecond)
		watcher.SetRootsProvider(func(context.Context) ([]string, error) {
			return append([]string(nil), activeRoots...), nil
		})
		if err := watcher.pollOnce(t.Context()); err != nil {
			t.Fatalf("pollOnce(baseline) error = %v", err)
		}

		activeRoots = []string{replacementRoot}
		if err := watcher.pollOnce(t.Context()); err != nil {
			t.Fatalf("pollOnce(replacement) error = %v", err)
		}
		if got := spy.calls(); got != 1 {
			t.Fatalf("RefreshGlobal() calls = %d, want 1", got)
		}
		watcher.mu.Lock()
		_, retainedRemoved := watcher.snapshots[removedPath]
		_, retainedReplacement := watcher.snapshots[replacementPath]
		watcher.mu.Unlock()
		if retainedRemoved || !retainedReplacement {
			t.Fatalf("watcher snapshots retain removed=%t replacement=%t, want false/true", retainedRemoved, retainedReplacement)
		}
	})
}

func TestWatcherUsesExplicitAgentRootIdentity(t *testing.T) {
	t.Parallel()

	t.Run("Should treat a custom skill root named agents as a skill root", func(t *testing.T) {
		t.Parallel()

		root := filepath.Join(t.TempDir(), "agents")
		if err := os.MkdirAll(root, 0o755); err != nil {
			t.Fatalf("MkdirAll(%q) error = %v", root, err)
		}
		watcher := newTestWatcher(nil, time.Millisecond, root)
		if changed, _, _, err := watcher.detectChanges(context.Background()); err != nil {
			t.Fatalf("detectChanges() initial error = %v", err)
		} else if changed {
			t.Fatal("detectChanges() initial changed = true, want false")
		}
		agentFile := filepath.Join(root, "unexpected", "AGENT.md")
		if err := os.MkdirAll(filepath.Dir(agentFile), 0o755); err != nil {
			t.Fatalf("MkdirAll(%q) error = %v", filepath.Dir(agentFile), err)
		}
		if err := os.WriteFile(agentFile, []byte("agent"), 0o644); err != nil {
			t.Fatalf("WriteFile(%q) error = %v", agentFile, err)
		}

		if changed, _, _, err := watcher.detectChanges(context.Background()); err != nil {
			t.Fatalf("detectChanges() error = %v", err)
		} else if changed {
			t.Fatal("detectChanges() changed = true, want AGENT.md ignored in custom skill root")
		}
	})

	t.Run("Should watch AGENT.md only for an explicitly typed agent root", func(t *testing.T) {
		t.Parallel()

		root := filepath.Join(t.TempDir(), "custom-agent-definitions")
		watcher := newTestWatcher(nil, time.Millisecond)
		watcher.SetAgentRootsProvider(func(context.Context) ([]string, error) {
			return []string{root}, nil
		})
		if changed, _, _, err := watcher.detectChanges(context.Background()); err != nil {
			t.Fatalf("detectChanges() initial error = %v", err)
		} else if changed {
			t.Fatal("detectChanges() initial changed = true, want false")
		}
		agentFile := filepath.Join(root, "coder", "AGENT.md")
		if err := os.MkdirAll(filepath.Dir(agentFile), 0o755); err != nil {
			t.Fatalf("MkdirAll(%q) error = %v", filepath.Dir(agentFile), err)
		}
		if err := os.WriteFile(agentFile, []byte("agent"), 0o644); err != nil {
			t.Fatalf("WriteFile(%q) error = %v", agentFile, err)
		}

		changed, _, changes, err := watcher.detectChanges(context.Background())
		if err != nil {
			t.Fatalf("detectChanges() error = %v", err)
		}
		if !changed || len(changes) != 1 || changes[0].path != agentFile {
			t.Fatalf("detectChanges() = changed %v, changes %#v; want explicit agent file", changed, changes)
		}
	})
}

func TestNewWatcherOnlyUsesGlobalRoots(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	userDir := filepath.Join(root, "global-user")
	agentsDir := filepath.Join(root, "global-agents")
	workspace := filepath.Join(root, "workspace")

	registry := newTestRegistry(t, RegistryConfig{
		BundledFS:        bundledSkillFS(map[string]string{"bundled": "Bundled skill"}),
		GlobalSkillRoots: testGlobalSkillRoots(userDir),
		GlobalAgentsDir:  agentsDir,
	})
	watcher := NewWatcher(registry, 0)
	watcher.logger = slog.New(slog.NewTextHandler(io.Discard, nil))

	expectedRoots := watcherRoots(userDir, agentsDir)
	if !slices.Equal(watcher.roots, expectedRoots) {
		t.Fatalf("watcher.roots = %#v, want %#v", watcher.roots, expectedRoots)
	}
	if watcher.interval != defaultWatcherInterval {
		t.Fatalf("watcher.interval = %v, want %v", watcher.interval, defaultWatcherInterval)
	}

	if changed, _, _, err := watcher.detectChanges(context.Background()); err != nil {
		t.Fatalf("detectChanges() initial error = %v", err)
	} else if changed {
		t.Fatal("detectChanges() initial changed = true, want false")
	}

	writeSkillFile(
		t,
		filepath.Join(workspace, ".compozy", "agents", "workspace-only", "skills"),
		skillFileName,
		skillWithDescription("workspace-only", "Workspace skill"),
	)
	writeSkillFile(
		t,
		filepath.Join(workspace, ".compozy", "skills"),
		filepath.Join("workspace-compozy", skillFileName),
		skillWithDescription("workspace-compozy", "Workspace compozy skill"),
	)

	if changed, _, _, err := watcher.detectChanges(context.Background()); err != nil {
		t.Fatalf("detectChanges() after workspace-only updates error = %v", err)
	} else if changed {
		t.Fatal("detectChanges() after workspace-only updates changed = true, want false")
	}
}

func TestNewWatcherSeedsSnapshotsFromRegistryLoadAll(t *testing.T) {
	t.Parallel()

	t.Run("Should added after empty baseline", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		userDir := filepath.Join(root, "user")
		registry := newTestRegistry(t, RegistryConfig{
			GlobalSkillRoots: testGlobalSkillRoots(userDir),
		})
		if err := registry.LoadAll(context.Background()); err != nil {
			t.Fatalf("LoadAll() error = %v", err)
		}

		skillPath := writeSkillFile(
			t,
			userDir,
			filepath.Join("added", skillFileName),
			skillWithDescription("added", "Added after load"),
		)
		watcher := NewWatcher(registry, time.Millisecond)
		watcher.logger = slog.New(slog.NewTextHandler(io.Discard, nil))

		changed, _, changes, err := watcher.detectChanges(context.Background())
		if err != nil {
			t.Fatalf("detectChanges() error = %v", err)
		}
		if !changed {
			t.Fatal("detectChanges() changed = false, want true for post-load addition")
		}
		if len(changes) != 1 || changes[0].path != skillPath || changes[0].action != "added" {
			t.Fatalf("detectChanges() changes = %#v, want added change for %q", changes, skillPath)
		}
	})

	t.Run("Should modified after populated baseline", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		userDir := filepath.Join(root, "user")
		skillPath := writeSkillFile(
			t,
			userDir,
			filepath.Join("modified", skillFileName),
			skillWithDescription("modified", "Version one"),
		)

		registry := newTestRegistry(t, RegistryConfig{
			GlobalSkillRoots: testGlobalSkillRoots(userDir),
		})
		if err := registry.LoadAll(context.Background()); err != nil {
			t.Fatalf("LoadAll() error = %v", err)
		}

		rewriteSkillFile(t, skillPath, skillWithDescription("modified", "Version two with larger content"))
		watcher := NewWatcher(registry, time.Millisecond)
		watcher.logger = slog.New(slog.NewTextHandler(io.Discard, nil))

		changed, _, changes, err := watcher.detectChanges(context.Background())
		if err != nil {
			t.Fatalf("detectChanges() error = %v", err)
		}
		if !changed {
			t.Fatal("detectChanges() changed = false, want true for post-load modification")
		}
		if len(changes) != 1 || changes[0].path != skillPath || changes[0].action != "modified" {
			t.Fatalf("detectChanges() changes = %#v, want modified change for %q", changes, skillPath)
		}
	})
}

func TestWatcherStartRefreshesOnlyWhenGlobalStateChanges(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	spy := newRefreshSpy()
	watcher := newTestWatcher(spy, 10*time.Millisecond, root)
	if changed, _, _, err := watcher.detectChanges(context.Background()); err != nil {
		t.Fatalf("detectChanges() baseline error = %v", err)
	} else if changed {
		t.Fatal("detectChanges() baseline changed = true, want false")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() {
		watcher.Start(ctx)
		close(done)
	}()

	writeSkillFile(t, root, filepath.Join("hot", skillFileName), skillWithDescription("hot", "Hot reload skill"))

	if err := spy.waitForCalls(1, time.Second); err != nil {
		t.Fatalf("waitForCalls(1) error = %v", err)
	}

	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("watcher did not stop after cancellation")
	}

	if calls := spy.calls(); calls != 1 {
		t.Fatalf("refresh calls after global change = %d, want 1", calls)
	}
}

func TestWatcherStartStopsOnContextCancellation(t *testing.T) {
	t.Parallel()

	watcher := newTestWatcher(nil, 10*time.Millisecond, t.TempDir())

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		watcher.Start(ctx)
		close(done)
	}()

	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("watcher did not stop after cancellation")
	}
}

func TestWatcherPollCancellation(t *testing.T) {
	t.Parallel()

	t.Run("Should skip refresh when cancellation arrives after change detection", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		spy := newRefreshSpy()
		watcher := newTestWatcher(spy, time.Millisecond, root)
		if changed, _, _, err := watcher.detectChanges(context.Background()); err != nil {
			t.Fatalf("detectChanges() baseline error = %v", err)
		} else if changed {
			t.Fatal("detectChanges() baseline changed = true, want false")
		}
		writeSkillFile(
			t,
			root,
			filepath.Join("pending", skillFileName),
			skillWithDescription("pending", "Pending skill"),
		)
		ctx := newCancelAfterContext(context.Background(), 2)

		err := watcher.pollOnce(ctx)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("pollOnce() error = %v, want context.Canceled", err)
		}
		if calls := spy.calls(); calls != 0 {
			t.Fatalf("RefreshGlobal() calls after cancellation = %d, want 0", calls)
		}
	})
}

func TestWatcherStartDoesNotRefreshWithoutChangesAcrossMultiplePolls(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeSkillFile(t, root, filepath.Join("steady", skillFileName), skillWithDescription("steady", "Steady skill"))

	spy := newRefreshSpy()
	watcher := newTestWatcher(spy, 10*time.Millisecond, root)

	for poll := range 3 {
		if err := watcher.pollOnce(context.Background()); err != nil {
			t.Fatalf("pollOnce(%d) error = %v", poll, err)
		}
	}

	if calls := spy.calls(); calls != 0 {
		t.Fatalf("refresh calls = %d, want 0", calls)
	}
}

func newTestWatcher(registry globalRefresher, interval time.Duration, roots ...string) *Watcher {
	watcher := newWatcher(registry, interval, roots)
	watcher.logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	return watcher
}

func setFileTimes(t *testing.T, path string, modTime time.Time) {
	t.Helper()

	if err := os.Chtimes(path, modTime, modTime); err != nil {
		t.Fatalf("Chtimes(%q) error = %v", path, err)
	}
}

type refreshSpy struct {
	mu     sync.Mutex
	callsN int
	notify chan struct{}
}

func newRefreshSpy() *refreshSpy {
	return &refreshSpy{
		notify: make(chan struct{}, 16),
	}
}

func (s *refreshSpy) RefreshGlobal(ctx context.Context) error {
	s.mu.Lock()
	s.callsN++
	s.mu.Unlock()

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	select {
	case s.notify <- struct{}{}:
	default:
	}

	return nil
}

func (s *refreshSpy) calls() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.callsN
}

func (s *refreshSpy) waitForCalls(want int, timeout time.Duration) error {
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()

	for {
		if got := s.calls(); got >= want {
			return nil
		}

		select {
		case <-s.notify:
		case <-deadline.C:
			return context.DeadlineExceeded
		}
	}
}

type cancelAfterContext struct {
	context.Context

	mu        sync.Mutex
	remaining int
	done      chan struct{}
	canceled  bool
}

func newCancelAfterContext(parent context.Context, allowedErrChecks int) *cancelAfterContext {
	if parent == nil {
		parent = context.Background()
	}
	if allowedErrChecks < 0 {
		allowedErrChecks = 0
	}
	return &cancelAfterContext{
		Context:   parent,
		remaining: allowedErrChecks,
		done:      make(chan struct{}),
	}
}

func (c *cancelAfterContext) Done() <-chan struct{} {
	return c.done
}

func (c *cancelAfterContext) Err() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.canceled {
		return context.Canceled
	}
	if err := c.Context.Err(); err != nil {
		c.cancelLocked()
		return err
	}
	if c.remaining > 0 {
		c.remaining--
		return nil
	}

	c.cancelLocked()
	return context.Canceled
}

func (c *cancelAfterContext) cancelLocked() {
	if c.canceled {
		return
	}
	c.canceled = true
	close(c.done)
}
