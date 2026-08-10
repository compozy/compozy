# Loops

Agent operation guidance for Compozy Loops — deterministic goal → verify → settle programs the daemon
owns and runs. Use this reference when you author, configure, run, observe, approve, or control a Loop
from inside Compozy. Prefer the native `compozy__loop_*` tools; fall back to `compozy loop` CLI or HTTP with
structured output. Never guess a schema — resolve `compozy__tool_info` for the exact descriptor first.

## Contents

- Native tools, catalog reads, and authoring loop
- Run history, best state, and Goal commands
- Terminal outcomes, failure handling, approvals, and succession semantics
- Reference grammar, metric criteria, and runtime selection
- Hooks, SSE, watch sources, and watch events
- Agent-authored review/fix and channel-result harvesting

## The Tool Set And CLI Verbs

Toolset `compozy__loops` — 23 native tools. All 21 Loop tools have matching `compozy loop` verbs;
the two session-bound Goal tools use the session command/native report surfaces. The CLI adds one verb
(`edit`) that has no native tool.

| Native tool                  | Mode                            | CLI                         | Purpose                                                                      |
| ---------------------------- | ------------------------------- | --------------------------- | ---------------------------------------------------------------------------- |
| `compozy__loop_list`         | read                            | `compozy loop list`         | List Loop definitions in the workspace.                                      |
| `compozy__loop_inspect`      | read                            | `compozy loop inspect`      | Read one definition: inputs, contract, start bindings, version.              |
| `compozy__loop_validate`     | read                            | `compozy loop validate`     | Lint + compile a definition without saving.                                  |
| `compozy__loop_status`       | read                            | `compozy loop status`       | Read one run's status with generation detail.                                |
| `compozy__loop_runs`         | read                            | `compozy loop runs`         | List runs in the workspace.                                                  |
| `compozy__loop_create`       | mutating                        | `compozy loop create`       | Create/fork, or CAS-publish when `expected_version` is set.                  |
| `compozy__loop_run`          | mutating                        | `compozy loop run`          | Start a run, or dry-run with `dry: true` / `--dry-run`.                      |
| `compozy__loop_configure`    | mutating                        | `compozy loop configure`    | Write per-Loop runtime config overrides.                                     |
| `compozy__loop_pause`        | mutating                        | `compozy loop pause`        | Request a generation-boundary pause.                                         |
| `compozy__loop_resume`       | mutating                        | `compozy loop resume`       | Resume a paused or pause-requested run.                                      |
| `compozy__loop_approve`      | mutating · **capability-gated** | `compozy loop approve`      | Apply one human-gate decision.                                               |
| `compozy__loop_cancel`       | mutating                        | `compozy loop cancel`       | Request cooperative cancellation of one active run.                          |
| `compozy__loop_kill`         | destructive                     | `compozy loop kill`         | Immediately fence and cancel one active run.                                 |
| `compozy__loop_nodes`        | read                            | `compozy loop nodes`        | List waiting, quarantined, attention, or retrying nodes.                     |
| `compozy__loop_node_pause`   | mutating                        | `compozy loop node pause`   | Pause one authored node.                                                     |
| `compozy__loop_node_resume`  | mutating                        | `compozy loop node resume`  | Resume one paused node or manual wait.                                       |
| `compozy__loop_node_cancel`  | mutating                        | `compozy loop node cancel`  | Request cooperative node cancellation.                                       |
| `compozy__loop_node_kill`    | destructive                     | `compozy loop node kill`    | Immediately fence one authored node.                                         |
| `compozy__loop_node_requeue` | mutating                        | `compozy loop node requeue` | Requeue one quarantined node into a successor generation.                    |
| `compozy__loop_delete`       | destructive                     | `compozy loop delete`       | Delete a writable definition plus its config and editor annotations.         |
| `compozy__goal_get`          | read · session-scoped           | `/goal status`              | Read the caller session's visible Goal, including terminal-until-clear.      |
| `compozy__goal_report`       | mutating · prompt-scoped        | —                           | Record one current-prompt `complete` or evidenced `blocked` boundary intent. |
| `compozy__loop_turns`        | read                            | `compozy loop turns`        | Read a Run's total-order Goal turn audit with cursor and node/item filters.  |

