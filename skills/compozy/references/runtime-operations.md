# Runtime Operations

## Contents

- Operating model and beta distribution
- Daemon drain
- Session lifecycle and CLI
- Background roles and usage cost
- MCP serve and onboarding
- Automation suggestions
- Messaging bridge delivery, diagnostics, and runtime boundaries

## Operating Model

Compozy is a local-first daemon that starts ACP-compatible agents as managed subprocesses, records events, and exposes runtime control through CLI, HTTP/SSE, UDS, and agent tools. Treat the daemon as the source of truth for sessions, events, task state, network rooms, memory, skills, and extension resources.

Do not manage runtime state by editing SQLite databases, process internals, or generated projections. Use public Compozy surfaces with structured output.

## v0.3 Beta Distribution

Keep beta installations on the beta line. Use the verified hosted installer,
`npm install -g @compozy/cli@beta`, or
`go install github.com/compozy/compozy@v0.3.0-beta.2`. A beta build's
`compozy update` follows newer v0.3 beta releases and never offers the v0.2
stable line. Homebrew remains on deprecated v0.2 during beta and returns with
v0.3.0 stable; do not recommend it for a v0.3 beta install. Existing v0.2
operators must follow the migration guide and use `legacy/v0.2` only for
critical maintenance.

## Daemon Drain

Use `compozy drain -o json` (or `POST /api/drain` over HTTP/UDS) to close daemon-global new-work admission while admitted prompts and claimed runs finish. The stable response is `{"state":"draining"}`; repeated calls are no-ops. New session/prompt, task-run enqueue, retry/recover, and run-claim attempts return 503 with `daemon is draining; new work admission is closed`, while cancellation, terminal transitions, and teardown remain available.

Confirm drain through `.daemon.status == "draining"` in `compozy status -o json` or the informational `daemon_draining` item in `compozy doctor -o json`. Use `compozy undrain -o json` or `POST /api/undrain` to restore `{"state":"active"}`. Drain is in-memory, applies across every workspace, and clears on daemon restart.

## Session Lifecycle

Compozy sessions are daemon-owned runtimes. Common states:

- starting - the daemon accepted the session and is booting the provider.
- active - the provider is connected and ready for prompts.
- stopping - shutdown has started.
- stopped - the runtime exited. The durable record remains inspectable, but the public lifecycle is terminal.

Session types include user sessions and daemon-managed sessions such as dream, system, coordinator, worker, and reviewer sessions. Do not infer authority from a session type alone. Use the session context and daemon tools to confirm what the current session may do.

With `roles.auto_title.enabled = true`, an unnamed user session receives at most one daemon-owned durable title after its first assistant response is persisted. Configure its agent, provider, model, reasoning, and fallback routes under `[roles.auto_title]`. An explicit session name wins any race; daemon-managed session types are ineligible. Treat the persisted session name as catalog identity and leave the session unnamed when generation is disabled or fails.

Attachability is explicit live runtime state. Use `compozy session list --resumable -o json` before
`compozy session resume`. The command acquires an attach lease for an eligible live session; it does
not restart `stopped`, and stopped sessions reject attach.

After prompt admission, the daemon owns the turn lifetime. Closing a browser tab, navigating away from the web app, dropping an SSE stream, or disconnecting a CLI/UDS response only detaches that viewer; it does not cancel the accepted prompt. Use explicit runtime intent such as `compozy session stop`, prompt cancel, or interrupt controls when cancellation is required.

A dedicated prompt interrupt followed by steer submits the canceled authored prompt plus the explicit correction once under the new input generation. A plain interrupt replacement discards the canceled text. Do not retry a consumed salvage or bypass generation fencing.

The event store and materialized transcript are the durable source of truth for reattach. Transcript GET returns the newest bounded `entries` page plus `epoch`, `generation`, `max_sequence`, and `has_older`; request older entries with `before_sequence=next_before_sequence`. Each entry carries immutable `start_sequence` identity and its latest shaping `sequence`. Do not reconstruct session state from UI cache, memory notes, or JSONL sidecars.

Treat `transcript_marker.file_mutation_unverified` as a required verification signal: one or more persisted `edit` mutations failed without a later successful mutation for the same path in that turn. Inspect the bounded paths and verify them before trusting completion claims; the marker is advisory and does not replace filesystem inspection.

Pressure compaction preserves complete prior turns in the workspace checkpoint before marking their
event rows archived. Archived rows remain visible to `compozy session events` and `compozy session history`,
but degraded replay excludes them to avoid duplicating checkpoint-covered context. Inspect
`session.compaction_fired` for the admitted sequence span; the event is correlation evidence, not a
success verdict for the later archive.

