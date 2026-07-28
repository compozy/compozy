package runs

import (
	"context"
	"time"
)

// RunEventKind identifies the type of workspace run event.
type RunEventKind string

const (
	// RunEventCreated reports a newly observed run.
	RunEventCreated RunEventKind = "created"
	// RunEventStatusChanged reports a run whose status changed.
	RunEventStatusChanged RunEventKind = "status_changed"
	// RunEventRemoved reports a removed run directory.
	RunEventRemoved RunEventKind = "removed"
)

const workspaceWatchPollInterval = 100 * time.Millisecond

// RunEvent reports workspace-level run lifecycle changes.
type RunEvent struct {
	Kind    RunEventKind
	RunID   string
	Summary *RunSummary
}

// WatchWorkspace emits RunEvent notifications for daemon-managed runs in workspaceRoot.
func WatchWorkspace(ctx context.Context, workspaceRoot string) (<-chan RunEvent, <-chan error) {
	out := make(chan RunEvent)
	errs := make(chan error, 4)

	go func() {
		defer close(out)
		defer close(errs)

		client, err := resolveRunsDaemonReader()
		if err != nil {
			sendRunError(ctx, errs, err)
			return
		}

		cleanRoot := cleanWorkspaceRoot(workspaceRoot)
		known := make(map[string]RunSummary)
		initial, err := client.ListRuns(ctx, cleanRoot, ListOptions{Limit: defaultRunListQueryLimit})
		if err != nil {
			sendRunError(ctx, errs, err)
			return
		}
		for i := range initial {
			known[initial[i].RunID] = initial[i]
		}

		ticker := time.NewTicker(workspaceWatchPollInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				current, err := client.ListRuns(ctx, cleanRoot, ListOptions{Limit: defaultRunListQueryLimit})
				if err != nil {
					if !sendRunError(ctx, errs, err) {
						return
					}
					continue
				}
				if err := diffWorkspaceRuns(ctx, client, cleanRoot, known, current, out); err != nil {
					if !sendRunError(ctx, errs, err) {
						return
					}
				}
			}
		}
	}()

	return out, errs
}

func diffWorkspaceRuns(
	ctx context.Context,
	client daemonRunReader,
	workspaceRoot string,
	known map[string]RunSummary,
	current []RunSummary,
	out chan<- RunEvent,
) error {
	currentByID := make(map[string]RunSummary, len(current))
	for i := range current {
		currentByID[current[i].RunID] = current[i]
		if err := applyWorkspaceRunSummary(
			ctx,
			client,
			workspaceRoot,
			current[i].RunID,
			current[i],
			known,
			out,
		); err != nil {
			return err
		}
	}

	for runID := range known {
		if _, ok := currentByID[runID]; ok {
			continue
		}
		delete(known, runID)
		if err := sendWorkspaceEvent(ctx, out, RunEvent{
			Kind:  RunEventRemoved,
			RunID: runID,
		}); err != nil {
			return err
		}
	}
	return nil
}

func applyWorkspaceRunSummary(
	ctx context.Context,
	client daemonRunReader,
	workspaceRoot string,
	runID string,
	summary RunSummary,
	known map[string]RunSummary,
	out chan<- RunEvent,
) error {
	previous, exists := known[runID]
	if exists && previous.Status == summary.Status {
		known[runID] = summary
		return nil
	}

	hydrated, err := hydrateRunSummaryDetails(ctx, client, workspaceRoot, summary)
	if err != nil {
		return err
	}
	known[runID] = hydrated

	kind := RunEventCreated
	if exists {
		kind = RunEventStatusChanged
	}
	return sendWorkspaceEvent(ctx, out, RunEvent{
		Kind:    kind,
		RunID:   runID,
		Summary: summaryPointer(hydrated),
	})
}

func sendWorkspaceEvent(ctx context.Context, out chan<- RunEvent, event RunEvent) error {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case out <- event:
		return nil
	}
}

func summaryPointer(summary RunSummary) *RunSummary {
	copyValue := cloneRunSummary(summary)
	return &copyValue
}