There is **no `compozy__loop_edit` native tool**. Agents edit a definition through the authoring loop
(validate → dry-run → `compozy__loop_create` with `expected_version`) or by a filesystem write. The CLI
`compozy loop edit` is a `$EDITOR` convenience for operators and publishes through the same
compare-and-swap path.

## Catalog Reads

Use `compozy loop list --workspace <ref> -o json`, HTTP/UDS `GET /api/workspaces/{workspace_id}/loops`, or native `compozy__loop_list`. Filters are name/contract-goal search (`--query` in CLI, `q` elsewhere), `kind` (`read_only` or `workspace`), exact category, exact latest-run status, name sort, cursor, and limit.

The response is `loops`, exact self-filtered `facets` (`kinds`, `categories`, `statuses`), and counted `page` (`total`, normalized `limit`, `has_more`, `next_cursor`). Self-filtered means each facet omits its own active filter while respecting search and every other filter. Pages default to 50 and cap at 200.

Opaque cursors bind workspace, search, kind, category, status, and sort; limit may change. Stable order is read-only before workspace, then normalized name and ID. Compozy computes the cut from lean records and loads definition YAML only for selected rows. `last_run` is the all-time latest run and includes `best_generation`/`best_score` when the run has a scored best candidate; only `aggregate_30d` and `success_rate_30d` use the 30-day window.

`compozy loop runs` / `compozy__loop_runs` is a different, non-cursor contract: it returns `runs` plus aggregates, defaults to 100 rows, caps at 500, and does not expose `has_more` or `next_cursor`.
Each run summary exposes `best_generation`/`best_score` but never embeds generation history.

## Run History And Best State

Use `compozy loop status` / `compozy__loop_status` for detail. The run carries its current
`generation` plus optional `best_generation`/`best_score`; `generations[]` carries durable
`parent_generation`, `origin`, `verdicts[]`, and outputs. Origins are `initial`, `stop_when`,
`reattempt`, `gate_revise`, `gate_next_generation`, `dod_retry`, `ratchet_restore`, and `requeue`. Each verdict
has `gate_id`, machine `outcome`, optional `score`, and optional `route_cause_rank`. The list surfaces
remain summaries; use status when an agent needs the lineage or gate decisions.

Best fields are absent until an approved finite metric score establishes a baseline. A
`ratchet_restore` generation may point `parent_generation` at an older best while `previous.*`
still describes generation `N-1`. On `exhausted` or `stalled`, inspect `best_generation` rather than
assuming the last generation is the best candidate.

## The Authoring Loop

Follow **inspect → validate → dry-run → publish (with `expected_version`) → run**. Every step before
`run` is structured and spends no tokens.

1. **inspect** — `compozy__loop_inspect` returns the definition and its current `version`. Read the
   version before you change anything.
2. **validate** — `compozy__loop_validate` lints and compiles a candidate without saving; it returns
   per-node codes (`unknown_reference`, `node_id_invalid`, `verdict_policy_requires_judge`,
   `fan_out_ceiling_exceeded`).
3. **dry-run** — `compozy__loop_run` with `dry: true` resolves inputs and returns the first generation's
   plan without creating a run or spending budget. It returns the authored `contract` beside the
   input-resolved `materialized_contract` that Goal agents and judges receive. It also builds and
   reloads the executed-definition snapshot used by submission; a template or condition manifest
   mismatch reports the exact key and source before any run is created.
4. **publish** — `compozy__loop_create` with `expected_version` set to the version from step one (or
   HTTP `PATCH /loops/:name`). This is compare-and-swap: a stale version is rejected `409` with the
   current version. Native: `tool_conflict`/`loop_version_conflict` with
   `partial_result.structured.version_conflict.current_version`; re-inspect before retrying. Use PATCH/create-with-version
   for **all** programmatic editing — the filesystem write path is last-write-wins and unsafe for
   concurrent agents.