The HTTP/UDS stream defaults to `transcript_snapshot`, batched `transcript_delta`, and terminal `session_stopped` frames. Reconnect with the last SSE cursor plus the snapshot's `epoch` and `generation`; a fence mismatch returns an explicit reset snapshot. The removed `replay` query is invalid. Use `frames=raw` for persisted `SessionEventPayload` rows; `compozy session events --follow` already requests raw frames.

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
    compozy session new --provider codex --model gpt-5.6-sol --reasoning-effort high --speed fast --prompt "Inspect the failing build."
    compozy session new --agent general --no-wait -o json
    compozy session list --all -o json
    compozy session list --type user --state active --sort last_activity -o json
    compozy session list --resumable -o json
    compozy session status <session-id> -o json
    compozy session health <session-id> -o json
    compozy session inspect <session-id> --include-wake-events -o json
    compozy session recap <session-id> --limit 20 -o json
    compozy session events <session-id> --follow
    compozy session events <session-id> --after 42
    compozy session history <session-id>
    compozy session history <session-id> --last 20 --after 42
    compozy session prompt <session-id> "Summarize the last three tool results."
    compozy session resume <session-id>
    compozy session resume --latest --workspace checkout-api
    compozy session stop <session-id>
    compozy session remove <session-id>
    compozy session repair <session-id> --dry-run -o json
    compozy session soul refresh <session-id> --expected-digest sha256:old -o json
    compozy session approve <session-id> --request-id req_123 --turn-id turn_123 --decision allow-once
    compozy session clarify pending <session-id> -o json
    compozy session clarify answer <session-id> <request-id> --choice 1 -o json
    compozy session clarify answer <session-id> <request-id> --text "Use staging" -o json
    compozy session wait <session-id>

`compozy session new` waits for the accepted session to become `active` and returns a startup error if
the durable session stops with a failure. Use `--no-wait` when a controller needs the accepted
`starting` record immediately; then observe that session through status/list or the workspace detail
API until it becomes `active` or durably `stopped` with `failure.kind=startup_failure`.

Use `--prompt` with non-whitespace text to atomically create a session and stage its first user turn.
Compozy persists the `starting` session and the trimmed prompt before returning, then dispatches the
prompt once after the selected provider, model, reasoning effort, and requested speed become active. Speed defaults
to `normal`; `fast` is applied only through an unambiguous ACP select/value-ID option. An absent or
ambiguous speed capability is reported as `unsupported` without failing startup, while a provider
rejection fails before the first prompt with `speed_rejected`. Empty or
whitespace-only values keep create-only behavior. `--no-wait --prompt` returns the durable `starting`
record with the prompt still queued. A startup failure retains that prompt and its diagnostic
evidence, but the public attach command cannot restart that stopped session. Inspect the failure,
correct its cause, and create a new session for new work. Deleting the failed session removes the
queued input. Do not send the same prompt again to the original create request.

If a Compozy-native session tool is visible, prefer the tool because it is policy-aware and easier for the daemon to audit. Use the CLI when the tool is denied, absent, or explicitly requested.

## Background Roles

Compozy routes six daemon-owned background responsibilities through the closed `[roles]` roster:
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
an accepted ACP session is never silently rerouted. Immediately before each fallback attempt, Compozy
emits `role.fallback.used`; the event records that the route was tried, not that it succeeded.

Session-backed roles accept `enabled`, `agent`, `provider`, `model`, `reasoning_effort`, and
`fallback_chain`. Coordinator additionally owns `ttl`, `max_children`, and
`max_active_sessions_per_workspace`. The in-process `memory_controller` has no `agent`; it owns
`timeout`, `top_k`, `prompt_version`, and `max_tokens_out`. Do not move Loop runtime defaults/rules,
TaskExecutionProfile selectors, automation resources, or subsystem policy into `[roles]`.

### Usage cost truth

The session usage endpoint
`GET /api/workspaces/{workspace_id}/sessions/{session_id}/usage` returns token totals plus
`cost_status` and `cost_source`. Interpret money by status: `actual` is agent-reported;
`estimated` is a catalog-rate projection; `included` has no amount because a `native_cli` provider
owns subscription billing; `unknown` has no amount because rates or aggregate provenance are
incompatible. Never treat `included` as a confirmed account balance or add `actual` and `estimated`.
Sources are `agent_reported`, `catalog_config`, `models_dev`, `builtin`, or `none`.
Configured model input, output, cache-read, cache-write, and reasoning rates affect subsequent
estimates only; Compozy does not reprice stored token statistics after a catalog or config change. Every
nonzero bucket requires its own finite, non-negative rate. Never infer a cache rate from input or a
reasoning rate from output.

