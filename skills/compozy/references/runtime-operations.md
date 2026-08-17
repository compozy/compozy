# Runtime Operations

## Contents

- Operating model and updating
- Daemon drain
- Session lifecycle and CLI
- Session event store ownership
- Background roles and usage cost
- MCP serve and onboarding
- Gateway exposure and device authentication
- Remote CLI profiles and SSH forwards
- Automation suggestions
- Messaging bridge delivery, diagnostics, and runtime boundaries

## Operating Model

CompozyOS is a local-first daemon that starts ACP-compatible agents as managed subprocesses, records events, and exposes runtime control through CLI, HTTP/SSE, UDS, and agent tools. Treat the daemon as the source of truth for sessions, events, task state, network rooms, memory, skills, and extension resources.

Do not manage runtime state by editing SQLite databases, process internals, or generated projections. Use public CompozyOS surfaces with structured output.

## Updating

`compozy update` follows the installed release line and never switches lines on its own; a beta
build tracks newer beta releases. For install channels and moving between lines, follow the
compozy.com docs instead of package managers, which may lag the active line.

The command checks both the runtime and desktop app, applies the runtime first, and stages a closed
app for its next launch. Use `--check` for a read-only result and `--cancel` only for a dormant
operation. Read or control the same host-global operation through `GET /api/settings/update`,
`POST /api/settings/update/apply`, and `POST /api/settings/update/cancel` over HTTP or UDS.

When an artifact-policy failure prevents an in-place update, follow the command's release-specific
manual replacement recommendation. The same release cannot succeed through another immediate
`compozy update` attempt.

## Daemon Drain

Use `compozy drain -o json` (or `POST /api/drain` over HTTP/UDS) to close daemon-global new-work admission while admitted prompts and claimed runs finish. The stable response is `{"state":"draining"}`; repeated calls are no-ops. New session/prompt, task-run enqueue, retry/recover, and run-claim attempts return 503 with `daemon is draining; new work admission is closed`, while cancellation, terminal transitions, and teardown remain available.

Confirm drain through `.daemon.status == "draining"` in `compozy status -o json` or the informational `daemon_draining` item in `compozy doctor -o json`. Use `compozy undrain -o json` or `POST /api/undrain` to restore `{"state":"active"}`. Drain is in-memory, applies across every workspace, and clears on daemon restart.

## Session Lifecycle

CompozyOS sessions are daemon-owned runtimes. Common states:

- starting - the daemon accepted the session and is booting the provider.
- active - the logical session accepts prompts; its runtime may be `unbound` or ready.
- stopping - shutdown has started.
- stopped - the runtime exited. The durable record and transcript remain promptable.

Creation and runtime binding are separate. `compozy session new` creates an `active` logical session
without starting a provider process: its nested status is `runtime.status="unbound"` and it has no
`runtime.effective` selection. Send the first prompt to bind a provider. Read the nested `runtime`
object from `session status -o json` (or any session read), rather than inferring process state from
the top-level session state: it reports `status`, `transition`, redacted `failure`, `selected`,
`selection_revision`, `effective`, ACP session ID, and advertised ACP capabilities. `selected` is
durable next-prompt intent; `effective` is the runtime already bound to the current process.

Session types include user sessions and daemon-managed sessions such as dream, system, coordinator, worker, and reviewer sessions. Do not infer authority from a session type alone. Use the session context and daemon tools to confirm what the current session may do.

The daemon owns `coordinator` and `spawned` classification. A `session.pre_create` hook cannot change
a session into or out of either type, and later lifecycle hooks cannot change any persisted session
type or workspace identity.

With `roles.auto_title.enabled = true`, an unnamed user session receives at most one daemon-owned durable title after its first assistant response is persisted. Configure its agent, provider, model, reasoning, and fallback routes under `[roles.auto_title]`. An explicit session name wins any race; daemon-managed session types are ineligible. Treat the persisted session name as catalog identity and leave the session unnamed when generation is disabled or fails.

Attachability is explicit live runtime state. Use `compozy session list --resumable -o json` before
`compozy session resume`. The command acquires an attach lease for an eligible live session; it does
not restart `stopped`, and stopped sessions reject attach.

A normal prompt to a stopped user session restarts its ACP process, reloads the durable provider
history, and submits the new prompt against the same session ID. Concurrent prompts share one
restart. Attach, queue management, steer, interrupt, and other control operations never trigger this
restart.

If ACP disconnects during a prompt, CompozyOS keeps every event already persisted, emits a terminal
error, and stops the failed runtime with `failure.kind=process_exit`. A JSONL prompt command writes
the frames it received, including the error frame, then exits nonzero. `compozy__session_prompt`
returns `tool_backend_failed` with `backend_dead` instead of reporting a successful tool call. Inspect
status, events, and the crash bundle before sending another prompt. CompozyOS never automatically
replays the interrupted prompt because its external effects may be indeterminate; a new explicit
prompt is the safe restart boundary.

After prompt admission, the daemon owns the turn lifetime. Closing a browser tab, navigating away from the web app, dropping an SSE stream, or disconnecting a CLI/UDS response only detaches that viewer; it does not cancel the accepted prompt. Use explicit runtime intent such as `compozy session stop`, prompt cancel, or interrupt controls when cancellation is required.

Use `interrupt` with replacement text to cancel the expected active turn and submit that replacement
as the next generation-fenced input. Use `steer` for the same fenced replacement semantics when the
operator intent is guidance. Neither mode reuses or combines the canceled authored prompt.

The event store and materialized transcript are the durable source of truth for reattach. Transcript GET returns the newest bounded `entries` page plus `epoch`, `generation`, `max_sequence`, and `has_older`; request older entries with `before_sequence=next_before_sequence`. Each entry carries immutable `start_sequence` identity and its latest shaping `sequence`. Do not reconstruct session state from UI cache, memory notes, or JSONL sidecars.

### Session event store ownership

Each `events.db` is bound to one exact session and workspace. Session reads and writes through CLI,
HTTP/UDS, or native tools must match that persisted owner. A missing or mismatched owner refuses the
open before migration or data mutation; CompozyOS does not adopt, rebind, or repair the database.

If an owner check fails, stop the daemon and preserve the complete containing `COMPOZY_HOME`, including
the database and every SQLite sidecar. Restore a matching complete backup, or create a new session when
discarding the retained state is acceptable. Never edit the owner row, move `events.db` between session
directories, or use `compozy session repair` for this condition; session repair only appends transcript
repair events to an already valid session store.

