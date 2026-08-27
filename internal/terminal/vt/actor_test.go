package vt

// Suite: terminal VT actor.
// Invariant: one actor owns emulator state, screen snapshots are whole and bounded, and overflow rebuilds from the ring.
// Boundary IN: absolute-sequence terminal bytes. Boundary OUT: ANSI-free rendered screen snapshots.

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestActorScreenContract(t *testing.T) {
	t.Parallel()

	t.Run("Should render plain alternate-screen and wide-cell grids [UT-038]", func(t *testing.T) {
		t.Parallel()
		cases := []struct {
			name  string
			input string
			want  string
		}{
			{name: "Should render plain text", input: "hello\nworld", want: "hello\n     world"},
			{
				name:  "Should render an alternate screen",
				input: "primary\x1b[?1049h\x1b[2J\x1b[Halt\nrow",
				want:  "alt\n   row",
			},
			{name: "Should preserve CJK and emoji widths", input: "界🙂x", want: "界🙂x"},
		}
		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()
				actor := New(20, 5, nil)
				t.Cleanup(func() {
					if err := actor.Close(); err != nil {
						t.Errorf("Close() error = %v", err)
					}
				})
				if _, err := actor.Write([]byte(testCase.input)); err != nil {
					t.Fatalf("Write() error = %v", err)
				}
				got := waitForContent(t, actor, testCase.want)
				if got != testCase.want {
					t.Fatalf("Screen() = %q, want %q", got, testCase.want)
				}
			})
		}
	})

	t.Run("Should serialize concurrent reads with flood writes [UT-039]", func(t *testing.T) {
		t.Parallel()
		actor := New(40, 10, nil)
		t.Cleanup(func() {
			if err := actor.Close(); err != nil {
				t.Errorf("Close() error = %v", err)
			}
		})
		var writers sync.WaitGroup
		workerErrors := make(chan error, 1)
		for index := range 4 {
			writers.Go(func() {
				for line := range 200 {
					if _, err := fmt.Fprintf(actor, "worker-%d-%03d\r\n", index, line); err != nil {
						select {
						case workerErrors <- fmt.Errorf("worker %d write: %w", index, err):
						default:
						}
						return
					}
					if _, err := actor.Screen(context.Background()); err != nil {
						select {
						case workerErrors <- fmt.Errorf("worker %d screen: %w", index, err):
						default:
						}
						return
					}
				}
			})
		}
		writers.Wait()
		select {
		case err := <-workerErrors:
			t.Fatal(err)
		default:
		}
		if _, err := actor.Screen(context.Background()); err != nil {
			t.Fatalf("Screen() error = %v", err)
		}
	})

	t.Run("Should return only complete repaint frames [UT-040]", func(t *testing.T) {
		t.Parallel()
		actor := New(20, 5, nil)
		t.Cleanup(func() {
			if err := actor.Close(); err != nil {
				t.Errorf("Close() error = %v", err)
			}
		})
		frameA := "\x1b[2J\x1b[HAAAAAAAAAAAAAAAAAAAA\nAAAAAAAAAAAAAAAAAAAA"
		frameB := "\x1b[2J\x1b[HBBBBBBBBBBBBBBBBBBBB\nBBBBBBBBBBBBBBBBBBBB"
		if _, err := actor.Write([]byte(frameA)); err != nil {
			t.Fatalf("Write(frame A) error = %v", err)
		}
		if _, err := actor.Write([]byte(frameB)); err != nil {
			t.Fatalf("Write(frame B) error = %v", err)
		}
		content := waitForContent(t, actor, "BBBBBBBBBBBBBBBBBBBB")
		if strings.Contains(content, "A") || !strings.Contains(content, "BBBBBBBBBBBBBBBBBBBB") {
			t.Fatalf("Screen() returned incomplete or torn frame: %q", content)
		}
	})

	t.Run("Should drain and retain the final screen during close [UT-041][UT-042]", func(t *testing.T) {
		t.Parallel()
		actor := New(20, 5, nil)
		if _, err := actor.Write([]byte("final\x1b[31m red\x1b[0m")); err != nil {
			t.Fatalf("Write() error = %v", err)
		}
		waitForContent(t, actor, "final red")
		if err := actor.Close(); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
		snapshot, err := actor.Screen(context.Background())
		if err != nil {
			t.Fatalf("Screen(closed) error = %v", err)
		}
		if !snapshot.Ended || snapshot.Content != "final red" || strings.ContainsRune(snapshot.Content, '\x1b') {
			t.Fatalf("final snapshot = %#v", snapshot)
		}
		if len(snapshot.Content) > 20*5*4 {
			t.Fatalf("screen bytes = %d, want grid-bounded", len(snapshot.Content))
		}
	})
}

