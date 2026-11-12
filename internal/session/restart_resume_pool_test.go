package session

import (
	"context"
	"slices"
	"sync"
	"testing"

	"github.com/compozy/compozy/internal/store"
)

// Invariant: restart recovery resumes interrupted user sessions with bounded concurrency while
// instance mains go first, a parent settles before its child, and sessions sharing a worktree keep
// their serial order. Owner: session restart continuation.
func TestRestartResumePool(t *testing.T) {
	t.Parallel()
	interrupted := func(id string, mutate ...func(*Info)) *Info {
		info := &Info{ID: id, Type: SessionTypeUser, StopReason: store.StopShutdown}
		for _, apply := range mutate {
			apply(info)
		}
		return info
	}
	childOf := func(parent string) func(*Info) {
		return func(info *Info) { info.Lineage = &store.SessionLineage{ParentSessionID: parent} }
	}
	inWorktree := func(id string) func(*Info) { return func(info *Info) { info.WorktreeID = id } }
	instanceMain := func(info *Info) { info.InstanceID = "inst-1" }

	t.Run("Should order mains first and record parent and worktree dependencies", func(t *testing.T) {
		t.Parallel()
		items := planRestartResumes([]*Info{
			interrupted("child", childOf("parent"), inWorktree("wt-1")),
			interrupted("plain"),
			nil,
			{ID: "explicit-stop", Type: SessionTypeUser, StopReason: store.StopUserCanceled},
			{ID: "spawned", Type: SessionTypeSpawned, StopReason: store.StopShutdown},
			interrupted("parent", inWorktree("wt-1")),
			interrupted("main", instanceMain),
			interrupted("sibling", inWorktree("wt-1")),
		})
		ids := make([]string, 0, len(items))
		for _, item := range items {
			ids = append(ids, item.id)
		}
		if want := []string{"main", "plain", "parent", "sibling", "child"}; !slices.Equal(ids, want) {
			t.Fatalf("resume order = %v, want %v", ids, want)
		}
		wantAfter := [][]int{nil, nil, nil, {2}, {2, 3}}
		for index, item := range items {
			if !slices.Equal(item.after, wantAfter[index]) {
				t.Fatalf("item %q waits for %v, want %v", item.id, item.after, wantAfter[index])
			}
		}
	})

	t.Run("Should bound concurrency and start dependents only after their owners settle", func(t *testing.T) {
		t.Parallel()
		items := []restartResumeItem{
			{id: "a"}, {id: "b"}, {id: "c"}, {id: "d", after: []int{0}}, {id: "e", after: []int{3}},
		}
		var mu sync.Mutex
		running, peak := 0, 0
		var settled []string
		release := make(chan struct{})
		entered := make(chan string, len(items))
		done := make(chan error, 1)
		go func() {
			done <- runRestartResumes(t.Context(), items, 2, func(_ context.Context, id string) {
				mu.Lock()
				running++
				peak = max(peak, running)
				for _, item := range items {
					if item.id != id {
						continue
					}
					for _, dependency := range item.after {
						if !slices.Contains(settled, items[dependency].id) {
							t.Errorf("%q started before %q settled", id, items[dependency].id)
						}
					}
				}
				mu.Unlock()
				entered <- id
				<-release
				mu.Lock()
				running--
				settled = append(settled, id)
				mu.Unlock()
			})
		}()
		first, second := <-entered, <-entered
		if got := []string{first, second}; !slices.Contains(got, "a") || !slices.Contains(got, "b") {
			t.Fatalf("first resumes = %v, want the two highest-priority sessions", got)
		}
		close(release)
		if err := <-done; err != nil {
			t.Fatalf("runRestartResumes() error = %v", err)
		}
		if peak != 2 || len(settled) != len(items) {
			t.Fatalf("peak concurrency = %d, settled = %v; want 2 in flight and every session settled", peak, settled)
		}
	})

	t.Run("Should stop starting sessions once canceled", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithCancel(t.Context())
		var resumed []string
		err := runRestartResumes(ctx, []restartResumeItem{{id: "a"}, {id: "b"}}, 1,
			func(_ context.Context, id string) {
				resumed = append(resumed, id)
				cancel()
			})
		if err == nil || !slices.Equal(resumed, []string{"a"}) {
			t.Fatalf("runRestartResumes() = %v after resuming %v; want cancellation after the first", err, resumed)
		}
	})
}
