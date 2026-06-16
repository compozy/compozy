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
| `kind`           | `string`            | One of the 60 public event kinds below.                                |
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
`TaskRunMultipleSnapshot` returned by the multi-run snapshot endpoint. All eight
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
- `worktree_status`: worktree preservation status, currently always `preserved`

`parallel_limit` and the `worktree_*` fields are additive and optional. They are
populated only for parallel-mode runs once a child worktree is planned, and they
stay empty for enqueued runs and for older parent events emitted before this
metadata existed. Snapshot reconstruction treats any empty field as unknown so
older event streams stay compatible.

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

The parent queue settled. Carries `total` and, on aggregate failure, the summary `error`.

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

- `run.started`, `run.completed`, `run.failed`, `run.cancelled`
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

The streamer waits for a terminal event (`run.completed`, `run.failed`, or `run.cancelled`) before finalizing. If no terminal event arrives within 5 seconds after the bus closes, the streamer exits gracefully.