5. **run** — `compozy__loop_run`. Only now does the Loop spend tokens.

New Loops start as a fork (`compozy__loop_create` with `fork_from_name`); there is no blank-canvas
authoring. Read-only sources — including the default `dev-cycle` Loops — must be forked before you
adapt them; native mutation is `tool_denied`/`loop_source_immutable`.

Deleting a workspace-authored Loop also removes its stored config override and editor annotations.
Run records and executed-definition snapshots remain available as history. Config and annotation
reads or writes for a missing definition return not found and never create detached sidecars.

## Goal Nodes And Session Commands

A Goal is the reserved action `kind: goal`. Its `params` require `agent`, non-empty `objective`, at
least one supported `judge`, positive `max_turns`, and an `output_schema` whose `status` enum can
represent `blocked`. `on_exhausted` is `halt` (default) or `escalate`. Goal v1 judges are `command`
(`check` required), `agent-judge` (rubric/prompt required), or `extension` (tool required); `human`
is rejected.

Missing `session` compiles to `mode: continuous`. An isolated session is the other valid strategy;
the two cannot be combined. Operational retry uses `retry.max_attempts`, which counts total
pre-submission attempts including the first. `on_failure: fresh_session` requires continuous mode
and at least two attempts. It applies only when Compozy proves the prompt effect never started. Compozy
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

`compozy__goal_get` follows an active moved-binding alias and remains readable for terminal state until
clear. `compozy__goal_report` accepts `complete` or `blocked`; blocked requires evidence. It is
available to the current bound prompt for session-, catalog-, HTTP-, and native-tool-origin Runs.
Evidence is limited to 16 KiB, and the daemon revalidates workspace, Run, prompt, control, and
binding identity. It records a durable boundary intent, not immediate completion or proof of
provider-side effect uniqueness. Retry of the same intent deduplicates; conflict or revoke fails
with a stable reason.

`compozy__loop_turns` and `compozy loop turns --run <id> --after-seq <n>` read the run-wide monotonic audit;
optional node/item filters narrow one instance. `compozy loop runs --origin session
--origin-session <id>` isolates conversational Runs. Turn result, reason, stop reason, verdict,
evidence, usage, and end time remain nullable until durable evidence exists. Settled turns retain
bounded criterion diagnostics and aggregate warnings. Command criteria include outcome, exit code,
standard output, standard error, blockers, and warnings when present; continuation prompts and the
Web timeline use that durable evidence.

## Terminal Outcomes And Live States

A run holds one of twelve states. Report the terminal outcome exactly — never round an error or an
exhausted budget up to success.

**Terminal (7):**

- `done` — the goal was verified. The only success outcome.
- `no-op` — ran, found nothing to do. A clean watch tick is `no-op`, not a fake `done`.
- `blocked` — an external dependency blocked progress (missing dependency/credential/resource, a
  human-gate `reject`, or a `loop.gate.pre` denial).
- `failed` — an unrecoverable node/gate error or a `loop.generation.pre` denial.
- `canceled` — an operator cancel or kill, with cause `operator_cancel` or `operator_kill`.
- `exhausted` — the iteration cap or fan-out ceiling tripped before the goal.
- `stalled` — no progress: the no-progress window elapsed, the failure circuit breaker tripped, the
  blocker-ID signature repeated, or a watched source went silent.

Failure streaks are evaluated per node across generations, so a healthy sibling cannot reset a
failing node's breaker. An unbounded watch run also stalls after consecutive failed generations;
healthy waiting ticks remain `watching`.

**Live (5):** `queued` (deferred start under `concurrency: queue`), `running`, `watching` (dormant
watch tick), `needs-approval` (parked on a human gate — a live pause, not terminal), `paused`
(operator paused at a boundary). `ready` and `awaiting_child` are node-level, never run states.

