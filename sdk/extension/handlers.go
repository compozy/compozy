package extension

import (
	"context"
	"encoding/json"
	"fmt"
)

func registerMutableHook[Payload any, Patch any](
	e *Extension,
	event HookName,
	handler func(context.Context, HookContext, Payload) (Patch, error),
) *Extension {
	if e == nil || handler == nil {
		return e
	}

	return e.Handle(event, func(ctx context.Context, hook HookContext, raw json.RawMessage) (json.RawMessage, error) {
		var payload Payload
		if err := unmarshalJSON(raw, &payload); err != nil {
			return nil, newInvalidParamsError(map[string]any{
				"method": "execute_hook",
				"hook":   string(event),
				"error":  err.Error(),
			})
		}

		patch, err := handler(ctx, hook, payload)
		if err != nil {
			return nil, err
		}
		return marshalJSON(patch)
	})
}

func registerObserverHook[Payload any](
	e *Extension,
	event HookName,
	handler func(context.Context, HookContext, Payload) error,
) *Extension {
	if e == nil || handler == nil {
		return e
	}

	return e.Handle(event, func(ctx context.Context, hook HookContext, raw json.RawMessage) (json.RawMessage, error) {
		var payload Payload
		if err := unmarshalJSON(raw, &payload); err != nil {
			return nil, newInvalidParamsError(map[string]any{
				"method": "execute_hook",
				"hook":   string(event),
				"error":  err.Error(),
			})
		}
		if err := handler(ctx, hook, payload); err != nil {
			return nil, err
		}
		return nil, nil
	})
}

// OnPlanPreDiscover registers the plan.pre_discover handler.
func (e *Extension) OnPlanPreDiscover(
	handler func(context.Context, HookContext, PlanPreDiscoverPayload) (ExtraSourcesPatch, error),
) *Extension {
	return registerMutableHook(e, HookPlanPreDiscover, handler)
}

// OnPlanPostDiscover registers the plan.post_discover handler.
func (e *Extension) OnPlanPostDiscover(
	handler func(context.Context, HookContext, PlanPostDiscoverPayload) (EntriesPatch, error),
) *Extension {
	return registerMutableHook(e, HookPlanPostDiscover, handler)
}

// OnPlanPreGroup registers the plan.pre_group handler.
func (e *Extension) OnPlanPreGroup(
	handler func(context.Context, HookContext, PlanPreGroupPayload) (EntriesPatch, error),
) *Extension {
	return registerMutableHook(e, HookPlanPreGroup, handler)
}

// OnPlanPostGroup registers the plan.post_group handler.
func (e *Extension) OnPlanPostGroup(
	handler func(context.Context, HookContext, PlanPostGroupPayload) (GroupsPatch, error),
) *Extension {
	return registerMutableHook(e, HookPlanPostGroup, handler)
}

// OnPlanPrePrepareJobs registers the plan.pre_prepare_jobs handler.
func (e *Extension) OnPlanPrePrepareJobs(
	handler func(context.Context, HookContext, PlanPrePrepareJobsPayload) (GroupsPatch, error),
) *Extension {
	return registerMutableHook(e, HookPlanPrePrepareJobs, handler)
}

// OnPlanPreResolveTaskRuntime registers the plan.pre_resolve_task_runtime handler.
func (e *Extension) OnPlanPreResolveTaskRuntime(
	handler func(context.Context, HookContext, PlanPreResolveTaskRuntimePayload) (TaskRuntimePatch, error),
) *Extension {
	return registerMutableHook(e, HookPlanPreResolveTaskRuntime, handler)
}

// OnPlanPostPrepareJobs registers the plan.post_prepare_jobs handler.
func (e *Extension) OnPlanPostPrepareJobs(
	handler func(context.Context, HookContext, PlanPostPrepareJobsPayload) (JobsPatch, error),
) *Extension {
	return registerMutableHook(e, HookPlanPostPrepareJobs, handler)
}

