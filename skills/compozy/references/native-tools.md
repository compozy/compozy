# Native Tools

## Contents

- Operating rule
- Error handling
- Discovery and catalog toolsets
- Command palette tools
- Runtime and workspace tools
- Profile tools
- Workspace boundary
- Window management tools
- Terminal tools
- Command and skill tools
- Task and autonomy tools
- Loop tools
- Config, hooks, automation, marketplace, extensions, resources, and MCP tools
- Observability tools
- CLI/HTTP-only management surfaces
- Descriptor discipline
- Operator notification acknowledgement
- All-projects aggregates

## Operating Rule

For `pi_acp`, CompozyOS loads its versioned session extension through Pi's
`--extension` option. Pi's native MCP client exposes the hosted tools under its
`mcp__...` names; discover by the canonical `compozy__*` ID and call the returned
reference. Do not construct the session-specific namespace. The bridge requires
Pi's `registerMcpServer` API and native MCP support (verified on Pi 1.4.0).
Startup checks tool registration before advertising native tools. A
`pi_native_tools_*` runtime warning means the bridge is unavailable and startup
uses CLI guidance. The launch layer does not edit native Pi settings or copy login
credentials; bound-secret sessions keep their managed Pi directory.

Inside CompozyOS, prefer callable daemon-native tools over shelling out. They are policy-filtered, structured, auditable, and redaction-aware. Use shell only when a native tool is absent, denied, too narrow, or explicitly requested.

Never guess a tool schema from this reference. Resolve canonical `compozy__tool_info` for the exact descriptor, input schema, risks, and availability diagnostics before the first call.

Management-only surfaces include diagnostics, support bundles, scheduler controls, task inspection/pause/force recovery, config apply history, and some session repair/recap/approval flows.

`workspace` is optional and defaults to the bound session's workspace. `"@user"` names the User scope of the caller's profile (work that belongs to no project). Naming another workspace, or User from a project session, is a cross-scope request governed by the session permission mode; see Workspace Boundary below. Aggregate reads require an operator or an effective unrestricted grant; desktop `global` remains operator-only.

## Error Handling

Branch on the structured error `code` and any specific `reason_codes`, not the human-readable
`message`. Native tools retain registered daemon domain codes instead of collapsing state refusals
into `tool_backend_failed` or a generic conflict. Existing specific tool codes and reasons remain
authoritative. The error envelope is unchanged; messages can include a bounded, sanitized cause.

| Code                                           | Meaning and next action                                                                                                                    |
| ---------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------ |
| `subagent_archive_follows_parent`              | A child cannot be archived independently. Archive through its parent.                                                                      |
| `session_archive_requires_stopped`             | Stop the session before archiving. A detached runtime is not a stopped session.                                                            |
| `session_in_later`                             | The session is parked in Later. Bring it back with `compozy__session_resume_later`, or let the operator prompt it.                         |
| `subagent_later_follows_parent`                | A subagent moves to Later with its parent. Park the parent instead.                                                                        |
| `session_later_stop_failed`                    | Later could not stop the session tree or its processes; nothing was parked. Inspect the session and retry.                                 |
| `worktree_name_taken`                          | Inspect the existing worktree or choose another name.                                                                                      |
| `worktree_session_active`                      | Inspect and release the session binding before removing the worktree.                                                                      |
| `tool_backend_failed` with `backend_unhealthy` | A backend operation failed. Inspect the sanitized cause before retrying; for example, workspace pruning may be blocked by active sessions. |

Host paths and credentials are redacted from newly surfaced causes. A generic backend failure is
not proof of a transport outage. Do not repeatedly retry a domain refusal without changing the
state that caused it. A provider may display a reason before the message; use the structured
envelope when available.

## Discovery And Catalog Toolsets

- Toolset `compozy__bootstrap`: `compozy__tool_list`, `compozy__tool_search`, `compozy__tool_info`.
- Toolset `compozy__catalog`: agent, command, skill, and redacted Vault-reference catalog access plus bootstrap tools.

Bare managed sessions receive the full availability-gated callable catalog through hosted MCP. CompozyOS
does not add an automatic bootstrap/catalog allowlist over the hosted projection. Explicit agent
`tools`, `toolsets`, `deny_tools`, session lineage, disabled sources, and approval/risk gates still
apply.

The discovery loop and denial-handling rules live in the preceding tools-and-skills prompt section; this reference supplies the stable tool map.

## Command Palette Tools

Use `compozy__cmd_palette_list` to read the daemon-canonical command catalog for the bound workspace.
Optional `source` filters one provider and optional `client` resolves client-context availability.

Use `compozy__cmd_palette_invoke` with `id`, optional `args`, and optional `client`. The command's own
availability, targeting, single-flight, and approval rules still apply. An `approval_pending` result
returns `approval_id`. Operators inspect it with `compozy approvals show <id>`, decide with
`compozy approvals resolve <id> --decision approved|denied`, or cancel with
`compozy approvals cancel <id>`. These commands use the selected profile (flag, environment, then
remembered selection). Pass `--profile <owner>` after switching away from the approval's owner.
Approval admits asynchronous execution; inspect `execution_status` before claiming completion.
The Web palette presents Approve/Deny for its pending invocation and reads the terminal result.
HTTP/UDS clients use `POST /api/tools/approvals/{id}/resolve?profile=<owner>` with an explicit
`decision`. The request rechecks profile availability and session binding; HTTP decisions require
a loopback listener. Foreign-profile and already-terminal approvals remain refused.
CLI catalog fallback is `compozy cmd-palette list|inspect|invoke|clients`.
The clients list contains only currently connected command channels. Closing a tab removes it
from targeting; reconnecting the tab restores eligibility. Refresh this list before choosing an
explicit client. With one connected client, UI commands can select it automatically; with none,
they return no_attached_shell.

Manage workspace command bindings with `compozy cmd-palette bind|unbind|bindings` and aliases with
`compozy cmd-palette alias set|clear`. Conflicts name the current owner; `--overwrite` transfers the
chord or alias atomically. Pin state uses `compozy cmd-palette pin|unpin`. HTTP/UDS clients read and
patch bindings and aliases through `/api/settings/window-manager`.

Add `--global` to `bind|unbind` for desktop-global hotkeys. Read the complete
`window_manager.global_shortcuts` map before mutation because the Settings PATCH replaces it. Pass a
shell `client_id` when reading `/api/settings/window-manager` to receive that shell's confirmed
registration state; an intended chord without `active_chord` is not active.