Treat `transcript_marker.file_mutation_unverified` as a required verification signal: one or more persisted `edit` mutations failed without a later successful mutation for the same path in that turn. Inspect the bounded paths and verify them before trusting completion claims; the marker is advisory and does not replace filesystem inspection.

Pressure compaction preserves complete prior turns in the workspace checkpoint before marking their
event rows archived. Read archived rows with `compozy session events|history --archive archived`, or
combine active and archived rows with `--archive all`. Degraded replay excludes them to avoid
duplicating checkpoint-covered context. Inspect
`session.compaction_fired` for the admitted sequence span; the event is correlation evidence, not a
success verdict for the later archive.

The HTTP/UDS stream defaults to `transcript_snapshot`, batched `transcript_delta`, and terminal `session_stopped` frames. Reconnect with the last SSE cursor plus the snapshot's `epoch` and `generation`; a fence mismatch returns an explicit reset snapshot. The removed `replay` query is invalid. Use `frames=raw` for persisted `SessionEventPayload` rows; `compozy session events --follow` already requests raw frames.

### Workspace knowledge on live turns

Markdown files under `<workspace>/knowledge/` are current workspace data. On each accepted user,
Network, or synthetic turn, CompozyOS reopens the tree and supplies a bounded
`<workspace-knowledge-snapshot>` with workspace-relative paths, current bytes, a revision digest,
and omission metadata. Treat the newest snapshot as authoritative over earlier copies of the same
file.

The reader accepts regular `.md` files, does not follow symbolic links, and stays inside the session
workspace. A file change does not wake a session. The next eligible turn—including task and
Heartbeat wakes—carries the changed bytes without an additional operator prompt. This is prompt
context, not durable CompozyOS memory; use the memory tools when information must be curated or
searched.

## Session CLI

Use structured output when agents need to inspect or route results.

Workspace-scoped commands use one context chain: positional workspace ref, `--workspace`,
`COMPOZY_WORKSPACE`, validated session identity, then cwd discovery. `--workspace` is an override,
not a prerequisite, and accepts an ID, name, or path. Inside a workspace-bound session, omit the
workspace argument unless an explicit override is required. Absent or partial credentials, and
validated global or workspace-less identities, fall through to cwd. Configured credentials that
cannot be validated fail closed.

    compozy session new --agent general --name review-run
    compozy session new --agent codex --cwd /absolute/path/to/worktree --name fix-task
    compozy session prompt <session-id> "Inspect the failing build." --provider codex --model gpt-5.6-sol --reasoning-effort high --speed fast
    compozy session attachments upload <session-id> ./diagram.png -o json
    compozy session list --all -o json
    compozy session list --type user --state active --sort last_activity -o json
    compozy session list --resumable -o json
    compozy session list --attention -o json
    compozy session list --badge done --all-workspaces -o json
    compozy session list --summary -o json
    compozy session list --archived -o json
    compozy session list --include-archived -o json
    compozy session status <session-id> -o json
    compozy session health <session-id> -o json
    compozy session inspect <session-id> --include-wake-events -o json
    compozy session recap <session-id> --limit 20 -o json
    compozy session events <session-id> --follow
    compozy session events <session-id> --after 42
    compozy session history <session-id>
    compozy session history <session-id> --last 20 --after 42
    compozy session rewind <session-id> --message-id <message-id>
    compozy session prompt <session-id> "Summarize the last three tool results."
    compozy session runtime set <session-id> --provider claude --model claude-fable-5 --reasoning-effort max
    compozy session runtime clear <session-id>
    compozy session resume <session-id>
    compozy session resume --latest --workspace checkout-api
    compozy session stop <session-id>
    compozy session archive <session-id>
    compozy session unarchive <session-id>
    compozy session remove <session-id>
    compozy session repair <session-id> --dry-run -o json
    compozy session soul refresh <session-id> --expected-digest sha256:old -o json
    compozy session approve <session-id> --request-id req_123 --turn-id turn_123 --decision allow-once
    compozy session clarify pending <session-id> -o json
    compozy session interactions <session-id> -o json
    compozy session clarify answer <session-id> <request-id> --choice 1 -o json
    compozy session clarify answer <session-id> <request-id> --text "Use staging" -o json
    compozy session prompt-cancel <session-id> -o json
    compozy session wait <session-id>

`compozy session new` is promptless and accepts no runtime selection. It returns the durable active,
unbound session; use its ID in a later `compozy session prompt`. A prompt to an unbound session must
select `--provider`; `--model`, `--reasoning-effort`, and `--speed` refine that prompt's snapshot.
`--speed` defaults to `normal`; `fast` applies only through an unambiguous ACP select/value-ID option.

Session attachments are durable, workspace/session-scoped refs. Upload with
`POST /api/workspaces/:workspace_id/sessions/:session_id/attachments` or
`compozy session attachments upload`. HTTP and UDS share that route contract: the upload response
returns attachment metadata, and `POST /api/workspaces/:workspace_id/sessions/:session_id/prompt`
sends the returned `PromptAttachmentRef` metadata object in `attachments`. In contrast,
`compozy__session_prompt` accepts the `att_...` ID or a file path under the resolved workspace root or
configured `additional_dirs` and can submit an attachment-only prompt. Images require ACP image input
and PDFs require embedded context. Markdown and plain text fall back to text blocks when embedded
context is unavailable. Archive and conversation clear retain attachments; `compozy session remove`
removes a session's attachments, and `compozy workspace remove` removes the workspace attachment tree.

`session runtime set` persists the complete default selection without starting or reconfiguring ACP;
the next prompt applies it. `session runtime clear` returns resolution to the effective/agent default.
Both commands use `runtime.selection_revision` as a compare-and-swap fence; omit
`--expected-revision` to let the CLI read the current value, or pass it for an explicitly fenced write.

`session rewind` cuts the active conversation immediately before one durable user message, returns
that message as `draft_text`, and starts a fresh ACP context under the same CompozyOS session ID. The
CLI reads the current transcript fences; pass all three `--expected-*` values only when retrying a
previously fenced request. HTTP, UDS, and the native tool require the epoch, generation, and maximum
sequence returned by the transcript API. Stale values return a conflict without changing the session.
Rewind is available only for idle ordinary user sessions.
It archives the removed suffix for audit. It does not undo file changes, tool or network effects,
saved memory, or external provider actions. Use `--archive archived` or `--archive all` on events
and history to inspect the discarded suffix.

