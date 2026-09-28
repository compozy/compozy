package daemon

import (
	"context"

	"testing"

	taskpkg "github.com/compozy/compozy/internal/task"
)

func TestTaskEventObserverFanout(t *testing.T) {
	t.Run("Should notify every task event observer", func(t *testing.T) {
		t.Parallel()

		first := &recordingTaskEventObserver{}
		second := &recordingTaskEventObserver{}
		fanout := newTaskEventObserverFanout(discardLogger(), first, nil, second)
		if fanout == nil {
			t.Fatal("newTaskEventObserverFanout() = nil, want fanout")
		}

		record := taskpkg.EventRecord{Event: taskpkg.Event{ID: "evt-1", TaskID: "task-1"}}
		fanout.OnTaskEvent(context.Background(), record)

		if got, want := first.count, 1; got != want {
			t.Fatalf("first observer count = %d, want %d", got, want)
		}
		if got, want := second.count, 1; got != want {
			t.Fatalf("second observer count = %d, want %d", got, want)
		}
	})
}

type recordingTaskEventObserver struct {
	count int
}

func (o *recordingTaskEventObserver) OnTaskEvent(context.Context, taskpkg.EventRecord) {
	o.count++
}
