package worker

import (
	"go.temporal.io/sdk/workflow"

	"github.com/compozy/compozy/engine/core"
	wf "github.com/compozy/compozy/engine/workflow"
)

type workflowStreamTracker struct {
	state *wf.StreamState
}

func newWorkflowStreamTracker(ctx workflow.Context, input WorkflowInput) (*workflowStreamTracker, error) {
	state := wf.NewStreamState(core.StatusRunning)
	tracker := &workflowStreamTracker{state: state}
	if err := workflow.SetQueryHandler(ctx, wf.StreamQueryName, func() (*wf.StreamState, error) {
		return state.Clone(), nil
	}); err != nil {
		return nil, err
	}
	if err := tracker.append(ctx, wf.StreamEventWorkflowStart, map[string]any{
		"workflow_id":  input.WorkflowID,
		"execution_id": input.WorkflowExecID,
	}); err != nil {
		return nil, err
	}
	if err := tracker.append(ctx, wf.StreamEventWorkflowStatus, map[string]any{
		"status": core.StatusRunning,
	}); err != nil {
		return nil, err
	}
	return tracker, nil
}

func (t *workflowStreamTracker) append(ctx workflow.Context, event string, payload any) error {
	if t == nil {
		return nil
	}
	return t.state.Append(event, workflow.Now(ctx), payload)
}

func (t *workflowStreamTracker) Fail(ctx workflow.Context, err error) {
	if t == nil {
		return
	}
	t.state.SetStatus(core.StatusFailed)
	payload := map[string]any{"status": core.StatusFailed}
	if err != nil {
		payload["error"] = err.Error()
	}
	if appendErr := t.append(ctx, wf.StreamEventError, payload); appendErr != nil {
		workflow.GetLogger(ctx).Warn("failed to append workflow error stream event", "error", appendErr)
	}
}

func (t *workflowStreamTracker) Success(ctx workflow.Context, state *wf.State) {
	if t == nil || state == nil {
		return
	}
	t.state.SetStatus(state.Status)
	payload := map[string]any{"status": state.Status}
	if state.Usage != nil {
		payload["usage"] = state.Usage
	}
	if state.Output != nil {
		payload["output"] = state.Output
	}
	if appendErr := t.append(ctx, wf.StreamEventComplete, payload); appendErr != nil {
		workflow.GetLogger(ctx).Warn("failed to append workflow completion stream event", "error", appendErr)
	}
}
