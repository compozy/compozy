package automation

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/compozy/agh/internal/network/participation"
	taskpkg "github.com/compozy/agh/internal/task"
)

// LoopStartKind is the automation-local copy of loop start-surface kinds.
type LoopStartKind string

const (
	// LoopStartKindTrigger starts a Loop from non-webhook automation trigger ingress.
	LoopStartKindTrigger LoopStartKind = "trigger"
	// LoopStartKindSchedule starts a Loop from scheduled automation jobs.
	LoopStartKindSchedule LoopStartKind = "schedule"
	// LoopStartKindWebhook starts a Loop from webhook trigger ingress.
	LoopStartKindWebhook LoopStartKind = "webhook"
)

// LoopTargetValidationRequest validates an automation loop target without firing it.
type LoopTargetValidationRequest struct {
	WorkspaceID  string
	LoopName     string
	Kind         LoopStartKind
	Inputs       map[string]any
	InputMapping map[string]string
}

// LoopCatchUpPolicyRequest asks the composition root for a target-aware schedule default.
type LoopCatchUpPolicyRequest struct {
	WorkspaceID string
	LoopName    string
	Kind        LoopStartKind
}

// LoopStartRequest starts one loop target from an automation fire.
type LoopStartRequest struct {
	WorkspaceID          string
	LoopName             string
	Kind                 LoopStartKind
	Inputs               map[string]any
	InputMapping         map[string]string
	TriggerPayload       map[string]any
	NetworkParticipation *participation.Request
	Actor                taskpkg.ActorContext
	AutomationRunID      string
	ScheduledAt          *time.Time
	CatchUp              bool
	CatchUpPolicy        SchedulerCatchUpPolicy
}

// LoopStartResult returns the observable loop_run correlation for an automation fire.
type LoopStartResult struct {
	RunID string
}

// LoopStarter is implemented at the composition root so automation does not import internal/loop.
type LoopStarter interface {
	ValidateLoopTarget(ctx context.Context, req LoopTargetValidationRequest) error
	StartLoop(ctx context.Context, req LoopStartRequest) (LoopStartResult, error)
}

// LoopCatchUpPolicyResolver is optionally implemented by LoopStarter.
type LoopCatchUpPolicyResolver interface {
	DefaultLoopCatchUpPolicy(ctx context.Context, req LoopCatchUpPolicyRequest) (SchedulerCatchUpPolicy, error)
}

// WithDispatcherLoopStarter injects the loop start boundary used by loop-target automations.
func WithDispatcherLoopStarter(starter LoopStarter) DispatcherOption {
	return func(dispatcher *Dispatcher) {
		dispatcher.loopStarter = starter
	}
}

func (d *Dispatcher) dispatchLoopBackedAttempt(
	ctx context.Context,
	req DispatchRequest,
	scheduledRun *Run,
	attempt int,
) (*Run, error) {
	target := req.loopTarget()
	if target == nil {
		return d.finishRun(ctx, scheduledRun, RunFailed, errors.New("automation: loop target is required"))
	}
	if d.loopStarter == nil {
		return d.finishRun(ctx, scheduledRun, RunFailed, errors.New("automation: loop target requires loop starter"))
	}

	_, canceled, hookErr := d.dispatchPreFireHook(ctx, req, "", attempt)
	if hookErr != nil {
		return d.finishRun(ctx, scheduledRun, RunFailed, hookErr)
	}
	if canceled {
		return d.finishRun(ctx, scheduledRun, RunCancelled, nil)
	}

	actor, err := loopTargetActorContext(req, scheduledRun.ID)
	if err != nil {
		return d.finishRun(ctx, scheduledRun, RunFailed, err)
	}
	result, err := d.loopStarter.StartLoop(ctx, LoopStartRequest{
		WorkspaceID:          strings.TrimSpace(target.WorkspaceID),
		LoopName:             strings.TrimSpace(target.LoopName),
		Kind:                 req.loopStartKind(),
		Inputs:               cloneJSONMap(target.Inputs),
		InputMapping:         cloneStringMap(target.InputMapping),
		TriggerPayload:       cloneJSONMap(req.envelopeData()),
		NetworkParticipation: cloneParticipationRequest(target.NetworkParticipation),
		Actor:                actor,
		AutomationRunID:      strings.TrimSpace(scheduledRun.ID),
		ScheduledAt:          cloneTimePointer(req.ScheduledAt),
		CatchUp:              req.CatchUp,
		CatchUpPolicy:        req.CatchUpPolicy,
	})
	if err != nil {
		if req.CatchUp && errors.Is(err, ErrLoopConcurrencyConflict) {
			scheduledRun.Metadata = loopCatchUpFailureMetadata(req, "loop_concurrency_conflict")
		}
		return d.finishRun(ctx, scheduledRun, classifyDispatchError(err), err)
	}
	if strings.TrimSpace(result.RunID) == "" {
		return d.finishRun(ctx, scheduledRun, RunFailed, errors.New("automation: loop starter returned empty run id"))
	}

	delegatedRun, err := d.delegateRunToLoop(ctx, scheduledRun, result.RunID)
	if err != nil {
		return delegatedRun, err
	}
	d.dispatchPostFireHook(ctx, req, *delegatedRun)
	return delegatedRun, nil
}