// OnPromptPreBuild registers the prompt.pre_build handler.
func (e *Extension) OnPromptPreBuild(
	handler func(context.Context, HookContext, PromptPreBuildPayload) (BatchParamsPatch, error),
) *Extension {
	return registerMutableHook(e, HookPromptPreBuild, handler)
}

// OnPromptPostBuild registers the prompt.post_build handler.
func (e *Extension) OnPromptPostBuild(
	handler func(context.Context, HookContext, PromptPostBuildPayload) (PromptTextPatch, error),
) *Extension {
	return registerMutableHook(e, HookPromptPostBuild, handler)
}

// OnPromptPreSystem registers the prompt.pre_system handler.
func (e *Extension) OnPromptPreSystem(
	handler func(context.Context, HookContext, PromptPreSystemPayload) (SystemAddendumPatch, error),
) *Extension {
	return registerMutableHook(e, HookPromptPreSystem, handler)
}

// OnAgentPreSessionCreate registers the agent.pre_session_create handler.
func (e *Extension) OnAgentPreSessionCreate(
	handler func(context.Context, HookContext, AgentPreSessionCreatePayload) (SessionRequestPatch, error),
) *Extension {
	return registerMutableHook(e, HookAgentPreSessionCreate, handler)
}

// OnAgentPostSessionCreate registers the agent.post_session_create handler.
func (e *Extension) OnAgentPostSessionCreate(
	handler func(context.Context, HookContext, AgentPostSessionCreatePayload) error,
) *Extension {
	return registerObserverHook(e, HookAgentPostSessionCreate, handler)
}

// OnAgentPreSessionResume registers the agent.pre_session_resume handler.
func (e *Extension) OnAgentPreSessionResume(
	handler func(context.Context, HookContext, AgentPreSessionResumePayload) (ResumeSessionRequestPatch, error),
) *Extension {
	return registerMutableHook(e, HookAgentPreSessionResume, handler)
}

// OnAgentOnSessionUpdate registers the agent.on_session_update handler.
func (e *Extension) OnAgentOnSessionUpdate(
	handler func(context.Context, HookContext, AgentOnSessionUpdatePayload) error,
) *Extension {
	return registerObserverHook(e, HookAgentOnSessionUpdate, handler)
}

// OnAgentPostSessionEnd registers the agent.post_session_end handler.
func (e *Extension) OnAgentPostSessionEnd(
	handler func(context.Context, HookContext, AgentPostSessionEndPayload) error,
) *Extension {
	return registerObserverHook(e, HookAgentPostSessionEnd, handler)
}

// OnJobPreExecute registers the job.pre_execute handler.
func (e *Extension) OnJobPreExecute(
	handler func(context.Context, HookContext, JobPreExecutePayload) (JobPatch, error),
) *Extension {
	return registerMutableHook(e, HookJobPreExecute, handler)
}

// OnJobPostExecute registers the job.post_execute handler.
func (e *Extension) OnJobPostExecute(
	handler func(context.Context, HookContext, JobPostExecutePayload) error,
) *Extension {
	return registerObserverHook(e, HookJobPostExecute, handler)
}

// OnJobPreRetry registers the job.pre_retry handler.
func (e *Extension) OnJobPreRetry(
	handler func(context.Context, HookContext, JobPreRetryPayload) (RetryDecisionPatch, error),
) *Extension {
	return registerMutableHook(e, HookJobPreRetry, handler)
}

// OnRunPreStart registers the run.pre_start handler.
func (e *Extension) OnRunPreStart(
	handler func(context.Context, HookContext, RunPreStartPayload) (RuntimeConfigPatch, error),
) *Extension {
	return registerMutableHook(e, HookRunPreStart, handler)
}

// OnRunPostStart registers the run.post_start handler.
func (e *Extension) OnRunPostStart(
	handler func(context.Context, HookContext, RunPostStartPayload) error,
) *Extension {
	return registerObserverHook(e, HookRunPostStart, handler)
}

// OnRunPreShutdown registers the run.pre_shutdown handler.
func (e *Extension) OnRunPreShutdown(
	handler func(context.Context, HookContext, RunPreShutdownPayload) error,
) *Extension {
	return registerObserverHook(e, HookRunPreShutdown, handler)
}