Prefer `compozy session usage <session-id> -o json` when an agent needs the same aggregate over UDS.

`compozy session stop` preserves durable history and ends attach eligibility. `compozy session remove`
is destructive: it stops an active runtime when necessary, then removes the catalog row and persisted
session directory. Use removal only when the operator intends to discard that history.

The session catalog is counted and workspace-scoped. Dream sessions are internal and never appear in catalog results. HTTP and UDS clients can filter exact public session type with `type=user|system|coordinator|spawned`; the CLI exposes the same filter as `--type`. Browser integrations should subscribe once to `/api/sessions/catalog-stream`, route each wake signal by its authoritative `workspace_id`, and refetch that workspace's catalog page instead of incrementing local counters.

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

Native session tools are read-oriented. Clarification answers, recap, repair, approval, session
inspect, and Soul refresh use CLI/HTTP/UDS management surfaces unless the live registry exposes a
scoped native tool. `compozy__clarify` asks from inside the active session; it does not answer another
session's question.

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

Credential-bearing API, OAuth, and service destinations are operator-owned adapter environment, not instance configuration. `provider_config` rejects `api_base_url`, `oauth_token_url`, `service_url`, `openid_metadata_url`, and `token_url`; use the provider's `COMPOZY_BRIDGE_*` process variables for trusted overrides. Provider clients use `bridgesdk.CredentialedHTTPClient`, returning the original `3xx` for classification without forwarding credentials or replaying mutation bodies. `webhook.public_url` must be public HTTPS, and verification blocks internal/special-use addresses, proxying, and redirects before reachability is attempted. Bridge reads expose the validated callback as optional `webhook_public_url`; clients use that projection for setup readiness instead of re-parsing `provider_config`.

Terminal replies are split provider-side on natural boundaries with `(N/M)` markers. Compozy measures Slack at 40,000 UTF-16 code units, Telegram at 4,096 UTF-16 code units, Discord at 2,000 Unicode code points, Teams at 28,000 Unicode code points, Google Chat at 32,000 UTF-8 bytes, and WhatsApp at 4,096 Unicode code points. Every multi-chunk delivery acknowledges its last remote message. Edit-capable providers keep an oversized non-terminal response in one mutable preview and materialize its continuations only on the terminal update. Slack converts common Markdown to mrkdwn; Telegram sends escaped MarkdownV2 and retries a typed parse rejection as plain text.

Configure the typed `delivery_defaults.progress` block with `tool_progress` (`off`, `new`, `all`, or `verbose`), `grouping` (`accumulate` or `separate`), `typing`, and `reactions`. Slack, Telegram, and Discord default to `new` plus `accumulate` with typing and reactions enabled; other platforms default to `off` plus `accumulate` with both affordances disabled unless the instance overrides them. `new` deduplicates consecutive starts but still emits completed and failed phases.

Slack, Telegram, Discord, Teams, and Google Chat can update an accumulated progress bubble; Slack and Telegram apply their platform dialects to the daemon-rendered line. WhatsApp is append-only, so prefer `new` plus `separate` when enabling its sparse one-line statuses. GitHub and Linear acknowledge progress without writing to issues.

The CLI exposes the same fields on `bridge create` and `bridge update`:

    --delivery-progress <off|new|all|verbose>
    --delivery-progress-grouping <accumulate|separate>
    --delivery-progress-typing[=true|false]
    --delivery-progress-reactions[=true|false]

Use structured output to inspect the saved resource after mutation. An adapter that has not registered a progress handler acknowledges these events without a provider-side effect; final answer delivery remains independent. Progress previews are daemon-rendered and redacted before they cross the extension boundary.

Supported Slack and Telegram message edits reach the agent as a typed `edit` prompt block. Slack, Telegram, and Google Chat replies include quoted parent text and author only when an embedded snapshot or the bounded workspace/instance/conversation cache has it; a miss stays empty and never triggers a provider fetch. At startup, Compozy reconciles durable in-flight delivery checkpoints before accepting new prompt or registration side effects. The ledger contains routing, sent/acknowledged sequence, remote-message, terminal, and aggregate-metric state—not streamed response or progress text. A sequence sent but not acknowledged is terminalized locally as indeterminate with no provider replay. An unfinished row without an unmatched send intent gets one write-ahead terminal error post, including on append-only providers or when its old remote anchor no longer exists.

