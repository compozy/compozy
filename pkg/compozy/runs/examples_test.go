package runs

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/compozy/compozy/pkg/compozy/events"
)

func ExampleList() {
	workspaceRoot := mustExampleWorkspaceRoot()
	defer os.RemoveAll(workspaceRoot)

	mustWriteExampleRun(workspaceRoot, "run-early", exampleRunFixture{
		runJSON: map[string]any{
			"run_id":         "run-early",
			"mode":           "prd-tasks",
			"workspace_root": workspaceRoot,
			"created_at":     time.Date(2026, 4, 6, 12, 0, 0, 0, time.UTC),
		},
		resultJSON: map[string]any{"status": "failed"},
	})
	mustWriteExampleRun(workspaceRoot, "run-late", exampleRunFixture{
		runJSON: map[string]any{
			"run_id":         "run-late",
			"mode":           "exec",
			"status":         "succeeded",
			"workspace_root": workspaceRoot,
			"created_at":     time.Date(2026, 4, 6, 13, 0, 0, 0, time.UTC),
		},
	})

	summaries, err := List(workspaceRoot, ListOptions{})
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
	workspaceRoot := mustExampleWorkspaceRoot()
	defer os.RemoveAll(workspaceRoot)

	mustWriteExampleRun(workspaceRoot, "run-open", exampleRunFixture{
		runJSON: map[string]any{
			"run_id":         "run-open",
			"mode":           "exec",
			"status":         "succeeded",
			"workspace_root": workspaceRoot,
			"created_at":     time.Date(2026, 4, 6, 12, 0, 0, 0, time.UTC),
		},
	})

	run, err := Open(workspaceRoot, "run-open")
	if err != nil {
		panic(err)
	}

	summary := run.Summary()
	fmt.Printf("%s %s %s\n", summary.RunID, summary.Mode, summary.Status)

	// Output:
	// run-open exec completed
}

func ExampleRun_Replay() {
	workspaceRoot := mustExampleWorkspaceRoot()
	defer os.RemoveAll(workspaceRoot)

	mustWriteExampleRun(workspaceRoot, "run-replay", exampleRunFixture{
		runJSON: map[string]any{
			"run_id":         "run-replay",
			"mode":           "exec",
			"workspace_root": workspaceRoot,
			"created_at":     time.Date(2026, 4, 6, 12, 0, 0, 0, time.UTC),
		},
		events: []events.Event{
			exampleEvent("run-replay", 1, events.EventKindRunStarted),
			exampleEvent("run-replay", 2, events.EventKindJobCompleted),
			exampleEvent("run-replay", 3, events.EventKindRunCompleted),
		},
	})

	run, err := Open(workspaceRoot, "run-replay")
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
	workspaceRoot := mustExampleWorkspaceRoot()
	defer os.RemoveAll(workspaceRoot)

	runDir := mustWriteExampleRun(workspaceRoot, "run-tail", exampleRunFixture{
		runJSON: map[string]any{
			"run_id":         "run-tail",
			"mode":           "exec",
			"workspace_root": workspaceRoot,
			"created_at":     time.Date(2026, 4, 6, 12, 0, 0, 0, time.UTC),
		},
		events: []events.Event{
			exampleEvent("run-tail", 1, events.EventKindRunStarted),
		},
	})

	run, err := Open(workspaceRoot, "run-tail")
	if err != nil {
		panic(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	eventsCh, errsCh := run.Tail(ctx, 2)
	mustAppendExampleEvents(filepath.Join(runDir, "events.jsonl"), []events.Event{
		exampleEvent("run-tail", 2, events.EventKindRunCompleted),
	})

	event := mustReadExampleTailEvent(eventsCh, errsCh)
	fmt.Printf("%d %s\n", event.Seq, event.Kind)

	// Output:
	// 2 run.completed
}

func ExampleWatchWorkspace() {
	workspaceRoot := mustExampleWorkspaceRoot()
	defer os.RemoveAll(workspaceRoot)

	if err := os.MkdirAll(filepath.Join(workspaceRoot, ".compozy", "runs"), 0o755); err != nil {
		panic(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	eventsCh, errsCh := WatchWorkspace(ctx, workspaceRoot)
	mustWriteExampleRun(workspaceRoot, "run-created", exampleRunFixture{
		runJSON: map[string]any{
			"run_id":         "run-created",
			"mode":           "exec",
			"status":         "running",
			"workspace_root": workspaceRoot,
			"created_at":     time.Date(2026, 4, 6, 12, 0, 0, 0, time.UTC),
		},
	})

	event := mustReadExampleRunEvent(eventsCh, errsCh)
	fmt.Printf("%s %s %s\n", event.Kind, event.RunID, event.Summary.Status)

	// Output:
	// created run-created running
}

type exampleRunFixture struct {
	runJSON    map[string]any
	resultJSON map[string]any
	events     []events.Event
}

func mustExampleWorkspaceRoot() string {
	workspaceRoot, err := os.MkdirTemp("", "compozy-runs-example-*")
	if err != nil {
		panic(err)
	}
	return workspaceRoot
}

func mustWriteExampleRun(workspaceRoot, runID string, fixture exampleRunFixture) string {
	runDir := filepath.Join(workspaceRoot, ".compozy", "runs", runID)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		panic(err)
	}

	if fixture.runJSON != nil {
		payload, err := json.Marshal(fixture.runJSON)
		if err != nil {
			panic(err)
		}
		if err := os.WriteFile(filepath.Join(runDir, "run.json"), payload, 0o600); err != nil {
			panic(err)
		}
	}
	if fixture.resultJSON != nil {
		payload, err := json.Marshal(fixture.resultJSON)
		if err != nil {
			panic(err)
		}
		if err := os.WriteFile(filepath.Join(runDir, "result.json"), payload, 0o600); err != nil {
			panic(err)
		}
	}
	if len(fixture.events) > 0 {
		file, err := os.OpenFile(filepath.Join(runDir, "events.jsonl"), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
		if err != nil {
			panic(err)
		}
		encoder := json.NewEncoder(file)
		for _, event := range fixture.events {
			if err := encoder.Encode(event); err != nil {
				file.Close()
				panic(err)
			}
		}
		if err := file.Close(); err != nil {
			panic(err)
		}
	}
	return runDir
}

func mustAppendExampleEvents(eventsPath string, items []events.Event) {
	file, err := os.OpenFile(eventsPath, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o600)
	if err != nil {
		panic(err)
	}
	encoder := json.NewEncoder(file)
	for _, event := range items {
		if err := encoder.Encode(event); err != nil {
			file.Close()
			panic(err)
		}
	}
	if err := file.Close(); err != nil {
		panic(err)
	}
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
		Payload:       json.RawMessage(`{"seq":1}`),
	}
}