Native control rejections are `tool_invalid_input`: `invalid_status_transition` for an unsupported
live state, or `terminal_loop_run` after termination. `schema_invalid` is reserved for malformed input.

## Failure Contract And Node Recovery

Failure handling has fixed precedence: eligible automatic retry → authored `on_error.route` or
`allow_fail` → effects → generation-level repair or terminal policy. An unannotated failure
escalates; absorption is never implicit. The closed classes are `transport`, `payload_declared`,
`quality_rejection`, `authoring`, `cancellation`, `attempt_timeout`, `budget_exhausted`, and
`target_unavailable`. Only `transport` and `attempt_timeout` are retry-eligible.

`retry` carries `max_attempts`, optional `on_failure`, `backoff.{base,max}`, and `non_retryable`.
Mechanical actions inherit family defaults; `run-agent` and `run-loop` require explicit node retry;
`goal` does not use generic retry. `timeout` bounds one attempt and `deadline` bounds attempts plus
backoff. `result_contract` maps an application failure payload into `payload_declared`.

Effects declare exactly one `emit` or `tool` plus optional `with`. Node triggers are `on_retry`,
`on_success`, `on_pause`, `on_timeout`, `on_cancel`, and `on_quarantine`; contract triggers cover all
seven terminal outcomes. Effects observe committed state, fail open, and tool delivery is at least
once with a stable `delivery_id`. Templates can read `inputs` and
`effect.identity|failure|quarantine|attempt|links`.

Use `compozy__loop_nodes` or `compozy loop nodes --state waiting|quarantined|attention|retrying` to
find workspace-scoped cells. Pages default to 50 and cap at 200; narrow with Loop or run ID. Then use
the exact run/node/item identity with node pause (`drain|cancel`), resume
(`plain|reset_attempts|immediate`, optional manual-wait payload), cancel, kill, or requeue. Requeue is
quarantine-only and creates a bounded successor generation. Cancel is cooperative; Kill fences
immediately. Run cancel/kill both end `canceled` with distinct operator causes.

Silence raises attention and never auto-kills or auto-pauses. Confirmed process/transport death may
resume from progress through the bounded death-streak authority; parked nodes are never
death-resumed. Paused nodes, durable waits, approval waits, and quarantined cells suspend node
clocks and the run wall-clock work budget; token spend still counts.

## Re-attempt And Succession Semantics

Node failure and gate rejection use different controls:

| Cause                            | Next generation                                                                   | Origin                 |
| -------------------------------- | --------------------------------------------------------------------------------- | ---------------------- |
| Node failure, `failed_only`      | Failed/pending nodes plus transitive dependents rerun; unrelated successes carry. | `reattempt`            |
| Node failure, `full_body`        | Every body node reruns.                                                           | `reattempt`            |
| In-body gate `revise`            | Producers of every route-causing gate, those gates, and dependents rerun.         | `gate_revise`          |
| Metric-gate `revise` with a best | The producer-scoped repair carries unrelated outputs from the best baseline.      | `ratchet_restore`      |
| In-body gate `next_generation`   | A fresh full-body generation starts.                                              | `gate_next_generation` |
| DoD gate `next_generation`       | A fresh full-body generation starts instead of terminating.                       | `dod_retry`            |

Both `next_generation` surfaces preserve the rejecting verdicts in `previous.verdicts.*` and their
ordered gate IDs in `previous.route_causes`. `revise` is targeted repair; `next_generation` is a
fresh pass. Bounds still apply. For evidence-sensitive work, put the acceptance rule in a typed gate
and route a weak verdict through `revise`, `next_generation`, or `halt`; do not hide the transition
inside prompt prose.

## The Approve Capability Gate

`compozy__loop_approve` requires the `loops.approve` capability, and **an agent can never approve a run
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