A bridge route reuses its active Compozy session. If that session is busy, ingress retries admission locally up to three times within roughly five seconds; it does not automatically queue, interrupt, or steer the turn. Stop the route's `session_id` with `compozy session stop` when the next accepted provider event must start a clean session; the old transcript remains history and the route rebinds to a replacement.

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

Inspect `.subprocess_health` in `compozy status -o json` and run
`compozy doctor --only runtime.subprocess_health -o json` for active ACP subprocess verdicts. With the
default `daemon.subprocess_health_escalation_threshold = 3`, three consecutive failed checks—or an
unexpected ACP process exit—move the exact linked nonterminal task run to `needs_attention` once.
Terminal runs stay terminal. After repairing the provider or command, use
`compozy task run recover <run-id> --reason <reason> -o json`; Compozy never restarts the subprocess
automatically. Set the threshold to `0` and restart to keep diagnostics without task mutation.

For workspace-scoped MCP servers and bridges, `compozy doctor -o json` also reports durable dead-runtime
marks with the workspace, entity, redacted reason, and mark time. A mark follows five consecutive
confirmed permanent failures. Ordinary attempts are suppressed until the 60-second recovery window
opens; then a runtime probe may try once, and success clears the mark without a daemon restart.
Doctor is read-only and never consumes that recovery attempt. There is no manual clear/revive
surface: use the diagnostic to repair the underlying command, configuration, permission, or
authentication problem, then let the next due runtime access recover automatically.

For `legacy_database`, stop Compozy, cold-move the complete containing `COMPOZY_HOME` or workspace `.compozy` family, and select a separate fresh home. Preserve every sibling database and SQLite sidecar together; never edit migration history or move one live file. For `schema_ahead`, first use a newer compatible Compozy binary against the stopped, intact family—the state-preserving recovery. Use a fresh home only if discarding that state is acceptable. If Compozy reports SQLite corruption, stop it and cold-copy the complete containing family before inspection. Compozy leaves the named database and its `-wal` and `-shm` sidecars unchanged instead of quarantining or recreating them; diagnose a copy, then restore a complete known-good family or select a fresh home only when discarding the retained state is acceptable. Stopped-daemon provider-auth, extension, and MCP-auth direct opens emit one JSON error document with `diagnostic.code` set to `legacy_database` or `schema_ahead`; use its surface and canonical-path evidence instead of parsing prose. `compozy doctor -o json` runs diagnostic probes; `--only`, `--exclude`, and `--quiet` bound the probe set for agents. Its `runtime.memory` item reports the latest daemon-owned heap, goroutine, uptime, and resident-memory snapshot. Treat `resident_memory_kind=peak` as a high-water mark, not current use. When `enabled=false`, set `daemon.memory_report_interval` above zero and restart the daemon; there is no native doctor tool.

`compozy logs --follow -o jsonl` streams redacted runtime logs over SSE. Use filters such as `--session`, `--workspace`, `--run`, `--actor kind:id`, `--provider`, `--component`, `--outcome`, and `--error-only` before broad log reads.

`redact.enabled` controls the additive heuristic for likely credentials in content and log
messages. The daemon snapshots it at boot; `compozy config set redact.enabled <true|false> -o json` or
the live `compozy__config_set` descriptor records restart-required desired state and does not change the
running process. Exact claim-token, secret-reference, and registered-secret protections remain
active when the heuristic is disabled. Correlation IDs, hashes, digests, and fingerprints stay
structural rather than heuristic candidates.

`compozy support bundle --yes` creates and downloads a redacted support bundle. It may include status, doctor, provider, event-summary, log-tail, and config-apply snapshots unless `--no-status` is passed. Treat support bundles as operator artifacts, not native tool calls.

## Runtime Boundaries

Compozy must remain agent-manageable. Any runtime capability that affects state should have a deterministic CLI, HTTP/UDS, or tool path with machine-readable output. UI-only management is incomplete.

Management flows involving daemon lifecycle, raw secrets, OAuth, trust roots, provider bootstrap, destructive repair, and cross-session terminal-state mutation stay on control surfaces unless Compozy explicitly exposes a scoped tool for them.

Marketplace catalog configuration is global-only because its projection and refresh service are global. `compozy__config_set` and `compozy__config_unset` may change `marketplace.catalog.ttl` and `marketplace.catalog.timeout` at global scope; each mutation runs the daemon settings apply lifecycle and returns the real `applied`, `apply_record_id`, `active_generation`, `next_action`, and reconciliation diagnostics. `marketplace.catalog.base_url` is a trust root and remains operator-only through global `compozy config set`. Workspace overlays and workspace-scoped writes are rejected.