### Session attention and pending interactions

Session badges have one daemon-owned precedence order. `waiting-for-auth`, `waiting-for-input`, and
`failed` form the `needs-you` class. `done` means the latest settled turn has not been seen; it forms
the `finished` class. A new turn, terminal lifecycle state, or higher-priority pending interaction
always outranks `done`. CLI and API reads never mark a session seen.

Use `compozy session list --attention` for the `needs-you` class, `--badge <token>` for an exact
badge, and `--all-workspaces` to remove the workspace filter. `--summary` returns exact `needs_you`,
`finished`, and per-workspace totals across the whole catalog. Attention lists default to
`--sort attention`, ordered by the latest attention transition with a stable session-ID tie-break.

`compozy session interactions <session-id>` lists the sanitized pending and restart-orphaned
questions and permission requests. The same projection is available from
`GET /api/workspaces/{workspace_id}/sessions/{session_id}/interactions` and is embedded in session
detail and status payloads. Interaction IDs are durable across daemon restart; resolve the returned
ID through the existing approve or clarify-answer surface instead of matching display text.

Resolution outcomes are explicit. A live permission decision reports `applied`, and a live
clarification answer reports `answered`. Resolving an orphaned request reports
`resolved-after-restart`; repeating that resolution reports `already-resolved` with the original
winning decision or answer. `queue-full` leaves the interaction untouched and is safe to retry.
`compozy session status <session-id> -o json` returns both the canonical badge and this same bounded
pending-interaction projection.

Operator clients acquire a per-client visibility lease with
`POST /api/workspaces/{workspace_id}/sessions/{session_id}/presence`. A first request with
`{"visible":true}` returns `lease_id`; renew or release only that lease by sending the ID back.
When a turn settles under any live lease, the daemon marks it seen and does not derive `done`.
Leases expire after 15 seconds unless renewed. Presence, attention-summary, and cross-workspace
catalog surfaces are operator-only; agent identity receives `403 agent_scope_denied`. Interaction
discovery accepts a validated agent identity only for that agent's workspace.

Subscribe to `session_attention_changed` on the session catalog stream for committed badge edges.
Extension hooks use the separate async-only `session.attention.changed` event. Its payload carries
`from`, `to`, `class`, `at`, and the session/workspace context; hook failure never rolls back the
canonical attention change.

### Waiting for session state

`compozy session wait <session-id>` blocks until the session settles or needs someone. The default
target set is `waiting-for-input`, `waiting-for-auth`, `idle`, `stopped`, and `failed`; `done` also
satisfies `idle`. Override it with `--until`, bound the request with `--timeout` (default 5 minutes),
or use `--unbounded` to let the CLI transparently resume bounded server waits without a blind spot.
Valid explicit targets also include `done`, `running`, `hung`, and `unhealthy`.

The structured result names `state-reached`, `timeout`, `session-gone`, `canceled`, or `overflow`,
plus the observed state, elapsed milliseconds, and attention revision when available. Exit `75`
means timeout, `69` means the session or daemon is unavailable, and `65` means an invalid target.
The HTTP/UDS wait route always requires a positive `timeout_ms` no greater than 1,800,000; timeout is
a normal `200` result, while a gone session returns `410`. Server waits are always bounded even when
the CLI uses `--unbounded`.

Each accepted prompt records its immutable runtime snapshot with the authored user event. Omitting
runtime flags uses the durable `selected` value first, then the current `effective` selection; both
being absent is invalid while the session is unbound. A queued prompt retains its submitted snapshot until dispatch. An interrupt advances the
input generation, drops stale queued entries, then applies the replacement prompt's snapshot only
after the current turn becomes idle. Inspect the prompt result's queue ID, queue position, and queue
generation instead of assuming a busy input ran immediately.

Busy-input admission is explicit. Submit `mode=queue`, `mode=interrupt`, or `mode=steer` with a
prompt, then use the returned `status` and `delivery` as the authoritative result. `queue` returns a
durable queue entry for later dispatch; `interrupt` and promotion require the active turn's
`expected_turn_id`, which prevents a stale client from replacing a newer turn. Transcript markers may
describe a queued, steered, interrupted, or canceled action, but they are history, not a second
operator result channel.

HTTP and UDS expose the same daemon-owned queue at
`GET /api/workspaces/{workspace_id}/sessions/{session_id}/prompt/queue`. Replace one item with
`PUT .../prompt/queue/{queue_entry_id}`, remove it with `DELETE` on that path, or promote it with
`POST .../prompt/queue/{queue_entry_id}/steer`. Replace and promotion submit new `message_id` and
`idempotency_key`; promotion also submits `expected_turn_id`. Re-read the queue after each mutation
instead of keeping a client-side shadow list.

### Prompt identities and explicit retries

Every prompt and steer submission carries a durable `message_id` and an `idempotency_key`. The CLI
generates both for a new `compozy session prompt` command. To retry one uncertain submission, provide
both original values together:

    compozy session prompt <session-id> "<same message>" \
      --message-id <original-message-id> \
      --idempotency-key <original-idempotency-key>

The same rule applies to `--queue`, `--interrupt`, and `--steer`. Do not supply either identity flag
by itself. `--cancel` does not accept prompt identity flags.

An exact retry returns the stored prompt result with `replayed: true` and does not create another
prompt stream or repeat its accepted side effect. Treat the result's `message_id` as the durable
transcript message identity. A `409` means the key or message ID conflicts with another request, or
the earlier dispatch is indeterminate. For an indeterminate dispatch, inspect the session and obtain
an operator decision before doing anything else; do not blindly retry, and use new identities only
for an intentional new command after that decision.

At a prompt boundary, a same-provider change uses live configuration when the provider supports it.
If that is unavailable or fails, or the provider changes, CompozyOS replaces the ACP process and
replays the canonical durable session context before sending the newly accepted prompt. A failed live
configuration or replacement restores the prior binding and reports the runtime failure; because the
user event is persisted only after a runtime is ready, rollback creates no user prompt event to retry.
Read `runtime.transition` to distinguish `initial_bind`, `live_configuration`, and
`process_replacement`.

Use `compozy session prompt-cancel <session-id>` to cancel only the current prompt while keeping the
session alive. The result is `canceled` with the affected `turn_id`, or `nothing-in-flight` when the
session has no active prompt. Repeating the command is safe: it follows the same idempotent
cancellation path used by HTTP, UDS, and `compozy__session_prompt_cancel`. The CLI exits `0` for a
cancel and `66` for nothing in flight.

