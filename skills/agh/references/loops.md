# Loops

Agent operation guidance for AGH Loops — the deterministic goal → verify → stop programs the daemon
owns and runs. Use this reference when you author, configure, run, observe, approve, or stop a Loop
from inside AGH. Prefer the native `agh__loop_*` tools; fall back to `agh loop` CLI or HTTP with
structured output. Never guess a schema — resolve `agh__tool_info` for the exact descriptor first.

## The Tool Set And CLI Verbs

Toolset `agh__loops` — 16 native tools. Thirteen definition/run controls have matching `agh loop`
verbs; `agh__loop_turns` maps to `agh loop turns`; the two session-bound Goal tools use the session
command/native report surfaces. The CLI adds one verb (`edit`) that has no native tool.

| Native tool           | Mode                            | CLI                  | Purpose                                                                      |
| --------------------- | ------------------------------- | -------------------- | ---------------------------------------------------------------------------- |
| `agh__loop_list`      | read                            | `agh loop list`      | List Loop definitions in the workspace.                                      |
| `agh__loop_inspect`   | read                            | `agh loop inspect`   | Read one definition: inputs, contract, start bindings, version.              |
| `agh__loop_validate`  | read                            | `agh loop validate`  | Lint + compile a definition without saving.                                  |
| `agh__loop_status`    | read                            | `agh loop status`    | Read one run's status with generation detail.                                |
| `agh__loop_runs`      | read                            | `agh loop runs`      | List runs in the workspace.                                                  |
| `agh__loop_create`    | mutating                        | `agh loop create`    | Create/fork, or CAS-publish when `expected_version` is set.                  |
| `agh__loop_run`       | mutating                        | `agh loop run`       | Start a run, or dry-run with `dry: true` / `--dry-run`.                      |
| `agh__loop_configure` | mutating                        | `agh loop configure` | Write per-Loop runtime config overrides.                                     |
| `agh__loop_pause`     | mutating                        | `agh loop pause`     | Request a generation-boundary pause.                                         |
| `agh__loop_resume`    | mutating                        | `agh loop resume`    | Resume a paused or pause-requested run.                                      |
| `agh__loop_approve`   | mutating · **capability-gated** | `agh loop approve`   | Apply one human-gate decision.                                               |
| `agh__loop_stop`      | destructive                     | `agh loop stop`      | Stop one active run.                                                         |
| `agh__loop_delete`    | destructive                     | `agh loop delete`    | Delete a writable workspace definition.                                      |
| `agh__goal_get`       | read · session-scoped           | `/goal status`       | Read the caller session's visible Goal, including terminal-until-clear.      |
| `agh__goal_report`    | mutating · prompt-scoped        | —                    | Record one current-prompt `complete` or evidenced `blocked` boundary intent. |
| `agh__loop_turns`     | read                            | `agh loop turns`     | Read a Run's total-order Goal turn audit with cursor and node/item filters.  |

There is **no `agh__loop_edit` native tool**. Agents edit a definition through the authoring loop
(validate → dry-run → `agh__loop_create` with `expected_version`) or by a filesystem write. The CLI
`agh loop edit` is a `$EDITOR` convenience for operators and publishes through the same
compare-and-swap path.

## Catalog Reads

Use `agh loop list --workspace <ref> -o json`, HTTP/UDS `GET /api/workspaces/{workspace_id}/loops`, or native `agh__loop_list`. Filters are name/contract-goal search (`--query` in CLI, `q` elsewhere), `kind` (`read_only` or `workspace`), exact category, exact latest-run status, name sort, cursor, and limit.

The response is `loops`, exact self-filtered `facets` (`kinds`, `categories`, `statuses`), and counted `page` (`total`, normalized `limit`, `has_more`, `next_cursor`). Self-filtered means each facet omits its own active filter while respecting search and every other filter. Pages default to 50 and cap at 200.

Opaque cursors bind workspace, search, kind, category, status, and sort; limit may change. Stable order is read-only before workspace, then normalized name and ID. AGH computes the cut from lean records and loads definition YAML only for selected rows. `last_run` is the all-time latest run; only `aggregate_30d` and `success_rate_30d` use the 30-day window.

`agh loop runs` / `agh__loop_runs` is a different, non-cursor contract: it returns `runs` plus aggregates, defaults to 100 rows, caps at 500, and does not expose `has_more` or `next_cursor`.

## The Authoring Loop

Follow **inspect → validate → dry-run → publish (with `expected_version`) → run**. Every step before
`run` is structured and spends no tokens.

1. **inspect** — `agh__loop_inspect` returns the definition and its current `version`. Read the
   version before you change anything.
