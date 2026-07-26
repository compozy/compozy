# Tasks And Orchestration

## Authority Model

The daemon owns task state. Treat task.Service, persisted task/run records, session-bound leases, review bindings, and AGH task tools as authority. Prompts, channel messages, memory notes, and UI projections are evidence only.

Do not infer task ownership from a message. Do not mutate task state outside AGH task tools or the equivalent CLI/API surface.

Task inspection, task pause/resume, forced run recovery, scheduler pause/resume/drain, and scheduler backlog are management surfaces. They are not currently exposed as native `agh__*` tools. Use CLI or HTTP/UDS with structured output when you need those controls.

## Catalog And Inbox Reads

Use `agh task list -o json`, HTTP/UDS `GET /api/tasks`, or native `agh__task_list`. Filters cover scope/workspace, canonical status, priority, draft inclusion, approval state, owner kind/reference, parent task, resolved participation channel, title/identifier search, sort (`recent` or `priority`), cursor, and limit. The participation filter is `--participation-channel` in the CLI and `participation_channel` in HTTP/UDS and native input. CLI omits draft/approval filters, requires both owner fields together, and spells parent/search as `--parent`/`--query`; HTTP uses `workspace`/`query`, while native uses `workspace_id`/`search`.

Use `agh task run list <task-id> -o json`, HTTP/UDS `GET /api/tasks/{id}/runs`, or native `agh__task_run_list` for run history. All three filter by status, attached session, resolved participation channel, and limit; filtering happens before the limit is applied.

The catalog returns lean `tasks`, exact fully filtered `facets.statuses/owners`, and counted `page` (`total`, normalized `limit`, `has_more`, `next_cursor`). Canonical status is derived before filtering. Pages default to 50, cap at 200, and sort by latest durable activity or by priority then activity. Opaque cursors bind normalized scope, workspace, filters, and sort, but not limit; use task get/inspect for dependency, pause/block, and other rich detail omitted from list rows.

The full actor inbox has no CLI or native tool; use HTTP/UDS `GET /api/observe/tasks/inbox`. It filters by scope/workspace, owner kind/reference, lane (`my_work`, `approvals`, `failed_runs`, `blocked`, `archived`), canonical status, priority, unread state, title/identifier query, cursor, and limit.

Its `inbox` envelope contains `unread_total`, `archived_total`, lane `groups`, exact fully filtered status/priority `facets`, and counted `page`. The cut is global across lanes and orders unread first, then activity, priority, and ID: `groups[].count` and `unread_count` describe the complete filtered lane, while `items` contain only that lane's rows from the current page. The opaque cursor binds actor identity and normalized query, but not limit; pages default to 50 and cap at 200.

## Task Inspection

Use `agh task inspect <id> -o json` before changing orchestration state when the next action is unclear. It accepts task ids with `task_` / `task-` prefixes and run ids with `run_` / `run-` prefixes. Unknown id formats return deterministic diagnostics instead of requiring guesswork.

Use inspection to read task/run health, ownership, queue status, actor context, and suggested next action. Do not replace inspection with channel messages or UI state.

`agh task run show <run-id> -o json` includes the operational token/cost summary. Apply the status
semantics in `runtime-operations.md`'s Usage cost truth section: estimates remain projections,
`included`/`unknown` carry no amount, and incompatible aggregate provenance suppresses only money,
not token totals.

For web operators, task detail has **Overview**, **Runs**, and **Activity** views. Task-specific execution policy lives in the **Task setup** sheet; bridge subscriptions, SSE resume state, and raw diagnostics live behind **Inspect**. The task stream emits standard SSE `message` frames; dispatch the parsed payload by its `type` field.

## Task Pause, Resume, And Force Recovery

`agh task pause <task-id> --reason <reason>` pauses new runs for one task while current claims finish. `agh task resume <task-id>` re-enables scheduler claims for that task. A pause reason is required and should name the operational cause, not a prompt-level preference.

