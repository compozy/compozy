# Compozy Event Taxonomy

This document is the canonical public reference for the `pkg/compozy/events` envelope and the payloads under `pkg/compozy/events/kinds`.

All event payload fields use their JSON tag names below. Fields tagged with `omitempty` are omitted when their value is empty or zero.

## Envelope

Every line in `events.jsonl` is one `events.Event` object:

| Field            | Type                | Description                                                            |
| ---------------- | ------------------- | ---------------------------------------------------------------------- |
| `schema_version` | `string`            | Current schema version. The current public value is `1.0`.             |
| `run_id`         | `string`            | Stable identifier for the workflow or exec run that emitted the event. |
| `seq`            | `uint64`            | Monotonic sequence number within a run.                                |
| `ts`             | `RFC3339 timestamp` | Event timestamp in UTC.                                                |
| `kind`           | `string`            | One of the 82 public event kinds below.                                |
| `payload`        | `object`            | Kind-specific payload from `pkg/compozy/events/kinds`.                 |

## Run Events

### `run.queued`

Payload type: `kinds.RunQueuedPayload`

- `mode`: execution mode such as `prd-tasks`, `pr-review`, or `exec`
- `name`: workflow name when the run is workflow-backed
- `workspace_root`: resolved workspace root
- `ide`: configured ACP runtime id
- `model`: effective model name
- `reasoning_effort`: effective reasoning level
- `access_mode`: effective runtime access mode

### `run.started`

Payload type: `kinds.RunStartedPayload`

- `mode`
- `name`
- `workspace_root`
- `ide`
- `model`
- `reasoning_effort`
- `access_mode`
- `artifacts_dir`: run artifact directory under `~/.compozy/runs/<run-id>`
- `jobs_total`: number of prepared jobs

### `run.crashed`

Payload type: `kinds.RunCrashedPayload`

- `artifacts_dir`
- `duration_ms`
- `error`
- `result_path`

### `run.completed`

Payload type: `kinds.RunCompletedPayload`

- `artifacts_dir`
- `jobs_total`
- `jobs_succeeded`
- `jobs_failed`
- `jobs_canceled`
- `duration_ms`
- `result_path`: path to `result.json`
- `summary_message`

### `run.failed`

Payload type: `kinds.RunFailedPayload`

- `artifacts_dir`
- `duration_ms`
- `error`
- `result_path`

### `run.cancelled`

Payload type: `kinds.RunCancelledPayload`

- `reason`
- `requested_by`
- `duration_ms`

### `run.recovery_started`

Payload type: `kinds.RunRecoveryStartedPayload`

- `attempt`: recovery attempt number
- `strategy`: remediation strategy name
- `recovery_run_id`: run ID of the recovery agent execution

### `run.recovery_restarting`

Payload type: `kinds.RunRecoveryRestartingPayload`

- `failed_job_ids`: stable `SafeName` values selected for restart

### `run.recovered`

Payload type: `kinds.RunRecoveredPayload`

- `attempts`: number of recovery attempts used

### `run.recovery_exhausted`

Payload type: `kinds.RunRecoveryExhaustedPayload`

- `error`: recovery failure or rejection reason
- `result_path`: path to the terminal `result.json`

## Job Events

### `job.queued`

Payload type: `kinds.JobQueuedPayload`

- `index`: zero-based job index within the run
- `code_file`: primary code file for a single-file job
- `code_files`: grouped code files for a batch
- `issues`: number of issue entries represented by the job
- `task_title`: parsed PRD task title when available
- `task_type`: parsed PRD task type when available
- `safe_name`: artifact-safe job name
- `out_log`: stdout log path
- `err_log`: stderr log path

### `job.started`

Payload type: `kinds.JobStartedPayload`

- `index`
- `attempt`
- `max_attempts`

### `job.attempt_started`

Payload type: `kinds.JobAttemptStartedPayload`

- `index`
- `attempt`
- `max_attempts`

### `job.attempt_finished`

Payload type: `kinds.JobAttemptFinishedPayload`