If a CompozyOS-native session tool is visible, prefer the tool because it is policy-aware and easier for the daemon to audit. Use the CLI when the tool is denied, absent, or explicitly requested.

## Background Roles

CompozyOS routes six daemon-owned background responsibilities through the closed `[roles]` roster:
`coordinator`, `dream`, `checkpoint_summary`, `memory_extractor`, `auto_title`, and
`memory_controller`. Inspect the effective global or workspace projection with structured output:

    compozy roles list -o json
    compozy roles list --workspace <id|name|path> -o json
    compozy roles show dream --workspace <id|name|path> -o json

HTTP and UDS expose the same `GET /api/roles` and `GET /api/roles/{role}` payloads. Each projection
reports `enabled`, `resolution_mode`, nullable agent/provider/model/reasoning values, controller-only
`timeout`, the ordered `fallback_chain`, per-field `provenance`, and current `diagnostics`. Preserve
nulls: `resolution_mode=inherit` means the invoking context decides at invocation, not that a client
should substitute `[defaults]`.

`coordinator` and `dreaming-curator` are virtual builtin identities and never fleet entries. A
configured authored agent that cannot be resolved produces `role_agent_not_found`; an unknown role
returns `role_unknown`. The reads are diagnostic only and never simulate a provider invocation.

Scalar role keys can be written through `compozy config set roles.<role>.<field> <value> -o json` or the
live `compozy__config_set` descriptor. Role writes are Live desired state at global or workspace scope
and affect later invocations without restarting the daemon. Use `config.toml` or the Settings Roles
API/UI for the ordered `fallback_chain`, which is an array of route tables and replaces as a whole in
a workspace overlay. A fallback may advance only at the owning invocation's pre-acceptance boundary;
an accepted ACP session is never silently rerouted. Immediately before each fallback attempt, CompozyOS
emits `role.fallback.used`; the event records that the route was tried, not that it succeeded.

Session-backed roles accept `enabled`, `agent`, `provider`, `model`, `reasoning_effort`, and
`fallback_chain`. Coordinator additionally owns `ttl`, `max_children`, and
`max_active_sessions_per_workspace`. The in-process `memory_controller` has no `agent`; it owns
`timeout`, `top_k`, `prompt_version`, and `max_tokens_out`. Do not move Loop runtime defaults/rules,
TaskExecutionProfile selectors, automation resources, or subsystem policy into `[roles]`.

### Usage cost truth

Provider model metadata is global config. Use Provider Settings HTTP/UDS or `config.toml` for the
five pricing fields and `models.reasoning.apply`; use atomic model curation for flags and default
effort. `acp_option` applies an advertised ACP effort, while `none` exposes no selectable strategy.
Inspect the redacted effective state with `compozy config show` after a live apply or restart.
`compozy__provider_models_status` is read-only. `compozy__provider_models_refresh` accepts optional
provider/source filters, `force`, and `request_id`; it retains successful sources on partial failure
and redacts credential material from errors. CLI fallbacks are `compozy provider models status` and
`compozy provider models refresh`.

The session usage endpoint
`GET /api/workspaces/{workspace_id}/sessions/{session_id}/usage` returns token totals plus
`cost_status` and `cost_source`. Interpret money by status: `actual` is agent-reported;
`estimated` is a catalog-rate projection; `included` has no amount because a `native_cli` provider
owns subscription billing; `unknown` has no amount because rates or aggregate provenance are
incompatible. Never treat `included` as a confirmed account balance or add `actual` and `estimated`.
Sources are `agent_reported`, `catalog_config`, `models_dev`, `builtin`, or `none`.
Configured model input, output, cache-read, cache-write, and reasoning rates affect subsequent
estimates only; CompozyOS does not reprice stored token statistics after a catalog or config change. Every
nonzero bucket requires its own finite, non-negative rate. Never infer a cache rate from input or a
reasoning rate from output.

Prefer `compozy session usage <session-id> -o json` when an agent needs the same aggregate over UDS.

`compozy session stop` preserves durable history and ends attach eligibility; a later normal prompt
restarts the same logical session. `compozy session archive` hides a stopped session from the
default catalog without deleting it. Archived sessions keep their history and remain directly
readable, but cannot resume, attach, or accept a prompt until `compozy session unarchive` clears the
archive marker. Use `session list --archived` for only archived sessions or
`session list --include-archived` for both catalog sections. `compozy session remove`
is destructive: it stops an active runtime when necessary, then removes the catalog row and persisted
session directory. Use removal only when the operator intends to discard that history.

`compozy session rename <id> <name>` changes only a user session's durable display name. It works
for active, stopped, and archived user sessions without starting or replacing ACP and preserves the
session ID, transcript, archive state, and lineage.

The session catalog is counted and workspace-scoped. Dream sessions are internal and never appear in catalog results. HTTP and UDS clients can filter exact public session type with `type=user|system|coordinator|spawned`; the CLI exposes the same filter as `--type`. Browser integrations should subscribe once to `/api/sessions/catalog-stream`, route each wake signal by its authoritative `workspace_id`, and refetch that workspace's catalog page instead of incrementing local counters.

Sessions created from inside another session record creation provenance in `lineage`: `compozy__session_create` links the calling session automatically (same-workspace only), and `session new --parent <id>` / `parent_session_id` on `POST /api/sessions` link explicitly. Provenance keeps `type=user` and carries no TTL, auto-stop, budget, or permission narrowing — governed children still come only from `compozy spawn`. Query hierarchy with `parent=<id>` (direct children) or `root=<id>` (whole tree, root included) on the catalog — CLI `session list --parent/--root`, same fields on `compozy__session_list`.

`compozy spawn` and `compozy__session_spawn` create governed children with a required TTL and
permission subsets. The parent receives one sanitized synthetic turn when an eligible child stops,
fails, or enters a needs-you state. This `notify_creator` behavior defaults to on and has no
`config.toml` key. Use `--no-notify-creator` in the CLI or explicit `notify_creator: false` in the
HTTP/UDS or native-tool request to opt out for that child.

Loop Goal sessions also record the session that started the Loop as internal creation provenance when that origin is available. They remain `type=system`: the parent/root/depth fields are informational and do not grant safe-spawn policy, caps, TTL, or parent-stop cleanup. If the origin was deleted before Goal creation, the Goal is created as its own root; deleting an origin after creation preserves the Goal and its recorded lineage.