// OnRunPostShutdown registers the run.post_shutdown handler.
func (e *Extension) OnRunPostShutdown(
	handler func(context.Context, HookContext, RunPostShutdownPayload) error,
) *Extension {
	return registerObserverHook(e, HookRunPostShutdown, handler)
}

// OnReviewPreFetch registers the review.pre_fetch handler.
func (e *Extension) OnReviewPreFetch(
	handler func(context.Context, HookContext, ReviewPreFetchPayload) (FetchConfigPatch, error),
) *Extension {
	return registerMutableHook(e, HookReviewPreFetch, handler)
}

// OnReviewPostFetch registers the review.post_fetch handler.
func (e *Extension) OnReviewPostFetch(
	handler func(context.Context, HookContext, ReviewPostFetchPayload) (IssuesPatch, error),
) *Extension {
	return registerMutableHook(e, HookReviewPostFetch, handler)
}

// OnReviewPreBatch registers the review.pre_batch handler.
func (e *Extension) OnReviewPreBatch(
	handler func(context.Context, HookContext, ReviewPreBatchPayload) (GroupsPatch, error),
) *Extension {
	return registerMutableHook(e, HookReviewPreBatch, handler)
}

// OnReviewPostFix registers the review.post_fix handler.
func (e *Extension) OnReviewPostFix(
	handler func(context.Context, HookContext, ReviewPostFixPayload) error,
) *Extension {
	return registerObserverHook(e, HookReviewPostFix, handler)
}

// OnReviewPreResolve registers the review.pre_resolve handler.
func (e *Extension) OnReviewPreResolve(
	handler func(context.Context, HookContext, ReviewPreResolvePayload) (ResolveDecisionPatch, error),
) *Extension {
	return registerMutableHook(e, HookReviewPreResolve, handler)
}

// OnReviewWatchPreRound registers the review.watch_pre_round handler.
func (e *Extension) OnReviewWatchPreRound(
	handler func(context.Context, HookContext, ReviewWatchPreRoundPayload) (ReviewWatchPreRoundPatch, error),
) *Extension {
	return registerMutableHook(e, HookReviewWatchPreRound, handler)
}

// OnReviewWatchPostRound registers the review.watch_post_round handler.
func (e *Extension) OnReviewWatchPostRound(
	handler func(context.Context, HookContext, ReviewWatchPostRoundPayload) error,
) *Extension {
	return registerObserverHook(e, HookReviewWatchPostRound, handler)
}

// OnReviewWatchPrePush registers the review.watch_pre_push handler.
func (e *Extension) OnReviewWatchPrePush(
	handler func(context.Context, HookContext, ReviewWatchPrePushPayload) (ReviewWatchPrePushPatch, error),
) *Extension {
	return registerMutableHook(e, HookReviewWatchPrePush, handler)
}

// OnReviewWatchFinished registers the review.watch_finished handler.
func (e *Extension) OnReviewWatchFinished(
	handler func(context.Context, HookContext, ReviewWatchFinishedPayload) error,
) *Extension {
	return registerObserverHook(e, HookReviewWatchFinished, handler)
}

// OnArtifactPreWrite registers the artifact.pre_write handler.
func (e *Extension) OnArtifactPreWrite(
	handler func(context.Context, HookContext, ArtifactPreWritePayload) (ArtifactWritePatch, error),
) *Extension {
	return registerMutableHook(e, HookArtifactPreWrite, handler)
}

// OnArtifactPostWrite registers the artifact.post_write handler.
func (e *Extension) OnArtifactPostWrite(
	handler func(context.Context, HookContext, ArtifactPostWritePayload) error,
) *Extension {
	return registerObserverHook(e, HookArtifactPostWrite, handler)
}

// MustJSON marshals value into json.RawMessage or panics.
func MustJSON(value any) json.RawMessage {
	raw, err := marshalJSON(value)
	if err != nil {
		panic(fmt.Sprintf("marshal json: %v", err))
	}
	return raw
}
