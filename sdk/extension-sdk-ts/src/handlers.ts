import type {
  AgentOnSessionUpdatePayload,
  AgentPostSessionCreatePayload,
  AgentPostSessionEndPayload,
  AgentPreSessionCreatePayload,
  AgentPreSessionResumePayload,
  ArtifactPostWritePayload,
  ArtifactPreWritePayload,
  ArtifactWritePatch,
  BatchParamsPatch,
  EntriesPatch,
  Event,
  ExecuteHookRequest,
  ExtraSourcesPatch,
  FetchConfigPatch,
  GroupsPatch,
  HealthCheckRequest,
  HealthCheckResponse,
  HookContext,
  HookInfo,
  HookName,
  IssuesPatch,
  JobPatch,
  JobPostExecutePayload,
  JobPreExecutePayload,
  JobPreRetryPayload,
  JobsPatch,
  PlanPostDiscoverPayload,
  PlanPostGroupPayload,
  PlanPostPrepareJobsPayload,
  PlanPreDiscoverPayload,
  PlanPreGroupPayload,
  PlanPrePrepareJobsPayload,
  PromptPostBuildPayload,
  PromptPreBuildPayload,
  PromptPreSystemPayload,
  PromptTextPatch,
  ResolveDecisionPatch,
  ResumeSessionRequestPatch,
  ReviewPostFetchPayload,
  ReviewPostFixPayload,
  ReviewPreBatchPayload,
  ReviewPreFetchPayload,
  ReviewPreResolvePayload,
  ReviewWatchFinishedPayload,
  ReviewWatchPostRoundPayload,
  ReviewWatchPrePushPatch,
  ReviewWatchPrePushPayload,
  ReviewWatchPreRoundPatch,
  ReviewWatchPreRoundPayload,
  RetryDecisionPatch,
  RunPostShutdownPayload,
  RunPostStartPayload,
  RunPreShutdownPayload,
  RunPreStartPayload,
  RuntimeConfigPatch,
  SessionRequestPatch,
  ShutdownRequest,
  SystemAddendumPatch,
} from "./types.js";

/** Callback invoked when the host forwards a Compozy event to the extension. */
export type EventHandler = (event: Event) => Promise<void> | void;
/** Low-level hook handler that receives the raw JSON payload and returns a raw patch. */
export type RawHookHandler = (context: HookContext, payload: unknown) => Promise<unknown> | unknown;
/** Callback that overrides the default health check response. */
export type HealthCheckHandler = (
  request: HealthCheckRequest
) => Promise<HealthCheckResponse> | HealthCheckResponse;
/** Callback invoked during graceful shutdown before the process exits. */
export type ShutdownHandler = (request: ShutdownRequest) => Promise<void> | void;

/** Typed handler for mutable hooks. Receives a strongly-typed payload and returns an optional patch describing what to mutate. */
export type MutableHookHandler<Payload, Patch> = (
  context: HookContext,
  payload: Payload
) => Promise<Patch | void> | Patch | void;

/** Typed handler for observer-only hooks. Receives the payload for side-effects with no return value. */
export type ObserverHookHandler<Payload> = (
  context: HookContext,
  payload: Payload
) => Promise<void> | void;

/** Internal representation of a registered hook handler with its mutability flag. */
export interface RegisteredHook {
  mutable: boolean;
  handler: RawHookHandler;
}

/** Minimal interface for registering raw hook handlers on an extension. */
export interface HookRegistrationSurface {
  handle(hook: HookName, handler: RawHookHandler): this;
}

