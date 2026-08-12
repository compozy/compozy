package worktree

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/config"
)

// Canonical suite: registry/discovery merge, cache semantics, and missing tombstones.
func TestServiceDiscovery(t *testing.T) {
	t.Parallel()

	t.Run("Should read registry identity only inside the owning workspace", func(t *testing.T) {
		t.Parallel()
		fixture := newDiscoveryTestFixture(t)
		item := Worktree{
			ID: "wt-owned", WorkspaceID: fixture.workspace.ID, Name: "owned", Path: t.TempDir(),
			State: StateReady, Origin: OriginManual, SetupState: SetupNone,
			CreatedAt: fixture.now, UpdatedAt: fixture.now,
		}
		if err := fixture.store.Insert(context.Background(), item); err != nil {
			t.Fatalf("seed owned worktree: %v", err)
		}
		got, err := fixture.service.Get(context.Background(), fixture.workspace.ID, item.Name)
		if err != nil || got.ID != item.ID {
			t.Fatalf("Get(owner) = %#v, %v, want %s", got, err, item.ID)
		}
		if crossWorkspace, err := fixture.service.Get(context.Background(), "ws-other", item.ID); !errors.Is(err, ErrNotFound) || crossWorkspace != nil {
			t.Fatalf("Get(cross-workspace) = %#v, %v, want ErrNotFound", crossWorkspace, err)
		}
	})

	t.Run("Should merge Git-known entries without duplicating registered canonical paths", func(t *testing.T) {
		t.Parallel()
		fixture := newDiscoveryTestFixture(t)
		registeredPath := t.TempDir()
		discoveredPath := t.TempDir()
		missingPath := filepath.Join(t.TempDir(), "unreachable")
		now := fixture.now
		if err := fixture.store.Insert(context.Background(), Worktree{
			ID: "wt-registered", WorkspaceID: fixture.workspace.ID, Name: "registered",
			Branch: "registered", Path: registeredPath, State: StateReady, Origin: OriginManual,
			SetupState: SetupNone, CreatedAt: now, UpdatedAt: now,
		}); err != nil {
			t.Fatalf("seed registered worktree: %v", err)
		}
		fixture.listOutput = []byte(
			"worktree " + fixture.workspace.Root + "\x00HEAD root\x00branch refs/heads/main\x00\x00" +
				"worktree " + registeredPath + "\x00HEAD registered\x00branch refs/heads/registered\x00\x00" +
				"worktree " + discoveredPath + "\x00HEAD discovered\x00branch refs/heads/discovered\x00\x00" +
				"worktree " + missingPath + "\x00HEAD stale\x00branch refs/heads/stale\x00prunable missing admin dir\x00\x00",
		)
		listing, err := fixture.service.List(context.Background(), fixture.workspace.ID, true)
		if err != nil {
			t.Fatalf("List() error = %v", err)
		}
		if len(listing.Worktrees) != 1 || len(listing.Discovered) != 2 {
			t.Fatalf("List() = %#v, want one registered and two discovered", listing)
		}
		if !listing.Discovered[0].Selectable || !listing.Discovered[1].Stale ||
			listing.Discovered[1].Selectable || !listing.Discovered[1].Unavailable {
			t.Fatalf("discovered states = %#v, want selectable live plus stale unavailable", listing.Discovered)
		}
	})

	t.Run("Should serve repeat reads entirely from the discovery cache and bypass on refresh", func(t *testing.T) {
		t.Parallel()
		fixture := newDiscoveryTestFixture(t)
		fixture.listOutput = worktreeListFixture(fixture.workspace.Root, "main")
		if _, err := fixture.service.List(context.Background(), fixture.workspace.ID, false); err != nil {
			t.Fatalf("List(first) error = %v", err)
		}
		firstCalls := len(fixture.runner.invocations())
		if _, err := fixture.service.List(context.Background(), fixture.workspace.ID, false); err != nil {
			t.Fatalf("List(cached) error = %v", err)
		}
		if got := len(fixture.runner.invocations()); got != firstCalls {
			t.Fatalf("cached List() added %d Git calls, want zero", got-firstCalls)
		}
		if _, err := fixture.service.List(context.Background(), fixture.workspace.ID, true); err != nil {
			t.Fatalf("List(refresh) error = %v", err)
		}
		if got := len(fixture.runner.invocations()); got <= firstCalls {
			t.Fatalf("refresh List() Git calls = %d, want a fresh scan", got)
		}
	})

	t.Run("Should not repopulate stale cache data after a concurrent mutation invalidates the scan", func(t *testing.T) {
		t.Parallel()
		fixture := newDiscoveryTestFixture(t)
		fixture.listOutput = worktreeListFixture(fixture.workspace.Root, "main")
		entered, release := make(chan struct{}), make(chan struct{})
		var calls atomic.Int64
		var blockFirst atomic.Bool
		blockFirst.Store(true)
		baseRun := fixture.runner.run
		fixture.runner.run = func(call gitInvocation) gitResponse {
			if strings.Contains(strings.Join(call.args, " "), "worktree list --porcelain -z") {
				calls.Add(1)
				if blockFirst.CompareAndSwap(true, false) {
					close(entered)
					<-release
				}
			}
			return baseRun(call)
		}
		firstDone := make(chan error, 1)
		go func() {
			_, err := fixture.service.List(context.Background(), fixture.workspace.ID, false)
			firstDone <- err
		}()
		<-entered
		fixture.service.invalidateDiscovery(fixture.workspace.ID)
		close(release)
		if err := <-firstDone; err != nil {
			t.Fatalf("List(in-flight) error = %v", err)
		}
		if _, err := fixture.service.List(context.Background(), fixture.workspace.ID, false); err != nil {
			t.Fatalf("List(after invalidation) error = %v", err)
		}
		if got := calls.Load(); got != 2 {
			t.Fatalf("discovery scans = %d, want stale in-flight result discarded", got)
		}
	})

	t.Run("Should retain registry rows and diagnostics when discovery fails", func(t *testing.T) {
		t.Parallel()
		fixture := newDiscoveryTestFixture(t)
		fixture.scanErr = errors.New("scan failed with token ghp_abcdefghijklmnopqrstuvwxyz0123456789")
		now := fixture.now
		if err := fixture.store.Insert(context.Background(), Worktree{
			ID: "wt-retained", WorkspaceID: fixture.workspace.ID, Name: "retained", Path: t.TempDir(),
			State: StateReady, Origin: OriginManual, SetupState: SetupNone, CreatedAt: now, UpdatedAt: now,
		}); err != nil {
			t.Fatalf("seed retained worktree: %v", err)
		}
		listing, err := fixture.service.List(context.Background(), fixture.workspace.ID, true)
		if err != nil {
			t.Fatalf("List(failed discovery) error = %v", err)
		}
		if len(listing.Worktrees) != 1 || listing.Repo.Diagnostic == "" || strings.Contains(listing.Repo.Diagnostic, "ghp_") {
			t.Fatalf("List(failed discovery) = %#v, want retained row and redacted diagnostic", listing)
		}
	})

	t.Run("Should mark a vanished registry row missing exactly once", func(t *testing.T) {
		t.Parallel()
		fixture := newDiscoveryTestFixture(t)
		fixture.listOutput = worktreeListFixture(fixture.workspace.Root, "main")
		now := fixture.now
		vanished := filepath.Join(t.TempDir(), "vanished")
		if err := fixture.store.Insert(context.Background(), Worktree{
			ID: "wt-vanished", WorkspaceID: fixture.workspace.ID, Name: "vanished", Path: vanished,
			State: StateReady, Origin: OriginManual, SetupState: SetupNone, CreatedAt: now, UpdatedAt: now,
		}); err != nil {
			t.Fatalf("seed vanished worktree: %v", err)
		}
		first, err := fixture.service.List(context.Background(), fixture.workspace.ID, true)
		if err != nil || len(first.Worktrees) != 1 || first.Worktrees[0].State != StateMissing {
			t.Fatalf("List(vanished) = %#v, %v, want missing", first, err)
		}
		second, err := fixture.service.List(context.Background(), fixture.workspace.ID, true)
		if err != nil || len(second.Worktrees) != 1 || second.Worktrees[0].State != StateMissing {
			t.Fatalf("List(vanished repeat) = %#v, %v, want stable missing", second, err)
		}
		if got := fixture.events.count(EventMissing); got != 1 {
			t.Fatalf("missing events = %d, want exactly one", got)
		}
	})

	t.Run("Should return an empty non-git listing without scanning Git", func(t *testing.T) {
		t.Parallel()
		workspace := Workspace{ID: "ws-nongit", Name: "Non Git", Root: t.TempDir()}
		runner := &recordingGitRunner{}
		service := NewService(
			newMemoryWorktreeStore(), runner, WithCapabilityGate(readyCapabilityGate()),
			WithWorkspaceResolver(staticWorkspaceResolver{workspaces: map[string]Workspace{workspace.ID: workspace}}),
		)
		listing, err := service.List(context.Background(), workspace.ID, false)
		if err != nil || listing.Repo.GitBacked || len(listing.Worktrees) != 0 || len(runner.invocations()) != 0 {
			t.Fatalf("List(non-git) = %#v, %v calls=%#v, want empty truthful listing", listing, err, runner.invocations())
		}
	})

	t.Run("Should return an empty Git-backed listing when only the main checkout exists", func(t *testing.T) {
		t.Parallel()
		fixture := newDiscoveryTestFixture(t)
		fixture.listOutput = worktreeListFixture(fixture.workspace.Root, "main")
		listing, err := fixture.service.List(context.Background(), fixture.workspace.ID, true)
		if err != nil || !listing.Repo.GitBacked || len(listing.Worktrees) != 0 || len(listing.Discovered) != 0 {
			t.Fatalf("List(empty Git repository) = %#v, %v, want empty Git-backed listing", listing, err)
		}
	})
}