- `index`
- `attempt`
- `max_attempts`
- `status`
- `exit_code`
- `retryable`
- `error`

### `job.retry_scheduled`

Payload type: `kinds.JobRetryScheduledPayload`

- `index`
- `attempt`
- `max_attempts`
- `reason`

### `job.stalled`

Payload type: `kinds.JobStalledPayload`

Emitted when the stall watchdog classifies the current attempt as frozen,
before the runtime decides whether to retry or park the job. It is followed by
`job.retry_scheduled` when a clean-state retry can proceed, or by `job.parked`
when recovery is exhausted or cannot proceed safely.

- `index`
- `attempt`
- `max_attempts`
- `reason`
- `last_tool_call`

### `job.parked`

Payload type: `kinds.JobParkedPayload`

Emitted when a stalled job cannot continue: its stall-retry budget is exhausted,
an extension declines or fails the retry, or the runtime cannot reset the
worktree safely. Parked is a terminal job state distinct from failed. The run
still exits non-zero, while the worktree and log are retained for triage.

- `index`
- `attempt`
- `max_attempts`
- `reason`
- `last_tool_call`
- `last_progress_seq`
- `worktree_path`
- `log_path`

### `job.pausing`

Payload type: `kinds.JobPausingPayload`

- `index`
- `attempt`
- `max_attempts`
- `session_id`

### `job.paused`

Payload type: `kinds.JobPausedPayload`

- `index`
- `attempt`
- `max_attempts`
- `session_id`

### `job.resumed`

Payload type: `kinds.JobResumedPayload`

- `index`
- `attempt`
- `max_attempts`
- `session_id`
- `message_id`

### `job.completed`

Payload type: `kinds.JobCompletedPayload`

- `index`
- `attempt`
- `max_attempts`
- `exit_code`
- `duration_ms`

### `job.failed`

Payload type: `kinds.JobFailedPayload`

- `index`
- `attempt`
- `max_attempts`
- `code_file`
- `exit_code`
- `out_log`
- `err_log`
- `error`

### `job.cancelled`

Payload type: `kinds.JobCancelledPayload`

- `index`
- `attempt`
- `max_attempts`
- `reason`

## Session Events

### `session.started`

Payload type: `kinds.SessionStartedPayload`

- `index`
- `acp_session_id`
- `agent_session_id`
- `resumed`

### `session.update`

Payload type: `kinds.SessionUpdatePayload`

- `index`
- `update`: `kinds.SessionUpdate`

`kinds.SessionUpdate` fields:

- `kind`: semantic update variant such as `agent_message_chunk`, `tool_call_started`, or `plan_updated`
- `tool_call_id`
- `tool_call_state`: one of `pending`, `in_progress`, `completed`, `failed`, `waiting_for_confirmation`
- `blocks`: content blocks rendered to the user
- `thought_blocks`: internal thought blocks when the runtime exposes them
- `plan_entries`: plan rows with `content`, `priority`, and `status`
- `available_commands`: slash-command style actions with `name`, `description`, and `argument_hint`
- `current_mode_id`
- `usage`: `kinds.Usage`
- `status`: session lifecycle status, typically `running`, `completed`, or `failed`

`blocks` and `thought_blocks` are `kinds.ContentBlock` values. Their `type` field determines the JSON payload shape:

- `text`: `text`
- `tool_use`: `id`, `name`, `title`, `tool_name`, `input`, `raw_input`
- `tool_result`: `tool_use_id`, `content`, `is_error`
- `diff`: `file_path`, `diff`, `old_text`, `new_text`
- `terminal_output`: `command`, `output`, `exit_code`, `terminal_id`
- `image`: `data`, `mime_type`, `uri`

### `session.completed`

Payload type: `kinds.SessionCompletedPayload`

- `index`
- `usage`: `kinds.Usage`

### `session.failed`

Payload type: `kinds.SessionFailedPayload`

- `index`
- `error`
- `usage`: `kinds.Usage`

## Reusable Agent Events

### `reusable_agent.lifecycle`

