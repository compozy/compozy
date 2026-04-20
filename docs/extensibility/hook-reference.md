# Hook Reference

This page mirrors the v1 hook taxonomy from protocol section 6.5.

## Using hooks in the SDK

You can register handlers in three ways:

- fluent helpers such as `onPromptPostBuild(...)`
- generic `handle(HOOKS.promptPostBuild, handler)`
- lifecycle methods `onEvent(...)`, `onHealthCheck(...)`, and `onShutdown(...)`

All hook payload and patch interfaces are exported from `@compozy/extension-sdk`.

## Plan phase

All `plan.*` hooks require `plan.mutate`.

| Event                           | TS helper                     | Payload type                                                 | Patch type                                              | Notes                                                                                    |
| ------------------------------- | ----------------------------- | ------------------------------------------------------------ | ------------------------------------------------------- | ---------------------------------------------------------------------------------------- |
| `plan.pre_discover`             | `onPlanPreDiscover`           | `PlanPreDiscoverPayload` `{run_id, workflow, mode}`          | `ExtraSourcesPatch` `{extra_sources?: string[]}`        | Adds extra discovery sources before issue discovery runs.                                |
| `plan.post_discover`            | `onPlanPostDiscover`          | `PlanPostDiscoverPayload` `{run_id, workflow, entries}`      | `EntriesPatch` `{entries?: IssueEntry[]}`               | Rewrites the discovered issue entry list.                                                |
| `plan.pre_group`                | `onPlanPreGroup`              | `PlanPreGroupPayload` `{run_id, entries}`                    | `EntriesPatch`                                          | Adjusts entries before grouping.                                                         |
| `plan.post_group`               | `onPlanPostGroup`             | `PlanPostGroupPayload` `{run_id, groups}`                    | `GroupsPatch` `{groups?: Record<string, IssueEntry[]>}` | Rewrites grouped issue batches.                                                          |
| `plan.pre_prepare_jobs`         | `onPlanPrePrepareJobs`        | `PlanPrePrepareJobsPayload` `{run_id, groups}`               | `GroupsPatch`                                           | Adjusts groups before job creation.                                                      |
| `plan.pre_resolve_task_runtime` | `onPlanPreResolveTaskRuntime` | `PlanPreResolveTaskRuntimePayload` `{run_id, task, runtime}` | `TaskRuntimePatch` `{runtime?: TaskRuntime}`            | PRD-only seam for selecting one task's effective runtime before job derivation finishes. |
| `plan.post_prepare_jobs`        | `onPlanPostPrepareJobs`       | `PlanPostPrepareJobsPayload` `{run_id, jobs}`                | `JobsPatch` `{jobs?: Job[]}`                            | Rewrites the prepared job list, but cannot mutate prepared job runtime fields.           |

## Prompt phase

All `prompt.*` hooks require `prompt.mutate`.

| Event               | TS helper           | Payload type                                                               | Patch type                                         | Notes                                                       |
| ------------------- | ------------------- | -------------------------------------------------------------------------- | -------------------------------------------------- | ----------------------------------------------------------- |
| `prompt.pre_build`  | `onPromptPreBuild`  | `PromptPreBuildPayload` `{run_id, job_id, batch_params}`                   | `BatchParamsPatch` `{batch_params?: BatchParams}`  | Mutates prompt build parameters before prompt construction. |
| `prompt.post_build` | `onPromptPostBuild` | `PromptPostBuildPayload` `{run_id, job_id, prompt_text, batch_params}`     | `PromptTextPatch` `{prompt_text?: string}`         | Most common prompt decorator hook.                          |
| `prompt.pre_system` | `onPromptPreSystem` | `PromptPreSystemPayload` `{run_id, job_id, system_addendum, batch_params}` | `SystemAddendumPatch` `{system_addendum?: string}` | Mutates the system addendum before session start.           |

## Agent phase

All `agent.*` hooks require `agent.mutate`.

| Event                       | TS helper                  | Payload type                                                             | Patch type                                                            | Notes                                            |
| --------------------------- | -------------------------- | ------------------------------------------------------------------------ | --------------------------------------------------------------------- | ------------------------------------------------ |
| `agent.pre_session_create`  | `onAgentPreSessionCreate`  | `AgentPreSessionCreatePayload` `{run_id, job_id, session_request}`       | `SessionRequestPatch` `{session_request?: SessionRequest}`            | Mutates the create-session request.              |
| `agent.post_session_create` | `onAgentPostSessionCreate` | `AgentPostSessionCreatePayload` `{run_id, job_id, session_id, identity}` | none                                                                  | Observe-only.                                    |
| `agent.pre_session_resume`  | `onAgentPreSessionResume`  | `AgentPreSessionResumePayload` `{run_id, job_id, resume_request}`        | `ResumeSessionRequestPatch` `{resume_request?: ResumeSessionRequest}` | Mutates the resume request.                      |
| `agent.on_session_update`   | `onAgentOnSessionUpdate`   | `AgentOnSessionUpdatePayload` `{run_id, job_id, session_id, update}`     | none                                                                  | Observe-only update tap on the synchronous path. |
| `agent.post_session_end`    | `onAgentPostSessionEnd`    | `AgentPostSessionEndPayload` `{run_id, job_id, session_id, outcome}`     | none                                                                  | Observe-only final session callback.             |