Contract narrative fields (`goal`, `definition_of_done`, `constraints[]`, `boundaries[]`) accept only
declared `inputs` references and materialize once before Goal work. Goal params and nested gate inputs
materialize recursively at their execution boundary; direct references retain JSON types and raw
`output_schema` values are never rendered. Literal `{{ ... }}` inside an input value stays literal —
agents and judges do not run a second template pass. Run detail exposes raw `executed_definition`
beside the input-resolved `materialized_contract`.

`command` criteria keep the exact shell program written by the Loop author. Every runtime template
value emitted inside `check` must end with `| shellQuote`; compilation rejects unquoted substitutions.
This preserves authored pipes, redirects, and chaining while preventing inputs, trigger payloads, or
node outputs from introducing shell syntax.

Namespace roots: `inputs.<name>`, `nodes.<id>.output.<path>`, `nodes.<id>.status`, `item`/`index`
(fan-out scope only), `trigger.<path>` (trigger/webhook starts only), `event.<path>` (`watch-events`
`events[].filter` scope only), `generation`, plus these read-only history roots:

- `previous.generation`, `previous.nodes.<id>.{status,output}`,
  `previous.verdicts.<gate_id>.{outcome,score,blocking_issues,criteria}`, and ordered
  `previous.route_causes`.
- `best.generation`, `best.score`, and `best.nodes.<id>.output`; best status and verdicts are not
  projected.

`previous` is empty on generation 1; `best` is empty until one metric candidate becomes eligible.
Guard history-dependent templates with `{{ with .previous }}` or `{{ with .best }}`. Node IDs match
`^[a-z][a-z0-9_]*$` (lowercase snake_case) so the same ID is valid in templates and CEL.

Node classes: `action` (open), `control` (closed), `source` (closed). Reserved **action** kinds are
`goal`, `run-agent`, `run-loop`, `transform`; every other action kind is a literal tool ID
(`compozy__*`/`ext__*`/`mcp__*`). Control kinds: `fan-out`, `collect`, `branch`, `gate`, `wait`,
`sub-loop`. A `wait` declares exactly one of `for`, `until`, or `event`, with optional `expect`,
`ahead_arrival`, and `expires`. Source kinds: `input`, `file-import`, `watch-source`, `watch-events`.
A gate's
`verdict_policy: revise_until_clean` requires an `agent-judge` or `human` criterion. For a command
criterion with `expect: stdout_contains`, set the typed `contains` field to the required stdout
substring.

### Metric Criteria

One `command`, `agent-judge`, or `extension` criterion per definition may declare
`metric: {direction: maximize|minimize, min_delta?: <finite non-negative number>}`. `human` cannot.
Unset `min_delta` means `0`, but improvement remains strict. A candidate advances
`best_generation`/`best_score` only when its finite score improves in the declared direction and
the aggregate gate verdict is approved. Missing or non-finite required scores become
`invalid_output`; rejected candidates never establish or advance best.

Score contracts are criterion-specific:

- command standard output is exactly one score-only JSON object, e.g. `{"score":0.72}`;
- the agent-judge verdict object includes numeric `score` with its verdict, evidence, and blockers;
- extension structured output includes numeric `score` with `verdict` (and may include evidence and
  blockers).

A rejected metric candidate routed through `revise` restores from an existing best; with no best it
repairs from the last generation. A distinct `next_generation` route starts a fresh full body.

Runtime routing belongs to the Loop runtime. Worker fields resolve independently in this order:
per-run rules, imported task frontmatter, configured runtime rules, `params.runtime`,
`runtime_defaults.worker`, then the agent definition. A higher layer replaces only the fields it
sets. Each rule matches exactly one of `id`, `type`, or `complexity`; specificity is
`id > type > complexity`, and the later rule wins when specificity is equal. Child `run-loop`
definitions resolve their own rules and never inherit the parent's per-run rules.

