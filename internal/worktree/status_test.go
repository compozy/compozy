package worktree

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/diagnostics"
)

// Canonical suite: truthful status assembly and NULL-unknown persistence.
func TestServiceStatus(t *testing.T) {
	t.Parallel()

	t.Run("Should serve cached status without invoking Git", func(t *testing.T) {
		t.Parallel()
		store := newMemoryWorktreeStore()
		item := statusTestWorktree()
		if err := store.Insert(t.Context(), item); err != nil {
			t.Fatalf("seed worktree: %v", err)
		}
		dirty := 3
		cached := Status{WorktreeID: item.ID, DirtyFiles: &dirty}
		if err := store.SaveStatus(t.Context(), item.WorkspaceID, item.ID, cached); err != nil {
			t.Fatalf("seed cached status: %v", err)
		}
		runner := &recordingGitRunner{}
		service := NewService(store, runner, WithCapabilityGate(readyCapabilityGate()))
		got, err := service.Status(t.Context(), item.WorkspaceID, item.Name, false)
		if err != nil || got.DirtyFiles == nil || *got.DirtyFiles != dirty || len(runner.invocations()) != 0 {
			t.Fatalf("Status(cached) = %#v, %v calls=%#v, want cached value", got, err, runner.invocations())
		}
	})

	t.Run("Should reject missing and cross-workspace status targets before Git", func(t *testing.T) {
		t.Parallel()
		store := newMemoryWorktreeStore()
		item := statusTestWorktree()
		item.State = StateMissing
		if err := store.Insert(t.Context(), item); err != nil {
			t.Fatalf("seed missing worktree: %v", err)
		}
		runner := &recordingGitRunner{}
		service := NewService(store, runner, WithCapabilityGate(readyCapabilityGate()))
		if got, err := service.Status(
			t.Context(),
			item.WorkspaceID,
			item.ID,
			false,
		); !errors.Is(err, ErrMissing) ||
			got != nil {
			t.Fatalf("Status(missing) = %#v, %v, want ErrMissing", got, err)
		}
		if got, err := service.Status(
			t.Context(),
			"ws-other",
			item.ID,
			true,
		); !errors.Is(err, ErrNotFound) ||
			got != nil {
			t.Fatalf("Status(cross-workspace) = %#v, %v, want ErrNotFound", got, err)
		}
		if len(runner.invocations()) != 0 {
			t.Fatalf("status refusal Git calls = %#v, want none", runner.invocations())
		}
	})

	t.Run("Should assemble and persist canonical status after a refreshed name lookup", func(t *testing.T) {
		t.Parallel()
		store := newMemoryWorktreeStore()
		item := statusTestWorktree()
		if err := store.Insert(t.Context(), item); err != nil {
			t.Fatalf("seed worktree: %v", err)
		}
		runner := &recordingGitRunner{run: func(call gitInvocation) gitResponse {
			switch strings.Join(call.args, " ") {
			case "status --porcelain=v2 --branch -z":
				return gitResponse{stdout: []byte("# branch.oid abc\x00# branch.head feature/status\x00" +
					"# branch.upstream origin/feature/status\x00# branch.ab +1 -0\x00" +
					"1 M. N... a.txt\x001 .M N... b.txt\x00? c.txt\x00")}
			case "diff --numstat -z":
				return gitResponse{stdout: []byte("4\t1\ta.txt\x001\t0\tb.txt\x00")}
			case "diff --cached --numstat -z":
				return gitResponse{stdout: []byte("5\t1\ta.txt\x00")}
			default:
				return gitResponse{err: errors.New("unexpected status call")}
			}
		}}
		events := &recordingEventSink{}
		service := NewService(
			store, runner, WithCapabilityGate(readyCapabilityGate()), WithClock(statusTestClock), WithEvents(events),
		)
		status, err := service.Status(t.Context(), item.WorkspaceID, item.Name, true)
		if err != nil {
			t.Fatalf("Status() error = %v", err)
		}
		metricsMatch := valueOrZero(status.DirtyFiles) == 3 &&
			valueOrZero(status.Insertions) == 10 &&
			valueOrZero(status.Deletions) == 2
		upstreamMatches := status.Ahead != nil && *status.Ahead == 1 &&
			status.Behind != nil && *status.Behind == 0 &&
			status.HasUpstream != nil && *status.HasUpstream
		if status.WorktreeID != item.ID || !metricsMatch || !upstreamMatches {
			t.Fatalf("Status() = %#v, want 3 files +10 -2 and upstream +1 -0", status)
		}
		persisted, err := store.GetStatus(t.Context(), item.WorkspaceID, item.ID)
		if err != nil || persisted == nil || persisted.WorktreeID != item.ID || persisted.RefreshedAt == nil ||
			!persisted.RefreshedAt.Equal(statusTestClock()) {
			t.Fatalf("persisted status = %#v, %v, want refreshed timestamp", persisted, err)
		}
		if got := events.count(EventStatusRefreshed); got != 1 {
			t.Fatalf("status refreshed events = %d, want one", got)
		}
	})

	// Invariant: a clean snapshot has zero diff totals while retaining branch divergence.
	t.Run("Should read clean status without starting redundant diff processes", func(t *testing.T) {
		t.Parallel()
		item := statusTestWorktree()
		runner := &recordingGitRunner{run: func(call gitInvocation) gitResponse {
			if strings.Join(call.args, " ") != "status --porcelain=v2 --branch -z" {
				return gitResponse{err: errors.New("unexpected clean-checkout command")}
			}
			return gitResponse{stdout: []byte("# branch.oid abc\x00# branch.head feature/status\x00" +
				"# branch.upstream origin/feature/status\x00# branch.ab +2 -1\x00")}
		}}
		service := NewService(newMemoryWorktreeStore(), runner, WithClock(statusTestClock))
		status := service.collectStatus(t.Context(), item)
		if status.ReadError != "" || status.DirtyFiles == nil || *status.DirtyFiles != 0 ||
			status.Insertions == nil || *status.Insertions != 0 || status.Deletions == nil || *status.Deletions != 0 ||
			status.Ahead == nil || *status.Ahead != 2 || status.Behind == nil || *status.Behind != 1 ||
			status.HeadSHA == nil || *status.HeadSHA != "abc" || status.RefreshedAt == nil ||
			!status.RefreshedAt.Equal(statusTestClock()) {
			t.Fatalf("clean status = %#v, want zero diff totals with complete branch facts", status)
		}
		if len(runner.invocations()) != 1 {
			t.Fatalf("clean status commands = %#v, want only porcelain", runner.invocations())
		}
	})

	t.Run("Should clear formerly known values when a status read fails", func(t *testing.T) {
		t.Parallel()
		store := newMemoryWorktreeStore()
		item := statusTestWorktree()
		if err := store.Insert(t.Context(), item); err != nil {
			t.Fatalf("seed worktree: %v", err)
		}
		known := 7
		if err := store.SaveStatus(t.Context(), item.WorkspaceID, item.ID, Status{
			WorktreeID: item.ID, DirtyFiles: &known, Ahead: &known, Behind: &known,
		}); err != nil {
			t.Fatalf("seed known status: %v", err)
		}
		runner := &recordingGitRunner{run: func(gitInvocation) gitResponse {
			return gitResponse{stderr: []byte("fatal: cannot read index"), err: errors.New("exit 128")}
		}}
		service := NewService(store, runner, WithCapabilityGate(readyCapabilityGate()), WithClock(statusTestClock))
		status, err := service.Status(t.Context(), item.WorkspaceID, item.ID, true)
		if err != nil {
			t.Fatalf("Status(read failure) error = %v", err)
		}
		if status.ReadError == "" || status.DirtyFiles != nil || status.Ahead != nil || status.Behind != nil ||
			status.HasUpstream != nil {
			t.Fatalf("Status(read failure) = %#v, want explicit error and unknown numeric fields", status)
		}
	})

	t.Run("Should map malformed porcelain into read_error instead of returning a parser failure", func(t *testing.T) {
		t.Parallel()
		store := newMemoryWorktreeStore()
		item := statusTestWorktree()
		if err := store.Insert(t.Context(), item); err != nil {
			t.Fatalf("seed worktree: %v", err)
		}
		runner := &recordingGitRunner{run: func(gitInvocation) gitResponse {
			return gitResponse{stdout: []byte("garbled\x00")}
		}}
		service := NewService(store, runner, WithCapabilityGate(readyCapabilityGate()), WithClock(statusTestClock))
		status, err := service.Status(t.Context(), item.WorkspaceID, item.ID, true)
		if err != nil || status.ReadError == "" || !strings.Contains(status.ReadError, "parse status-v2") {
			t.Fatalf("Status(garbled) = %#v, %v, want persisted parser diagnostic", status, err)
		}
	})

	t.Run("Should keep remote counts absent while exposing ahead-of-base separately by context", func(t *testing.T) {
		t.Parallel()
		store := newMemoryWorktreeStore()
		item := statusTestWorktree()
		if err := store.Insert(t.Context(), item); err != nil {
			t.Fatalf("seed worktree: %v", err)
		}
		runner := &recordingGitRunner{run: func(call gitInvocation) gitResponse {
			switch strings.Join(call.args, " ") {
			case "status --porcelain=v2 --branch -z":
				return gitResponse{stdout: []byte("# branch.oid abc\x00# branch.head feature/status\x00")}
			case "diff --numstat -z", "diff --cached --numstat -z":
				return gitResponse{}
			case "rev-list --count main..HEAD":
				return gitResponse{stdout: []byte("2\n")}
			default:
				return gitResponse{err: errors.New("unexpected status call")}
			}
		}}
		service := NewService(store, runner, WithCapabilityGate(readyCapabilityGate()), WithClock(statusTestClock))
		status, err := service.Status(t.Context(), item.WorkspaceID, item.ID, true)
		if err != nil {
			t.Fatalf("Status(no upstream) error = %v", err)
		}
		if status.HasUpstream == nil || *status.HasUpstream || status.Ahead != nil ||
			status.AheadOfBase == nil || *status.AheadOfBase != 2 || status.Behind != nil {
			t.Fatalf("Status(no upstream) = %#v, want ahead-of-base 2 and no remote behind value", status)
		}
	})

	t.Run("Should derive cache staleness from the persisted observation timestamps", func(t *testing.T) {
		t.Parallel()
		refreshedAt := statusTestClock()
		ttl := 30 * time.Second
		status := Status{RefreshedAt: &refreshedAt}
		forge := ForgeStatus{FetchedAt: &refreshedAt}
		if status.IsStale(refreshedAt.Add(29*time.Second), ttl) || forge.IsStale(refreshedAt.Add(29*time.Second), ttl) {
			t.Fatal("fresh cached values were marked stale")
		}
		if !status.IsStale(refreshedAt.Add(ttl), ttl) || !forge.IsStale(refreshedAt.Add(ttl), ttl) {
			t.Fatal("expired cached values were not marked stale")
		}
		if !(Status{}).IsStale(refreshedAt, ttl) || !(ForgeStatus{}).IsStale(refreshedAt, ttl) {
			t.Fatal("missing timestamps must remain stale instead of pretending freshness")
		}
	})

	t.Run("Should redact registered secrets before persisting read errors", func(t *testing.T) {
		t.Parallel()
		const secret = "status-bound-secret-123456"
		cleanup := diagnostics.RegisterRequiredSecret(secret)
		t.Cleanup(cleanup)
		store := newMemoryWorktreeStore()
		item := statusTestWorktree()
		if err := store.Insert(t.Context(), item); err != nil {
			t.Fatalf("seed worktree: %v", err)
		}
		runner := &recordingGitRunner{run: func(gitInvocation) gitResponse {
			return gitResponse{stderr: []byte("token=" + secret), err: errors.New("exit 128")}
		}}
		service := NewService(store, runner, WithCapabilityGate(readyCapabilityGate()), WithClock(statusTestClock))
		status, err := service.Status(t.Context(), item.WorkspaceID, item.ID, true)
		if err != nil || status == nil || strings.Contains(status.ReadError, secret) ||
			!strings.Contains(status.ReadError, "[REDACTED]") {
			t.Fatalf("Status(secret failure) = %#v, %v, want redacted read_error", status, err)
		}
	})

	// Invariant: detached worktrees have no display PR facts and never perform a lookup.
	// Owner: worktree status details; canonical status suite.
	t.Run("Should return no forge facts for detached worktrees during explicit refresh", func(t *testing.T) {
		t.Parallel()
		for _, enabled := range []bool{false, true} {
			t.Run(fmt.Sprintf("Should handle configured forge %t", enabled), func(t *testing.T) {
				t.Parallel()
				store := newMemoryWorktreeStore()
				item := statusTestWorktree()
				item.Branch = ""
				if err := store.Insert(t.Context(), item); err != nil {
					t.Fatal(err)
				}
				if err := store.SaveStatus(
					t.Context(),
					item.WorkspaceID,
					item.ID,
					Status{WorktreeID: item.ID},
				); err != nil {
					t.Fatal(err)
				}
				runner := &recordingGitRunner{run: func(call gitInvocation) gitResponse {
					t.Errorf("unexpected Git lookup: %v", call.args)
					return gitResponse{err: errors.New("unexpected lookup")}
				}}
				service := NewService(store, runner, WithCapabilityGate(readyCapabilityGate()))
				if enabled {
					forge := pullRequestTestForge{
						status: func(context.Context, ForgeStatusRequest) (*ForgeStatus, error) {
							t.Error("unexpected forge lookup")
							return nil, ErrForge
						},
					}
					attachTestPullRequests(t, service, item, forge)
				}
				details, err := service.StatusDetails(t.Context(), item.WorkspaceID, item.ID, false, true)
				if err != nil || details == nil || details.ForgeStatus != nil || details.Status == nil {
					t.Fatalf("detached details=%#v error=%v", details, err)
				}
			})
		}
	})

	t.Run("Should preserve an unavailable forge cause during explicit refresh", func(t *testing.T) {
		t.Parallel()
		store := newMemoryWorktreeStore()
		item := statusTestWorktree()
		if err := store.Insert(t.Context(), item); err != nil {
			t.Fatalf("seed worktree: %v", err)
		}
		if err := store.SaveStatus(t.Context(), item.WorkspaceID, item.ID, Status{
			WorktreeID: item.ID,
		}); err != nil {
			t.Fatalf("seed cached status: %v", err)
		}
		runner := &recordingGitRunner{run: func(call gitInvocation) gitResponse {
			if strings.Join(call.args, " ") == "remote get-url origin" {
				return gitResponse{stdout: []byte("https://github.com/acme/repo.git\n")}
			}
			return gitResponse{err: errors.New("unexpected Git call")}
		}}
		forge := &recordingExitForge{statusErr: ErrForgeUnavailable}
		service := NewService(store, runner, WithCapabilityGate(readyCapabilityGate()), WithForge(forge))
		attachTestPullRequests(t, service, item, forge)
		_, err := service.StatusDetails(t.Context(), item.WorkspaceID, item.ID, false, true)
		if !errors.Is(err, ErrForgeUnavailable) || errors.Is(err, ErrForge) {
			t.Fatalf("StatusDetails(forge unavailable) error = %v, want only ErrForgeUnavailable", err)
		}
	})

	t.Run("Should report the canonical identity after status lookup by name", func(t *testing.T) {
		t.Parallel()
		store := newMemoryWorktreeStore()
		item := statusTestWorktree()
		if err := store.Insert(t.Context(), item); err != nil {
			t.Fatalf("seed worktree: %v", err)
		}
		if err := store.SaveStatus(t.Context(), item.WorkspaceID, item.ID, Status{
			WorktreeID: item.ID,
		}); err != nil {
			t.Fatalf("seed cached status: %v", err)
		}
		runner := &recordingGitRunner{run: func(call gitInvocation) gitResponse {
			if strings.Join(call.args, " ") == "remote get-url origin" {
				return gitResponse{stdout: []byte("https://github.com/acme/repo.git\n")}
			}
			return gitResponse{err: errors.New("unexpected Git call")}
		}}
		forge := &recordingExitForge{status: &ForgeStatus{WorktreeID: item.ID, Provider: "github"}}
		service := NewService(
			store,
			runner,
			WithCapabilityGate(readyCapabilityGate()),
			WithForge(forge),
		)
		cache := attachTestPullRequests(t, service, item, forge)
		details, err := service.StatusDetails(
			t.Context(), item.WorkspaceID, item.Name, false, true,
		)
		key := PullRequestKey{item.WorkspaceID, "github.com/acme/repo", item.Branch}
		entries, cacheErr := cache.ReadPullRequests(t.Context(), []PullRequestKey{key})
		cachedForge := entries[key].Facts
		if err != nil || details == nil || details.WorktreeID != item.ID ||
			forge.lastStatus.Branch != item.Branch || forge.lastStatus.WorkspaceID != item.WorkspaceID || cacheErr != nil || cachedForge == nil {
			t.Fatalf("StatusDetails(name) = %#v, %v, want canonical id %q", details, err, item.ID)
		}
	})
}