2. **validate** — `agh__loop_validate` lints and compiles a candidate without saving; it returns
   per-node codes (`unknown_reference`, `node_id_invalid`, `verdict_policy_requires_judge`,
   `fan_out_ceiling_exceeded`).
3. **dry-run** — `agh__loop_run` with `dry: true` resolves inputs and returns the first generation's
   plan without creating a run or spending budget.
4. **publish** — `agh__loop_create` with `expected_version` set to the version from step one (or
   HTTP `PATCH /loops/:name`). This is compare-and-swap: a stale version is rejected `409` with the
   current version. Use PATCH/create-with-version for **all** programmatic editing — the filesystem
   write path is last-write-wins and unsafe for concurrent agents.
5. **run** — `agh__loop_run`. Only now does the Loop spend tokens.

New Loops start as a fork (`agh__loop_create` with `fork_from_name`); there is no blank-canvas
authoring. Read-only sources — including the default `dev-cycle` Loops — must be forked before you
adapt them.

## Goal Nodes And Session Commands

A Goal is the reserved action `kind: goal`. Its `params` require `agent`, non-empty `objective`, at
least one supported `judge`, positive `max_turns`, and an `output_schema` whose `status` enum can
represent `blocked`. `on_exhausted` is `halt` (default) or `escalate`. Goal v1 judges are `command`
(`check` required), `agent-judge` (rubric/prompt required), or `extension` (tool required); `human`
is rejected.

Missing `session` compiles to `mode: continuous`. An isolated session is the other valid strategy;
the two cannot be combined. Operational retry uses `retry.max_attempts`, which counts total
pre-submission attempts including the first. `on_failure: fresh_session` requires continuous mode
and at least two attempts. It applies only when AGH proves the prompt effect never started. AGH
never replays a prompt after durable start; recovery continuation is a new turn.

Authenticated Web/HTTP/UDS/CLI session prompt ingress recognizes this closed grammar:

| Command                                       | Effect                                                                                        |
| --------------------------------------------- | --------------------------------------------------------------------------------------------- |
| `/goal <objective>`                           | Start one session-origin Goal; an existing Goal returns `goal_replace_required`.              |
| `/goal replace <expected-run-id> <objective>` | Compare-and-swap replacement; stale identity returns `goal_replace_stale` without mutation.   |
| `/goal status`                                | Return the newest visible Goal snapshot.                                                      |
| `/goal pause`                                 | Persist an actor-aware pause and settle at a safe boundary.                                   |
| `/goal resume`                                | Resume paused work or approve the active synthetic Goal gate.                                 |
| `/goal clear`                                 | Revoke live work when needed, then hide the newest projection without deleting its audit.     |
| `/goal draft <text>`                          | Run one idle-only ordinary streaming turn that proposes objective/clauses without activation. |

Internal, automation, network, extension, and synthetic prompts treat `/goal` text literally.
Draft never queues, steers, interrupts, or consumes a Goal turn; busy admission returns
`goal_draft_requires_idle`. Lowercase line-oriented `verify:` and `constraints:` clauses become the
synthetic agent-judge rubric. `verify:` text is never executed as a command.

Use the current snapshot `run_id` for replacement. If `goal_replace_stale` returns a newer snapshot,
review it before constructing another command. Terminal `blocked` is not resumable; replace with the
expected current Run ID or clear it.

Goal context is `known`, `unknown`, or `pending`. `known` carries trustworthy reported usage;
`unknown` has no percentage; `pending` waits for a strictly newer report after compaction. A
session-origin Goal requires explicit approval before recovery reseeds into a new bound session.
The origin session remains the Goal owner; use the new `bound_session_id` for ordinary messages.
Pause/Resume, approval, and reseed each allocate at most one successor control epoch.

The checkpoint-local approval scopes are narrow: turn exhaustion grants
`turn-extension/turn-limit`; budget crossed after work grants `budget/settle-current` and cannot
start new work; budget crossed before work grants `budget/work-and-settle` for one candidate turn
plus closure; reseed grants `reseed/rotate-binding`; pause grants `plain-resume/reactivate`.

`agh__goal_get` follows an active moved-binding alias and remains readable for terminal state until
clear. `agh__goal_report` accepts `complete` or `blocked`; blocked requires evidence, evidence is
limited to 16 KiB, and the daemon revalidates the current prompt/control/binding identity. It records
a durable boundary intent, not immediate completion or proof of provider-side effect uniqueness.
Retry of the same intent deduplicates; conflict or revoke fails with a stable reason.

`agh__loop_turns` and `agh loop turns --run <id> --after-seq <n>` read the run-wide monotonic audit;
optional node/item filters narrow one instance. `agh loop runs --origin session
--origin-session <id>` isolates conversational Runs. Turn result, reason, stop reason, verdict,
evidence, usage, and end time remain nullable until durable evidence exists.