Payload type: `kinds.ReusableAgentLifecyclePayload`

- `stage`: one of `resolved`, `prompt-assembled`, `mcp-merged`, `nested-started`, `nested-completed`, or `nested-blocked`
- `agent_name`: resolved reusable-agent name for the stage being reported
- `agent_source`: source scope such as `workspace` or `global`
- `parent_agent_name`: parent reusable agent when the stage refers to a nested `run_agent` call
- `available_agents`: number of other reusable agents visible to the assembled discovery catalog
- `system_prompt_bytes`: byte size of the assembled reusable-agent system prompt
- `mcp_servers`: ordered MCP server names attached to the ACP session after reserved-server merge
- `resumed`: true when the reusable-agent lifecycle event belongs to a resumed ACP session
- `tool_call_id`: ACP tool-call id when the stage is tied to `run_agent`
- `nested_depth`: attempted child depth for nested execution
- `max_nested_depth`: configured host-owned depth ceiling
- `output_run_id`: nested child run id when the child run was started
- `success`: nested child completion status
- `blocked`: true when nested execution was blocked instead of run
- `blocked_reason`: one of `depth-limit`, `cycle-detected`, `access-denied`, `invalid-agent`, or `invalid-mcp`
- `error`: structured diagnostic text for blocked or failed nested runs

## Tool Call Events

### `tool_call.started`

Payload type: `kinds.ToolCallStartedPayload`

- `index`
- `tool_call_id`
- `name`
- `title`
- `tool_name`
- `input`
- `raw_input`

### `tool_call.updated`

Payload type: `kinds.ToolCallUpdatedPayload`

- `index`
- `tool_call_id`
- `state`
- `input`
- `raw_input`

### `tool_call.failed`

Payload type: `kinds.ToolCallFailedPayload`

- `index`
- `tool_call_id`
- `state`
- `error`

## Usage Events

### `usage.updated`

Payload type: `kinds.UsageUpdatedPayload`

- `index`
- `usage`: `kinds.Usage`

### `usage.aggregated`

Payload type: `kinds.UsageAggregatedPayload`

- `usage`: `kinds.Usage`

`kinds.Usage` fields:

- `input_tokens`
- `output_tokens`
- `total_tokens`
- `cache_reads`
- `cache_writes`

## Task Events

### `task.file_updated`

Payload type: `kinds.TaskFileUpdatedPayload`

- `tasks_dir`
- `task_name`
- `file_path`
- `old_status`
- `new_status`

### `task.file_skipped`

Payload type: `kinds.TaskFileSkippedPayload`

Emitted when an agent session ends cleanly but does not produce any
workspace changes. The task frontmatter is left at its prior status so the
runner will redispatch the same task on the next invocation. See issue #144.

- `tasks_dir`
- `task_name`
- `file_path`
- `preserved_status`
- `reason` (currently always `no_workspace_changes`)

### `task.metadata_refreshed`

Payload type: `kinds.TaskMetadataRefreshedPayload`

- `tasks_dir`
- `created_at`
- `updated_at`
- `total`
- `completed`
- `pending`

### `task.memory_updated`

Payload type: `kinds.TaskMemoryUpdatedPayload`

- `workflow`
- `task_file`
- `path`
- `mode`
- `bytes_written`

## Multi-Run Events

Multi-run events are emitted by the daemon-owned parent run created by
`compozy tasks run --multiple`. They are persisted in the parent run journal,
streamed through the regular run stream APIs, and reconstructed into the
`TaskRunMultipleSnapshot` returned by the multi-run snapshot endpoint. All nine
kinds share one payload type, `kinds.TaskRunMultiplePayload`.

`kinds.TaskRunMultiplePayload` fields:

