# Reader Library

`pkg/compozy/runs` is the public read-only library for inspecting persisted runs under `.compozy/runs`.

The snippets below are mirrored by executable examples in `pkg/compozy/runs/examples_test.go`, so `go test ./pkg/compozy/runs` compiles and runs the same usage patterns.

## Import

```go
import "github.com/compozy/compozy/pkg/compozy/runs"
```

## List

Use `List` to enumerate persisted runs under one workspace.

```go
workspaceRoot := "/path/to/workspace"

summaries, err := runs.List(workspaceRoot, runs.ListOptions{
	Status: []string{"completed", "failed"},
	Mode:   []string{"prd-tasks", "pr-review"},
	Limit:  20,
})
if err != nil {
	return err
}

for _, summary := range summaries {
	fmt.Printf("%s %s %s\n", summary.RunID, summary.Mode, summary.Status)
}
```

## Open

Use `Open` when you already know the run id and want the resolved metadata plus replay/tail access.

```go
workspaceRoot := "/path/to/workspace"
runID := "run-20260406-120000"

run, err := runs.Open(workspaceRoot, runID)
if err != nil {
	return err
}

summary := run.Summary()
fmt.Printf("%s %s %s\n", summary.RunID, summary.Mode, summary.Status)
```

## Replay

Use `Replay` to iterate historical events from a sequence number.

```go
workspaceRoot := "/path/to/workspace"
runID := "run-20260406-120000"

run, err := runs.Open(workspaceRoot, runID)
if err != nil {
	return err
}

for event, replayErr := range run.Replay(0) {
	if replayErr != nil {
		return replayErr
	}
	fmt.Printf("%d %s\n", event.Seq, event.Kind)
}
```

`Replay` tolerates a truncated final line in `events.jsonl` and reports `runs.ErrPartialEventLine` when it encounters one.

## Tail

Use `Tail` to replay historical events and then follow newly appended events until the context is canceled.

```go
workspaceRoot := "/path/to/workspace"
runID := "run-20260406-120000"

run, err := runs.Open(workspaceRoot, runID)
if err != nil {
	return err
}

ctx, cancel := context.WithCancel(context.Background())
defer cancel()

eventsCh, errsCh := run.Tail(ctx, 0)
for {
	select {
	case event, ok := <-eventsCh:
		if !ok {
			return nil
		}
		fmt.Printf("%d %s\n", event.Seq, event.Kind)
	case err, ok := <-errsCh:
		if ok && err != nil {
			return err
		}
	}
}
```

## WatchWorkspace

Use `WatchWorkspace` to observe run lifecycle changes across the whole workspace.

```go
workspaceRoot := "/path/to/workspace"

ctx, cancel := context.WithCancel(context.Background())
defer cancel()

eventsCh, errsCh := runs.WatchWorkspace(ctx, workspaceRoot)
for {
	select {
	case event, ok := <-eventsCh:
		if !ok {
			return nil
		}
		status := ""
		if event.Summary != nil {
			status = event.Summary.Status
		}
		fmt.Printf("%s %s %s\n", event.Kind, event.RunID, status)
	case err, ok := <-errsCh:
		if ok && err != nil {
			return err
		}
	}
}
```

`WatchWorkspace` emits:

- `created` when a new run directory becomes readable
- `status_changed` when `run.json` changes enough to alter the normalized `RunSummary`
- `removed` when a run directory disappears