func statusTestWorktree() Worktree {
	now := statusTestClock()
	return Worktree{
		ID: "wt-status", WorkspaceID: "ws-status", Name: "status", Branch: "feature/status",
		Path: "/tmp/worktree-status", State: StateReady, Origin: OriginManual,
		SetupState: SetupNone, BaseRef: "main", CreatedAt: now, UpdatedAt: now,
	}
}

func statusTestClock() time.Time {
	return time.Date(2026, 8, 12, 13, 0, 0, 0, time.UTC)
}

func attachTestPullRequests(
	t *testing.T,
	service *Service,
	item Worktree,
	forge ForgeProvider,
) *pullRequestMemoryStore {
	t.Helper()
	cache := newPullRequestMemoryStore()
	service.pullRequests = NewPullRequestService(t.Context(), cache, forge, service.runner, nil)
	service.resolver = staticWorkspaceResolver{
		workspaces: map[string]Workspace{item.WorkspaceID: {ID: item.WorkspaceID, Root: item.Path}},
	}
	t.Cleanup(service.pullRequests.Close)
	return cache
}

func BenchmarkCollectCleanWorktreeStatus(b *testing.B) {
	runner, err := NewRealGitRunner(0)
	if err != nil {
		b.Fatal(err)
	}
	root := b.TempDir()
	for _, args := range [][]string{{"init", "--initial-branch=main"}, {"config", "user.name", "Performance fixture"}, {"config", "user.email", "fixture@example.test"}} {
		if _, _, err := runner.Run(b.Context(), root, args...); err != nil {
			b.Fatal(err)
		}
	}
	for index := range 256 {
		if err := os.WriteFile(
			filepath.Join(root, fmt.Sprintf("file-%03d.txt", index)),
			[]byte("fixture\n"),
			0600,
		); err != nil {
			b.Fatal(err)
		}
	}
	for _, args := range [][]string{{"add", "."}, {"commit", "-m", "fixture"}} {
		if _, _, err := runner.Run(b.Context(), root, args...); err != nil {
			b.Fatal(err)
		}
	}
	service := NewService(newMemoryWorktreeStore(), runner)
	item := Worktree{ID: "fixture", Path: root}
	b.ReportAllocs()
	for b.Loop() {
		status := service.collectStatus(b.Context(), item)
		if status.ReadError != "" || status.DirtyFiles == nil || *status.DirtyFiles != 0 || status.Insertions == nil ||
			*status.Insertions != 0 ||
			status.Deletions == nil ||
			*status.Deletions != 0 {
			b.Fatalf("unexpected clean status %#v", status)
		}
	}
}