## Job and run phase

`job.*` hooks require `job.mutate`. `run.*` hooks require `run.mutate`.

| Event               | TS helper           | Payload type                                              | Patch type                                                    | Notes                                                                                                                         |
| ------------------- | ------------------- | --------------------------------------------------------- | ------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------- |
| `job.pre_execute`   | `onJobPreExecute`   | `JobPreExecutePayload` `{run_id, job}`                    | `JobPatch` `{job?: Job}`                                      | Rewrites one job before execution, but cannot mutate runtime fields chosen during planning.                                   |
| `job.post_execute`  | `onJobPostExecute`  | `JobPostExecutePayload` `{run_id, job, result}`           | none                                                          | Observe-only.                                                                                                                 |
| `job.pre_retry`     | `onJobPreRetry`     | `JobPreRetryPayload` `{run_id, job, attempt, last_error}` | `RetryDecisionPatch` `{proceed?: boolean, delay_ms?: number}` | Can veto or delay a retry.                                                                                                    |
| `run.pre_start`     | `onRunPreStart`     | `RunPreStartPayload` `{run_id, config, artifacts}`        | `RuntimeConfigPatch` `{config?: RuntimeConfig}`               | Rewrites late execution settings before execution begins, but cannot change workflow planning state already consumed earlier. |
| `run.post_start`    | `onRunPostStart`    | `RunPostStartPayload` `{run_id, config}`                  | none                                                          | Observe-only.                                                                                                                 |
| `run.pre_shutdown`  | `onRunPreShutdown`  | `RunPreShutdownPayload` `{run_id, reason}`                | none                                                          | Observe-only.                                                                                                                 |
| `run.post_shutdown` | `onRunPostShutdown` | `RunPostShutdownPayload` `{run_id, reason, summary}`      | none                                                          | Observe-only. Common lifecycle observer target.                                                                               |

## Review phase

All `review.*` hooks require `review.mutate`. They are only active under `compozy reviews fix`.

| Event                | TS helper            | Payload type                                                   | Patch type                                                     | Notes                               |
| -------------------- | -------------------- | -------------------------------------------------------------- | -------------------------------------------------------------- | ----------------------------------- |
| `review.pre_fetch`   | `onReviewPreFetch`   | `ReviewPreFetchPayload` `{run_id, pr, provider, fetch_config}` | `FetchConfigPatch` `{fetch_config?: FetchConfig}`              | Mutates review fetch configuration. |
| `review.post_fetch`  | `onReviewPostFetch`  | `ReviewPostFetchPayload` `{run_id, pr, issues}`                | `IssuesPatch` `{issues?: IssueEntry[]}`                        | Rewrites fetched review issues.     |
| `review.pre_batch`   | `onReviewPreBatch`   | `ReviewPreBatchPayload` `{run_id, pr, groups}`                 | `GroupsPatch`                                                  | Rewrites review issue batches.      |
| `review.post_fix`    | `onReviewPostFix`    | `ReviewPostFixPayload` `{run_id, pr, issue, outcome}`          | none                                                           | Observe-only.                       |
| `review.pre_resolve` | `onReviewPreResolve` | `ReviewPreResolvePayload` `{run_id, pr, issue, outcome}`       | `ResolveDecisionPatch` `{resolve?: boolean, message?: string}` | Can suppress remote resolution.     |

## Artifact phase

All `artifact.*` hooks require `artifacts.write`.

| Event                 | TS helper             | Payload type                                                | Patch type                                                                 | Notes                                                 |
| --------------------- | --------------------- | ----------------------------------------------------------- | -------------------------------------------------------------------------- | ----------------------------------------------------- |
| `artifact.pre_write`  | `onArtifactPreWrite`  | `ArtifactPreWritePayload` `{run_id, path, content_preview}` | `ArtifactWritePatch` `{path?: string, content?: string, cancel?: boolean}` | Can rewrite the path or content, or cancel the write. |
| `artifact.post_write` | `onArtifactPostWrite` | `ArtifactPostWritePayload` `{run_id, path, bytes_written}`  | none                                                                       | Observe-only.                                         |

## Event and lifecycle callbacks

These are not `execute_hook` events but they are part of the public author surface:

| Method         | Registration API             | Purpose                                                                       |
| -------------- | ---------------------------- | ----------------------------------------------------------------------------- |
| `on_event`     | `onEvent(handler, ...kinds)` | Receives bus events after initialize if the extension accepted `events.read`. |
| `health_check` | `onHealthCheck(handler)`     | Answers host health probes.                                                   |
| `shutdown`     | `onShutdown(handler)`        | Performs graceful cleanup before process exit.                                |

## Behavior rules worth remembering

- Mutable hooks are chained in priority order.
- Observe-only hooks are best-effort and concurrent.
- `{}` and `{"patch": {}}` are both treated as no-op responses.
- Arrays are replaced wholesale, not merged.
- Returning a patch from an observe-only hook is allowed for forward compatibility but ignored by the host.
- `plan.pre_resolve_task_runtime` is the only supported seam for extension-driven task runtime selection. Do not change runtime in `plan.post_prepare_jobs` or `job.pre_execute`.
- In workflow runs, `run.pre_start` is late in the pipeline. It can still tune output/runtime behavior such as timeout, retries, or sound settings, but it cannot rewrite fields already consumed by planning.