func loopCatchUpFailureMetadata(req DispatchRequest, reason string) map[string]any {
	metadata := map[string]any{
		"catch_up": true,
		"reason":   strings.TrimSpace(reason),
	}
	if req.CatchUpPolicy != "" {
		metadata["catch_up_policy"] = string(req.CatchUpPolicy)
	}
	if req.ScheduledAt != nil && !req.ScheduledAt.IsZero() {
		scheduledAt := req.ScheduledAt.UTC().Format(time.RFC3339Nano)
		metadata["scheduled_at"] = scheduledAt
		metadata["original_due_at"] = scheduledAt
	}
	return metadata
}

func (d *Dispatcher) delegateRunToLoop(ctx context.Context, current *Run, loopRunID string) (*Run, error) {
	updatedRun, updateErr := d.transitionRun(ctx, current, func(run *Run, now time.Time) {
		run.LoopRunID = strings.TrimSpace(loopRunID)
		run.TaskID = ""
		run.TaskRunID = ""
		run.Status = RunDelegated
		run.EndedAt = timePointer(now)
		run.Error = ""
	})
	if updateErr != nil {
		return updatedRun, updateErr
	}

	d.logger.Info(
		"automation.dispatch.loop_delegated",
		"run_id", updatedRun.ID,
		"job_id", strings.TrimSpace(updatedRun.JobID),
		"trigger_id", strings.TrimSpace(updatedRun.TriggerID),
		"loop_run_id", strings.TrimSpace(updatedRun.LoopRunID),
		"attempt", updatedRun.Attempt,
	)
	return updatedRun, nil
}

func loopTargetActorContext(req DispatchRequest, runID string) (taskpkg.ActorContext, error) {
	actorRef := req.definitionID()
	if actorRef == "" {
		return taskpkg.ActorContext{}, errors.New("automation: loop target automation id is required")
	}
	return taskpkg.DeriveAutomationActorContext(actorRef, automationTaskOriginRef(runID))
}

func (r DispatchRequest) loopTarget() *LoopTarget {
	switch {
	case r.Job != nil && r.Job.IsLoopTarget():
		return r.Job.LoopTarget
	case r.Trigger != nil && r.Trigger.IsLoopTarget():
		return r.Trigger.LoopTarget
	default:
		return nil
	}
}

func (r DispatchRequest) loopStartKind() LoopStartKind {
	return loopStartKindForAutomation(r.Job, r.Trigger)
}

func (r DispatchRequest) definitionID() string {
	if r.Job != nil {
		return strings.TrimSpace(r.Job.ID)
	}
	if r.Trigger != nil {
		return strings.TrimSpace(r.Trigger.ID)
	}
	return ""
}

func validateLoopTargetWithStarter(
	ctx context.Context,
	starter LoopStarter,
	target *LoopTarget,
	kind LoopStartKind,
) error {
	if target == nil || starter == nil {
		return nil
	}
	return starter.ValidateLoopTarget(ctx, LoopTargetValidationRequest{
		WorkspaceID:  strings.TrimSpace(target.WorkspaceID),
		LoopName:     strings.TrimSpace(target.LoopName),
		Kind:         kind,
		Inputs:       cloneJSONMap(target.Inputs),
		InputMapping: cloneStringMap(target.InputMapping),
	})
}

func loopStartKindForAutomation(job *Job, trigger *Trigger) LoopStartKind {
	if job != nil {
		return LoopStartKindSchedule
	}
	if trigger != nil && strings.TrimSpace(trigger.Event) == triggerEventWebhook {
		return LoopStartKindWebhook
	}
	return LoopStartKindTrigger
}
