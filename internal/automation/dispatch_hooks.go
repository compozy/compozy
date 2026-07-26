package automation

import (
	"context"
	"errors"

	"strings"

	hookspkg "github.com/compozy/agh/internal/hooks"
)

func (d *Dispatcher) dispatchPreFireHook(
	ctx context.Context,
	req DispatchRequest,
	prompt string,
	attempt int,
) (string, bool, error) {
	if d == nil || d.hooks == nil {
		return prompt, false, nil
	}

	switch {
	case req.Job != nil:
		payload := hookspkg.AutomationJobPreFirePayload{
			JobID:       strings.TrimSpace(req.Job.ID),
			JobName:     strings.TrimSpace(req.Job.Name),
			AgentName:   strings.TrimSpace(req.Job.AgentName),
			WorkspaceID: strings.TrimSpace(req.Job.WorkspaceID),
			Prompt:      prompt,
			Schedule:    hookSchedulePayload(req.Job.Schedule),
			Payload:     cloneJSONMap(req.Payload),
			Attempt:     attempt,
		}
		next, err := d.hooks.DispatchAutomationJobPreFire(ctx, payload)
		if err != nil {
			if errors.Is(err, hookspkg.ErrAutomationFireCancelled) {
				return prompt, true, nil
			}
			return prompt, false, err
		}
		return strings.TrimSpace(next.Prompt), false, nil
	case req.Trigger != nil:
		payload := hookspkg.AutomationTriggerPreFirePayload{
			TriggerID:   strings.TrimSpace(req.Trigger.ID),
			TriggerName: strings.TrimSpace(req.Trigger.Name),
			Event:       strings.TrimSpace(req.Trigger.Event),
			AgentName:   strings.TrimSpace(req.Trigger.AgentName),
			WorkspaceID: strings.TrimSpace(req.Trigger.WorkspaceID),
			Prompt:      prompt,
			Payload:     cloneJSONMap(req.envelopeData()),
			Attempt:     attempt,
		}
		next, err := d.hooks.DispatchAutomationTriggerPreFire(ctx, payload)
		if err != nil {
			if errors.Is(err, hookspkg.ErrAutomationFireCancelled) {
				return prompt, true, nil
			}
			return prompt, false, err
		}
		return strings.TrimSpace(next.Prompt), false, nil
	default:
		return prompt, false, nil
	}
}

func (d *Dispatcher) dispatchPostFireHook(ctx context.Context, req DispatchRequest, run Run) {
	if d == nil || d.hooks == nil {
		return
	}

	switch {
	case req.Job != nil:
		if _, err := d.hooks.DispatchAutomationJobPostFire(ctx, hookspkg.AutomationJobPostFirePayload{
			JobID:       strings.TrimSpace(req.Job.ID),
			JobName:     strings.TrimSpace(req.Job.Name),
			AgentName:   strings.TrimSpace(req.Job.AgentName),
			WorkspaceID: strings.TrimSpace(req.Job.WorkspaceID),
			RunID:       strings.TrimSpace(run.ID),
			SessionID:   strings.TrimSpace(run.SessionID),
		}); err != nil {
			d.logHookDispatchError(
				"automation.dispatch.job_post_fire_hook_failed",
				err,
				"job_id",
				strings.TrimSpace(req.Job.ID),
				"run_id",
				strings.TrimSpace(run.ID),
			)
		}
	case req.Trigger != nil:
		if _, err := d.hooks.DispatchAutomationTriggerPostFire(ctx, hookspkg.AutomationTriggerPostFirePayload{
			TriggerID:   strings.TrimSpace(req.Trigger.ID),
			TriggerName: strings.TrimSpace(req.Trigger.Name),
			Event:       strings.TrimSpace(req.Trigger.Event),
			AgentName:   strings.TrimSpace(req.Trigger.AgentName),
			WorkspaceID: strings.TrimSpace(req.Trigger.WorkspaceID),
			RunID:       strings.TrimSpace(run.ID),
			SessionID:   strings.TrimSpace(run.SessionID),
		}); err != nil {
			d.logHookDispatchError(
				"automation.dispatch.trigger_post_fire_hook_failed",
				err,
				"trigger_id",
				strings.TrimSpace(req.Trigger.ID),
				"run_id",
				strings.TrimSpace(run.ID),
			)
		}
	}
}

func (d *Dispatcher) emitRunLifecycleHooks(
	ctx context.Context,
	req DispatchRequest,
	run Run,
	dispatchErr error,
	willRetry bool,
) {
	if d == nil || d.hooks == nil {
		return
	}
	if run.Status == RunCompleted {
		if _, err := d.hooks.DispatchAutomationRunCompleted(ctx, hookspkg.AutomationRunCompletedPayload{
			RunID:       strings.TrimSpace(run.ID),
			JobID:       strings.TrimSpace(run.JobID),
			TriggerID:   strings.TrimSpace(run.TriggerID),
			AgentName:   req.agentName(),
			WorkspaceID: req.workspaceID(),
			SessionID:   strings.TrimSpace(run.SessionID),
			Attempt:     run.Attempt,
			DurationMS:  runDurationMilliseconds(run),
		}); err != nil {
			d.logHookDispatchError(
				"automation.dispatch.run_completed_hook_failed",
				err,
				"run_id",
				strings.TrimSpace(run.ID),
			)
		}
		return
	}
	if run.Status != RunFailed {
		return
	}

	errText := strings.TrimSpace(run.Error)
	if errText == "" && dispatchErr != nil {
		errText = dispatchErr.Error()
	}
	if _, err := d.hooks.DispatchAutomationRunFailed(ctx, hookspkg.AutomationRunFailedPayload{
		RunID:       strings.TrimSpace(run.ID),
		JobID:       strings.TrimSpace(run.JobID),
		TriggerID:   strings.TrimSpace(run.TriggerID),
		AgentName:   req.agentName(),
		WorkspaceID: req.workspaceID(),
		SessionID:   strings.TrimSpace(run.SessionID),
		Error:       errText,
		Attempt:     run.Attempt,
		WillRetry:   willRetry,
	}); err != nil {
		d.logHookDispatchError("automation.dispatch.run_failed_hook_failed", err, "run_id", strings.TrimSpace(run.ID))
	}
}
