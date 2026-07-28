package hooks

import (
	"context"

	"fmt"
)

// DispatchAutomationJobPreFire runs the automation.job.pre_fire hook pipeline.
func (h *Hooks) DispatchAutomationJobPreFire(
	ctx context.Context,
	payload AutomationJobPreFirePayload,
) (AutomationJobPreFirePayload, error) {
	return executeDispatch(
		ctx,
		h,
		HookAutomationJobPreFire,
		payload,
		dispatchConfig[AutomationJobPreFirePayload, AutomationFirePatch]{
			match:  matchAutomationJobPreFire,
			apply:  applyAutomationJobPreFirePatch,
			denied: automationFirePatchDenied,
			denyErr: func(AutomationJobPreFirePayload, dispatchReport) error {
				return fmt.Errorf("%w: %s", ErrAutomationFireCancelled, HookAutomationJobPreFire)
			},
		},
	)
}

// DispatchAutomationJobPostFire runs the automation.job.post_fire hook dispatch.
func (h *Hooks) DispatchAutomationJobPostFire(
	ctx context.Context,
	payload AutomationJobPostFirePayload,
) (AutomationJobPostFirePayload, error) {
	return executeDispatch(
		ctx,
		h,
		HookAutomationJobPostFire,
		payload,
		dispatchConfig[AutomationJobPostFirePayload, AutomationObservationPatch]{
			match: matchAutomationJobPostFire,
			apply: applyNoop[AutomationJobPostFirePayload, AutomationObservationPatch],
		},
	)
}

// DispatchAutomationTriggerPreFire runs the automation.trigger.pre_fire hook pipeline.
func (h *Hooks) DispatchAutomationTriggerPreFire(
	ctx context.Context,
	payload AutomationTriggerPreFirePayload,
) (AutomationTriggerPreFirePayload, error) {
	return executeDispatch(
		ctx,
		h,
		HookAutomationTriggerPreFire,
		payload,
		dispatchConfig[AutomationTriggerPreFirePayload, AutomationFirePatch]{
			match:  matchAutomationTriggerPreFire,
			apply:  applyAutomationTriggerPreFirePatch,
			denied: automationFirePatchDenied,
			denyErr: func(AutomationTriggerPreFirePayload, dispatchReport) error {
				return fmt.Errorf("%w: %s", ErrAutomationFireCancelled, HookAutomationTriggerPreFire)
			},
		},
	)
}

// DispatchAutomationTriggerPostFire runs the automation.trigger.post_fire hook dispatch.
func (h *Hooks) DispatchAutomationTriggerPostFire(
	ctx context.Context,
	payload AutomationTriggerPostFirePayload,
) (AutomationTriggerPostFirePayload, error) {
	return executeDispatch(
		ctx,
		h,
		HookAutomationTriggerPostFire,
		payload,
		dispatchConfig[AutomationTriggerPostFirePayload, AutomationObservationPatch]{
			match: matchAutomationTriggerPostFire,
			apply: applyNoop[AutomationTriggerPostFirePayload, AutomationObservationPatch],
		},
	)
}

// DispatchAutomationRunCompleted runs the automation.run.completed hook dispatch.
func (h *Hooks) DispatchAutomationRunCompleted(
	ctx context.Context,
	payload AutomationRunCompletedPayload,
) (AutomationRunCompletedPayload, error) {
	return executeDispatch(
		ctx,
		h,
		HookAutomationRunCompleted,
		payload,
		dispatchConfig[AutomationRunCompletedPayload, AutomationObservationPatch]{
			match: matchAutomationRunCompleted,
			apply: applyNoop[AutomationRunCompletedPayload, AutomationObservationPatch],
		},
	)
}

// DispatchAutomationRunFailed runs the automation.run.failed hook dispatch.
func (h *Hooks) DispatchAutomationRunFailed(
	ctx context.Context,
	payload AutomationRunFailedPayload,
) (AutomationRunFailedPayload, error) {
	return executeDispatch(
		ctx,
		h,
		HookAutomationRunFailed,
		payload,
		dispatchConfig[AutomationRunFailedPayload, AutomationObservationPatch]{
			match: matchAutomationRunFailed,
			apply: applyNoop[AutomationRunFailedPayload, AutomationObservationPatch],
		},
	)
}