- `run_id`: parent multi-run id
- `mode`: scheduling mode, `enqueued` or `parallel`
- `slug`: child workflow slug for item-scoped events
- `slugs`: ordered child workflow slugs, emitted on the started event
- `index`: zero-based child index within the queue
- `total`: total number of queued children
- `parallel_limit`: resolved concurrent-child cap, emitted on the started event in parallel mode
- `status`: item status, one of `queued`, `running`, `completed`, `failed`, or `canceled`
- `child_run_id`: child task run id once the child run exists
- `error`: actionable error text for failed or canceled items and the queue summary
- `worktree_path`: child git worktree path in parallel mode
- `base_branch`: parent branch the child worktree was created from
- `base_commit`: parent `HEAD` commit the child worktree was created from
- `worktree_status`: actual lifecycle state, one of `active`, `removed`, or `preserved`
- `worktree_reason`: why cleanup removed or preserved the tree
- `result_branch`: deterministic retained-output branch for parallel multi-spec children
- `completed`: children that finished successfully, emitted on the summary event
- `recovered`: children that stalled and then completed, emitted on the summary event
- `parked`: children parked for triage after a second stall, emitted on the summary event

`parallel_limit` and the `worktree_*` fields are additive and optional. They are
populated only for parallel-mode runs once a child worktree is planned, and they
stay empty for enqueued runs and for older parent events emitted before this
metadata existed. Snapshot reconstruction treats any empty field as unknown so
older event streams stay compatible.

`completed`, `recovered`, and `parked` are populated only on
`task.multi.summary`; every other kind leaves them at zero.

### `task.multi.started`

Payload type: `kinds.TaskRunMultiplePayload`

Parent queue lifecycle start. Carries `mode`, `slugs`, `total`, and
`parallel_limit` (parallel mode only).

### `task.multi.item_queued`

Payload type: `kinds.TaskRunMultiplePayload`

One ordered child item entered the queue. In parallel mode this is re-emitted
with `worktree_path`, `base_branch`, `base_commit`, and `worktree_status` before
the child launches so snapshots survive detach or daemon restart.

### `task.multi.child_started`

Payload type: `kinds.TaskRunMultiplePayload`

A child task run started. Carries `slug`, `child_run_id`, and `worktree_path`
when allocated.

### `task.multi.child_completed`

Payload type: `kinds.TaskRunMultiplePayload`

A child task run completed successfully.

### `task.multi.child_failed`

Payload type: `kinds.TaskRunMultiplePayload`

A child task run failed. Carries `error`; siblings keep running (fail-late).

### `task.multi.item_canceled`

Payload type: `kinds.TaskRunMultiplePayload`

A queued or running child item was canceled, typically by parent cancellation.

### `task.multi.queue_canceled`

Payload type: `kinds.TaskRunMultiplePayload`

The parent queue was canceled. Carries the aggregate summary `error` when present.

### `task.multi.queue_completed`

Payload type: `kinds.TaskRunMultiplePayload`

The parent queue settled successfully. Carries `total`.

### `task.multi.queue_failed`

Payload type: `kinds.TaskRunMultiplePayload`

The parent queue settled with one or more failed children. Carries `total` and
the aggregate failure summary in `error`.

### `task.multi.summary`

Payload type: `kinds.TaskRunMultiplePayload`

The end-of-run recovery summary for a parallel parent run, emitted once every
launched child has settled and before the parent reports its own outcome. It is
emitted whether the batch succeeded, failed, parked a child, or was canceled, so
the closing counts are always available.

Carries `total`, `completed`, `recovered`, and `parked`. A child counts as
`recovered` when it emitted `job.stalled` and then completed; a plain completion
counts as `completed` only. A child that emitted `job.parked` counts as `parked`
and never as `completed`.

## Parallel Task Events

Parallel task events are emitted by the `ParallelExecutionOrchestrator` during an
explicitly opted-in parallel PRD-tasks run (`--parallel-tasks`, the wizard, or a
per-run runtime override). Workspace `[tasks.run.parallel]` values configure an
opted-in run but do not activate worktrees by themselves. Events are persisted in the
parent run journal, streamed through the regular run stream APIs, and drive the
wave-grouped sidebar and the persistent `INTEGRATION` pane in the run TUI. All
parallel runs emit one plan event before execution starts. The plan event uses
`kinds.TaskParallelPlanPayload`; the remaining thirteen lifecycle kinds share
`kinds.TaskParallelPayload`.