## MCP Serve

Use `compozy mcp serve` to expose the approved Host API subset to a trusted external MCP client over
stdio. It infers the workspace through the shared context chain; pass
`--workspace <id|name|path>` only when the client's launch directory is not the intended workspace.
The command is a foreground relay to the running daemon; it does not start another daemon or open
stores directly. Published names use `compozy_host__<family>__<verb>`, not the native `compozy__*`
namespace. Sessions, workspace-safe task operations, Network, memory, and resources are included;
target-only task mutations and unrelated Host API families are excluded.

The resolved workspace binding is injected into every call. Conflicting caller workspace fields are
rejected, so a relay bound to one workspace cannot read another workspace's data. Stdio has no
separate authentication exchange: the spawning process receives operator authority for the
published methods in that workspace.

Each connected MCP client receives an independent principal and resource-source lifetime. Closing
one client removes only that client's authority and resources; closing the relay removes all
remaining client-scoped registrations.

For a local client that cannot spawn stdio, use authenticated loopback HTTP:

    export COMPOZY_MCP_SERVE_TOKEN='replace-with-a-high-entropy-token'
    compozy mcp serve --workspace <workspace> --transport http --listen 127.0.0.1:3210

Connect to `http://127.0.0.1:3210/mcp` with `Authorization: Bearer <token>`. HTTP refuses non-loopback
listeners, an empty token environment variable, and unauthenticated requests. `--token-env` selects
an alternate environment-variable name. There is no bearer-token CLI value or `config.toml` key.
Stop the foreground relay when the client no longer needs authority.

## Onboarding State

First-run onboarding completion is a global instance flag (stored in the `app_metadata` table, not per-workspace). Inspect or manage it through the CLI or the HTTP/UDS `/api/onboarding` endpoints:

    compozy onboarding status -o json
    compozy onboarding complete    # mark first-run onboarding as done
    compozy onboarding reset       # clear the flag so the web wizard runs again

The web first-run wizard blocks the dashboard until this flag is set. Resetting it surfaces the wizard again on next load. Fresh daemon boot registers the operator `$HOME` as the default workspace before the wizard starts, so the workspace step should not require manual project registration on a clean machine.

Native session tools include scoped wait, governed spawn, stop, approval, clarification answer, and
prompt cancel. Recap, repair, inspect, and Soul refresh remain CLI/HTTP/UDS management surfaces unless
the live registry exposes a matching tool. `compozy__clarify` asks from inside the active session;
`compozy__session_clarify_answer` answers a pending question on another same-workspace session.

## Gateway Exposure and Device Authentication

Gateway policy is operator-global desired state. Inspect its effective posture in
`.daemon.gateway` from `compozy status -o json`, through `GET /api/status`, or directly through
`GET /api/gateway/status` on the private HTTP listener or UDS. The status reports enabled tiers,
durable surfaces, provider state, resolved loopback listener addresses, paired devices, and any
refusal that prevented exposure. A configured port of `0` asks the daemon to select a free port;
use the resolved status address instead of assuming a default.

Run `compozy gateway audit -o json`, call `GET /api/gateway/audit`, or use the `audit` action of
`compozy__gateway` to check the same posture as ranked findings. A completed report always has
`ran=true`; an empty result also has `no_findings=true`, so it cannot be confused with a skipped
check. Findings have stable `id`, `severity`, and `remediation` fields. Provider downtime is a
finding, not an audit transport error. The audit only reads current state, so re-run it after a
repair to confirm that the finding cleared.

Manage tier surfaces, providers, pairings, and devices through the private authenticated HTTP
listener or UDS under `/api/gateway`. Mint pairing artifacts with `POST /api/gateway/pairings`, redeem them with
`POST /api/gateway/pairings/redeem`, and list, rename, or revoke devices with `GET /api/gateway/devices`,
`PATCH /api/gateway/devices/{id}`, and `DELETE /api/gateway/devices/{id}`. Pairing mint and redeem are physically absent from
the public listener. Browser redemption installs a `Secure`, `HttpOnly`, `SameSite=Lax` cookie and
does not return the raw credential. CLI redemption generates and durably stores the credential on
the client before sending it over TLS; the daemon stores only its hash.

Select a connectivity provider with `POST /providers/{name}/enable`, naming the exact tier, live
install source, current confirmed control digest, and expected generation. Read status first and do
not reconstruct a digest. A third-party provider whose live registry digest changed fails closed
until the extension requirement and Gateway provider selection both confirm the current value.
Disable it with `POST /providers/{name}/disable?tier=<private|public>`. Only one provider may own a
tier; replace that selection explicitly instead of racing two enables.

Before enabling the bundled Tailscale provider, bind an auth key from the operator's Tailscale
account through hidden input:

    compozy extension secrets set tailscale --env TS_AUTHKEY

The provider embeds `tsnet`; do not install or supervise a separate Tailscale client. The live
manifest must include the selected tier as `gateway.private` or `gateway.public` in
`channel_scopes`; a mismatch fails before provider code starts.

Public endpoint proof resolves through authenticated DNS-over-TLS at
`gateway.verify.public_dns_resolver`, not the host resolver, so MagicDNS cannot turn a Funnel proof
into a private-tailnet connection. A provider waiting for public DNS remains staged and unadvertised
while bounded recovery retries. Read `gateway status -o json` until the public tier reports
`advertised=true` (the human rendering shows summary counts only); disable the provider to stop a
staged attempt.

Authenticated streaming over a tier listener uses a short-lived, single-use ticket. Obtain one with
`POST /api/gateway/stream-tickets`, then pass it as the `ticket` query parameter on the SSE or
WebSocket request. A consumed, expired, malformed, or revoked-device ticket is rejected uniformly;
mint a fresh ticket for each reconnect. Revoking a device invalidates its future requests and closes
its registered live streams.

Tier listeners bind to daemon-owned loopback sockets. Connectivity providers publish those local
listeners; they do not replace Gateway authentication or expose UDS-only APIs. The private tier owns
operator management and the full operator surface. The public tier exposes only the selected
operator and ingress surface union, never pairing mint or redeem and never ingress-binding
management.

