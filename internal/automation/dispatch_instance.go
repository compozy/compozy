package automation

import (
	"context"
	"encoding/json/v2"
	"errors"
)

func WithDispatcherInstanceEvents(sink InstanceEventSink) DispatcherOption {
	return func(d *Dispatcher) { d.instanceEvents = sink }
}

func (d *Dispatcher) dispatchInstanceEvent(
	ctx context.Context,
	req DispatchRequest,
	run *Run,
	attempt int,
) (*Run, error) {
	if d.instanceEvents == nil {
		return d.finishRun(ctx, run, RunFailed, errors.New("automation: instance event sink is unavailable"))
	}
	prompt, err := req.prompt()
	if err != nil {
		return d.finishRun(ctx, run, RunFailed, err)
	}
	prompt, canceled, err := d.dispatchPreFireHook(ctx, req, prompt, attempt)
	if err != nil {
		return d.finishRun(ctx, run, RunFailed, err)
	}
	if canceled {
		return d.finishRun(ctx, run, RunCancelled, nil)
	}
	// Rendered template values can contain external text. Keep the entire rendered prompt
	// inside the event fence; the stored trigger template remains the trusted instruction.
	data, err := json.Marshal(struct {
		Prompt string         `json:"prompt"`
		Data   map[string]any `json:"data"`
	}{Prompt: prompt, Data: req.envelopeData()})
	if err != nil {
		return d.finishRun(ctx, run, RunFailed, err)
	}
	trigger := req.Trigger
	result, err := d.instanceEvents.Deliver(ctx, InstanceTarget{
		ProfileID:       trigger.ProfileID,
		WorkspaceID:     trigger.WorkspaceID,
		AgentName:       trigger.AgentName,
		ChecksSupported: true,
	}, InstanceEvent{RunID: run.ID, TriggerID: trigger.ID, Summary: string(data)})
	if err != nil {
		return d.finishRun(ctx, run, RunFailed, err)
	}
	if run.Metadata == nil {
		run.Metadata = map[string]any{}
	}
	run.Metadata["instance_event_id"] = result.EventID
	run.Metadata["instance_event_instance_id"] = result.InstanceID
	run.Metadata["instance_event_check_scheduled"] = result.CheckScheduled
	run.Metadata["instance_event_reason"] = result.Reason
	d.dispatchPostFireHook(ctx, req, *run)
	return d.finishRun(ctx, run, RunCompleted, nil)
}