`kinds.TaskParallelPlanPayload` fields:

- `run_id`: parent parallel-run id
- `workflow`: workflow slug for the task suite
- `integration_branch`: dedicated integration branch `compozy/parallel-<run-id>`
- `parallel_limit`: effective task concurrency limit
- `tasks`: complete planned task list from `_tasks.md`
- `waves`: deterministic topological waves derived from graph edges

`kinds.TaskParallelPlanTask` fields:

- `id`: task identity such as `task_01`
- `number`: numeric task number
- `title`: task title from the task file
- `file`: task artifact file, such as `task_01.md`
- `status`: task status when known
- `dependencies`: direct predecessor task ids from graph edges
- `wave_index`: zero-based planned wave index

`kinds.TaskParallelPlanWave` fields:

- `index`: zero-based planned wave index
- `task_ids`: task ids scheduled in the wave

`kinds.TaskParallelPayload` fields:

- `run_id`: parent parallel-run id
- `child_run_id`: child task run id emitted by `task_started`
- `wave_index`: zero-based topological wave the event belongs to
- `wave_total`: total number of waves in the run, emitted with `wave_started`
- `task_id`: PRD task identity such as `task_01`; empty for wave-level events
- `phase`: lifecycle phase, including `running`, `merging`, `resolving`,
  `advancing_base`, `finalizing`, `fast_forwarding`, `syncing_artifacts`,
  `cleaning_up`, `completed`, `canceled`, `failed`, and `rolled_back`
- `integration_branch`: dedicated integration branch `compozy/parallel-<run-id>`
- `conflict_files`: relative paths with unresolved conflicts during a squash merge
- `attempt`: current bounded conflict-resolution attempt
- `max_attempts`: configured conflict-resolution attempt ceiling
- `worktree_path`: per-task git worktree path
- `worktree_status`: actual lifecycle state, one of `active`, `removed`, or `preserved`
- `worktree_reason`: cleanup decision detail when available
- `result_branch`: retained output branch when the producer uses a named result branch
- `status`: terminal per-task status, one of `merged`, `recovered`, `failed`, `skipped`, or `canceled`
- `error`: terminal diagnostic for non-rollback parallel failures

Per-task events (`wave_started`, `task_started`, `conflict_detected`,
`conflict_resolving`, `merged`, `task_completed`) carry `task_id` and `wave_index` so the TUI can
assign each task card to its wave. `task_started` also carries `child_run_id` so
remote UIs can attach the real child run transcript to the selected task card.
Wave-level events (`wave_completed`, `merge_started`) leave `task_id` empty.
Empty fields are treated as unknown so older event streams stay compatible.

### `task.parallel.plan_started`

Payload type: `kinds.TaskParallelPlanPayload`

The full task DAG was validated and planned. Carries `workflow`,
`parallel_limit`, `integration_branch`, `tasks`, and `waves` so remote UIs can
render all task cards and pending waves before child task output exists.

### `task.parallel.wave_started`

Payload type: `kinds.TaskParallelPayload`

A task entered a running wave. Carries `wave_index`, `wave_total`, `task_id`,
`integration_branch`, `worktree_path`, and `phase` (`running`).

### `task.parallel.task_started`

Payload type: `kinds.TaskParallelPayload`

A task child run was created and can be observed. Carries `wave_index`,
`wave_total`, `task_id`, `child_run_id`, `worktree_path`,
`integration_branch`, and `phase` (`running`).

### `task.parallel.task_completed`

Payload type: `kinds.TaskParallelPayload`

The canonical, exactly-once per-task settlement. Carries `task_id`,
`wave_index`, terminal `status`, optional `error`, and the final
`worktree_status`/`worktree_reason`. `task.parallel.merged` remains available as
the compatibility signal that integration occurred.

### `task.parallel.wave_completed`

Payload type: `kinds.TaskParallelPayload`

A wave finished running and merging. Carries `wave_index` and `wave_total`.

