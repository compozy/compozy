package automation

import (
	"testing"
	"time"

	"github.com/compozy/agh/internal/testutil"
)

func TestDispatcherSessionStopTimeout(t *testing.T) {
	t.Run("Should keep completed runs successful while session teardown finishes", func(t *testing.T) {
		t.Parallel()

		store := newMemoryRunStore()
		stopStarted := make(chan struct{}, 1)
		stopRelease := make(chan struct{})
		creator := newRecordingSessionCreator(sessionAttemptPlan{
			stopStarted: stopStarted,
			stopRelease: stopRelease,
		})
		dispatcher := newTestDispatcher(t, creator, store)
		job := testJob(AutomationScopeGlobal, "job-slow-stop", "")

		go func() {
			<-stopStarted
			timer := time.NewTimer(3 * time.Second)
			defer timer.Stop()
			<-timer.C
			close(stopRelease)
		}()

		run, err := dispatcher.Dispatch(testutil.Context(t), DispatchRequest{
			Kind: DispatchKindManual,
			Job:  &job,
		})
		if err != nil {
			t.Fatalf("Dispatch() error = %v", err)
		}
		if got, want := run.Status, RunCompleted; got != want {
			t.Fatalf("run.Status = %q, want %q", got, want)
		}

		reloadedRuns, err := store.ListRuns(testutil.Context(t), RunQuery{JobID: job.ID})
		if err != nil {
			t.Fatalf("ListRuns() error = %v", err)
		}
		if got, want := len(reloadedRuns), 1; got != want {
			t.Fatalf("len(ListRuns()) = %d, want %d", got, want)
		}
		if got, want := reloadedRuns[0].Status, RunCompleted; got != want {
			t.Fatalf("ListRuns()[0].Status = %q, want %q", got, want)
		}
		if reloadedRuns[0].EndedAt == nil {
			t.Fatal("ListRuns()[0].EndedAt = nil, want populated")
		}
	})
}