Palette personalization is profile-owned within a workspace and management-only. Inspect or reset
the selected profile's state with `compozy --profile <profile> cmd-palette personalization
show|reset --workspace <workspace>`. HTTP/UDS clients use
`GET /api/cmd-palette/rank-signals`, `POST /api/cmd-palette/usage`, `PUT|DELETE
/api/cmd-palette/pins/{id}`, and `GET|DELETE /api/cmd-palette/personalization`; keep rank signals in
session memory and report only the normalized pre-selection query.
Observe cross-client changes through `cmd_palette.pin.changed`,
`cmd_palette.personalization.reset`, and the following `cmd_palette.catalog.changed` invalidation.

## Runtime And Workspace Tools

Session tools: `compozy__session_list`, `compozy__session_create`, `compozy__session_prompt`,
`compozy__session_rewind`, `compozy__session_continue`, `compozy__session_fork`,
`compozy__session_compact`, `compozy__session_status`, `compozy__session_history`,
`compozy__session_events`, `compozy__session_describe`, `compozy__session_health`, `compozy__session_runtime_set`,
`compozy__session_runtime_clear`, `compozy__session_archive`,
`compozy__session_unarchive`, `compozy__session_later`, `compozy__session_resume_later`, `compozy__session_rename`, `compozy__session_reparent`, `compozy__session_wait`,
`compozy__session_spawn`, `compozy__session_stop`, `compozy__session_approve`,
`compozy__session_clarify_answer`, `compozy__session_prompt_cancel`.

Session reads are paged. `compozy__session_events` returns the newest 50 events and
`compozy__session_history` the newest 10 turns when `limit` is omitted or 0; an explicit `limit` is
honored. Every page returns `page` with `limit`, `has_more`, and a continuation cursor: pass
`page.next_before_sequence` as `before_sequence` to read older rows, or start a forward walk with
`after_sequence` (for example `0`) and continue with `page.next_after_sequence`; forward history pages
hold the oldest turns that start after the cursor. The forward history cursor is the first
event sequence of the last returned turn, so interleaved turns are not skipped. Forward reads
scan bounded event chunks and hydrate only the returned turns. Backward history also selects
turns by their first sequence and returns their complete matching events, including later events
interleaved beyond the cursor. `before_sequence`
cannot be combined with `after_sequence`. `compozy__session_describe` applies the same defaults and
returns `events_page` and `history_page`; page further through the dedicated tools.

For an agent's answer, call `compozy__session_history` with `session_id`,
`type: "agent_message"`, and `limit: 1`; include `turn_id` when recovering a specific reply.
This existing filter applies before content is read, so tool calls, tool results, and thoughts
are excluded. Concatenate the matching text chunks in sequence order. Keep `type` and other
filters unchanged when following page cursors; cursors refer to the first matching event in a
turn. Omit `type` when the full execution history is needed. A turn limit is not a byte limit:
if the answer itself exceeds the result budget, page its returned artifact normally to `eof`.

`compozy__notify` sends one operator notification from the bound session and workspace. Pass a
required `title` of at most 80 characters and an optional `body` of at most 240 characters. The
daemon sanitizes and redacts both fields before delivery, then returns exactly one provable outcome:
`delivered` when at least one live operator catalog-stream subscriber accepted the event,
`no-client`, `muted-workspace`, or `rate-limited` with `retry_after_ms`. The per-session limit is one
send per second, including sends suppressed by a workspace mute. CLI fallback: `compozy notify`.

Monitor tools, also in the `sessions` toolset: `compozy__monitor_create`, `compozy__monitor_list`,
`compozy__monitor_cancel`. A monitor is a daemon-owned watch that wakes the bound session with a new
turn when it fires, which is the only way background work can resume you after your turn ends. Use one
for every wait on external work, inside or outside your turn; `terminal.md` describes the shell-wait
guard that refuses `gh pr checks --watch`, long `sleep`, and polling loops.
Provider-internal background shells, monitors, scheduled wakeups, and "I will report back" promises
cannot start a CompozyOS turn. `kind="terminal"` watches a `terminal_id` (from `terminal_exec` with
`still_running`, or `terminal_open`) until `until="exit"` (default) or `until="match"` with a Go
regular expression `pattern`; `timeout_seconds` defaults to 1,800, caps at 21,600, and the monitor
still wakes you on timeout. `kind="timer"` fires once after `delay_seconds` (5 to 86,400). A session
holds at most 8 active monitors, a terminal at most 4, and the daemon at most 256. There is no remote
polling kind: wait on CI or deploys by running the bounded watch command (`gh pr checks --watch`) through
`terminal_exec`, monitoring that terminal, and setting `timeout_seconds` past the expected wait.
Registering the same terminal condition (`terminal_id`, `until`, `pattern`, normalized timeout, and description) again returns the existing
active monitor rather than adding a second wake. After
`monitor_create`, do not poll or sleep: keep working on steps that do not depend on the monitor, and
end your turn only when every remaining step waits on a registered monitor or subagent. The wake is a synthetic `monitor`
message headed `[COMPOZY MONITOR - NOT USER INPUT]` with the monitor id, status (exit code, match,
timeout, or timer), and an entity-encoded, untrusted output tail. A busy session runs it after the
current turn; a full input queue retries the wake for up to 15 minutes (`status: "delivering"`).
Stopping the session cancels its monitors and drops their history, and a stopped session is never
revived. `monitor_cancel` only stops the wake, not the watched process. Finished monitors stay
listable for one hour. Monitors live in the daemon process and do not survive a daemon restart.
Operators see `monitor_summary.active` on session payloads and can cancel monitors (no wake).

`compozy__session_create` accepts workspace, agent, optional name, and exactly one of `worktree` or
`new_worktree`. It creates an active logical session with `runtime.status="unbound"`; it does not
accept or send a prompt or runtime. A named Worktree must be ready; new creation reaches ready before
session persistence.
The calling session is recorded automatically as `lineage.parent_session_id` (provenance only, no
governance) when the new session lands in the caller's workspace; the link is not a tool input.

### Session message origin

Use the native `compozy__session_prompt` tool when one agent messages another session. The daemon
stamps `origin` from the calling session, including its session/workspace IDs, agent, title at send,
and hop. Neither the message body nor a tool input can set or replace this identity. CLI, HTTP,
and operator tool prompts remain operator input without an origin. Cross-workspace unsolicited sends remain governed by `workspace_access`; the sender workspace
in origin identifies the correct session link. A text reply may cross workspaces within the same
profile when `in_reply_to` names a persisted message that the target sent to the caller. This
exception does not authorize attachments, runtime changes, status/history reads, or other workspace
operations. Include the received message ID; knowing a session ID alone grants no reply authority.

Origin survives queue, steer, interrupt and daemon restart. The receiving provider sees a sender
header before the message. Stored text remains the authored message; queue and event JSON expose
origin separately. Without `notify_on_complete`, the header tells the recipient to reply with
`compozy__session_prompt` targeting the sender. `notify_on_complete: true` requests the reply-watch
flow; it requires an agent session and is part of the admission identity.

A finite batch of queued session messages and reply wakes precedes a prepared Goal prompt at an
idle turn boundary; later admissions wait until after that Goal prompt. An active turn still runs
to its boundary; queue mode does not interrupt it. See
`references/loops.md#queued-inputs-and-goal-continuation` for ordering and Goal accounting.

Keep the same `message_id` and `idempotency_key` when retrying. Another sender or a changed
`notify_on_complete` conflicts. Renaming the sender does not change the admitted origin. Historical
v4 receipts replay unchanged, without invented origin or reply watches; that replay shim is removed
in v0.5.0.

Agent-authored queue entries cannot be edited (`input_agent_authored`). Self-targeting returns
`invalid_request`. Message other sessions with this tool, never the CLI. An agent `interrupt` becomes
`steer`, including attachments; without live injection it waits after the turn. Prefer `steer` for a busy target. Send at milestones; `supersede: true` replaces your queued
status in any mode; answer with `in_reply_to` (keeps the hop). On `message_hop_limit` or
`message_rate_limited`, stop and batch. Queue order, promotion, hops and replies: `references/runtime-operations.md`.

Use `compozy__session_prompt` with `session_id`, optional `message`, optional `attachments`, and an
optional `runtime` snapshot. `attachments` is an array of file paths within the resolved workspace
root or configured `additional_dirs`, or existing workspace/session-scoped `att_...` IDs; at least
`message` or one attachment is required. Paths are uploaded through the attachment store before
submission; use `compozy session attachments upload <session-id> <file> -o json` for a durable ID.
The store enforces `session.attachments.max_file_bytes`, `session.attachments.max_files_per_prompt`,
and `session.attachments.allowed_mime` (see `references/configuration.md`).
`message` may carry `@` mention tokens (`@path`, `@dir/`, `@"path with spaces"`, `@session:<id>`);
they resolve against the receiving session, not the caller, and are not gated like slash commands.
Syntax, caps, and delivery: the "Prompt text may carry mentions" paragraph in
`references/runtime-operations.md`.
The first prompt to a never-bound session requires `runtime.provider`; a session whose idle runtime
was released keeps its selection and rebinds without it. Model, reasoning effort, speed, and typed
`acp_options` are optional snapshot fields. Each option sets exactly one of `value_id` or
`bool_value`. Runtime semantics, queued/interrupt snapshot behavior, and the rollback rule live in
`references/runtime-operations.md`; read the nested `runtime` object from `compozy__session_status`
rather than deriving it from the session state.

For ask-and-wait with an existing agent session, send `compozy__session_prompt` with
`notify_on_complete: true` and keep the returned `reply_watch.id`. Once every remaining step needs
that answer, end your turn; the daemon queues one reply wake when the turn that consumed your message settles. Do not poll
history or hold `session_wait` just to collect the answer. `compozy__session_status` lists outstanding
`reply_watches` in state `armed` or `fired`; delivered and abandoned watches drop off that list.
A reply contains the last non-empty assistant message (up to 12000 Unicode characters), or the failure
summary. `dropped` means the input was removed before it ran; `unknown` means delivery was uncertain,
so inspect the target session. An offline sender receives its pending wake on resume.
Archiving or deleting the sender abandons its watches. Canceling a queued reply wake does not cause
redelivery. `notify_on_complete` is agent-only, not an HTTP operator prompt field; operator-scoped
native calls requesting it return `invalid_request` with
`notify_on_complete requires an agent session`. A same-key retry after a pre-dispatch send failure
re-arms the same watch; the result reports its persisted state (`armed`, `fired`, `delivered`, or
`abandoned`). Deleting or archiving an unconsumed target produces `unknown` immediately.

`compozy__session_wait` blocks on one same-workspace session other than the caller. `until` accepts
the canonical attention/lifecycle badges; omission uses the settled set, and `done` satisfies `idle`.
`timeout_ms` defaults to 300,000 and cannot exceed 1,800,000. Outcomes are `state-reached`, `timeout`,
`session-gone`, `canceled`, and `overflow`. A native wait emits activity heartbeats while blocked so
session supervision does not mistake orchestration for inactivity.