### `task.parallel.merge_started`

Payload type: `kinds.TaskParallelPayload`

A wave began squash-merging its task worktrees into the integration branch.
Carries `wave_index` and `integration_branch`, with `phase` set to `merging`.

### `task.parallel.conflict_detected`

Payload type: `kinds.TaskParallelPayload`

A squash merge produced unmerged files. Carries `task_id`, `wave_index`,
`conflict_files`, `attempt`, and `max_attempts`.

### `task.parallel.conflict_resolving`

Payload type: `kinds.TaskParallelPayload`

The bounded agentic conflict resolver started an attempt. Carries `task_id`,
`wave_index`, `attempt`, `max_attempts`, and `phase` (`resolving`).

### `task.parallel.merged`

Payload type: `kinds.TaskParallelPayload`

A task was integrated into the integration branch. Carries `task_id`,
`wave_index`, `worktree_path`, and `status` (`merged` or `recovered`).

### `task.parallel.phase_changed`

Payload type: `kinds.TaskParallelPayload`

The orchestrator entered a post-wave or finalize phase. Carries `phase`,
`wave_index`, and `integration_branch`, allowing observers to distinguish base
advancement, finalization, fast-forward, artifact sync, and cleanup.

### `task.parallel.completed`

Payload type: `kinds.TaskParallelPayload`

The parallel-task orchestrator settled successfully. Carries `status`
(`completed`) and `phase` (`completed`). This settles the parallel subsystem; it
does not terminate the parent run stream.

### `task.parallel.failed`

Payload type: `kinds.TaskParallelPayload`

The run hit a blocking execution, integration, persistence, or sync failure.
Unsafe worktrees and unretained integration output are preserved for inspection.
Carries `integration_branch`,
`wave_index`, `status` (`failed`), `phase` (`failed`), and `error`.

### `task.parallel.rolled_back`

Payload type: `kinds.TaskParallelPayload`

The run exhausted conflict resolution and left the working branch untouched.
Cleanup removes only trees proven safe; dirty or unretained integration state is
preserved with an explicit reason. Carries `integration_branch`, `wave_index`,
and `phase` (`rolled_back`).

### `task.parallel.canceled`

Payload type: `kinds.TaskParallelPayload`

The parallel-task orchestrator settled after cancellation. Carries `status`
(`canceled`), `phase` (`canceled`), and the cancellation diagnostic in `error`.
This settles the parallel subsystem; the complete stream still ends on
`run.cancelled`.

## Multi/Parallel Terminal Ladder

Lifecycle settlement is deliberately layered:

1. `task.parallel.task_completed` settles one dependency-graph task.
2. `task.parallel.completed|failed|rolled_back|canceled` settles the within-spec
   parallel orchestrator.
3. `task.multi.queue_completed|queue_failed|queue_canceled` settles the
   daemon-owned multi-run queue.
4. Only `run.completed|run.failed|run.cancelled|run.crashed` terminates the
   complete run stream.

Queue and parallel settlements are durably persisted before live publication,
but consumers must keep watching until the corresponding `run.*` terminal.
Snapshots expose lifecycle events and a `next_cursor` from the same high-water
mark so attach clients apply each event once and resume strictly after it.

### Execution identity and intentional differences

| Execution kind        | User intent                                                         | Worktrees                              | Failure/recovery policy                                                                           |
| --------------------- | ------------------------------------------------------------------- | -------------------------------------- | ------------------------------------------------------------------------------------------------- |
| `task_standard`       | One workflow without explicit parallel-task opt-in                  | None                                   | Standard task runner                                                                              |
| `task_parallel`       | One workflow with explicit `--parallel-tasks`/wizard/runtime opt-in | Per-task plus integration              | Dependency waves; failed tasks block final fast-forward; bounded orchestrator recovery            |
| `task_multi_enqueued` | Multiple workflows in serial mode                                   | None                                   | Ordered, fail-fast queue                                                                          |
| `task_multi_parallel` | Multiple workflows in parallel mode                                 | One named-result worktree per workflow | Fail-late siblings; child-run recovery remains enabled; no automatic merge into the user's branch |