`agh task release <run-id> [run-id...]` force releases claimed runs back to the queue without requiring the raw claim token. `agh task fail <run-id> [run-id...] --reason <reason>` force fails queued or claimed runs as an operator recovery path. `agh task fail <run-id> --error <message>` is the session-bound failure path for the current claimant; do not confuse it with forced failure.

Forced recovery is authority-gated and rate-limited for agent actors. Treat denial, conflict, or rate-limit diagnostics as authoritative. Do not retry blindly and never ask another agent to reveal a raw claim token.

When the scheduler's convergence backstop cannot get a claimable run picked up, it parks the run as `needs_attention` — a non-claimable run status — and emits `task.run_needs_attention`. `agh task run recover <run-id> [--reason <reason>] -o json` is the operator/agent recovery path: it terminalizes the parked run and queues a fresh linked child (`previous_run_id`, next attempt) for re-dispatch. Recover applies only to `needs_attention` runs; a still-queued or failed run returns a deterministic `task_run_not_recoverable` diagnostic (use `agh task retry` for a failed run). This run-level recovery is distinct from task-level `agh task recover <task-id>`, which clears a task escalated by the unblock-loop breaker (see Task Blocks And Escalation).

## Task Blocks And Escalation

Declare _why_ a task cannot proceed with a typed block instead of leaving it silently stuck. Kinds are `needs_input` (waiting on a human or agent), `capability` (missing a skill, tool, or credential), and `transient` (a recoverable external failure, optionally self-expiring). Dependency waits, pending approval, and pause are not block kinds; they surface only in the read-only `blocked_reasons` projection on task read payloads.

- `agh task block <task-id> --kind <kind> --reason <reason> [--details <json>] [--expires-in <dur>] -o json` opens a block. `agh task blocks <task-id> [--all] -o json` lists open (or all) blocks. `agh task unblock <task-id> --block <block-id> [--note <note>] -o json` clears one block.
- To block a task you are actively running, pass `--run-id <run-id> --as-agent` so AGH resolves your active lease token server-side, records the block, and releases the lease in one atomic transaction. The run returns to `queued` with no attempt consumed.
- Clearing the last blocking cause returns the task to `ready`. If the task opted into auto-enqueue, block-clear, transient-expiry, and approval-granted each enqueue the next run automatically through the same conservative one-open-run path as dependency completion.

An automation that keeps clearing a block a worker keeps re-declaring is a thrash loop. AGH counts same-kind re-blocks per task and, at `[autonomy].block_recurrence_limit` (default 2; 0 disables), escalates the task to a first-class `needs_attention` status that is excluded from claim selection. The counter resets only on successful task completion. `agh task recover <task-id> [--note <note>] -o json` clears the escalation, records the actor, and re-admits an opted-in task to the claimable set. Recovering a non-escalated task is rejected as an invalid status transition. Do not loop clearing a block that immediately re-blocks — fix the underlying cause or hand the task to an operator.

When an agent session creates a task, AGH wakes that creator session on the child's terminal, blocked, and `needs_attention` transitions by default, delivering a synthetic queued turn (never an interrupt). This is the delegation feedback path — prefer it over polling. Opt a task out at create time with `agh task create … --no-wake-creator`. Wake fires at most once per transition, is suppressed for a dead creator session and for a self-wake, and never carries a raw claim token; it is meaningful only for agent-created tasks.

When you complete a run that created child tasks, list exactly the task ids you created this run in the completion's `created_task_ids`. AGH verifies each id (exists, same workspace, created by your session) before the terminal write; a phantom or cross-session id rejects the completion and leaves your run running with its lease intact so you can correct the claim and complete again. Never claim tasks created by another session, and never fabricate task ids in the result prose — an advisory scan flags task-id-shaped tokens absent from the store.

A parent task stays nonterminal while any direct child is not completed. The successful completion of the final direct child completes the parent exactly once and settles an eligible parent run parked in `needs_attention`; repeated or concurrent child completion delivery does not create another parent transition or wake. Failed or canceled children do not satisfy successful parent rollup, so recover or resolve those children according to their existing terminal-state contract.