`compozy__session_spawn` creates one governed child of the caller. It requires `agent_name` and a
positive `ttl_seconds`; permission arrays only narrow the parent. Omitted lists grant nothing unless
`inherit_permissions: true` keeps the parent's grant for each omitted list; `toolsets` adds the
parent-granted members of named toolsets. `notify_creator` defaults to true. `auto_stop_on_parent`
defaults to true, also for a child in another workspace: the spawn reaper stops the child when its
parent stops. The stopped child stays resumable; resuming it after the parent stopped releases that
binding, and the operator can take it over with `compozy__session_reparent` `top_level: true`. `provider`, `model`,
`reasoning_effort`, `speed`, and `acp_options` override the agent's runtime.

`workspace` targets another registered workspace when the access policy allows it. `worktree` binds
a ready worktree of that workspace; `new_worktree: {name?, base_ref?, branch?}` creates and binds
one (mutually exclusive). The workspace is authorized before any worktree is touched. The checkout
becomes the child's cwd and provider sandbox root, and a worktree created for a refused spawn is
removed. Use this instead of `workspace_paths`, which only records a narrowing atom.

`prompt` sends the first prompt from the caller with `notify_on_complete` (message ID
`spawn-prompt-<child_session_id>`), so the reply wake carries the child's final answer. The result
returns the child ID, role, depth, TTL expiry, effective `permissions`, and optional `worktree` and
`prompt` objects; a failed first prompt returns `prompt.status: "failed"` with `error`, and the
child stays live for a `session_prompt` retry. Refusals are structured: `operator_cause` names the field, value, and rule
(`subset_of_parent`, `blank_atom`, `known_toolset`) and `operator_recovery` the next step. A
live-child cap fails with reason `spawn_budget_exhausted` and names the cap and live count; stopped
children and subagents hold no slot. A depth refusal uses the same reason with cause `max_depth`:
a spawned child cannot spawn governed children, so delegate a subagent instead. A `model` override
needs `provider` too; the refusal says so. Check `limits.can_spawn_children` in your situation
context before spawning.

Subagent tools, also in the `sessions` toolset: `compozy__subagent_capabilities`,
`compozy__subagent_delegate`, `compozy__subagent_status`, `compozy__subagent_cancel`. A subagent is a
delegated child session owned by the caller's current turn. Every subagent tool requires the caller to
have an active turn and otherwise returns `parent_not_active`. Use them instead of chaining
`session_spawn` → `session_prompt` → `session_wait` → `session_history` when you want one task answered
by another agent, provider, or model while you keep working:

1. `compozy__subagent_capabilities {}` returns the caller's inherited runtime, permission mode, depth,
   live subagent count, and per-agent and per-provider `can_delegate` with `constraints` and advertised
   models. Pick a target whose `can_delegate` is true. Only delegable providers include models,
   capped at 40 per provider with the caller's current model first, then the provider default.
   `models_total` counts the full catalog; `models_truncated` indicates omitted entries (including
   those hidden for unavailable providers). Any model id advertised by the provider is accepted
   even if absent from this preview; delegation validates against the full catalog.
2. `compozy__subagent_delegate` takes a required, self-contained `task` (≤ 120,000 characters). The
   subagent sees only `task`: no conversation history and no attachments, so include every fact it
   needs. Optional `title` (≤ 512), `role` (`general`, `implementation`, `research`, `review`, `design`,
   `test`), `target` (`agent`, `provider`, `model`, `reasoning_effort`, `speed`, `acp_options`; omitted
   fields inherit from the caller, and the model is inherited only when the provider is unchanged),
   `mode`, `timeout_ms`, `idempotency_key` (defaults to the tool call id), `permission_mode`, and
   `tools`/`skills`/`mcp_servers`/`workspace_paths`. Unlike `session_spawn`, omitted permission lists
   inherit the caller's full budget, `[]` means none, and every value can only narrow
   (`permission_escalation_denied` otherwise). There is no TTL, depth, or count cap.
   `isolation` defaults to `shared`. Choose `worktree` for a separate branch; `base_ref` is valid only
   with `worktree` and defaults to the caller checkout's current HEAD commit. Uncommitted changes
   are not copied. Include "commit, then deliver with `compozy worktree deliver`" in a code task.
   Reusing an idempotency key requires the same isolation and requested base ref. Worktree/setup
   hook denial returns `capability_denied`; creation/setup failure returns `isolation_failed`.
   The first prompt names the isolated branch and delivery command. `spawn.pre_create` exposes
   read-only `subagent.isolation` and `subagent.worktree_id`.
3. Prefer `mode: "async"` (default): the call returns `status: "running"` with `subagent_id` and
   `child_session_id`. Keep working on steps that do not need the result; end your turn only when
   every remaining step waits on a registered subagent or monitor. Do not poll or wait in loops.
4. When the subagent settles, CompozyOS wakes you once per batch with a pointer message:
   `Subagent "<title>" (<subagent_id>) finished: <status>.` lines followed by
   `Call compozy__subagent_status to read its result.` (one subagent) or `… to read each result.`
   (several). The wake is steered into your running turn when your agent supports steering,
   otherwise queued ahead of user prompts. An isolated wake also names its branch and PR URL,
   or says that PR status is unknown. `subagent_status` includes `isolation` and optional `worktree`:
   id, name, branch, path, requested `base_ref`, pinned `base_sha`, and settlement Git/PR facts.
   Missing integers mean unknown, not zero. PR status `none` means the forge answered without a
   PR; `unknown` means unavailable or failed lookup. Shared subagents omit worktree facts.