type discoveryTestFixture struct {
	service    *Service
	store      *memoryWorktreeStore
	runner     *recordingGitRunner
	workspace  Workspace
	commonDir  string
	listOutput []byte
	scanErr    error
	now        time.Time
	events     *recordingEventSink
}

func newDiscoveryTestFixture(t *testing.T) *discoveryTestFixture {
	t.Helper()
	root := t.TempDir()
	canonicalRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatalf("canonicalize workspace root: %v", err)
	}
	commonDir := filepath.Join(canonicalRoot, ".git")
	if err := os.MkdirAll(commonDir, 0o700); err != nil {
		t.Fatalf("create common directory: %v", err)
	}
	workspace := Workspace{ID: "ws-discovery", Name: "Discovery", Root: root}
	worktreesConfig := config.DefaultWorktreesConfig()
	worktreesConfig.DiscoveryCacheTTL = time.Minute
	fixture := &discoveryTestFixture{
		store: newMemoryWorktreeStore(), workspace: workspace, commonDir: commonDir,
		now: time.Date(2026, 8, 12, 15, 0, 0, 0, time.UTC),
	}
	runner := &recordingGitRunner{run: func(call gitInvocation) gitResponse {
		switch strings.Join(call.args, " ") {
		case "rev-parse --path-format=absolute --git-common-dir":
			return gitResponse{stdout: []byte(commonDir + "\n")}
		case "--git-dir " + commonDir + " worktree list --porcelain -z":
			if fixture.scanErr != nil {
				return gitResponse{stderr: []byte(fixture.scanErr.Error()), err: fixture.scanErr}
			}
			return gitResponse{stdout: append([]byte(nil), fixture.listOutput...)}
		default:
			return gitResponse{err: errors.New("unexpected discovery call")}
		}
	}}
	fixture.runner = runner
	fixture.events = &recordingEventSink{}
	fixture.service = NewService(
		fixture.store, runner, WithCapabilityGate(readyCapabilityGate()),
		WithWorkspaceResolver(staticWorkspaceResolver{workspaces: map[string]Workspace{workspace.ID: workspace}}),
		WithConfig(worktreesConfig, t.TempDir()), WithClock(func() time.Time { return fixture.now }),
		WithEvents(fixture.events),
	)
	return fixture
}