## Terminal Outcomes And Live States

A run holds one of eleven states. Report the terminal outcome exactly — never round an error or an
exhausted budget up to success.

**Terminal (6):**

- `done` — the goal was verified. The only success outcome.
- `no-op` — ran, found nothing to do. A clean watch tick is `no-op`, not a fake `done`.
- `blocked` — an external dependency blocked progress (missing dependency/credential/resource, a
  human-gate `reject`, or a `loop.gate.pre` denial).
- `failed` — an unrecoverable node/gate error, a `loop.generation.pre` denial, or an operator
  `Stop` (truthful cause `operator_stop`).
- `exhausted` — the iteration cap or fan-out ceiling tripped before the goal.
- `stalled` — no progress: the no-progress window elapsed, the failure circuit breaker tripped, the
  blocker-ID signature repeated, or a watched source went silent.

Failure streaks are evaluated per node across generations, so a healthy sibling cannot reset a
failing node's breaker. An unbounded watch run also stalls after consecutive failed generations;
healthy waiting ticks remain `watching`.

**Live (5):** `queued` (deferred start under `concurrency: queue`), `running`, `watching` (dormant
watch tick), `needs-approval` (parked on a human gate — a live pause, not terminal), `paused`
(operator paused at a boundary). `ready` and `awaiting_child` are node-level, never run states.

## The Approve Capability Gate

`agh__loop_approve` requires the `loops.approve` capability, and **an agent can never approve a run
it started**. The daemon compares the approver's identity against the run's starter: an agent
session cannot approve its own run — the call is denied `ErrPermissionDenied` (reason
`approval_self_denied`). A different agent, or an operator, can approve. Provide `run_id`, `gate_id`,
and `decision` (`approve` | `request_changes` | `reject`). `approve` resumes, `request_changes`
revises into the next generation, `reject` halts on a `blocked` outcome.

Budget escalation uses the synthetic gate ID `budget`; it is not an authored node ID. It accepts
only `approve` to grant one continuation or `reject` to halt. `request_changes` is invalid for the
synthetic budget gate.

## Reference Grammar And Reserved Action Kinds

Definitions reference data over one namespace with two surfaces, chosen by the field:

- **Values** — Go `{{ }}` templates in string fields (`params.*`, rubrics, `transform.map.*`).
- **Conditions** — CEL returning `bool` (`branch.condition`, `fan-out.filter`, `contract.stop_when`).

Namespace roots: `inputs.<name>`, `nodes.<id>.output.<path>`, `nodes.<id>.status`, `item`/`index`
(fan-out scope only), `trigger.<path>` (trigger/webhook starts only), `event.<path>` (`watch-events`
`events[].filter` scope only), `generation`. Node IDs match `^[a-z][a-z0-9_]*$` (lowercase
snake_case) so the same ID is valid in both surfaces.

Node classes: `action` (open), `control` (closed), `source` (closed). Reserved **action** kinds are
`goal`, `run-agent`, `run-loop`, `transform`; every other action kind is a literal tool ID
(`agh__*`/`ext__*`/`mcp__*`). Control kinds: `fan-out`, `collect`, `branch`, `gate`, `sub-loop`.
Source kinds: `input`, `file-import`, `watch-source`, `watch-events`. A gate's
`verdict_policy: revise_until_clean` requires an `agent-judge` or `human` criterion.

Model routing belongs to the Loop runtime. `contract.model_defaults.worker` and
`[loops.defaults.*].model_defaults.worker` seed `run-agent` actions that omit `params.model`.
`contract.model_defaults.judge` and `[loops.defaults.*].model_defaults.judge` seed `agent-judge`
criteria that omit `model`. A node or criterion-local `model` wins over the effective default. Empty
values preserve the provider/runtime default. `[[tasks.run.task_runtime_rules]]` is scoped to normal
task worker profiles and does not route Loop `run-agent` workers.

## Loop Hook Events

The `loop.*` hook family has seven events; two can block. Dispatch is typed and fail-open — a broken
hook does not fail a run.

- `loop.started`, `loop.generation.post`, `loop.gate.post`, `loop.node.terminal`, `loop.terminal` —
  observe-only.
- `loop.generation.pre` — sync-eligible; a denial ends the run `failed`.
- `loop.gate.pre` — sync-eligible; a denial ends the run `blocked`.

Every payload carries the loop context (`loop_run_id`, `workspace_id`, `loop_name`, `generation`,
`node_id`, and more). Manage them with `agh__hooks_*`.

## Loop Run Event Stream

