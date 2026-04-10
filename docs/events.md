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
| `kind`           | `string`            | One of the 37 public event kinds below.                                |
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
- `artifacts_dir`: run artifact directory under `.compozy/runs/<run-id>`
- `jobs_total`: number of prepared jobs

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

## Artifact Events

### `artifact.updated`

Payload type: `kinds.ArtifactUpdatedPayload`

- `path`
- `bytes_written`

## Extension Events

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