Use `contract.runtime_defaults` and `contract.runtime_rules` in a Loop definition, or
`[loops.defaults.delivery|watch]` plus stored Loop config for operator defaults. `run-agent` nodes
use `params.runtime`. Imported task frontmatter may set `runtime: {provider, model, reasoning}`.
Judges use only `runtime_defaults.judge` plus the criterion's `runtime`; task rules never select a
judge. The retired `model_defaults`, scalar `params.model`, and scalar criterion `model` keys fail
with migration guidance.

`--runtime` is repeatable and preserves rule order:

```bash
compozy loop run \
  --workspace . \
  --name implement-tasks \
  --runtime worker=codex/gpt-5.4@high \
  --runtime type=frontend:claude/opus \
  --runtime id=task_03:-/gpt-5.5-codex@xhigh \
  --dry-run \
  -o json
```

The runtime expression is `provider/model@reasoning`. Use `-` to leave provider or model unset;
bare `worker=opus` is model-only shorthand, and gateway model IDs retain slashes after the first
provider separator. Dry-run resolves workspace, stored, definition, and per-run layers without
creating a run.

Declared Loop inputs may have global or workspace defaults under
`[loops.inputs.<loop-name>]`. Resolution is per key: run, workspace, global, then definition.
Dry-run reports the effective value plus `run|workspace|global|definition` origin. Manage dynamic
paths with `compozy config get|set|unset loops.inputs.<loop-name>.<key>`; HTTP and UDS expose the
same exact-scope lifecycle under `/loops/{name}/input-defaults`. Stored values are validated only
when that Loop is dry-run or submitted, and failures return typed `input_default` diagnostics.

`compozy loop validate` performs static definition validation. Dry-run and submission perform
effective validation after workspace, stored, and per-run layers resolve, then the daemon validates
again immediately before binding. Failures return structured `runtime_validation` items and spawn
no ACP process. Provider IDs must resolve; exact model IDs pass through unchanged and the ACP/provider
boundary reports a model rejection when it cannot bind that ID.

Successful generation outputs expose the binder-applied `resolved_runtime` triple plus per-field
provenance through `compozy loop status`, HTTP/UDS run detail, `compozy__loop_status`, and
`runtime_applied` SSE frames. The web run inspector displays the same durable values read-only.
Successful persisted `loop run` responses also expose `web_url`; human output prints it as the final
line. Dry-run has no run ID and never emits a URL.

## Loop Hook Events

The `loop.*` hook family has seven events; two can block. Dispatch is typed and fail-open — a broken
hook does not fail a run.

- `loop.started`, `loop.generation.post`, `loop.gate.post`, `loop.node.terminal`, `loop.terminal` —
  observe-only.
- `loop.generation.pre` — sync-eligible; a denial ends the run `failed`.
- `loop.gate.pre` — sync-eligible; a denial ends the run `blocked`.

Every payload carries the loop context (`loop_run_id`, `workspace_id`, `loop_name`, `generation`,
`node_id`, and more). Generation payloads expose `parent_generation` plus the closed `origin`
vocabulary: `initial`, `stop_when`, `reattempt`, `gate_revise`, `gate_next_generation`, `dod_retry`,
`ratchet_restore`, or `requeue`. Gate payloads expose `outcome`, optional `score`, and optional
`best_generation`. Manage hooks with `compozy__hooks_*`.

## Loop Run Event Stream

`GET /loop-runs/:run_id/events` streams durable named SSE frames for a run. Reconnect with
`Last-Event-ID` or `?after_sequence=` to resume after a sequence number. Payloads are redacted and
bounded before storage; reads are scoped to the run's workspace. The closed event vocabulary is:

- execution: `status_changed`, `generation_started`, `node_running`, `node_succeeded`, `node_failed`,
  `gate_verdict`, `runtime_applied`, `channel_msg`, `token_tick`, `needs_approval`,
  `goal_turn_started`, `goal_turn_completed`, `goal_status_changed`;