func TestActorOverflowRebuild(t *testing.T) {
	t.Parallel()
	t.Log("[UT-100] bounded overflow rebuild must equal a fresh parse of the retained ring")

	var mu sync.RWMutex
	ringData := []byte("initial")
	sequence := uint64(len(ringData))
	rebuildEntered := make(chan struct{})
	releaseRebuild := make(chan struct{})
	releaseRebuildOnce := sync.OnceFunc(func() { close(releaseRebuild) })
	var calls atomic.Int32
	snapshot := func() ([]byte, uint64) {
		if calls.Add(1) > 1 {
			select {
			case <-rebuildEntered:
			default:
				close(rebuildEntered)
			}
			<-releaseRebuild
		}
		mu.RLock()
		defer mu.RUnlock()
		return append([]byte(nil), ringData...), sequence
	}
	actor := New(20, 5, snapshot)
	t.Cleanup(func() {
		releaseRebuildOnce()
		if err := actor.Close(); err != nil {
			t.Errorf("Close(actor) error = %v", err)
		}
	})
	actor.capacity = 32
	large := []byte("\x1b[2J\x1b[Hrebuilt-from-ring")
	mu.Lock()
	ringData = append(ringData, large...)
	sequence += uint64(len(large))
	mu.Unlock()
	if _, err := actor.WriteAt(make([]byte, 64), sequence); err != nil {
		t.Fatalf("WriteAt(overflow) error = %v", err)
	}
	select {
	case <-rebuildEntered:
	case <-time.After(time.Second):
		t.Fatal("VT actor did not enter rebuild")
	}
	if pending := actor.pending.Load(); pending > actor.capacity {
		t.Fatalf("mailbox pending = %d, cap = %d", pending, actor.capacity)
	}
	busy, err := actor.Screen(context.Background())
	if err != nil || !busy.Busy {
		t.Fatalf("Screen(rebuilding) = %#v error=%v, want busy", busy, err)
	}
	releaseRebuildOnce()
	got := waitForContent(t, actor, "rebuilt-from-ring")

	fresh := New(20, 5, func() ([]byte, uint64) {
		mu.RLock()
		defer mu.RUnlock()
		return append([]byte(nil), ringData...), sequence
	})
	t.Cleanup(func() {
		if err := fresh.Close(); err != nil {
			t.Errorf("Close(fresh) error = %v", err)
		}
	})
	want := waitForContent(t, fresh, "rebuilt-from-ring")
	if got != want {
		t.Fatalf("rebuilt screen = %q, fresh parse = %q", got, want)
	}
	if err := actor.Close(); err != nil {
		t.Fatalf("Close(actor) error = %v", err)
	}
	if err := fresh.Close(); err != nil {
		t.Fatalf("Close(fresh) error = %v", err)
	}
}

func TestActorCloseShouldPreemptOverflowRebuild(t *testing.T) {
	t.Parallel()

	var snapshots atomic.Int32
	large := []byte(strings.Repeat("x", 8<<20))
	actor := New(20, 5, func() ([]byte, uint64) {
		if snapshots.Add(1) == 1 {
			return nil, 0
		}
		return large, uint64(len(large))
	})
	t.Cleanup(func() {
		if err := actor.Close(); err != nil {
			t.Errorf("Close() error = %v", err)
		}
	})
	actor.capacity = 1
	if _, err := actor.WriteAt([]byte("overflow"), uint64(len(large))); err != nil {
		t.Fatalf("WriteAt(overflow) error = %v", err)
	}
	deadline := time.Now().Add(time.Second)
	for !actor.rebuilding.Load() {
		if time.Now().After(deadline) {
			t.Fatal("VT actor did not enter overflow rebuild")
		}
		time.Sleep(time.Millisecond)
	}
	closed := make(chan error, 1)
	go func() { closed <- actor.Close() }()
	select {
	case err := <-closed:
		if err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Close() waited for the full overflow rebuild")
	}
}

func waitForContent(t *testing.T, actor *Actor, contains string) string {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		ctx, cancel := context.WithDeadline(t.Context(), deadline)
		snapshot, err := actor.Screen(ctx)
		cancel()
		if err != nil {
			t.Fatalf("Screen() error = %v", err)
		}
		if !snapshot.Busy && strings.Contains(snapshot.Content, contains) {
			return snapshot.Content
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("screen did not contain %q", contains)
	return ""
}