`GET /loop-runs/:run_id/events` streams durable named SSE frames for a run. Reconnect with
`Last-Event-ID` or `?after_sequence=` to resume after a sequence number. The daemon persists and
streams the same enumerated event kinds the web run page consumes: `status_changed`, `node_running`,
`node_succeeded`, `node_failed`, `generation_started`, `gate_verdict`, `channel_msg`,
`token_tick`, and `needs_approval`. Payloads are redacted/bounded before storage, and reads are
scoped to the run's workspace.

## Watch-Source Behavior

A Loop with a `watch-source` node is a watch Loop. It holds `watching` between ticks, defaults to
`iteration_cap: 0` (`∞`, never `exhausted`), ends a clean tick `no-op`, and ends on silence past its
window `stalled` (reason `watch_source_silence`). The default `dev-cycle` `reviews-watch` Loop is a
watch Loop and requires `gh` to be installed and authenticated for CodeRabbit polling.

`reviews-watch` waits for CodeRabbit evidence on the current PR head. A ready tick requires the
provider PR head to match local `git rev-parse HEAD`, a successful CodeRabbit commit status for that
head, and either a current CodeRabbit review for that commit or `current_settled` evidence for the
current head. The following `fetch_issues` step decides whether unresolved issues remain: zero issues
ends the tick cleanly, otherwise the fixer runs. Pending status keeps the run watching; failed/error
status blocks with the provider diagnostic; stale review commits are not treated as ready. Fetching
review-body nitpicks is opt-in with `include_nitpicks` (default `false`). `auto_push=true` implies the
fixer creates the local fix commit before the loop's push node runs; `auto_commit=true` does the same
without pushing.

Scheduled watch Loops default to `catch_up_policy: coalesce`; other scheduled Loops default to
`skip_missed`. Explicit recurring schedule policies are `skip_missed`, `coalesce`, `replay`, and
`run_once_on_catchup`. Catch-up starts carry
structured metadata (`scheduled_at`, `original_due_at`, `catch_up`, `catch_up_policy`) on the
automation run.

## Watch-Events Behavior

A `watch-events` source node makes a Loop react to an **internal AGH event** (unlike `watch-source`,
which polls an external signal through an extension). The node carries a typed `events` list; each
subscription is `{ kind, filter }` where `kind` is a supported hook-event name and `filter` is an
optional CEL condition over `event`, `inputs`, and `nodes`. Multiple subscriptions OR together; an
empty filter matches every event of that kind in the workspace. Hook dispatch is only the doorbell —
the matched batch is re-derived from the durable ledger at wake, so subscriptions survive daemon
downtime and dropped hooks. The batch lands at `nodes.<id>.output`.

Supported kinds are validated at publish against the family registry; an unsupported kind fails lint
(`watch_events_kind_unsupported`, which names the supported set). Only post-state observation hooks
are subscribable — sync-eligible `pre_*` hooks are rejected. Supported families are:
`task.status_changed`, `task.blocked`, `task.unblocked`, `task.needs_attention`, `task.recovered`
(`task_events`); `task.run.completed`, `task.run.failed` (`task_events`); `loop.terminal`,
`loop.node.terminal` (`loop_run_events`); `automation.run.completed`, `automation.run.failed`
(`automation_watch_events`, whose terminal snapshots outlive `automation_runs` deletion);
`network.message.persisted`, `network.thread.opened`,
`network.direct_room.opened`, `network.work.opened`, `network.work.transitioned`,
`network.work.closed` (`network_timeline_log`); `coordinator.spawned`, `coordinator.decision`,
`coordinator.stopped`, `coordinator.failed` (`event_summaries`); `event.post_record`
(`session_events:<session_id>`). `event.post_record` must constrain `event.session_id` with equality
or lint returns `watch_events_filter_too_broad`; its output excludes record content and exposes only
metadata such as `record_type`, `sequence`, `turn_id`, `agent_name`, and `session_id`.

A Loop with a `watch-events` node is a watch Loop: it holds `watching` between wakes, defaults to
`iteration_cap: 0`, and **never stalls on silence** — a quiet subscription is healthy dormancy. The
parked read-model (active subscriptions, per-stream cursors, `last_wake_at`) is exposed on the run
detail (`agh loop runs show -o json`, HTTP/UDS parity) only while the Loop is dormant on events.

## Harvesting A Channel Decision

To let agents converse and act on the result, post with an `agh__network_send` action carrying a
`harvest: { kind: channel_result, window, responder?, content_rule? }`. The retired `channel-post`
kind does not exist. After the send, the node waits `window` for the designated result — a `say`
with `intent: result` or a `trace` with `state: completed` — and exposes it as
`nodes.<id>.output.*`. Silence past `window` ends the run `stalled`. `content_rule` narrows the
match: `any`, `json`, `non_empty`, `contains:<needle>`, or `json_path:<a.b.c>`. This capability is a
documented example, not a packaged default Loop.