- node lifecycle: `node_retry_scheduled`, `node_paused`, `node_resumed`, `node_canceled`,
  `node_killed`, `node_quarantined`, `node_requeued`, `node_wait_started`, `node_wait_resumed`,
  `node_attention_flagged`, `node_attention_cleared`;
- observation and safety: `effect_results`, `custom_event`, `duplicate_suppressed`,
  `target_breaker_transition`, `stale_schedule_dropped`, `late_arrival`.

`generation_started` carries `generation`, `parent_generation`, `origin`, `reattempt_strategy`, and
`loop_name`. `gate_verdict` carries the sanitized `node_id`, `generation`, `gate_id`, `item_index`,
`verdict`, `reason`, `route`, `blocking_issues`, `criteria`, optional `score`, and optional
`best_generation`. `goal_turn_completed` carries the settled Goal result plus bounded `criteria` and
`warnings`, matching persisted turn reads. The retired `confidence` field is not part of either payload.

## Watch-Source Behavior

A Loop with a `watch-source` node is a watch Loop. It holds `watching` between ticks, defaults to
`iteration_cap: 0` (`∞`, never `exhausted`), ends a clean tick `no-op`, and ends on silence past its
window `stalled` (reason `watch_source_silence`). Watch sources are extension-defined; the bundled
`dev-cycle` extension does not publish one. Every `watch/poll` response must carry a stable
`event_key`; missing, invalid UTF-8, or values over 256 bytes fail before admission, with no fallback.

## Agent-Authored Review and Fix

The bundled `review-and-fix` Loop reviews a named task without a pull-request provider. Start it with
`compozy loop run --workspace <workspace-id> --name review-and-fix --input task_name=<task>`.
Optional `reviewer` and `fixer` inputs select agents; `auto_commit` defaults to `false`.

The reviewer returns source-agnostic structured issues. `ext__dev_cycle__write_review_artifacts`
creates the next exclusive `.compozy/tasks/<task>/reviews-NNN/` round inside the authenticated
workspace, validates file containment, and returns complete batches of issue-file paths. The fixer
reads each batch and changes `status: pending` only to `valid` or `invalid`; it never creates,
renames, timestamps, or resolves an issue file. `ext__dev_cycle__finalize_review_round` alone changes
triaged statuses to `resolved` and returns structured `{resolved, invalid, pending}` counts. The next
generation reviews the task again; an empty `issues` array ends the run. Neither artifact tool input
accepts a workspace root.

Scheduled watch Loops default to `catch_up_policy: coalesce`; other scheduled Loops default to
`skip_missed`. Explicit recurring schedule policies are `skip_missed`, `coalesce`, `replay`, and
`run_once_on_catchup`. Catch-up starts carry
structured metadata (`scheduled_at`, `original_due_at`, `catch_up`, `catch_up_policy`) on the
automation run.

## Watch-Events Behavior

A `watch-events` source node makes a Loop react to an **internal Compozy event** (unlike `watch-source`,
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

A Loop with a `watch-events` node holds `watching` between wakes and **never stalls on silence** — a
quiet subscription is healthy dormancy. It keeps the delivery default `iteration_cap: 50` unless the
definition sets `0`; only `watch-source` selects the unbounded watch default. The
parked read-model (active subscriptions, per-stream cursors, `last_wake_at`) is exposed on the run
detail (`compozy loop status --run-id <id> -o json`, HTTP/UDS parity) only while the Loop is dormant
on events.

## Harvesting A Channel Decision

To let agents converse and act on the result, post with a `compozy__network_send` action carrying a
`harvest: { kind: channel_result, window, responder?, content_rule? }`. The retired `channel-post`
kind does not exist. After the send, the node waits `window` for the designated result — a `say`
with `intent: result` or a `trace` with `state: completed` — and exposes it as
`nodes.<id>.output.*`. Silence past `window` ends the run `stalled`. `content_rule` narrows the
match: `any`, `json`, `non_empty`, `contains:<needle>`, or `json_path:<a.b.c>`. This capability is a
documented example, not a packaged default Loop.