For multi-spec parallel runs, committed output remains on deterministic
`compozy/multi-*` result branches. Clean worktrees may be removed; dirty or
unretained trees remain `preserved` with `worktree_reason`.

## Artifact Events

### `artifact.updated`

Payload type: `kinds.ArtifactUpdatedPayload`

- `path`
- `bytes_written`

## Extension Events

### `extension.loaded`

Payload type: `kinds.ExtensionLoadedPayload`

- `extension`
- `source`
- `version`
- `manifest_path`

### `extension.ready`

Payload type: `kinds.ExtensionReadyPayload`

- `extension`
- `source`
- `version`
- `protocol_version`
- `accepted_capabilities`
- `supported_hook_events`

### `extension.failed`

Payload type: `kinds.ExtensionFailedPayload`

- `extension`
- `source`
- `version`
- `phase`
- `error`

### `extension.event`

Payload type: `kinds.ExtensionEventPayload`

- `extension`
- `kind`
- `payload`

## Review Events

### `review.status_finalized`

Payload type: `kinds.ReviewStatusFinalizedPayload`

- `reviews_dir`
- `issue_ids`

### `review.round_refreshed`

Payload type: `kinds.ReviewRoundRefreshedPayload`

- `reviews_dir`
- `provider`
- `pr`
- `round`
- `created_at`
- `total`
- `resolved`
- `unresolved`

### `review.issue_resolved`

Payload type: `kinds.ReviewIssueResolvedPayload`

- `reviews_dir`
- `issue_id`
- `file_path`
- `provider`
- `pr`
- `provider_ref`
- `provider_posted`
- `posted_at`

Review-watch events are emitted by the daemon-owned parent run created by `compozy reviews watch`. They are persisted
in the parent run journal, streamed through the regular run stream APIs, and use `kinds.ReviewWatchPayload`.

### `review.watch_started`

Payload type: `kinds.ReviewWatchPayload`

- `provider`
- `pr`
- `workflow`
- `run_id`
- `head_sha`
- `remote`
- `branch`
- `dirty`
- `unpushed_commits`

### `review.watch_waiting`

Payload type: `kinds.ReviewWatchPayload`

- `provider`
- `pr`
- `workflow`
- `run_id`
- `head_sha`
- `status`
- `review_id`
- `review_state`

### `review.watch_round_fetched`

Payload type: `kinds.ReviewWatchPayload`

- `provider`
- `pr`
- `workflow`
- `round`
- `run_id`
- `head_sha`
- `total`
- `resolved`
- `unresolved`

### `review.watch_fix_started`

Payload type: `kinds.ReviewWatchPayload`

- `provider`
- `pr`
- `workflow`
- `round`
- `run_id`
- `child_run_id`
- `head_sha`

### `review.watch_fix_completed`

Payload type: `kinds.ReviewWatchPayload`

- `provider`
- `pr`
- `workflow`
- `round`
- `run_id`
- `child_run_id`
- `head_sha`
- `status`
- `error`

### `review.watch_push_started`

Payload type: `kinds.ReviewWatchPayload`

- `provider`
- `pr`
- `workflow`
- `round`
- `run_id`
- `head_sha`
- `remote`
- `branch`

### `review.watch_push_completed`

Payload type: `kinds.ReviewWatchPayload`

- `provider`
- `pr`
- `workflow`
- `round`
- `run_id`
- `head_sha`
- `remote`
- `branch`

### `review.watch_push_failed`

Payload type: `kinds.ReviewWatchPayload`

- `provider`
- `pr`
- `workflow`
- `round`
- `run_id`
- `head_sha`
- `remote`
- `branch`
- `error`

### `review.watch_clean`

Payload type: `kinds.ReviewWatchPayload`

- `provider`
- `pr`
- `workflow`
- `round`
- `run_id`
- `head_sha`
- `review_id`
- `review_state`
- `status`

### `review.watch_max_rounds`

Payload type: `kinds.ReviewWatchPayload`

