package worktree

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// Invariants: canonical display refresh admission, identity and failure retention.
// Owner: PullRequestService; this is its canonical suite (new subsystem).
func TestPullRequestService(t *testing.T) {
	t.Parallel()
	t.Run("Should leave the forge alone while a watcher's retry-after holds", func(t *testing.T) {
		t.Parallel()
		cache := newPullRequestMemoryStore()
		now := statusTestClock()
		retry := now.Add(time.Hour)
		key := PullRequestKey{"ws", "github.com/acme/repo", "limited"}
		if err := cache.SavePullRequest(t.Context(), PullRequestEntry{
			Key: key, LookupState: "error", LookupCause: "rate_limited", RetryAfter: &retry,
		}); err != nil {
			t.Fatal(err)
		}
		var calls atomic.Int32
		provider := pullRequestTestForge{status: func(context.Context, ForgeStatusRequest) (*ForgeStatus, error) {
			calls.Add(1)
			return &ForgeStatus{Reviews: &ForgeReviews{UnresolvedThreads: 1}}, nil
		}}
		service := NewPullRequestService(t.Context(), cache, provider, nil, nil)
		t.Cleanup(service.Close)
		service.now = func() time.Time { return now }
		held, err := service.RefreshDue(t.Context(), key)
		if err != nil || held.LookupState != "error" || calls.Load() != 0 {
			t.Fatalf("held = %+v err = %v calls = %d", held, err, calls.Load())
		}
		service.now = func() time.Time { return retry.Add(time.Second) }
		due, err := service.RefreshDue(t.Context(), key)
		if err != nil || due.LookupState != "ok" || calls.Load() != 1 {
			t.Fatalf("due = %+v err = %v calls = %d", due, err, calls.Load())
		}
		if due.Facts == nil || due.Facts.Reviews == nil || due.Facts.Reviews.UnresolvedThreads != 1 {
			t.Fatalf("live entry lost review activity: %+v", due.Facts)
		}
	})
	t.Run("Should bypass backoff when joining a queued automatic flight", func(t *testing.T) {
		t.Parallel()
		cache := newPullRequestMemoryStore()
		now := statusTestClock()
		retry := now.Add(time.Hour)
		key := PullRequestKey{"ws", "github.com/acme/repo", "backoff"}
		if err := cache.SavePullRequest(t.Context(), PullRequestEntry{
			Key: key, LookupState: "error", LookupCause: "rate_limited", RetryAfter: &retry,
		}); err != nil {
			t.Fatal(err)
		}
		entered := make(chan struct{}, 4)
		release := make(chan struct{})
		var targetCalls atomic.Int32
		provider := pullRequestTestForge{
			status: func(ctx context.Context, request ForgeStatusRequest) (*ForgeStatus, error) {
				if request.Branch == key.Branch {
					targetCalls.Add(1)
					return &ForgeStatus{}, nil
				}
				entered <- struct{}{}
				select {
				case <-release:
					return &ForgeStatus{}, nil
				case <-ctx.Done():
					return nil, ctx.Err()
				}
			},
		}
		service := NewPullRequestService(t.Context(), cache, provider, nil, nil)
		t.Cleanup(service.Close)
		service.now = func() time.Time { return now }
		for _, branch := range []string{"busy-a", "busy-b", "busy-c", "busy-d"} {
			service.start(PullRequestKey{key.WorkspaceID, key.RepoKey, branch}, true)
		}
		for range 4 {
			<-entered
		}
		automatic := service.start(key, false)
		explicit := service.start(key, true)
		close(release)
		<-explicit.done
		if explicit != automatic {
			t.Fatal("expected single flight")
		}
		if got := targetCalls.Load(); got != 1 {
			t.Fatalf(
				"explicit refresh made %d provider calls; want 1 despite automatic backoff (error=%v)",
				got,
				explicit.err,
			)
		}
	})

	// Invariant: a removed branch never spends quota on a queued automatic refresh.
	// Owner: PullRequestService admission; canonical service suite.
	t.Run("Should discard queued automatic refreshes on removal", func(t *testing.T) {
		t.Parallel()
		entered := make(chan struct{}, 4)
		release := make(chan struct{})
		var removedCalls atomic.Int32
		provider := pullRequestTestForge{
			status: func(ctx context.Context, request ForgeStatusRequest) (*ForgeStatus, error) {
				if request.Branch == "removed" {
					removedCalls.Add(1)
					return &ForgeStatus{}, nil
				}
				entered <- struct{}{}
				select {
				case <-release:
					return &ForgeStatus{}, nil
				case <-ctx.Done():
					return nil, ctx.Err()
				}
			},
		}
		service := NewPullRequestService(t.Context(), newPullRequestMemoryStore(), provider, nil, nil)
		t.Cleanup(service.Close)
		for i := range 4 {
			service.RequestRefresh(PullRequestKey{"ws", "github.com/acme/repo", fmt.Sprint(i)})
		}
		for range 4 {
			<-entered
		}
		key := PullRequestKey{"ws", "github.com/acme/repo", "removed"}
		service.RequestRefresh(key)
		service.ForgetAutomatic(key)
		close(release)
		service.Close()
		if removedCalls.Load() != 0 {
			t.Fatal("removed branch reached the forge")
		}
	})
	// Invariant: incomplete matches do not inherit confirmed absence backoff.
	// Owner: PullRequestService refresh; canonical service suite.
	t.Run("Should clear absence after an incomplete match", func(t *testing.T) {
		t.Parallel()
		incomplete := false
		provider := pullRequestTestForge{status: func(context.Context, ForgeStatusRequest) (*ForgeStatus, error) {
			if incomplete {
				return &ForgeStatus{Selection: pullRequestIncomplete}, nil
			}
			return &ForgeStatus{}, nil
		}}
		service := NewPullRequestService(t.Context(), newPullRequestMemoryStore(), provider, nil, nil)
		t.Cleanup(service.Close)
		key := PullRequestKey{"ws", "github.com/acme/repo", "topic"}
		if _, err := service.Refresh(t.Context(), key); err != nil {
			t.Fatal(err)
		}
		incomplete = true
		entry, err := service.Refresh(t.Context(), key)
		if err != nil || entry.LookupState != pullRequestIncomplete || entry.Facts.PRState != nil {
			t.Fatalf("incomplete entry = %+v, %v", entry, err)
		}
		if !service.stale(key, entry, entry.Facts.FetchedAt.Add(time.Minute)) {
			t.Fatal("incomplete match inherited absence window")
		}
	})

	// Invariant: explicit requests take the next free slot, including promoted automatic flights.
	t.Run("Should prioritize explicit refreshes over automatic backlog", func(t *testing.T) {
		t.Parallel()
		for _, promote := range []bool{false, true} {
			t.Run(fmt.Sprintf("Should prioritize with promotion %t", promote), func(t *testing.T) {
				t.Parallel()
				entered := make(chan string, 32)
				release := make(chan struct{})
				provider := pullRequestTestForge{
					status: func(ctx context.Context, request ForgeStatusRequest) (*ForgeStatus, error) {
						entered <- request.Branch
						select {
						case <-release:
							return &ForgeStatus{}, nil
						case <-ctx.Done():
							return nil, ctx.Err()
						}
					},
				}
				service := NewPullRequestService(t.Context(), newPullRequestMemoryStore(), provider, nil, nil)
				t.Cleanup(service.Close)
				for i := range 20 {
					service.RequestRefresh(PullRequestKey{"ws", "github.com/acme/repo", fmt.Sprintf("auto-%d", i)})
				}
				for range 4 {
					<-entered
				}
				key := PullRequestKey{"ws", "github.com/acme/repo", "explicit"}
				if promote {
					key.Branch = "auto-19"
				}
				flight := service.start(key, true)
				release <- struct{}{}
				if branch := <-entered; branch != key.Branch {
					t.Fatalf("next admitted branch=%q want %q", branch, key.Branch)
				}
				service.Close()
				<-flight.done
			})
		}
	})

	// Invariant: unavailable repository identities share the per-root freshness window.
	// Owner: PullRequestService; repository identity belongs to this canonical suite.
	t.Run("Should cache unavailable origins per root and recover after the identity window", func(t *testing.T) {
		t.Parallel()
		for _, remote := range []string{"missing", "local"} {
			t.Run("Should recover from "+remote+" origin", func(t *testing.T) {
				t.Parallel()
				runner, err := NewRealGitRunner(0)
				if err != nil {
					t.Fatal(err)
				}
				root := t.TempDir()
				git := func(dir string, args ...string) {
					t.Helper()
					if _, _, err := runner.Run(t.Context(), dir, args...); err != nil {
						t.Fatal(err)
					}
				}
				git(root, "init", "--initial-branch=main")
				if remote == "local" {
					git(root, "remote", "add", "origin", "/tmp/local-repository.git")
				}
				service := NewPullRequestService(t.Context(), newPullRequestMemoryStore(), nil, runner, nil)
				t.Cleanup(service.Close)
				now := statusTestClock()
				service.now = func() time.Time { return now }
				assertUnavailableOrigin := func() {
					t.Helper()
					key, err := service.RepoKey(t.Context(), root)
					if key != "" {
						t.Fatalf("unsupported origin key = %q", key)
					}
					if remote == "missing" {
						if _, ok := errors.AsType[*exec.ExitError](err); !ok {
							t.Fatalf("missing origin error = %v; want Git exit error", err)
						}
					} else if err == nil || err.Error() != "worktree: origin repository unavailable" {
						t.Fatalf("local origin error = %v; want unavailable repository", err)
					}
				}
				assertUnavailableOrigin()
				command := "add"
				if remote == "local" {
					command = "set-url"
				}
				git(root, "remote", command, "origin", "https://github.com/acme/recovered.git")
				assertUnavailableOrigin()
				other := t.TempDir()
				git(other, "init", "--initial-branch=main")
				git(other, "remote", "add", "origin", "https://github.com/acme/other.git")
				if key, err := service.RepoKey(t.Context(), other); err != nil || key != "github.com/acme/other" {
					t.Fatalf("other root identity = %q, %v", key, err)
				}
				now = now.Add(time.Minute)
				if key, err := service.RepoKey(t.Context(), root); err != nil || key != "github.com/acme/recovered" {
					t.Fatalf("recovered identity = %q, %v", key, err)
				}
			})
		}
	})
	t.Run("Should retry repository identity after interruption or an unavailable runner", func(t *testing.T) {
		t.Parallel()
		for _, failure := range []error{context.Canceled, context.DeadlineExceeded, ErrGitUnavailable} {
			t.Run("Should retry after "+failure.Error(), func(t *testing.T) {
				t.Parallel()
				runner := &scriptedGitRunner{err: fmt.Errorf("git interrupted: %w", failure)}
				service := NewPullRequestService(t.Context(), newPullRequestMemoryStore(), nil, runner, nil)
				t.Cleanup(service.Close)
				if _, err := service.RepoKey(t.Context(), "/workspace"); !errors.Is(err, failure) {
					t.Fatalf("interrupted read = %v", err)
				}
				runner.err = nil
				runner.stdout = []byte("https://github.com/acme/repo.git")
				if key, err := service.RepoKey(t.Context(), "/workspace"); err != nil || key != "github.com/acme/repo" {
					t.Fatalf("retry identity = %q, %v", key, err)
				}
			})
		}
	})

	// Invariant: SQL names and display filtering share origin HEAD and its TTL.
	// Owner: PullRequestService; canonical service suite, exercised with real Git.
	t.Run("Should share cached default branch names with display filtering", func(t *testing.T) {
		t.Parallel()
		runner, err := NewRealGitRunner(0)
		if err != nil {
			t.Fatal(err)
		}
		root := t.TempDir()
		if _, _, err := runner.Run(t.Context(), root, "init", "--initial-branch=main"); err != nil {
			t.Fatal(err)
		}
		service := NewPullRequestService(t.Context(), newPullRequestMemoryStore(), nil, runner, nil)
		t.Cleanup(service.Close)
		now := statusTestClock()
		service.now = func() time.Time { return now }
		names := service.DefaultBranchNames(t.Context(), root)
		if len(names) != 2 || names[0] != "main" || names[1] != "master" {
			t.Fatalf("fallback: %v", names)
		}
		if _, _, err := runner.Run(
			t.Context(),
			root,
			"symbolic-ref",
			"refs/remotes/origin/HEAD",
			"refs/remotes/origin/trunk",
		); err != nil {
			t.Fatal(err)
		}
		if !service.IsDefaultBranch(t.Context(), root, "main") {
			t.Fatal("cached fallback changed early")
		}
		now = now.Add(time.Minute)
		names = service.DefaultBranchNames(t.Context(), root)
		if len(names) != 1 || names[0] != "trunk" || !service.IsDefaultBranch(t.Context(), root, "trunk") ||
			service.IsDefaultBranch(t.Context(), root, "main") {
			t.Fatalf("origin default: %v", names)
		}
	})
	t.Run("Should retain successful facts and suppress identical failure wakes", func(t *testing.T) {
		t.Parallel()
		cache := newPullRequestMemoryStore()
		now := statusTestClock()
		key := PullRequestKey{"ws", "github.com/acme/repo", "topic"}
		var calls, wakes int
		provider := pullRequestTestForge{status: func(context.Context, ForgeStatusRequest) (*ForgeStatus, error) {
			calls++
			if calls == 1 {
				return &ForgeStatus{PRNumber: new(42), PRState: new("open"), HeadSHA: "abc"}, nil
			}
			return nil, NewForgeFailure(ErrForge, "rate_limited", errors.New("secret"))
		}}
		service := NewPullRequestService(t.Context(), cache, provider, nil, func(string) { wakes++ })
		service.now = func() time.Time { return now }
		t.Cleanup(service.Close)
		first, err := service.Refresh(t.Context(), key)
		if err != nil {
			t.Fatal(err)
		}
		for i, delay := range []time.Duration{time.Minute, 2 * time.Minute, 5 * time.Minute, 15 * time.Minute, 15 * time.Minute} {
			got, err := service.Refresh(t.Context(), key)
			if err == nil || got.Facts.HeadSHA != first.Facts.HeadSHA || got.LookupState != "error" ||
				got.LookupCause != "rate_limited" ||
				got.RetryAfter.Sub(now) != delay ||
				got.FailureCount != i+1 {
				t.Fatalf("failure %d: %#v %v", i, got, err)
			}
		}
		if wakes != 2 {
			t.Fatalf("wakes=%d want 2", wakes)
		}
		flight := service.start(key, false)
		<-flight.done
		if calls != 6 {
			t.Fatalf("backoff calls=%d", calls)
		}
	})
	t.Run("Should join flights and bound provider concurrency", func(t *testing.T) {
		t.Parallel()
		entered := make(chan struct{}, 8)
		release := make(chan struct{})
		var calls, active, maximum atomic.Int32
		provider := pullRequestTestForge{status: func(ctx context.Context, _ ForgeStatusRequest) (*ForgeStatus, error) {
			calls.Add(1)
			n := active.Add(1)
			defer active.Add(-1)
			for old := maximum.Load(); n > old; old = maximum.Load() {
				if maximum.CompareAndSwap(old, n) {
					break
				}
			}
			entered <- struct{}{}
			select {
			case <-release:
				return &ForgeStatus{}, nil
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}}
		service := NewPullRequestService(t.Context(), newPullRequestMemoryStore(), provider, nil, nil)
		t.Cleanup(service.Close)
		key := PullRequestKey{"ws", "github.com/acme/repo", "a"}
		first := service.start(key, true)
		if joined := service.start(key, true); joined != first {
			t.Fatal("did not join")
		}
		flights := []*pullRequestFlight{first}
		for _, branch := range []string{"b", "c", "d", "e", "f"} {
			key.Branch = branch
			flights = append(flights, service.start(key, true))
		}
		for range 4 {
			select {
			case <-entered:
			case <-t.Context().Done():
				t.Fatal("refresh did not start")
			}
		}
		if calls.Load() != 4 {
			t.Fatalf("calls=%d before release", calls.Load())
		}
		close(release)
		for _, flight := range flights {
			<-flight.done
			if flight.err != nil {
				t.Fatal(flight.err)
			}
		}
		if maximum.Load() > 4 || calls.Load() != 6 {
			t.Fatalf("max=%d calls=%d", maximum.Load(), calls.Load())
		}
	})
	t.Run("Should preserve provider retry time and reset it after success", func(t *testing.T) {
		t.Parallel()
		now := statusTestClock()
		retry := now.Add(time.Hour)
		failed := true
		provider := pullRequestTestForge{status: func(context.Context, ForgeStatusRequest) (*ForgeStatus, error) {
			if failed {
				return nil, &ForgeFailure{kind: ErrForge, cause: "rate_limited", RetryAfter: &retry}
			}
			return &ForgeStatus{}, nil
		}}
		service := NewPullRequestService(t.Context(), newPullRequestMemoryStore(), provider, nil, nil)
		service.now = func() time.Time { return now }
		t.Cleanup(service.Close)
		key := PullRequestKey{"ws", "github.com/acme/repo", "topic"}
		got, err := service.Refresh(t.Context(), key)
		if err == nil || !got.RetryAfter.Equal(retry) {
			t.Fatalf("retry=%#v %v", got, err)
		}
		failed = false
		got, err = service.Refresh(t.Context(), key)
		if err != nil || got.RetryAfter != nil || got.FailureCount != 0 || got.LookupState != "ok" {
			t.Fatalf("reset=%#v %v", got, err)
		}
	})
	// Invariant: open PRs refresh every minute, terminal ones every 30 minutes, and branches
	// without a PR start at 10 minutes and back off toward an hour.
	t.Run("Should apply open, terminal, and absent freshness windows", func(t *testing.T) {
		t.Parallel()
		now := statusTestClock()
		observed := now.Add(-time.Minute)
		entry := PullRequestEntry{Facts: &ForgeStatus{PRState: new("open"), FetchedAt: &observed}}
		if !pullRequestStale(entry, now, 0) {
			t.Fatal("open cache remained fresh")
		}
		entry.Facts.PRState = new("merged")
		if pullRequestStale(entry, observed.Add(29*time.Minute), 0) {
			t.Fatal("terminal cache expired early")
		}
		if !pullRequestStale(entry, observed.Add(30*time.Minute), 0) {
			t.Fatal("terminal cache remained fresh")
		}
		retry := observed.Add(5 * time.Minute)
		failed := PullRequestEntry{LookupState: "error", RetryAfter: &retry, Facts: entry.Facts}
		if pullRequestStale(failed, retry.Add(-time.Second), 0) || !pullRequestStale(failed, retry, 0) {
			t.Fatal("a failed lookup over terminal facts was not retried at its retry time")
		}
		entry.Facts.PRState = nil
		for _, state := range []string{pullRequestIncomplete, "error"} {
			entry.LookupState = state
			if !pullRequestStale(entry, observed.Add(time.Minute), 3) {
				t.Fatalf("a %s lookup took the confirmed no-PR window", state)
			}
		}
		entry.LookupState = "ok"
		for absent, window := range []time.Duration{
			10 * time.Minute, 10 * time.Minute, 20 * time.Minute, 40 * time.Minute, time.Hour, time.Hour,
		} {
			if pullRequestStale(entry, observed.Add(window-time.Second), absent) ||
				!pullRequestStale(entry, observed.Add(window), absent) {
				t.Fatalf("absent=%d window != %s", absent, window)
			}
		}
	})
	t.Run("Should back off branches without a pull request until one appears", func(t *testing.T) {
		t.Parallel()
		now := statusTestClock()
		key := PullRequestKey{"ws", "github.com/acme/repo", "quiet"}
		var opened atomic.Bool
		provider := pullRequestTestForge{status: func(context.Context, ForgeStatusRequest) (*ForgeStatus, error) {
			if opened.Load() {
				return &ForgeStatus{PRNumber: new(7), PRState: new("open")}, nil
			}
			return &ForgeStatus{}, nil
		}}
		service := NewPullRequestService(t.Context(), newPullRequestMemoryStore(), provider, nil, nil)
		t.Cleanup(service.Close)
		service.now = func() time.Time { return now }
		for range 3 {
			if _, err := service.Refresh(t.Context(), key); err != nil {
				t.Fatal(err)
			}
		}
		later := service.Read(t.Context(), []PullRequestKey{key})[key]
		service.now = func() time.Time { return now.Add(30 * time.Minute) }
		if later.Stale || service.Read(t.Context(), []PullRequestKey{key})[key].Stale {
			t.Fatal("third absent answer did not back off to 40 minutes")
		}
		opened.Store(true)
		got, err := service.Refresh(t.Context(), key)
		if err != nil || got.Facts.PRState == nil {
			t.Fatalf("opened = %+v, %v", got, err)
		}
		service.now = func() time.Time { return now.Add(32 * time.Minute) }
		if !service.Read(t.Context(), []PullRequestKey{key})[key].Stale {
			t.Fatal("open pull request kept the absent window")
		}
	})
	// Invariant: a refresh that only moves the fetch time publishes no catalog change, so clients do
	// not re-read the list and schedule more refreshes.
	t.Run("Should publish a change only when pull request facts change", func(t *testing.T) {
		t.Parallel()
		now := statusTestClock()
		key := PullRequestKey{"ws", "github.com/acme/repo", "steady"}
		var checks atomic.Int32
		provider := pullRequestTestForge{status: func(context.Context, ForgeStatusRequest) (*ForgeStatus, error) {
			return &ForgeStatus{
				PRNumber: new(7), PRState: new("open"),
				Checks: &ForgeChecks{State: "pending", Pending: int(checks.Load())},
			}, nil
		}}
		var wakes atomic.Int32
		service := NewPullRequestService(
			t.Context(), newPullRequestMemoryStore(), provider, nil, func(string) { wakes.Add(1) },
		)
		t.Cleanup(service.Close)
		for minute := range 3 {
			service.now = func() time.Time { return now.Add(time.Duration(minute) * time.Minute) }
			if _, err := service.Refresh(t.Context(), key); err != nil {
				t.Fatal(err)
			}
		}
		if wakes.Load() != 1 {
			t.Fatalf("unchanged facts published %d changes, want 1", wakes.Load())
		}
		checks.Store(1)
		if _, err := service.Refresh(t.Context(), key); err != nil || wakes.Load() != 2 {
			t.Fatalf("changed checks published %d changes (err %v), want 2", wakes.Load(), err)
		}
	})
	t.Run("Should normalize hosted subgroup repositories", func(t *testing.T) {
		t.Parallel()
		got, err := NormalizeRepoKey("git@gitlab.com:Group/Subgroup/Repo.git")
		if err != nil || got != "gitlab.com/group/subgroup/repo" {
			t.Fatalf("key=%s err=%v", got, err)
		}
	})
	t.Run("Should normalize repository identity without retaining credentials", func(t *testing.T) {
		t.Parallel()
		for _, remote := range []string{"git@GitHub.com:Acme/Repo.git", "https://secret@github.com/ACME/Repo.git", "ssh://git@github.com/Acme/repo.git"} {
			got, err := NormalizeRepoKey(remote)
			if err != nil || got != "github.com/acme/repo" {
				t.Fatalf("identity=%q %v", got, err)
			}
		}
	})
}

type pullRequestMemoryStore struct {
	mu      sync.Mutex
	entries map[PullRequestKey]PullRequestEntry
}

func newPullRequestMemoryStore() *pullRequestMemoryStore {
	return &pullRequestMemoryStore{entries: make(map[PullRequestKey]PullRequestEntry)}
}

func (s *pullRequestMemoryStore) ReadPullRequests(
	_ context.Context,
	keys []PullRequestKey,
) (map[PullRequestKey]PullRequestEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make(map[PullRequestKey]PullRequestEntry)
	for _, key := range keys {
		if entry, ok := s.entries[key]; ok {
			out[key] = entry
		}
	}
	return out, nil
}
func (s *pullRequestMemoryStore) SavePullRequest(_ context.Context, entry PullRequestEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries[entry.Key] = entry
	return nil
}

type pullRequestTestForge struct {
	status func(context.Context, ForgeStatusRequest) (*ForgeStatus, error)
}

func (f pullRequestTestForge) Status(ctx context.Context, r ForgeStatusRequest) (*ForgeStatus, error) {
	return f.status(ctx, r)
}
func (f pullRequestTestForge) Capabilities(context.Context, []string) (*ForgeCapabilities, error) {
	return nil, nil
}
func (f pullRequestTestForge) CreatePR(context.Context, ForgePRRequest) (*ForgePRResult, error) {
	return nil, nil
}

func BenchmarkPullRequestRepositoryIdentity(b *testing.B) {
	for _, remote := range []string{"missing", "local", "hosted"} {
		b.Run(remote, func(b *testing.B) {
			runner, err := NewRealGitRunner(0)
			if err != nil {
				b.Fatal(err)
			}
			root := b.TempDir()
			if _, _, err := runner.Run(b.Context(), root, "init", "--initial-branch=main"); err != nil {
				b.Fatal(err)
			}
			if remote != "missing" {
				origin := "/tmp/local-repository.git"
				if remote == "hosted" {
					origin = "https://github.com/acme/repo.git"
				}
				if _, _, err := runner.Run(b.Context(), root, "remote", "add", "origin", origin); err != nil {
					b.Fatal(err)
				}
			}
			service := NewPullRequestService(b.Context(), newPullRequestMemoryStore(), nil, runner, nil)
			b.Cleanup(service.Close)
			// Compare warm identity reads. Unsupported origins must stay unsupported.
			_, err = service.RepoKey(b.Context(), root)
			if (err == nil) != (remote == "hosted") {
				b.Fatalf("unexpected initial identity error: %v", err)
			}
			b.ReportAllocs()
			for b.Loop() {
				_, err = service.RepoKey(b.Context(), root)
				if (err == nil) != (remote == "hosted") {
					b.Fatalf("unexpected identity error: %v", err)
				}
			}
		})
	}
}