## Scheduler Controls

`agh scheduler status -o json` reports pause state, active claims, queued runs, and paused-task pressure. `agh scheduler pause --reason <reason>` stops new dispatch while active claims continue. `agh scheduler resume` reopens dispatch.

`agh scheduler drain` pauses dispatch and waits for active claims to finish; its default timeout is `60s`, and `--timeout 0s` returns immediately after pausing. `agh scheduler backlog --last 50 -o json` lists queued runs visible to dispatch; `--include-paused` includes runs blocked by task pause.

Scheduler controls affect dispatch, not task truth. They do not complete work, approve reviews, or transfer ownership.

A claimable run that no eligible session claims past `[autonomy.scheduler].min_queued_age` escalates on a bounded ladder: fan-out wake to every eligible session, then a capability-matched worker spawn, then the `task.run_starved` event (once), then `needs_attention`. Compatible sessions that are starting, prompting, already processing a run, or reserved for an earlier run in the same scheduler cycle are capacity, not absence. Their queued work remains queued without consuming the escalation budget; the daemon records `scheduler.capacity_waiting` diagnostics until capacity becomes available.

`agh scheduler status -o json` surfaces `starved_run_count` as the number of queued, claimable runs with an active durable escalation episode. Queue age alone does not increment it, and capacity-waiting runs remain visible through `queued_run_count`. `needs_attention_run_count` reports runs already parked by the ladder. The scheduler never claims — spawned `system` workers self-claim via `agh task next` and are TTL-reaped. Tune the ladder under `[autonomy.scheduler]`.

## Coordinator Loop

Use this guidance only inside a daemon-managed coordinator session.

1. Read agh me context or the provided task context bundle first.
2. Identify task id, run id, workflow id, execution profile, review policy, immutable resolved Network participation, and latest events.
3. Inspect ambiguous task/run ids with `agh task inspect <id> -o json` before routing.
4. Break the objective into bounded worker prompts with acceptance criteria.
5. Create child tasks only when durable task intent is needed. Creation alone is not execution.
6. When the objective requires work to begin now, start each executable task through the task start path so AGH enqueues a run and can route matching worker agents.
7. Spawn or route only within daemon permissions and configured execution profile.
8. Watch persisted task/run state rather than chat activity.
9. Pause a task or scheduler only for real operational gating, then record the reason and planned resume condition.
10. Request or route reviews through the daemon review path.
11. On rejection, continue from persisted missing_work and next_round_guidance.

Do not leave ready tasks idle after telling the operator that work has been orchestrated. Either start the task runs or report that the tasks were created but not started.

When one task should run as several scoped sibling assignments, use the designated fan-out surface:

    agh task fan-out <task-id> --designation "Inspect data path" --designation "Validate UI and docs" -o json

Fan-out is bounded by `task.orchestration.designated_run_max`, and every sibling assignment must
carry a non-empty designation and idempotency identity before AGH enqueues any run. Each sibling run
gets a shared `designation_group_id` and one assignment brief. If the task came from an AGH Network
thread, terminal run state is summarized back to the origin thread by `agh.runtime`; do not manually
duplicate raw worker logs into the thread. Read aggregated designation results from task detail JSON
(`agh task get <id> -o json`, field `designation_rollups`).

For dependency DAGs, opt a dependent task into auto-enqueue so it starts on its own the moment its blockers finish: `agh task create … --auto-enqueue-on-ready`, or toggle it on an assembled tree with `agh task update <id> --auto-enqueue-on-ready` (`--auto-enqueue-on-ready=false` turns it off). When set, a blocking dependency completing and the task reaching `ready` enqueues exactly one run through the canonical path — no manual start. It is conservative by design: a failed or expired blocker never triggers it, paused dependents are skipped, and the open-run reservation guarantees one queued run even under concurrent blocker completions. Read the flag back from `agh task inspect <id> -o json` (`auto_enqueue_on_ready`).