/** Type-safe mapping from each hook name to its corresponding handler signature, ensuring payload and patch types are correct at compile time. */
export type HookHandlerMatrix = {
  "plan.pre_discover": MutableHookHandler<PlanPreDiscoverPayload, ExtraSourcesPatch>;
  "plan.post_discover": MutableHookHandler<PlanPostDiscoverPayload, EntriesPatch>;
  "plan.pre_group": MutableHookHandler<PlanPreGroupPayload, EntriesPatch>;
  "plan.post_group": MutableHookHandler<PlanPostGroupPayload, GroupsPatch>;
  "plan.pre_prepare_jobs": MutableHookHandler<PlanPrePrepareJobsPayload, GroupsPatch>;
  "plan.post_prepare_jobs": MutableHookHandler<PlanPostPrepareJobsPayload, JobsPatch>;
  "prompt.pre_build": MutableHookHandler<PromptPreBuildPayload, BatchParamsPatch>;
  "prompt.post_build": MutableHookHandler<PromptPostBuildPayload, PromptTextPatch>;
  "prompt.pre_system": MutableHookHandler<PromptPreSystemPayload, SystemAddendumPatch>;
  "agent.pre_session_create": MutableHookHandler<AgentPreSessionCreatePayload, SessionRequestPatch>;
  "agent.post_session_create": ObserverHookHandler<AgentPostSessionCreatePayload>;
  "agent.pre_session_resume": MutableHookHandler<
    AgentPreSessionResumePayload,
    ResumeSessionRequestPatch
  >;
  "agent.on_session_update": ObserverHookHandler<AgentOnSessionUpdatePayload>;
  "agent.post_session_end": ObserverHookHandler<AgentPostSessionEndPayload>;
  "job.pre_execute": MutableHookHandler<JobPreExecutePayload, JobPatch>;
  "job.post_execute": ObserverHookHandler<JobPostExecutePayload>;
  "job.pre_retry": MutableHookHandler<JobPreRetryPayload, RetryDecisionPatch>;
  "run.pre_start": MutableHookHandler<RunPreStartPayload, RuntimeConfigPatch>;
  "run.post_start": ObserverHookHandler<RunPostStartPayload>;
  "run.pre_shutdown": ObserverHookHandler<RunPreShutdownPayload>;
  "run.post_shutdown": ObserverHookHandler<RunPostShutdownPayload>;
  "review.pre_fetch": MutableHookHandler<ReviewPreFetchPayload, FetchConfigPatch>;
  "review.post_fetch": MutableHookHandler<ReviewPostFetchPayload, IssuesPatch>;
  "review.pre_batch": MutableHookHandler<ReviewPreBatchPayload, GroupsPatch>;
  "review.post_fix": ObserverHookHandler<ReviewPostFixPayload>;
  "review.pre_resolve": MutableHookHandler<ReviewPreResolvePayload, ResolveDecisionPatch>;
  "review.watch_pre_round": MutableHookHandler<
    ReviewWatchPreRoundPayload,
    ReviewWatchPreRoundPatch
  >;
  "review.watch_post_round": ObserverHookHandler<ReviewWatchPostRoundPayload>;
  "review.watch_pre_push": MutableHookHandler<ReviewWatchPrePushPayload, ReviewWatchPrePushPatch>;
  "review.watch_finished": ObserverHookHandler<ReviewWatchFinishedPayload>;
  "artifact.pre_write": MutableHookHandler<ArtifactPreWritePayload, ArtifactWritePatch>;
  "artifact.post_write": ObserverHookHandler<ArtifactPostWritePayload>;
};

/** Returns true if the given hook name corresponds to a mutable hook that can return a patch. */
export function isMutableHook(hook: HookName): boolean {
  switch (hook) {
    case "agent.post_session_create":
    case "agent.on_session_update":
    case "agent.post_session_end":
    case "job.post_execute":
    case "run.post_start":
    case "run.pre_shutdown":
    case "run.post_shutdown":
    case "review.post_fix":
    case "review.watch_post_round":
    case "review.watch_finished":
    case "artifact.post_write":
      return false;
    default:
      return true;
  }
}

/** Registers a typed mutable hook handler on the given surface, wrapping it as a raw handler. */
export function registerMutableHook<Payload, Patch>(
  surface: HookRegistrationSurface,
  hook: HookName,
  handler: MutableHookHandler<Payload, Patch>
): HookRegistrationSurface {
  return surface.handle(hook, async (context, payload) => {
    return await handler(context, payload as Payload);
  });
}

/** Registers a typed observer hook handler on the given surface, wrapping it as a raw handler that returns undefined. */
export function registerObserverHook<Payload>(
  surface: HookRegistrationSurface,
  hook: HookName,
  handler: ObserverHookHandler<Payload>
): HookRegistrationSurface {
  return surface.handle(hook, async (context, payload) => {
    await handler(context, payload as Payload);
    return undefined;
  });
}

/** Builds a {@link HookContext} from an incoming execute-hook request and host metadata. */
export function requestContext(
  request: ExecuteHookRequest,
  host: HookContext["host"]
): HookContext {
  return {
    invocation_id: request.invocation_id,
    hook: request.hook as HookInfo,
    host,
  };
}