Webhook triggers and bridge instances expose honest public-ingress projections. Read the trigger's
`ingress` or the bridge's `gateway_ingress`; use its URL only when `reachability=live`. Confirm one
subject through the private listener or UDS with `POST /api/gateway/ingress-bindings` and
`{subject_kind, subject_id, confirmed:true}`. Remove it with
`DELETE /api/gateway/ingress-bindings/{subject_kind}/{subject_id}`. Subject scope and workspace are
resolved by the daemon, not accepted from the caller. An endpoint-generation change produces
`reconfirmation_required`. Public ingress has no store-and-forward queue: when the daemon or provider
is offline, the sender must retry the failed delivery.

Gateway API errors use stable codes. Branch on the returned code instead of matching prose. The
repairable policy and state codes are `gateway_exposure_refused`, `gateway_consent_required`,
`gateway_provider_trust_stale`, `gateway_digest_confirmation_required`,
`gateway_endpoint_unverified`, `gateway_provider_degraded`, `gateway_generation_conflict`, and
`gateway_tier_provider_conflict`. Authentication, pairing, device, ticket, ingress, and local-only
failures use their matching `gateway_device_*`, `gateway_pairing_*`, `gateway_stream_ticket_invalid`,
`gateway_ingress_*`, and `gateway_local_only_operation` codes.

### Remote CLI profiles and SSH forwards

Pair a direct HTTPS client with `compozy pair mint`. The command writes the raw artifact to the
private `0600` file named by `artifact_ref`; transfer that file out of band, then redeem its contents
on the client with `compozy pair redeem <artifact> --name <name> --address https://host[:port]`.
CLI output never contains the raw artifact. Use `--use` to make the profile active immediately, or
select it later with `compozy connect use <name>`. Inspect
non-secret metadata with `compozy connect list -o json`; `compozy connect remove <name>` removes both
the profile and its client-local credential. `compozy connect use local` returns the CLI to UDS.
Move the same revocable identity to another client with `compozy connect export <name>
--passphrase-file <private-file> --output-file <bundle>` and `compozy connect import <bundle>
--passphrase-file <private-file>`. The bundle uses passphrase-derived authenticated encryption;
protect it like a key. Copying `config.toml` alone never transfers an identity.
When a remote profile is active, commands report the selected target on stderr while structured
stdout remains parseable. `compozy open` uses the selected HTTPS origin.

Direct profiles can operate sessions; read tasks; and use Loops, memory, settings, bridges,
extensions, and private Gateway management. Task mutations, task-run queue and scheduler authority,
agent-internal routes, run lifecycle mutations, and resource mutations are local-only and fail before network I/O. Use the
local daemon or an SSH forward when that authority is required. Each remote SSE or WebSocket connect
and reconnect mints a fresh single-use stream ticket. A dropped stream reports
`gateway_stream_interrupted`; accepted work continues on the daemon and can be observed after
reconnect.

Use `compozy connect ssh <host>` when direct HTTPS is unavailable or local-equivalent authority is
needed. The command uses the system OpenSSH client and its existing configuration and agent, checks
that the remote `compozy` version exactly matches, starts the remote daemon only when necessary,
requires its HTTP listener to be loopback-only, and creates a loopback-only local forward. Pass
`--remote-home <path>` to run every remote probe and lifecycle command with that `COMPOZY_HOME`.
An intentional close tears down only its tunnel and stops the remote daemon only if this invocation
started the same process. Concurrent connects to an SSH-owned daemon return `gateway_ssh_busy`
instead of racing its lifecycle. An unexpected tunnel loss leaves that daemon running so accepted work can
be observed after reconnect. Changed host keys fail closed; install, version, reachability, and
tunnel-loss failures use stable `gateway_ssh_*` codes.

## Automation Suggestions

List one workspace's pending Job proposals with
`compozy automation suggestions --workspace <workspace> -o json` or
`compozy__automation_suggestions_list`. A list first seeds the fixed starter catalog idempotently; no
Job exists until acceptance. Use `compozy automation suggestions accept <id> --workspace <workspace>
-o json` to create the validated Job, or `dismiss` to durably latch that proposal away. Accept and
dismiss are compare-and-swap resolutions. On a conflict, list again instead of retrying the stale
action. Never move a suggestion id between workspaces; every read and mutation requires the exact
owner `workspace_id`. The workspace pending queue defaults to five entries. Change the positive,
restart-required limit with `compozy config set automation.suggestions.pending_cap <value>`; the store
applies that configured cap inside the serialized insert transaction.

## Messaging Bridge Delivery and Progress

Bridge instances own their delivery behavior. Manage them through `compozy bridge`, `/api/bridges`, or the equivalent UDS endpoints; do not edit extension state or storage directly. Tool progress is presentation-only bridge delivery data and never becomes session transcript or ACP history.

Use `compozy bridge manifest slack --instance <id>` for Slack app setup and `compozy bridge setup whatsapp|telegram|discord` for guided, write-only secret binding. Teams, Google Chat, GitHub, and Linear have no setup subcommand: use `compozy bridge create --enabled=false`, bind exact slots with `compozy bridge secret-bindings put`, verify, enable, then verify public reachability. Generic binding accepts secret contents through `--secret-value-stdin` or `--secret-value-file <path>` and rejects inline secret arguments. The setup commands accept strict headless JSON through the global `--json` flag; supplied and existing secrets are never echoed. WhatsApp verify tokens and Telegram `--print-only` webhook secrets must be supplied or generated with the explicit one-run `--reveal-generated-secrets` JSON disclosure, so an operator-needed value never becomes irretrievable. Telegram setup otherwise registers `setWebhook` through the daemon.

An empty bridge `dm_policy` normalizes to permissive `open`. The current create/update CLI has no DM-policy flag; keep the bridge disabled and use the Web editor or `PATCH /api/bridges/:id` to set `allowlist` or `pairing` plus the complete `provider_config.dm` lists. `pairing` consumes pre-populated `paired_user_ids`/`paired_usernames` with allowlist fallback; it does not enroll or approve senders interactively. DM policy does not govern groups, channels, spaces, repositories, or issues.

`compozy bridge verify <id> --json` asks the owning adapter for typed `pass|warn|fail|skipped` checks without changing instance lifecycle state; GitHub and Linear currently skip identity, so enabled runtime health owns their live auth result. Any failed check makes the command nonzero after it writes the records. `compozy doctor --only bridge --json` aggregates the same checks. After enablement, `compozy bridge send-test <id> --message ...` makes a real provider delivery. `compozy bridge test-delivery` remains a target-resolution dry run and sends nothing. HTTP and UDS expose `GET /api/bridges/providers/slack/manifest?instance=<id>` plus `POST /api/bridges/:id/verify`, `/send-test`, and `/webhook/register`; there is no `/api/bridges/setup` route.