- `provider`
- `pr`
- `workflow`
- `round`
- `run_id`
- `head_sha`
- `status`

## Provider Events

### `provider.call_started`

Payload type: `kinds.ProviderCallStartedPayload`

- `call_id`
- `provider`
- `endpoint`
- `method`
- `pr`
- `issue_count`

### `provider.call_completed`

Payload type: `kinds.ProviderCallCompletedPayload`

- `call_id`
- `provider`
- `endpoint`
- `method`
- `status_code`
- `duration_ms`
- `payload_bytes`

### `provider.call_failed`

Payload type: `kinds.ProviderCallFailedPayload`

- `call_id`
- `provider`
- `endpoint`
- `method`
- `status_code`
- `duration_ms`
- `payload_bytes`
- `error`

## Shutdown Events

### `shutdown.requested`

Payload type: `kinds.ShutdownRequestedPayload`

- `source`
- `requested_at`
- `deadline_at`

### `shutdown.draining`

Payload type: `kinds.ShutdownDrainingPayload`

- `source`
- `requested_at`
- `deadline_at`

### `shutdown.terminated`

Payload type: `kinds.ShutdownTerminatedPayload`

- `source`
- `requested_at`
- `deadline_at`
- `forced`

## Event Streaming (CLI)

Both the `exec` and workflow commands support real-time event streaming to stdout via the `--format` flag. When enabled, events are written as newline-delimited JSON (JSONL) to stdout.

### Output formats

| Flag value | Mode    | Description                                                             |
| ---------- | ------- | ----------------------------------------------------------------------- |
| `text`     | default | Human-readable TUI output. No event streaming.                          |
| `json`     | lean    | Emits a filtered subset of high-signal events as compact JSONL objects. |
| `raw-json` | raw     | Emits every bus event as its full `events.Event` envelope.              |

### Lean mode (`--format json`)

Lean mode streams only lifecycle and interactive events to keep output concise for CI pipelines and automation:

**Included event kinds:**

- `run.started`, `run.completed`, `run.failed`, `run.cancelled`, `run.crashed`
- all `task.multi.*` queue/item lifecycle events
- all `task.parallel.*` plan, task, phase, wave, integration, and settlement events
- `job.started`, `job.retry_scheduled`, `job.pausing`, `job.paused`, `job.resumed`, `job.completed`, `job.failed`, `job.cancelled`
- `session.started`, `session.completed`, `session.failed`
- `session.update` — only when the update kind is `user_message_chunk`, `agent_message_chunk`, `tool_call_started`, or `tool_call_updated`

**Lean JSONL shape:**

```json
{"type":"run.started","run_id":"abc123","seq":1,"time":"2026-04-13T10:00:00Z","payload":{...}}
```

| Field     | Type      | Description               |
| --------- | --------- | ------------------------- |
| `type`    | `string`  | Event kind                |
| `run_id`  | `string`  | Run identifier            |
| `seq`     | `uint64`  | Monotonic sequence number |
| `time`    | `RFC3339` | Event timestamp           |
| `payload` | `object`  | Kind-specific payload     |

### Raw mode (`--format raw-json`)

Raw mode streams the full `events.Event` envelope for every bus event, including internal events not shown in lean mode. The shape matches the envelope documented in the [Envelope](#envelope) section above.

### Examples

```bash
# Stream lean events for a single-prompt exec run
compozy exec --format json "Refactor the auth middleware"

# Stream all raw events for a daemon-backed review-fix workflow
compozy reviews fix my-feature --format raw-json

# Pipe lean events to jq for filtering
compozy exec --format json "Fix the tests" | jq 'select(.type == "session.update")'
```

### Terminal event detection

The streamer waits for a run terminal event (`run.completed`, `run.failed`,
`run.cancelled`, `run.crashed`, or `run.recovery_exhausted`) before finalizing.
Queue and parallel settlements never close the complete stream. Remote watchers
resume from the last cursor after EOF/overflow and resnapshot when integrity or
heartbeat gaps make incremental continuation unsafe.
