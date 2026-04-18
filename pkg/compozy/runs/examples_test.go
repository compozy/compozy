package runs

import (
	"context"
	"fmt"
	"time"

	"github.com/compozy/compozy/pkg/compozy/events"
)

func ExampleList() {
	previous := resolveRunsDaemonReader
	resolveRunsDaemonReader = func() (daemonRunReader, error) {
		return &stubDaemonRunReader{
			listSummaries: [][]RunSummary{{
				{
					RunID:     "run-early",
					Status:    "failed",
					StartedAt: time.Date(2026, 4, 6, 12, 0, 0, 0, time.UTC),
				},
				{
					RunID:     "run-late",
					Status:    "completed",
					StartedAt: time.Date(2026, 4, 6, 13, 0, 0, 0, time.UTC),
				},
			}},
		}, nil
	}
	defer func() { resolveRunsDaemonReader = previous }()

	summaries, err := List("/workspace", ListOptions{})
	if err != nil {
		panic(err)
	}
	for index := range summaries {
		fmt.Printf("%s %s\n", summaries[index].RunID, summaries[index].Status)
	}

	// Output:
	// run-late completed
	// run-early failed
}

func ExampleOpen() {
	previous := resolveRunsDaemonReader
	resolveRunsDaemonReader = func() (daemonRunReader, error) {
		return &stubDaemonRunReader{
			openSummary: RunSummary{
				RunID:  "run-open",
				Mode:   "exec",
				Status: "completed",
			},
		}, nil
	}
	defer func() { resolveRunsDaemonReader = previous }()

	run, err := Open("/workspace", "run-open")
	if err != nil {
		panic(err)
	}

	summary := run.Summary()
	fmt.Printf("%s %s %s\n", summary.RunID, summary.Mode, summary.Status)

	// Output:
	// run-open exec completed
}

func ExampleRun_Replay() {
	previous := resolveRunsDaemonReader
	resolveRunsDaemonReader = func() (daemonRunReader, error) {
		return &stubDaemonRunReader{
			openSummary: RunSummary{RunID: "run-replay"},
			eventPages: []remoteRunEventPage{{
				Events: []events.Event{
					exampleEvent("run-replay", 1, events.EventKindRunStarted),
					exampleEvent("run-replay", 2, events.EventKindJobCompleted),
					exampleEvent("run-replay", 3, events.EventKindRunCompleted),
				},
			}},
		}, nil
	}
	defer func() { resolveRunsDaemonReader = previous }()

	run, err := Open("/workspace", "run-replay")
	if err != nil {
		panic(err)
	}

	for event, replayErr := range run.Replay(2) {
		if replayErr != nil {
			panic(replayErr)
		}
		fmt.Printf("%d %s\n", event.Seq, event.Kind)
	}

	// Output:
	// 2 job.completed
	// 3 run.completed
}

func ExampleRun_Tail() {
	previous := resolveRunsDaemonReader
	resolveRunsDaemonReader = func() (daemonRunReader, error) {
		now := time.Date(2026, 4, 6, 12, 0, 0, 0, time.UTC)
		return &stubDaemonRunReader{
			openSummary: RunSummary{RunID: "run-tail"},
			snapshot: RemoteRunSnapshot{
				Status:     publicRunStatusRunning,
				NextCursor: remoteCursor(now, 1),
			},
			eventPages: []remoteRunEventPage{{
				Events: []events.Event{
					exampleEvent("run-tail", 1, events.EventKindRunStarted),
				},
				NextCursor: remoteCursor(now, 1),
			}},
			streams: []RemoteRunStream{
				newBufferedRemoteRunStream(
					RemoteRunStreamItem{Event: &events.Event{
						SchemaVersion: events.SchemaVersion,
						RunID:         "run-tail",
						Seq:           2,
						Timestamp:     now.Add(time.Second),
						Kind:          events.EventKindRunCompleted,
					}},
				),
			},
		}, nil
	}
	defer func() { resolveRunsDaemonReader = previous }()

	run, err := Open("/workspace", "run-tail")
	if err != nil {
		panic(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	eventsCh, errsCh := run.Tail(ctx, 2)
	event := mustReadExampleTailEvent(eventsCh, errsCh)
	fmt.Printf("%d %s\n", event.Seq, event.Kind)

	// Output:
	// 2 run.completed
}

func ExampleWatchWorkspace() {
	previous := resolveRunsDaemonReader
	resolveRunsDaemonReader = func() (daemonRunReader, error) {
		return &stubDaemonRunReader{
			listSummaries: [][]RunSummary{
				nil,
				{{
					RunID:     "run-created",
					Status:    "running",
					StartedAt: time.Date(2026, 4, 6, 12, 0, 0, 0, time.UTC),
				}},
			},
		}, nil
	}
	defer func() { resolveRunsDaemonReader = previous }()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	eventsCh, errsCh := WatchWorkspace(ctx, "/workspace")
	event := mustReadExampleRunEvent(eventsCh, errsCh)
	fmt.Printf("%s %s %s\n", event.Kind, event.RunID, event.Summary.Status)

	// Output:
	// created run-created running
}

func mustReadExampleTailEvent(eventsCh <-chan events.Event, errsCh <-chan error) events.Event {
	timeout := time.NewTimer(2 * time.Second)
	defer timeout.Stop()

	for {
		select {
		case <-timeout.C:
			panic("timed out waiting for tail event")
		case err, ok := <-errsCh:
			if ok && err != nil {
				panic(err)
			}
		case event, ok := <-eventsCh:
			if !ok {
				panic("tail events channel closed before event arrived")
			}
			return event
		}
	}
}

func mustReadExampleRunEvent(eventsCh <-chan RunEvent, errsCh <-chan error) RunEvent {
	timeout := time.NewTimer(2 * time.Second)
	defer timeout.Stop()

	for {
		select {
		case <-timeout.C:
			panic("timed out waiting for run event")
		case err, ok := <-errsCh:
			if ok && err != nil {
				panic(err)
			}
		case event, ok := <-eventsCh:
			if !ok {
				panic("run events channel closed before event arrived")
			}
			return event
		}
	}
}

func exampleEvent(runID string, seq uint64, kind events.EventKind) events.Event {
	return events.Event{
		SchemaVersion: events.SchemaVersion,
		RunID:         runID,
		Seq:           seq,
		Timestamp:     time.Unix(int64(seq), 0).UTC(),
		Kind:          kind,
	}
}