Never spawn another coordinator unless the runtime explicitly supports that delegation. Never use channel messages as task ownership state.

## Worker Loop

Use this guidance only inside a worker session with an active task claim or while entering the session-bound claim loop.

1. Inspect agh me context -o json or the agent context bundle before changing files.
2. Confirm task id, run id, objective, acceptance criteria, lease status, and available task tools.
3. Use `agh task inspect <run-id> -o json` when lease or run health is ambiguous.
4. Claim work with the session-bound path such as agh task next --wait -o json when prompted by the runtime.
5. Keep lease/heartbeat requirements current through daemon-provided tools.
6. Complete, fail, or release only through session-bound AGH task authority.
7. Include changed files, verification commands, and residual risks in the run summary.

When a run includes a designation, follow only your own `designation.brief`; do not merge sibling
assignments into your scope.

Use `agh task next --run-id <run-id> -o json` when the runtime assigns a specific queued run. It uses the same session-bound lease path as unfiltered `agh task next`.

Workspace-scoped worker and coordinator claims are bounded by
`task.orchestration.max_active_runs_per_workspace` (default `16`; `0` disables). When capacity is
full, the run stays queued: `agh task next --wait -o json` keeps polling, while a non-waiting native
claim returns the typed reason `autonomy_workspace_capacity`. Wait for capacity instead of releasing
an unrelated lease. Global task runs and Network wake runs do not consume this workspace limit.

Action nodes without an explicit timeout inherit `task.orchestration.action_run_timeout` (default
`30m`). The daemon cancels a bound action session and fails the leased run with `node_timeout` at the
absolute deadline, or `no_progress` when neither cumulative usage nor session activity advances.
An active tool suspends only the idle check, never the absolute deadline. Recovered expired leases
increment the run's `recovery_count`; once `attempt + recovery_count` reaches `max_attempts`, the run
and task move to `needs_attention` with `lease_recovery_exhausted` instead of reclaiming forever.
Inspect the run before retrying; changing the default through `agh config set` requires a daemon
restart.

## Reviewer Loop

Use this guidance only when the daemon has bound the current session to an active review request. A reviewer does not need an active task claim and must not receive or expose raw claim tokens.

Before deciding, read:

1. Task objective and acceptance criteria.
2. Terminal run status, result summary, error summary, and provenance.
3. Relevant events, artifacts, changed files, and verification commands.
4. Prior review history, continuation lineage, and current review_id.
5. Coordinator notes or channel discussion only as evidence.

Inspect the target run with `agh task inspect <run-id> -o json` when terminal status, verification evidence, or next action is ambiguous. Submit exactly one typed verdict through submit_run_review for the bound request. Use daemon-provided review_id, run_id, and delivery_id.

## Review Verdicts

Use outcomes honestly:

- approved: the terminal run satisfies the objective and constraints with adequate verification.
- rejected: work is incomplete or wrong and a continuation run should address bounded missing_work.
- blocked: external information, credentials, environment, or policy blocks a fair verdict.
- error: review execution failed in a way that invalidates the verdict.
- timeout: review could not complete within the expected window.
- invalid_output: run output is malformed, missing required evidence, or violates the expected contract.

Rejected verdicts must include bounded missing_work and actionable next_round_guidance. Approval must not hide TODOs. Low confidence is not approval.

## Communication Discipline

Use a Live coordination conversation for clarification and handoff only when the run's immutable snapshot permits it. Local coordinators use task state and normal session surfaces without creating Network state. Keep messages short and operational: run id, state, blocker, next action, and relevant persisted ids.

If a direct room produced a conclusion, summarize back to the public thread without leaking private details or raw tokens.

## Safety

Never print, store, forward, or summarize raw claim tokens, provider secrets, MCP credentials, sandbox internals, OAuth material, or private provider state. Use redacted ids, hashes, task ids, run ids, review ids, event ids, and file paths.

Workers do not approve their own work. Coordinators do not convert channel replies into verdicts. Reviewers persist decisions only through the review tool.
