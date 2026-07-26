package hooks

import (
	"context"
	"testing"
	"time"
)

func TestAsyncPoolCloseDeadline(t *testing.T) {
	t.Parallel()

	t.Run("Should return after drain deadline when active task ignores context", func(t *testing.T) {
		t.Parallel()

		pool := newAsyncPool(asyncPoolConfig{
			WorkerCount:   1,
			QueueCapacity: 1,
			DrainTimeout:  40 * time.Millisecond,
			Logger:        discardPoolLogger(),
		})
		pool.Start(t.Context())

		started := make(chan struct{})
		release := make(chan struct{})
		finished := make(chan struct{})
		if ok := pool.Submit(asyncTask{
			hook: RegisteredHook{
				Name:   "ignores-context",
				Event:  HookEventPostRecord,
				Source: HookSourceNative,
			},
			run: func(context.Context) {
				close(started)
				<-release
				close(finished)
			},
		}); !ok {
			t.Fatal("Submit() first task = false, want true")
		}
		waitForPoolSignal(t, started, "active task start")

		var queuedRan bool
		if ok := pool.Submit(asyncTask{
			run: func(context.Context) {
				queuedRan = true
			},
		}); !ok {
			t.Fatal("Submit() queued task = false, want true")
		}

		start := time.Now()
		pool.Close()
		elapsed := time.Since(start)

		if queuedRan {
			t.Fatal("queued task ran after drain deadline, want abandoned")
		}
		if elapsed < 40*time.Millisecond {
			t.Fatalf("Close() elapsed = %s, want at least drain timeout", elapsed)
		}
		if elapsed > time.Second {
			t.Fatalf("Close() elapsed = %s, want bounded close after drain timeout", elapsed)
		}

		close(release)
		waitForPoolSignal(t, finished, "ignored-context task release")
	})
}
