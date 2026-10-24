package worktree

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// Invariants: canonical display refresh admission, identity and failure retention.
// Owner: PullRequestService; this is its canonical suite (new subsystem).
func TestPullRequestService(t *testing.T) {
	t.Parallel()
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
	t.Run("Should apply open and terminal freshness windows", func(t *testing.T) {
		t.Parallel()
		now := statusTestClock()
		observed := now.Add(-time.Minute)
		entry := PullRequestEntry{Facts: &ForgeStatus{PRState: new("open"), FetchedAt: &observed}}
		if !pullRequestStale(entry, now) {
			t.Fatal("open cache remained fresh")
		}
		entry.Facts.PRState = new("merged")
		if pullRequestStale(entry, now) {
			t.Fatal("terminal cache expired early")
		}
		if !pullRequestStale(entry, observed.Add(10*time.Minute)) {
			t.Fatal("terminal cache remained fresh")
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