5. `compozy__subagent_status {subagent_id}` returns the status, `work_state`, `result` (the child's last
   assistant message, capped at `[subagents].result_max_chars`, with `result_truncated` and a hint to read
   the rest through `compozy__session_history` on `child_session_id` with `type: "agent_message"`
   and `limit: 1`), `error`, `delivery`, and `child_activity` (`working`, `idle`, or `stopped`: the
   child session's own liveness, which can change after the status is terminal). Reading a
   terminal result acknowledges it and removes it from a queued wake, so the tool is idempotent but not
   read-only.

`mode: "wait"` blocks until the result or `timeout_ms` (default 600,000, clamped to 1,000–3,600,000).
A timeout returns `wait_timed_out: true`, never cancels the subagent, and switches it to a normal wake.
Use wait only for short tasks; some providers abort long tool calls.

Statuses are `queued`, `running`, `waiting` (the child needs approval or input), `completed`, `failed`,
`canceled`, and `interrupted`. A clean daemon restart is not a cancel: the subagent resumes and wakes you
when it settles, or settles `interrupted` (still waking you) when it cannot resume. A daemon crash
settles a running subagent `failed`. A waiting subagent does not wake you: the operator approves it, or you
call `compozy__session_approve` or `compozy__session_clarify_answer` on its `child_session_id` after
status reports `waiting`. `compozy__subagent_cancel {subagent_id, reason?}` returns `cancel_requested`
for a live subagent, otherwise its terminal status; Claude's own Agent/Task subagents
(`origin: "provider_native"`) return `subagent_not_cancelable`; a background one stays `running` until
its own task finishes, then reads `completed`, `failed`, or `canceled` with the task's summary as
`result` once Claude reports it (briefly `null` before that); one with no task report settles
`completed` with `result: null` when its turn ends. For another review round, delegate again
with the original brief, prior findings, and open objections instead of prompting the old child. A
subagent may delegate its own subagents. Subagents share your workspace and worktree, so give
concurrent subagents disjoint files or read-only tasks. Stopping your session cancels every live
subagent depth-first; interrupting a turn keeps its subagents running, but their results no longer wake
you. CLI and HTTP/UDS fallback for reads and cancel: `compozy session subagents <session-id>`,
`compozy session subagents show|cancel <subagent-id>`.

`compozy__session_stop` stops another same-workspace session and is destructive/approval-gated. It is
idempotent for an already stopped target. `compozy__session_prompt_cancel` cancels only another
session's active prompt and returns `canceled` or `nothing-in-flight` without stopping the session.

`compozy__session_approve` resolves another session's permission request with `allow-once`,
`allow-always`, `reject-once`, or `reject-always`. `compozy__session_clarify_answer` resolves another
session's pending question with either a one-based `choice` or text. Both preserve durable
post-restart interactions and can return `already-resolved`, `resolved-after-restart`, or
`queue-full`; self-action is denied. Read `compozy session interactions` or
`compozy__session_status` for the durable request IDs instead of matching display text.

`compozy__session_rewind` is a destructive conversation-only mutation. Pass the durable user
`message_id`, an idempotency key, and the transcript epoch, generation, and maximum sequence returned
by the read API. The tool cuts before that message, restarts a fresh ACP context under the same
CompozyOS session ID, and returns the selected text as `draft_text`. It never rolls back files, tool or
network effects, or external provider actions. Resolve its descriptor and obtain approval
before calling it.

`compozy__session_continue` (risk `mutating`, same permission as `compozy__session_create`) starts a
new user session for `agent` with the source conversation carried over; the source is never changed.
Required: `session_id`, `agent`, `idempotency_key`. Optional: `workspace`, `message`, `name`, `runtime`
`{provider, model, reasoning_effort, speed, acp_options}` **or** `route` (1-based declared route of the
agent's `fallback_chain`), and the three transcript fences together. The result is `{session,
derived}`; repeat the same call with the same key to read the recorded outcome (`derived.replayed`,
`derived.child_deleted`) instead of creating a second session, even after the source was deleted. A
source outside the resolved workspace is `session_not_found`. A failure carries the HTTP error payload
(`code`, and `child_session_id` when the session was already created before the failure) as the tool
error's partial result: open that session or retry with the same key.

`compozy__session_fork` (risk `mutating`, same permission and result shape as
`compozy__session_continue`) forks a user session with the **same** agent, runtime, and account.
Required: `session_id`, `idempotency_key`. Optional: `workspace`, `message_id` (fork through that
durable user message and its turn; the whole session when omitted), `name`, and the three transcript
fences together. An unsettled cut turn fails with `session_turn_in_progress`; an unknown message with
`message_not_found`. `derived.seed` is `native_fork` (with `native_state: pending` and the clone's
`acp_session_id`) when the agent cloned its own session, otherwise `replay`; a failed clone request is
reported in `derived.native_fork_error`.

`compozy__session_compact` (toolset `compozy__sessions`, risk `mutating`, **experimental**) requests the
agent's advertised native compaction on one idle session in the caller's workspace. Pass `session_id`
(required; no other input); the result is `{session_id, prompt_id, command, status: "accepted"}`, where
`command` is `compact` or `compress`, the command the agent advertises. The compaction turn is sent as
`/<command>` in maintenance delivery mode, so it carries no skill expansion, augmenters, or startup
instructions. The tool returns the receipt only and never waits. A running turn (the caller's own session
is always mid-turn) or another in-flight request fails with the structural tool error code `session_busy`,
and an agent advertising neither command fails with `compaction_unsupported`; each error carries that
`code`, the tool ID, and the daemon message, the same codes the CLI, HTTP, and UDS error payloads use, and
neither is the generic `tool_conflict`; over `POST /api/tools/{id}/invoke` both are `409` responses carrying
those codes. A session that is not active fails as invalid input. Its
`session.compaction.requested` event records `requested_by: "tool"`. Observe the outcome through the
session's transcript Compaction item and `compactions[]` usage markers (`compozy session history` returns
the raw ledger rows instead); the CLI form, `compozy session compact <session-id>`, waits and prints the
outcome.

`compozy__session_runtime_set` persists complete next-prompt intent without starting or
reconfiguring ACP; `compozy__session_runtime_clear` removes it. Both accept optional
`expected_revision`; when omitted, the tool reads the current `runtime.selection_revision` before
the write. On conflict, read session status and decide from the new server value instead of replaying
the stale mutation.

Worktree tools: `compozy__worktree_list`, `compozy__worktree_inspect`,
`compozy__worktree_create`, and `compozy__worktree_remove`. List and inspect are read tools; create is
mutating; remove is destructive and approval-gated, and its opt-in `delete_branch` input also deletes
the local branch with the safety rules in `references/worktrees.md`. Remove takes `path` instead of
`ref` for a discovered (not adopted) checkout, exactly as `worktree_list` reports it; that form keeps
the branch and rejects `ref` and `delete_branch`. All carry an explicit or caller-resolved
workspace boundary. Adopt, creation cancel, exit planning/actions, exit cancel, and dismiss are
CLI/HTTP/UDS-only; read `references/worktrees.md` before using either path.

Remembered approvals: `compozy__tool_approvals_set`, `compozy__tool_approvals_list`, and
`compozy__tool_approvals_revoke`. `allow-always` or `reject-always` creates an exact profile +
workspace + agent + tool + input-digest decision. Explicit set accepts only `agent` or `tool` scope
and no input digest; an agent-wide set requires `agent_name`. Wider allows remain below the
configured tool-policy ceiling. List and revoke use the acting session's immutable profile, or the
root `--profile` selection outside a session. CLI:
`compozy --profile <profile> tool approvals set|list|revoke --workspace <workspace>`.

Clarification: `compozy__clarify` asks one active-session question with at most four choices and returns
zero-based `{choice,text,fallback}`. It is not approval. CLI: `compozy session clarify pending|answer`
(choice presentation is one-based).

`compozy__session_list` accepts `board_column: ["needs_you", "working", "waiting", "in_review", "done"]`
and `children: "exclude"` (default `include`). Columns filter before pagination/counting;
unknown values return `tool_invalid_input`. Excluding children removes every session with a parent,
independent of its spawn role. Page payloads include the derived `board_column`, direct-child
`children` counts, and scoped `parent_title`; `waiting_on` explains the `waiting` badge. These
filters participate in the cursor fingerprint. PR facts are display-cache reads, without forge I/O.

`compozy__session_list` returns one counted catalog page and accepts workspace, exact state, exact
session `type`, exact agent, exact `parent` (direct children) or `root` (whole tree, root included),
search, resumability, health, archive visibility, sort, cursor, and limit inputs. Archive visibility defaults to `exclude`; use `only` or `include` when the workflow needs
archived rows. Use `type: "user"` when a workflow needs operator-created sessions without
daemon-managed system, coordinator, or spawned sessions. `compozy__session_archive` archives a
stopped session; a live session is refused unless the input sets `stop: true`, which stops it
through the normal stop path (canceling any active turn and cascading to subagents) and then
archives it. A failed stop leaves the session unarchived with code `session_archive_stop_failed`.
The CLI fallback is `compozy session archive <id> [--stop]`. Unarchive before prompting or
resuming a session. `compozy__session_later` moves a session to Later: it stops the session and
its hierarchy children (forks and continues are not children), cancels their monitors and pending
wakes, terminates their processes and terminals, and parks them with an optional one-line `reason`
and optional `return_at` that brings the session back into Needs you with a notification. Parking
the caller or one of its ancestors is refused as a self-target. `compozy__session_list` excludes
parked sessions unless `later` is `only` or `include`. A `compozy__session_prompt` to a parked
session queues the message with disposition `later` and does not resume it; only the operator
resumes it. `compozy__session_resume_later` brings it back without starting it (`with_children`
also brings back parked children). CLI fallbacks: `compozy session later <id>` and
`compozy session resume-later <id>`. Archive and conversation clear preserve attachments;
conversation clear preserves attachments. `compozy session remove` and `compozy workspace remove`
remove their scoped attachment trees.
`compozy__session_rename` changes the durable display name of a user session without changing its
runtime or transcript identity. Pass `session_id` and a non-empty `name` of at most 64 characters;
the CLI fallback is `compozy session rename <id> <name>`.
`compozy__session_reparent` nests a top-level user session under another user session in the same
workspace and profile (`parent_session_id`), or makes an adopted child top-level again
(`top_level: true`); set exactly one. The session must have no parent to be adopted, the parent
must be a user session that is not the session or one of its descendants, and neither may be
archived. Subagent, continued, forked, and recovery sessions keep their lineage. The new
lineage kind is `provenance`; the subtree's `root_session_id` and `spawn_depth` follow, and no
governance field changes. `top_level: true` on a stopped spawned worker (not a subagent) takes it
over: it becomes a root `type=user` session and its spawn TTL, parent auto-stop, creator wake,
spawn budget, and permission narrowing end, so it gets the normal user-session controls. Only an
operator call may take over: an agent caller is refused with `operator_required`, because ending
another session's governance is the operator's decision. A worker that is not stopped with a
verified process exit is refused with `source_running`, and daemon roles such as `auto-title` with
`source_not_user`. Refusals carry a `session_reparent_*` code (`source_has_parent`,
`source_not_user`, `parent_ineligible`, `cycle`, `archived`, `not_detachable`, `source_running`,
`operator_required`). The CLI fallback is
`compozy session reparent <id> --parent <parent-id>` or `--top-level`.

Authored context tools: `compozy__agent_heartbeat_status`, `compozy__agent_heartbeat_wake`.

Workspace tools: `compozy__workspace_list`, `compozy__workspace_info`, `compozy__workspace_describe`,
`compozy__agent_list`, `compozy__agent_create`. `compozy__agent_list` returns exact names from the
resolved workspace catalog. `compozy__agent_create` authors one public `AGENT.md` at `global` or
`workspace` scope; provide `scope`, `name`, `prompt`, and `workspace` for workspace scope. Provider,
model, and reasoning are optional agent-level overrides; when omitted, the definition inherits the
target project runtime defaults. Optional `fallback_chain` is an ordered array of routes
(`provider` and `model` required; `reasoning_effort`, `speed`, `acp_options`, and `command`
optional). A route `command` selects the account with the same grammar as `providers.<name>.command`
(leading `NAME=value` assignments are forwarded literally; use absolute paths, no `~`). An unknown
route provider or a command without an executable is rejected as invalid input. The returned agent
payload carries `fallback_chain` with `command_fingerprint` (`sha256:`) on routes that set `command`.

Fresh daemon boot does not register a workspace automatically, so `compozy__workspace_list` may return an empty catalog on a clean install. Register the intended project explicitly before starting workspace-bound work.

`compozy__workspace_archive` and `compozy__workspace_unarchive` take `workspace` and preserve the
project ID, history, layouts, worktrees and configuration. Archive refuses non-stopped sessions,
nonterminal Loops and enabled automations with `workspace_busy`; inspect the returned blockers,
stop or disable them, then retry. Unarchive has no preconditions. New sessions and resumed sessions
in an archived project fail with `workspace_archived`; history remains readable.
`compozy__workspace_list` excludes archived projects by default; `archived: true` lists only archived
projects. CLI parity: `compozy workspace archive|unarchive <name-or-id>` and
`compozy workspace list --archived`. HTTP/UDS parity: `POST /api/workspaces/{id}/archive|unarchive`
and `GET /api/workspaces?archived=true`. Default All projects aggregates omit archived projects.
A registered project whose folder is unavailable stays listed with `root_missing: true`; listing
never deletes its history. Restore the folder to clear the flag. History reads, Archive and Remove
remain available; new sessions fail with `workspace_root_missing`. Only explicit Remove deletes
the registration and stopped history.

A successful workspace catalog read reconciles registered roots before returning: entries whose directories no longer exist are durably unregistered, while other filesystem or deletion failures fail the read instead of hiding uncertain state. `compozy__workspace_list`, `compozy workspace list`, and HTTP/UDS `GET /api/workspaces` share this catalog. A bound session's `compozy__workspace_list` adds every workspace its permission mode already lets it address (never prompting); pass one to a tool's `workspace` input.

The workspace setup browser reads `GET /api/fs/browse`; its response includes `home` and every
filesystem `root` so the UI can navigate outside the operator home. Agents should register a known
path through the workspace management surface instead of browsing interactively.

Workspace unregister is atomic with credential cleanup: it removes workspace-scoped MCP OAuth rows and their encrypted access/refresh values, preserves global and sibling-workspace credentials, and leaves all state intact when cleanup fails.

Provider model tools: `compozy__provider_models_list`, `compozy__provider_models_curate`, `compozy__provider_models_refresh`, `compozy__provider_models_status`.

Provider usage tools (experimental, toolset `compozy__provider_usage`): `compozy__provider_usage_list`
(optional `provider_id`) is read-only and returns the cached envelope for the session's own profile
without contacting providers. `compozy__provider_usage_refresh` (optional `provider_ids`,
`max_age_seconds`) is mutating, always runs with background intent, waits up to 15 seconds, and
returns the same envelope. Agent refreshes honor provider rate-limit cooldowns. With existing operator consent,
they may reread the exact routed macOS Keychain item through `security` after a noninteractive
ACL/partition/unlocked-state check. Unsafe metadata prevents the secret read. A concurrent item
change can still briefly prompt; a 1.5-second deadline kills the reader and reports
`keychain_read_timeout` / `needs_consent`. Agents cannot grant missing consent or
OS authorization, and
`compozy__config_set` refuses `provider_usage.providers.claude.keychain_access`: report it to the
operator. CLI fallbacks are `compozy provider usage list|refresh`; statuses and recovery are
in `references/runtime-operations.md#provider-usage-experimental`.

Vault catalog tool: `compozy__vault_list`. It returns global reference names and redacted metadata,
optionally filtered by prefix. It never returns secret values.

Gateway tool: `compozy__gateway`, in toolset `compozy__gateway`. Resolve its live descriptor before
calling it. `status`, `audit`, and `device_list` are read actions and remain callable in every
session permission mode while the Gateway service is available. Management actions are
`surface_set`, `provider_enable`, `provider_disable`, `device_rename`, `device_revoke`,
`ingress_bind`, and `ingress_unbind`. A local operator may call them directly; an agent session must
have effective permission mode `approve-all`. Lower modes return `tool_denied` with
`policy_denied`. The tool returns only redacted projections and intentionally has no pairing,
credential, or stream-ticket issuance action. Use the local CLI or private management API when a
one-time secret must be issued.

`compozy__provider_models_list` accepts `view=curated|all` and defaults to curated; the CLI equivalents are `compozy provider models list` and `compozy provider models list --all`. `compozy__provider_models_curate` is mutating, requires `providers.models.write`, and accepts required `provider_id`/`model_id` plus optional `hidden`, `featured`, `deprecated`, `default_effort`, and `default_speed`. Speed accepts `normal` or `fast`; explicit model configurations that omit Fast reject it with `speed_rejected`. Its CLI fallback is `compozy provider models set`. Treat `model_not_found`, `reasoning_effort_unsupported`, and `speed_rejected` as terminal input diagnostics; when the descriptor reports the settings backend unavailable, do not retry blindly.
For providers with an explicit curated set, the default view contains visible explicit or featured rows; live-only rows appear there only through the no-explicit-set fallback.
Claude, Codex and Agy catalog discovery starts a short-lived ACP inspection and stores its live
model and configuration options under `provider_live:<id>`. Cursor runs `cursor-agent models`, groups
transport aliases into logical models, and keeps valid Reasoning, Fast, and Thinking combinations plus
private launch bindings. OpenCode discovery detects the installed major with `--version`, then uses
`models --verbose` (v1) or `api --standalone GET /api/model` scoped to the workspace directory (v2);
the inherited launch prefers an installed `opencode acp` and uses the `npx -y opencode-ai@latest acp`
runner only when no native executable exists. Explicit commands keep their installation selector and
never gain an npm fallback; refresh re-detects the version and never replaces an active session.
Live sources refresh periodically and after their five-minute TTL; failed refreshes keep the last
successful rows as stale data. Built-in curated Claude IDs and all Cursor IDs without a live transport binding less than ten
minutes old remain
listed but report `available=false`, `availability_state=unknown`, and a `last_error` beginning
`live catalog unavailable:`; do not select those rows for a session. Live discovery defaults to
60 seconds independently of models.dev; `providers.<id>.models.discovery.timeout` overrides it.
An explicit curated set may keep a new live model
out of the default view, so use `view=all` before concluding it is absent. Refresh through `compozy__provider_models_refresh` or
`compozy provider models refresh <provider>`.
Changing `providers.<id>.models.discovery.*` refreshes that provider source; model metadata-only
writes do not invoke the provider.
Model-list and curation results may include a `cost` object with independent `input_per_million`, `output_per_million`, `cache_read_per_million`, `cache_write_per_million`, and `reasoning_per_million` fields. A missing field means that bucket is unpriced; never infer it from another field.

Provider authentication is a management surface. Write `providers.<id>.auth_login_command` only through `config.toml`, `compozy config set`, or `compozy__config_set`; it is write-only and redacted from config show, list, get, diff, and set reads. Provider status, doctor, API/UDS, Settings, and Web expose for it only `{configured, source, executable, presence, recommended_action}`, where `executable` is the basename. `compozy provider auth login <provider>` executes the configured login command internally and never returns the raw command.

Providers with `auth_acp_method` and no native login command (the `agy` built-in uses `oauth-personal`) log in through the exact advertised protocol method instead: login verifies readiness with a promptless session, remote probes never send `authenticate`, and read surfaces report `source: auth_acp_method` with basename and presence only. Never accept a key, token, or secret for these providers; a missing server is `install_cli`, not a credential prompt.

## Profile Tools

`compozy__profile_list` returns the active and archived profile catalog and marks the profile bound to
the caller session, or the permanent default outside a bound session.
`compozy__profile_current` returns that immutable session profile with
`source: "session"`; outside a bound session it returns the permanent default with
`source: "default"`. Both are read-only catalog tools with empty input. Resolve their live descriptors
before calling them.

Profile selection and lifecycle mutations remain management surfaces. Use `compozy profile`, local
HTTP/UDS `/api/profiles` routes, the desktop, or the stable `profile.*` command-palette actions. Remote
profile-state writes are forbidden. Read `references/profiles.md` before changing profile state.

## Workspace Boundary

Registered IDs, durable workspace IDs, names and paths resolve to the same registered project before
policy and dispatch. Naming the current project does not require cross-workspace approval, and
native automation operations use the same project registration as CLI and HTTP/UDS reads.

`"@user"` is parsed before registry lookup on every descriptor with a `workspace` input and selects
the User scope of the call's profile. It is never a registered workspace, name, or path. User is a
scope of its own: a User session's own scope is User of its profile, and every registered project,
including folders inside the User work directory, is foreign to it. A project session that reads or
acts on User (lists User sessions, spawns into User, opens a User terminal) is equally cross-scope.
"Same scope" means the same workspace and profile, or User of the same profile; the same workspace
under a different profile is cross-profile and always reaches the policy. User has no worktrees:
worktree parameters with `@user` fail with `user_scope_worktree_unsupported`, and other project-only
operations fail with `user_scope_unsupported`.

Renamed workspaces (reserved names `user`, `global`, and names starting with `@`) keep their former
name as a selector with a deprecation warning through v0.4.0; the aliases are removed in v0.5.0. Use
the workspace ID or the new name. On data tools `global` is not a workspace selector
(`workspace_reserved`) unless it is such a migration alias.

A call that names a workspace other than the bound session's is a cross-workspace request. An effective `workspace_access: unrestricted` grant allows it with audit source `agent_grant`; otherwise the session's effective permission mode decides:

- `approve-all`: allowed at every seam.
- `deny-all`: denied at every seam, including native tools. CompozyOS never prompts for crossing under this mode.
- `approve-reads`: the native-tool seam prompts the operator while the session is running; the agent-identity, task, spawn, and workspace-coordination seams deny.

Denied native calls return `workspace_access_denied` with this exact hint:

    cross-workspace access is denied by this session's permission mode; ask the operator to set the agent's permissions.mode to approve-all, or approve the prompt when asked

The prompt offers four daemon-computed options: `allow_once` (this call), `allow_session` (this call and every later crossing by this session, at every seam), `reject_once` (this call), and `reject_session` (this call and every later crossing by this session). A `*_session` answer lives only in daemon memory, expires when the session stops or the daemon restarts, and has no list or revoke surface. A prompt timeout, transport failure, or unknown answer denies and stores no session answer.

Granted agents may also select any active `profile`, use `all_profiles` on reads, and read aggregate routes. Work a granted agent creates in profile P is owned by P; the acting session's tool-approval grants apply. Tool-risk approvals still follow the session's `permissions` mode. Desktop tools remain restricted to project partitions and the caller profile's `user` partition; `global`, another profile's partitions, presence leases, and notification receipts stay operator-only. The full authority matrix and the operator-only authoring rule are in `references/agent-definitions.md` (Unrestricted Workspace Access). User Scope Work ADR-003 supersedes cross-workspace ADR-007 §1 for this one operator-authored field.

Each policy evaluation attempts a best-effort `workspace.access_granted` or `workspace.access_denied` audit append with actor/target profiles, target kind/workspace, seam, decision source, and mode. A prompt-eligible miss is a denied policy evaluation even when the operator then allows that one call; later evaluations allowed by `allow_session` are granted. Read persisted events through `compozy__logs`, `compozy__observe_search`, `compozy logs --type <event-type>`, or `GET /api/logs`.

On denial, report the hint to the operator and stop that line of work. Do not retry the same call, re-enter through another seam, spawn a child to cross for you, or edit `permissions.*` in config or an agent definition. `compozy__config_set` on `permissions.*` fail-closes; the operator owns the mode.

## Window Management Tools

Toolset `compozy__window_manager` requires `window_manager.read` for reads and `window_manager.write`
for mutations. Resolve descriptors, pass the workspace, and use the current revision.

- Desktops: `compozy__desktop_list`, `compozy__desktop_create`, `compozy__desktop_update`,
  `compozy__desktop_reorder`, `compozy__desktop_switch`, `compozy__desktop_delete`,
  `compozy__desktop_clients`.
- Windows: `compozy__window_list`, `compozy__window_open`, `compozy__window_close`, `compozy__window_focus`,
  `compozy__window_move`, `compozy__window_swap`, `compozy__window_float`, `compozy__window_zoom`,
  `compozy__window_navigate`.
- Tabs: `compozy__window_group`, `compozy__window_reorder`, `compozy__window_activate`,
  `compozy__window_pin`, `compozy__window_reopen`.
- Layouts: `compozy__layout_get`, `compozy__layout_preview`, `compozy__layout_arrange`,
  `compozy__layout_resize`, `compozy__layout_balance`, `compozy__layout_undo`, `compozy__layout_redo`,
  `compozy__layout_export`, `compozy__layout_validate`, `compozy__layout_apply`.

For operator calls, these descriptors accept `workspace: "global"` before registry normalization,
selecting the acting profile's reserved All projects partition. Every agent identity, including
agents with the unrestricted grant, is denied on it. `workspace: "@user"` selects the acting
profile's reserved `user` partition. Neither makes `global` a valid workspace on other tools. Read
`references/window-management.md` (Reserved partitions) for bindings and reconciliation.

Every tab tool mutates and needs `window_manager.write`. `window_group` takes `target_window_id` plus
a `window_ids` array and an optional `insert_index`; `window_reorder` moves one member inside its own
stack with a clamped `index`; `window_activate` sets the stack's durable active member and is the
public name of the internal `window.stack.set_active` command; `window_pin` takes a required `pinned`
boolean; `window_reopen` needs only the revision. Three existing tools carry tab inputs:
`window_open` accepts `stack_target_window_id` (plus `floating` for an explicit floating window;
it rejects `insert_tiled` or `stack_target_window_id` alongside it), `window_navigate` accepts `mode`
(`replace`/`push`/`pop`, and `pop` forbids `route`), and `window_close` accepts `scope`
(`tab`/`group`/`others`/`right`, rejected together with `minimize`).

`desktop_switch`, `window_focus`, and `window_zoom` require a connected `client_id`.
`window_navigate` changes presentation only when given one. Preview/validate never write;
desktop delete, window close, and layout apply carry destructive risk. Discover `window_layout`
resources with `compozy__resources_list`; `resource_id` is exclusive with inline arrange fields. CLI
fallbacks are `compozy desktop|window|layout`; read `window-management.md` for multi-step changes.

## Terminal Tools

Toolset `compozy__terminal` contains `compozy__terminal_exec`, `compozy__terminal_open`,
`compozy__terminal_write`, `compozy__terminal_read`, `compozy__terminal_wait`,
`compozy__terminal_signal`, `compozy__terminal_close`, `compozy__terminal_list`,
and `compozy__terminal_request_input`. Resolve each descriptor before calling it. Read `terminal.md`
for activation, approval, shared-input, generation, capability, error, and CLI-fallback rules. To wait
on a command, inside or outside the turn, pair `terminal_exec` with `compozy__monitor_create`.

## Command And Skill Tools

Command catalog tool: `compozy__command_list`. Pass `session_id`; optional `workspace` defaults to the
bound session workspace. The result is the daemon-owned command projection for that session.

Skill tools: `compozy__skill_list`, `compozy__skill_search`, `compozy__skill_view`. Use `name` for a
normal registry read. Use exactly one source-qualified `command_id` from `compozy__command_list` when
the operator selected a slash skill; `command_id` requires a session-bound caller and never switches
to another skill source.

Resolve canonical `compozy__skill_view`, then use its returned tool reference with a file/resource argument when reading `skills/compozy/references/*.md` from inside CompozyOS.

## Task And Autonomy Tools

Task tools: `compozy__task_list`, `compozy__task_read`, `compozy__task_create`, `compozy__task_child_create`, `compozy__task_update`, `compozy__task_cancel`, `compozy__task_fanout_runs`, `compozy__task_run_list`, `compozy__task_run_result`, `compozy__task_run_review_request`, `compozy__task_run_review_list`, `compozy__task_run_review_show`, `compozy__task_execution_profile_get`, `compozy__task_execution_profile_set`, `compozy__task_execution_profile_delete`, `compozy__task_worktree_policy_set`.

Session-bound autonomy tools: `compozy__task_run_claim_next`, `compozy__task_run_heartbeat`, `compozy__task_run_complete`, `compozy__task_run_fail`, `compozy__task_run_release`, `compozy__task_run_review_submit`.

Autonomy tools are bound to the caller session. `compozy__task_run_claim_next` claims and starts a worker run; a different returned `session_id` means execution moved to that dedicated session and the caller must end its turn. Do not substitute general task mutation tools for session-bound lease operations. Read references/tasks-and-orchestration.md before claiming, heartbeating, completing, failing, releasing, or submitting review verdicts.

## Loop Tools

Toolset `compozy__loops` has 29 tools: Goal get/control/report; Loop
list/inspect/validate/create/run/status/runs/turns/cancel/pause/resume/configure/approve/delete;
node list/pause/resume/cancel/requeue; request list/get/respond/amend; and diff/rerun/fork.

No `compozy__loop_edit`. See references/loops.md for publishing, approval/self-approval, and Goal report
binding semantics.

## Config, Hooks, Automation, Marketplace, Extensions, Resources, And MCP Tools

Config tools live under `compozy__config_*` for show/list/get/set/unset/diff/path. Hook tools live under `compozy__hooks_*` for list/info/events/runs/create/update/delete/enable/disable; hooks are typed dispatch, not an event bus.

Native `tool.pre_call` hooks run before the handler and may deny or amend input, but cannot change
the tool identity, read-only classification or bound workspace. Patched input still passes schema,
policy and approval checks. Native `tool.post_call` receives the canonical tool-result envelope
(`content`, `structured`, `preview`, and result metadata) in `tool_result`; a replacement uses that
same shape. `tool.post_error` may annotate the error while retaining its classification. Hosted
native calls record hook runs and lifecycle events in the owning session.

Background-role inspection has no `compozy__roles_*` native tool. Use `compozy roles list|show -o json` or
the HTTP/UDS `GET /api/roles` reads. Scalar `roles.<role>.*` routing and role-policy keys are exposed
through the live `compozy__config_set`/`compozy__config_unset` descriptors, including coordinator limits.
Fallback chains are structured arrays and must be changed through
`config.toml` or the Settings Roles API/UI, not guessed into a scalar config-tool call. Inspect the
live descriptor before any mutation; successful role writes report the `live` lifecycle and affect
later invocations.

Automations (jobs and triggers): the Web UI shows both in one Automations window — a job is a
scheduled automation, a trigger an automation on an event, a webhook trigger a link automation.
Tools, CLI verbs, routes, and config keep `job`/`trigger`; never send `automations` to them.
Automation catalogs use CLI, HTTP/UDS, and `compozy__automation_jobs_list` / `compozy__automation_triggers_list`.
Their counted cursor pages filter by scope/workspace, source, enabled, Loop target, `target`
(`agent`|`loop`|`task`; `task` matches no trigger), search, and event;
run history stays uncounted and must be bounded. Continue with the returned opaque cursor under
unchanged profile scope and filters; changing either starts a new first page. Other `compozy__automation_*` tools cover detail,
mutation, toggles, and manual trigger. Config/package definitions only toggle enabled and cannot be
deleted; dynamic definitions are fully mutable. List and get items carry `last_run` (`id`, `status`,
`started_at`, `ended_at`, `skip_reason` for a skipped fire stored as `canceled`), equal to the first
run of the owner's history and absent when it never ran; read it to answer "did it work?" without a
history call.

`compozy__automation_suggestions_{list,accept,dismiss}` accepts optional `workspace`; list, accept, or
dismiss; retry CAS conflicts.

`compozy__marketplace_search` returns extension catalog rows and rejects the retired `kind` argument.
Cursors bind the query, scope, workspace, and source revision. Paging and installed
identity rules live in `references/tools-and-skills.md`. Installed state is scoped to the caller's
exact workspace; never reuse it across workspace scopes.
Extension tools are
`compozy__extensions_{init,build,validate,dev,reload,logs,list,info,inventory,preview,search,provenance,publish,install,update,remove,enable,disable}`.
`validate`, `logs`, `list`, `info`, `search`, and `provenance` are read-only; `init`, `build`, `dev`,
and `reload` are mutating, with `build`, `dev`, and `reload` requiring interaction; `remove` is
destructive; `publish` is open-world, interaction-gated, and takes no credential field because the
daemon resolves and redacts that token server-side. `compozy__extensions_search` spans the `curated`
and `github` sources only and is not `compozy__marketplace_search`. Resolve live schemas first;
install sources, consent, authoring, workspace binding, generation handles, commit boundaries,
batch-update outcomes, kit lifecycle, and cleanup warnings live in `references/extensions.md`.

The `compozy__automation_jobs_create` and `compozy__automation_jobs_update` descriptors expose the complete recurring schedule shape, including `catch_up_policy` and `misfire_grace_seconds`. Resolve the live descriptor instead of guessing the enum or sending catch-up fields to a one-time `at` schedule.

`compozy__automation_jobs_create` and `compozy__automation_jobs_update` reject Agent prompts and Task descriptions containing command-shaped CompozyOS daemon restart, stop, or kill instructions before persistence, including resource-applied definitions. The tool error names `compozy_daemon`, `process_signal`, or `service_manager`; remove the lifecycle command before retrying. There is no bypass.

Resource tools live under `compozy__resources_*` for list/info/snapshot of desired-state resources.

MCP diagnostics are `compozy__mcp_status` and `compozy__mcp_auth_status`. A
workspace-scoped server with five consecutive confirmed permanent failures reports `state: "dead"`
from `compozy__mcp_status`; its nested runtime reason is `backend_dead`. During the same daemon lifetime,
resolve its last-known tools through `compozy__tool_info`, which retains their unavailable descriptors
and diagnostic instead of hiding them. Do not retry a dead tool blindly or invent a revive call: CompozyOS
admits at most one automatic recovery probe after the 60-second window and clears the mark when that
probe succeeds. Browser/OAuth login, raw auth material, and any required credential repair remain
management-surface operations.
Acquire Marketplace MCP capabilities through extension installation. Manage manual servers
through Settings MCP APIs; extension-owned management and diagnostics require the explicit
extension owner. Browser/OAuth login remains a management operation.

## Observability Tools

Runtime log inspection is available through `compozy__logs`. Logs and `compozy__observe_search`
accept `workspace: "@user"` to read only User events in the selected profile. The CLI equivalent is
`compozy logs --workspace @user --type workspace.access_granted`. Metrics and redacted event search are available through `compozy__observe_metrics` and `compozy__observe_search`.

## CLI/HTTP-Only Management Surfaces

CLI/HTTP/UDS owns diagnostics (`compozy status`, `compozy doctor`), session repair, recap, approval,
inspection, SOUL refresh, task controls, schedulers, config reload/history, and support bundles.

## Descriptor Discipline

This reference gives the stable map. The live descriptor gives exact input schema, output shape, risk flags, availability reason codes, and policy/dependency diagnostics.

If a descriptor is unavailable or denied, do not retry blindly. Choose a narrower tool, read-only status path, or CLI/control surface based on the reason code.

## Operator notification acknowledgement

The bell and Home expose durable notification acknowledgement independently of source work state.
The additive HTTP/UDS routes are operator surfaces; no new native tool or CLI verb is introduced.

- `GET /api/notifications/attention?profile=<destination>` reads all workspaces and source profiles.
  Its exact counts and `snapshot` cover every unread occurrence; `items` contains the first 100.
  The selected destination profile and operator identity own the receipts.
- `POST /api/notifications/attention/acknowledge?profile=<destination>` accepts
  `{ "snapshot": "<returned snapshot>", "id": "<occurrence>" }`. Omit `id` for all snapshot members.
  Repeated requests are safe; later occurrences are excluded. Writes are atomic.
- Home uses `GET /api/observe/overview`'s `attention.snapshot`. Submit the acknowledgement with
  `surface=home` and the same `workspace`, `profile`/`all_profiles` and `receipt_profile` values.
  Empty workspace selects global tasks. `receipt_profile` identifies the receipt owner when the
  view aggregates source profiles; it is operator-only.
- `204` confirms the receipts committed. Refresh both notification and overview reads. `409` means
  the snapshot expired (24-hour lifetime), is unavailable or belongs to another population; refresh
  before retrying. Other failures leave the requested set unacknowledged and must remain visible.

Acknowledgement never approves, rejects, retries, cancels, deletes or completes source work. Pending
requests remain reachable in their owning app. Session badges and attention-summary still represent
runtime state; bell/title counts represent unread notifications. Task candidates honor existing inbox
triage. An acknowledged occurrence stays hidden across reconnects; a new source revision/run/request
can notify again.

## All-Projects Aggregates

All projects means every registered project plus User rows (`workspace_id` absent or empty), within
the current profile lens. It never widens the profile lens; that remains `profile=` or
`all_profiles=true`. Aggregates are operator surfaces: ordinary agent identities get
`403 agent_scope_denied`, sessions with an effective unrestricted grant are allowed and audited with
target kind `aggregate`, and the new aggregate routes deny app tokens. Native list tools keep their
workspace-bound inputs and gain no aggregate parameters, so use the CLI or HTTP/UDS for these reads.

Breadth vocabulary, the same on every route (each route keeps its existing selector, omission
population, and envelope):

- `all_workspaces=true`: every project plus User rows.
- `workspace_ids=<id>`: repeatable, only with `all_workspaces=true`; narrows the aggregate. `@user`
  selects User rows. Values are trimmed, deduplicated, and sorted; an empty value fails with
  `workspace_breadth_invalid`, `global` with `workspace_reserved`.
- `all_workspaces=true` combined with the route's single-project selector or a workspace path, or
  `workspace_ids` without it, fails with `400 workspace_breadth_invalid`.
- Cursors embed breadth, filter, and profile. Reusing one under another selection fails with the
  route's existing query-invalid error; restart paging instead.

| Read                                  | CLI (`--all-workspaces`, exclusive with `--workspace`) | HTTP/UDS                                                                                     |
| ------------------------------------- | ------------------------------------------------------ | -------------------------------------------------------------------------------------------- |
| Sessions                              | `compozy session list --all-workspaces`                | `GET /api/sessions?all_workspaces=true`, facets add `by_profile`                             |
| Tasks                                 | `compozy task list --all-workspaces`                   | `GET /api/tasks?all_workspaces=true` (same population as operator omission; rejects `scope`) |
| Task dashboard, Home overview         | —                                                      | `GET /api/observe/tasks/dashboard`, `GET /api/observe/overview` with `all_workspaces=true`   |
| Loop runs (Goals: `--origin session`) | `compozy loop runs --all-workspaces`                   | `GET /api/loop-runs?all_workspaces=true`                                                     |
| Loop requests                         | `compozy loop requests --all-workspaces`               | `GET /api/loop-requests?all_workspaces=true`                                                 |
| Loop definitions                      | `compozy loop list --all-workspaces`                   | `GET /api/loops?all_workspaces=true`                                                         |
| Automation jobs, triggers             | `compozy automation jobs\|triggers --all-workspaces`   | `GET /api/automation/jobs\|triggers?all_workspaces=true`                                     |
| Automation runs                       | `compozy automation runs --all-workspaces`             | `GET /api/automation/runs?all_workspaces=true`                                               |
| Automation suggestions                | `compozy automation suggestions --all-workspaces`      | `GET /api/automation/suggestions?all_workspaces=true`                                        |
| Worktrees                             | `compozy worktree list --all-workspaces`               | `GET /api/worktrees?all_workspaces=true`                                                     |
| Terminals (local listener only)       | `compozy terminal list --all-workspaces`               | `GET /api/terminals`, `/api/terminals/catalog-stream` with `all_workspaces=true`             |
| Agents                                | `compozy agent list --all-workspaces`                  | `GET /api/agents/aggregate`                                                                  |

Human CLI output adds a `PROJECT` column: the project name, `User` for User rows, `?` for unknown.
JSON keeps `workspace_id` (and `attribution` on automation runs); join names from
`GET /api/workspaces`. On `/api/tasks`, `all_workspaces=true&workspace_ids=@user` equals
`scope=global`.

Route specifics:

- **Loop runs** use `created_at DESC, id DESC` only; `order=operational` requires a workspace path.
- **Loop requests** keep the shared pending/resolved order with a rowid cursor that carries the
  breadth; the profile lens applies through the owning run, and each request carries `workspace_id`.
- **Loop definitions** and **agents** resolve for one concrete profile (`profile`, else the acting
  profile); `all_profiles=true` is rejected. Identity is `(workspace_id, name)`. A User row
  (`workspace_id` empty) lists each user/profile-layer definition and counts only User runs; a
  project row lists definitions authored or overridden there, plus inherited user-layer definitions
  with a run in that project in the last 30 days. Loop definitions return `entries`, per-workspace
  `errors`, and `truncated` (200 entries per workspace, 2,000 total); each entry carries
  `workspace_id` and `source.layer`. The existing workspace catalogs keep their envelopes. Agent
  aggregate cursors and metrics use `(name, workspace_id)`.
- **Automation jobs and triggers** accept `all_workspaces=true` with the same population as
  omission, plus `workspace_ids`. **Runs** add `workspace_id`, `all_workspaces`, `workspace_ids`,
  and an opaque `cursor` (`page.next_cursor`). A run's `workspace_id` is the source definition's
  project at reservation, independent of where it executed; `attribution` is `recorded` for new
  runs, `derived` for historical runs whose source still exists, and `unknown` without evidence.
  Unknown runs are excluded by any `workspace_ids` filter and read as "Unknown project", never User.
- **Worktrees** read cached store state with no Git discovery or refresh; `--refresh` cannot
  accompany `--all-workspaces`.
- **Terminals** aggregate only on local listeners (gateway tiers return 404). The stream's
  `Last-Event-ID` is `<sequence>.<selection-fingerprint>`; a profile or filter change, or a replay
  older than retention, yields a fresh `terminal.snapshot` with `reset: true`. Workspace-path
  streams keep numeric cursors.
- **Existing app-allowed routes** (`/api/tasks`, `/api/automation/{jobs,triggers,runs}`,
  `/api/agents/catalog`) keep their app access; `all_workspaces=true` never widens what an app token
  already reads by omission. `/api/agents/catalog` rejects `all_workspaces`; use
  `/api/agents/aggregate`.

The needs-you bell (`GET /api/notifications/attention`) stays operator-wide across every profile and
labels User items `"User"` (previously `"Global"`; a display string).

### Session pull-request projection

When a registered project's folder is missing, `compozy__session_status` and
`compozy__session_describe` retain the session payload and add a `workspace_root_missing`
diagnostic. Stopping and archiving remain available; these controls do not need to load the
project's agent files. Creating or executing new work still requires an available root.

`compozy__session_list` and `compozy__session_status` include `branch`, nullable `pull_request`,
nullable `pull_request_lookup`, and `board_column`. The PR contains state, nullable draft, head SHA,
complete check counts, review decision, mergeability and `ready_to_merge`. A null lookup means no
branch; `pending`, `error`, and `incomplete` do not establish that no PR exists. Stale successful facts
remain visible during provider outages. Explicit refresh is available through
`POST /api/sessions/{session_id}/pull-request/refresh` or `compozy session pr <id> --refresh`.

### In-place session worktree tools

`compozy__session_worktree_bind` and `compozy__session_worktree_unbind` belong to the sessions
toolset and have mutating risk. They preserve the existing session rather than creating a fork.
Use the live descriptors for `session_id`, workspace scope, `worktree` (bind only), and optional
`expected_worktree_id`. App tokens cannot invoke these operations. Only idle ordinary project user
sessions qualify; busy calls return `session_busy`, and stale expected IDs return
`session_worktree_conflict`. Read `references/worktrees.md` before changing a binding, then confirm
through `GET /api/workspaces/{ws}/sessions/{id}` or the workspace session list. The Web detail's `worktree_binding` availability is advisory; mutation
rechecks eligibility. Existing fork tools and commands keep their new-session behavior.

## Session Processes

Use `compozy__session_processes_list` with `session_id` (omitted selects the caller) to inspect
session-owned background commands, active tools, and native terminals. Rows carry exact `pid` and
`started_at`, `kind`, `attribution` (`env` or `lineage`), command, CWD, age, and listening ports.
Raw environments are never exposed. `trees` groups the rows into logical process trees: each has
`id`, `kind`, `root`, `lead` (the process that names it, the leaf of a `sh -c`/`bun run`/`npm run`
wrapper chain), `wrappers`, `members`, and the union of `ports`; every row carries its `tree_id`. A
browser and its helpers, including detached crash handlers of the same app bundle, are one tree.
The session's `process_summary.trees` counts Background and Terminal trees; `background` and
`terminals` still count processes, with ports from that same population; foreground tools remain in
the detailed list. Catalog changes invalidate it.

`compozy__session_processes_terminate` accepts either `all: true` or a nonempty `targets` array of
`{pid, started_at}` copied from the list. Never terminate by PID alone. Each target stops every
verified member of its tree, root first, and returns one result per member. `all: true` stops every
Background and Terminal tree and leaves tool calls still running in the turn; add
`include_tools: true` to stop those too. Stop, archive, and delete cleanup include tools. It is a destructive tool,
subject to the current permission/approval policy and session workspace/profile boundaries. Agent
leaders and the daemon remain protected. Verified group members are signaled individually to
exclude later group joiners. Native terminals close through the terminal manager.
Inspect every result: only `terminated` and `already_exited` confirm completion;
`identity_mismatch`, `failed`, and `unproven` require inspection. Audit event:
`session.processes.terminated`.

Stop and archive native tools accept `processes: keep|terminate`; omission means keep. HTTP/UDS stop
places it in the JSON body with `wait`; archive and DELETE place it in the query. CLI stop, archive,
and remove accept mutually exclusive `--keep-processes` / `--terminate-processes`. A failed
ownership refresh does not block keep. Explicit termination rejects unsupported inspection before
the lifecycle mutation, and accepted cleanup survives client disconnect with a bounded lifetime.
CLI process termination reports an error on unsupported hosts. Read through
`compozy session processes <id> -o json`; terminate with `processes stop <id> --all` (add
`--include-tools` for in-turn tool calls) or the paired `--pid` and `--started-at`. Canceling a turn and idle release do not request cleanup.

The operator's **Processes left running** palette view spans stopped/archived/deleted/unknown
sessions. Its operator-only API is `GET /api/processes?all_workspaces=true&all_profiles=true&orphaned=true`;
`POST /api/processes/terminate` accepts exact `targets`, never `all`, including deleted owners.
`compozy process list --orphaned --all-workspaces --all-profiles -o json` reads the same inventory.

Environment markers are primary; durable exact-identity ancestry observations survive reparenting
and daemon restart. Boot re-verifies saved identities and prunes dead rows. Shared scans normally
run every five seconds on demand, with 250 ms tree observations during active tool calls. Windows
returns `supported:false`. macOS protected binaries that detach and change PGID before their first
observation cannot be attributed because XNU restricts environment reads
(https://github.com/apple-oss-distributions/xnu/blob/main/bsd/kern/kern_sysctl.c, `sysctl_procargsx`).
A bare legacy marker for a missing session cannot establish profile ownership without a retained
observation; unknown owners need this daemon's home and profile markers.

Shutdown keeps detached session descendants. The daemon still closes native terminals during
shutdown; PTY survival across restart is outside this feature. Keeping never prevents a process
from exiting or receiving its parent's or terminal lifecycle's signals.