For `send-test` and `test-delivery`, preserve valid UTF-8 bridge, peer, thread, and group IDs exactly across CLI, HTTP, and UDS; URL-encode path IDs that contain `/`. Delivery mode is the literal `direct-send` or `reply`; aliases, case changes, surrounding whitespace, explicit empty strings, and `null` are invalid. Omitting mode uses the bridge instance default, then `direct-send` when no default exists.

Credential-bearing API, OAuth, and service destinations are operator-owned adapter environment, not instance configuration. `provider_config` rejects `api_base_url`, `oauth_token_url`, `service_url`, `openid_metadata_url`, and `token_url`; use the provider's `COMPOZY_BRIDGE_*` process variables for trusted overrides. Provider clients use `bridgesdk.CredentialedHTTPClient`, returning the original `3xx` for classification without forwarding credentials or replaying mutation bodies. `webhook.public_url` must be public HTTPS, and verification blocks internal/special-use addresses, proxying, and redirects before reachability is attempted. Bridge reads expose the validated callback as optional `webhook_public_url`; clients use that projection for setup readiness instead of re-parsing `provider_config`.

For gateway-managed callbacks, set `provider_config.webhook.listen_addr` to a fixed literal loopback
IP and port and set `provider_config.webhook.path` to the adapter path. After explicit bridge binding,
the platform callback uses `/api/bridge-callbacks/{bridge_id}` on the verified public gateway and is
proxied only to that loopback target. Webhook registration uses the live `gateway_ingress.url` when
available. A bridge without a gateway binding keeps its validated `webhook_public_url` and external
proxy unchanged; its health omits gateway ingress.

Terminal replies are split provider-side on natural boundaries with `(N/M)` markers. CompozyOS measures Slack at 40,000 UTF-16 code units, Telegram at 4,096 UTF-16 code units, Discord at 2,000 Unicode code points, Teams at 28,000 Unicode code points, Google Chat at 32,000 UTF-8 bytes, and WhatsApp at 4,096 Unicode code points. Every multi-chunk delivery acknowledges its last remote message. Edit-capable providers keep an oversized non-terminal response in one mutable preview and materialize its continuations only on the terminal update. Slack converts common Markdown to mrkdwn; Telegram sends escaped MarkdownV2 and retries a typed parse rejection as plain text.

Configure the typed `delivery_defaults.progress` block with `tool_progress` (`off`, `new`, `all`, or `verbose`), `grouping` (`accumulate` or `separate`), `typing`, and `reactions`. Slack, Telegram, and Discord default to `new` plus `accumulate` with typing and reactions enabled; other platforms default to `off` plus `accumulate` with both affordances disabled unless the instance overrides them. `new` deduplicates consecutive starts but still emits completed and failed phases.

Slack, Telegram, Discord, Teams, and Google Chat can update an accumulated progress bubble; Slack and Telegram apply their platform dialects to the daemon-rendered line. WhatsApp is append-only, so prefer `new` plus `separate` when enabling its sparse one-line statuses. GitHub and Linear acknowledge progress without writing to issues.

The CLI exposes the same fields on `bridge create` and `bridge update`:

    --delivery-progress <off|new|all|verbose>
    --delivery-progress-grouping <accumulate|separate>
    --delivery-progress-typing[=true|false]
    --delivery-progress-reactions[=true|false]

Use structured output to inspect the saved resource after mutation. An adapter that has not registered a progress handler acknowledges these events without a provider-side effect; final answer delivery remains independent. Progress previews are daemon-rendered and redacted before they cross the extension boundary.

Supported Slack and Telegram message edits reach the agent as a typed `edit` prompt block. Slack, Telegram, and Google Chat replies include quoted parent text and author only when an embedded snapshot or the bounded workspace/instance/conversation cache has it; a miss stays empty and never triggers a provider fetch. At startup, CompozyOS reconciles durable in-flight delivery checkpoints before accepting new prompt or registration side effects. The ledger contains routing, sent/acknowledged sequence, remote-message, terminal, and aggregate-metric state—not streamed response or progress text. A sequence sent but not acknowledged is terminalized locally as indeterminate with no provider replay. An unfinished row without an unmatched send intent gets one write-ahead terminal error post, including on append-only providers or when its old remote anchor no longer exists.

A bridge route reuses its active CompozyOS session. If that session is busy, ingress retries admission locally up to three times within roughly five seconds; it does not automatically queue, interrupt, or steer the turn. Stop the route's `session_id` with `compozy session stop` when the next accepted provider event must start a clean session; the old transcript remains history and the route rebinds to a replacement.

Distinguish restart recovery from a remotely committed mutation whose required result was unavailable. After a provider accepts a mutation but its response or required ID cannot be materialized, adapters return `CommittedMutationError`; bridgesdk emits `committed_result_unavailable`, and the broker records a terminal error without replay or a fabricated remote ID. `compozy bridge send-test --json` reports that status, omits `remote_message_id`, and returns a redacted error; it is a direct control probe, so branch on `status` rather than command success and do not expect a broker ledger row or health-failure metric. Inspect the provider conversation before any manual resend because the remote artifact may already exist. An indeterminate progress mutation drops only that progress bubble; later final text remains eligible.

## Diagnostics Order

When a session behaves unexpectedly:

1. Run `compozy session status <id> -o json` to classify lifecycle and provider state.
2. Run `compozy session health <id> -o json` or `compozy session inspect <id> --include-wake-events -o json` when wake policy, stale health, or Heartbeat state is relevant.
3. Read `compozy session events <id>` for startup, prompt, tool, stop, and error events.
4. Read `compozy session history <id>` or `compozy session recap <id> -o json` for turn-grouped output and deterministic recent context.
5. Check workspace and agent resolution if the wrong prompt, tools, or skills appear.
6. Run `compozy doctor -o json` and only then check provider command availability or external auth state.
7. Use `compozy session repair <id> --dry-run -o json` before any repair write.

Do not treat stale UI state, chat messages, or memory notes as runtime authority.

## Status, Doctor, Logs, And Support

`compozy observe overview -o json` is the global home read model, optionally workspace-scoped: everything currently waiting on the user (`attention` items carry only the verbs the daemon accepts — `approve`, `reject`, `retry`, `open`), today's terminal counters, 14-day run outcomes, retention-bounded daily token usage with estimated cost and per-agent share (`--usage-window 7|30|90`), an hour-by-weekday event pulse, today's network message and hook-dispatch counters, and a `freshness` block. `--workspace <ref>` scopes aggregates to one workspace; omitting it selects the global home scope. The same payload serves `GET /api/observe/overview` on HTTP and UDS and backs the web home dashboard.

`compozy status -o json` is the consolidated daemon-wide status surface for daemon health, providers, MCP servers, config apply status, schema migration streams, and log tail summary. To resolve skill diagnostics for one workspace, call `GET /api/status?workspace_id=<id>` or `GET /api/status?workspace=<id|name|path>`; bare `compozy status` does not select a workspace. Inspect `schema_streams` after startup to confirm that the global and memory streams report their expected version, applied migration count, and digest. An incompatible daemon-global `compozy.db` is refused during boot, before readiness. By contrast, an incompatible per-session `events.db` can be discovered after the daemon is ready when a reader such as `compozy session history <id> -o json` or `GET /api/workspaces/{workspace_id}/sessions/{session_id}/history` opens that session; that operation fails without making the healthy daemon-global store unavailable.

`GET /api/status/identity` is the bounded HTTP/UDS process and listener identity used by the native
desktop shell for frequent liveness checks. It intentionally omits runtime diagnostics; use
`compozy status -o json`, `GET /api/status`, or `compozy doctor -o json` for inspection.

Inspect `.subprocess_health` in `compozy status -o json` and run
`compozy doctor --only runtime.subprocess_health -o json` for active ACP subprocess verdicts. With the
default `daemon.subprocess_health_escalation_threshold = 3`, three consecutive failed checks—or an
unexpected ACP process exit—move the exact linked nonterminal task run to `needs_attention` once.
Terminal runs stay terminal. After repairing the provider or command, use
`compozy task run recover <run-id> --reason <reason> -o json`; CompozyOS never restarts the subprocess
automatically. Set the threshold to `0` and restart to keep diagnostics without task mutation.

For workspace-scoped MCP servers and bridges, `compozy doctor -o json` also reports durable dead-runtime
marks with the workspace, entity, redacted reason, and mark time. A mark follows five consecutive
confirmed permanent failures. Ordinary attempts are suppressed until the 60-second recovery window
opens; then a runtime probe may try once, and success clears the mark without a daemon restart.
Doctor is read-only and never consumes that recovery attempt. There is no manual clear/revive
surface: use the diagnostic to repair the underlying command, configuration, permission, or
authentication problem, then let the next due runtime access recover automatically.

For `legacy_database`, stop CompozyOS, cold-move the complete containing `COMPOZY_HOME` or workspace `.compozy` family, and select a separate fresh home. Preserve every sibling database and SQLite sidecar together; never edit migration history or move one live file. For `schema_ahead`, first use a newer compatible CompozyOS binary against the stopped, intact family—the state-preserving recovery. Use a fresh home only if discarding that state is acceptable. If CompozyOS reports SQLite corruption, stop it and cold-copy the complete containing family before inspection. CompozyOS leaves the named database and its `-wal` and `-shm` sidecars unchanged instead of quarantining or recreating them; diagnose a copy, then restore a complete known-good family or select a fresh home only when discarding the retained state is acceptable. Stopped-daemon provider-auth, extension, and MCP-auth direct opens emit one JSON error document with `diagnostic.code` set to `legacy_database` or `schema_ahead`; use its surface and canonical-path evidence instead of parsing prose. `compozy doctor -o json` runs diagnostic probes; `--only`, `--exclude`, and `--quiet` bound the probe set for agents. Its `runtime.memory` item reports the latest daemon-owned heap, goroutine, uptime, and resident-memory snapshot. Treat `resident_memory_kind=peak` as a high-water mark, not current use. When `enabled=false`, set `daemon.memory_report_interval` above zero and restart the daemon; there is no native doctor tool.

`compozy logs --follow -o jsonl` streams redacted runtime logs over SSE. Use filters such as `--session`, `--workspace`, `--run`, `--actor kind:id`, `--provider`, `--component`, `--outcome`, and `--error-only` before broad log reads.

`redact.enabled` controls the additive heuristic for likely credentials in content and log
messages. The daemon snapshots it at boot; `compozy config set redact.enabled <true|false> -o json` or
the live `compozy__config_set` descriptor records restart-required desired state and does not change the
running process. Exact claim-token, secret-reference, and registered-secret protections remain
active when the heuristic is disabled. Correlation IDs, hashes, digests, and fingerprints stay
structural rather than heuristic candidates.

`compozy support bundle --yes` creates and downloads a redacted support bundle. It may include status, doctor, provider, event-summary, log-tail, and config-apply snapshots unless `--no-status` is passed. Treat support bundles as operator artifacts, not native tool calls.

For a failed desktop start, use `compozy app diagnose -o json` before daemon diagnostics. It returns
the redacted desktop report even when the daemon is unavailable. To save a local-only archive, use
`compozy app diagnose --bundle --yes -o json`; this explicit write never uploads or deletes data and
is separate from the daemon-owned support bundle. The archive contains `manifest.json` and may
include bounded, redacted `desktop.log` and `desktop-bootstrap.jsonl` tails from the current boot;
it never includes `compozy.log`, raw logs, databases, configuration, credentials, sessions, or
transcripts.

## Runtime Boundaries

CompozyOS must remain agent-manageable. Any runtime capability that affects state should have a deterministic CLI, HTTP/UDS, or tool path with machine-readable output. UI-only management is incomplete.

Management flows involving daemon lifecycle, raw secrets, OAuth, trust roots, provider bootstrap, destructive repair, and cross-session terminal-state mutation stay on control surfaces unless CompozyOS explicitly exposes a scoped tool for them.

Marketplace catalog configuration is global-only because its projection and refresh service are global. `compozy__config_set` and `compozy__config_unset` may change `marketplace.catalog.ttl` and `marketplace.catalog.timeout` at global scope; each mutation runs the daemon settings apply lifecycle and returns the real `applied`, `apply_record_id`, `active_generation`, `next_action`, and reconciliation diagnostics. `marketplace.catalog.base_url` is a trust root and remains operator-only through global `compozy config set`. Workspace overlays and workspace-scoped writes are rejected.
