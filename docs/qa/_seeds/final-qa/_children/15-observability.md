---
name: 15-observability
title: Observability + Events + Diagnostics + Transcripts + Logging — Real-LLM QA Plan
description: Behavior-first QA scenarios for the observability spine — canonical event recording, append-only ledger semantics, durable-append-before-broadcast, SSE replay, diagnostics health probes, transcript replay equivalence, structured logging discipline, secret/claim_token redaction, and the canonical-event coverage matrix. Real Claude Code subagents required where the scenario calls for live behavior.
type: final-qa-child
module: observability
parent: ../_parent.md
provider_lanes: [claude-code]
authoritative_runtime_truth: internal/CLAUDE.md
references:
  - /Users/pedronauck/Dev/compozy/agh/.compozy/tasks/final-qa/_references/openclaw-qa-patterns.md
  - /Users/pedronauck/Dev/compozy/agh/.compozy/tasks/final-qa/_references/hermes-qa-patterns.md
  - /Users/pedronauck/Dev/compozy/agh/CLAUDE.md
  - /Users/pedronauck/Dev/compozy/agh/internal/CLAUDE.md
  - /Users/pedronauck/Dev/compozy/agh/.compozy/tasks/final-qa/_children/03-acp-sessions.md
  - /Users/pedronauck/Dev/compozy/agh/.compozy/tasks/final-qa/_children/04-autonomy-kernel.md
---

# 15 — Observability + Events + Diagnostics + Transcripts + Logging

## 1. Module scope

This child stresses the **observability spine** of AGH: every place the
runtime records, projects, replays, or surfaces a domain event. The spine
is what every other module (autonomy, ACP/sessions, automation, network,
bridges, web UI, CLI) ultimately depends on for truth — if it lies, the
rest of AGH lies with it.

In-scope packages (file:line citations are repo-absolute):

| Surface                  | Path                                                                                | Authoritative API                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                            |
| ------------------------ | ----------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Observer / event ingest  | `/Users/pedronauck/Dev/compozy/agh/internal/observe/`                               | `Observer.OnSessionCreated` / `OnSessionStopped` / `OnAgentEvent` / `OnAgentEventForSession` (`internal/observe/observer.go:421-508`); `observeAgentEvent` validates + projects (`:510-555`); `writeObservedEventSummary` (`:685-699`); `aggregateObservedUsage` (`:701-728`); `writeObservedPermissionLog` (`:730-767`); query helpers (`internal/observe/query.go:14-117`); health snapshot (`internal/observe/health.go:99-200`); retention sweep (`internal/observe/retention.go:71-100`); task dashboard projection (`internal/observe/tasks.go:1-100`). |
| Logger                   | `/Users/pedronauck/Dev/compozy/agh/internal/logger/`                                | `New` builds a `slog.JSONHandler` (`internal/logger/logger.go:44-90`); `ParseLevel` (`:92-106`).                                                                                                                                                                                                                                                                                                                                                                                                                                              |
| Diagnostics / redaction  | `/Users/pedronauck/Dev/compozy/agh/internal/diagnostics/`                           | `Redact` (`internal/diagnostics/redact.go:61-70`) covers Bearer / quoted-JSON-secret / assignment / token-assignment / dynamic-secret patterns; `RedactAndBound` (`:96-110`); `RegisterDynamicSecret` (`:35-57`).                                                                                                                                                                                                                                                                                                                              |
| Transcript replay        | `/Users/pedronauck/Dev/compozy/agh/internal/transcript/`                            | `CanonicalSchema = "agh.session.event.v1"` (`internal/transcript/transcript.go:17`); `Assemble` (`:113-130`); turn-change flush (`:174-181`); tool lifecycle merge (`:239-318`); canonical encode/decode (`:737-822`).                                                                                                                                                                                                                                                                                                                       |
| SSE helpers              | `/Users/pedronauck/Dev/compozy/agh/internal/sse/`                                   | `Decode` reads SSE frames (`internal/sse/decode.go:33-96`); `Event` shape (`:18-23`); `maxEventBytes = 1MiB` (`:15-16`).                                                                                                                                                                                                                                                                                                                                                                                                                       |
| Append-only event store  | `/Users/pedronauck/Dev/compozy/agh/internal/store/`, `internal/store/sessiondb/`    | `SessionDB` per-session ledger; writes remain INSERT-only and sequence-monotonic; the embedded session Goose stream owns schema history and `atlas.sum`. Global ledger lives in `internal/store/globaldb/global_db_observe.go`. |
| Session SSE replay       | `/Users/pedronauck/Dev/compozy/agh/internal/api/core/`                              | `parseLastEventID` (`internal/api/core/session_stream.go:16-27`); `pollAndStreamSessionEvents` (`:69-101`); `pollSessionStreamTick` (`:104-151`); SSE write helpers (`internal/api/core/sse.go`); observe replay cursor (`internal/api/core/parsers.go:149-163`).                                                                                                                                                                                                                                                                              |
| Notifier fan-out         | `/Users/pedronauck/Dev/compozy/agh/internal/session/`                               | `Notifier` interface with `OnSessionCreated/Stopped` + `OnAgentEvent` (`internal/session/interfaces.go:306-311`); `AgentEventNotifier` extension (`:313-317`); `Manager.notifyAgentEvent` is invoked **after** durable append (`internal/session/notifier.go:5-16`).                                                                                                                                                                                                                                                                          |
| Claim-token redaction    | `/Users/pedronauck/Dev/compozy/agh/internal/task/`                                  | `rawClaimTokenPattern = agh_claim_[A-Za-z0-9_-]+` (`internal/task/lease.go:36`); `RedactClaimTokens` (`internal/task/lease.go:160-166`); raw-token-forbidden-on-the-wire policy is enforced by every event payload using `claim_token_hash` only (`internal/task/manager.go:3702-3791`).                                                                                                                                                                                                                                                       |
| Forensic classification  | `/Users/pedronauck/Dev/compozy/agh/internal/acp/`, `internal/session/`              | `IsLoadSessionResourceMissing` classifies stale ACP session id (`internal/acp/client.go:553-567`); `manager_lifecycle.go:77-101` calls fresh-start fallback and emits `session.resume.load_session_missing_fallback` (`internal/session/manager_lifecycle.go:86-90`).                                                                                                                                                                                                                                                                          |

Out of scope (covered by other children): full ACP/session lifecycle (03),
autonomy task_runs primitives (04), AGH Network channel transport (06),
automation cron/webhook (09), web UI surfaces (08).

## 2. Authoritative invariants under test

Coverage IDs follow the openclaw lowercase dotted/dashed convention. Every
scenario in §7 maps back to one or more of these IDs.

| Coverage ID                         | Invariant                                                                                                                                                                                                                                                          | Source                                                                                                                                                                |
| ----------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `obs.canonical-event`               | Every domain operation emits a canonical event with the documented correlation keys. There is a coverage-matrix test that fails if a required lifecycle path doesn't emit its canonical event.                                                                  | `internal/CLAUDE.md` "Observability" bullets (`internal/CLAUDE.md:48-52`)                                                                                              |
| `obs.append-only-ledger`            | `events.db` is append-only — only `INSERT` runs against the `events` table; updates/deletes are forbidden. The single legitimate exception is the v2 `strip_canonical_event_raw_payloads` migration, which scrubs `$.raw` from existing rows.                  | `internal/store/sessiondb/session_db.go:524-552, 788-808`                                                                                                              |
| `obs.durable-before-broadcast`      | Live broadcasters publish only **after** the durable append succeeds. Reconnect/replay uses `after_seq`/`Last-Event-ID`. No client may receive an event the daemon cannot later replay.                                                                          | `internal/CLAUDE.md:52`; `internal/api/core/session_stream.go:69-151`; `internal/session/notifier.go:5-16` (notifier fires via the manager prompt path after Record). |
| `obs.sequence-monotonic`            | Per-session event sequence IDs are strictly monotonic, assigned under exclusive writer-goroutine ownership; clock skew does not corrupt order.                                                                                                                  | `internal/store/sessiondb/session_db.go:474-505,532-550,723-728`                                                                                                       |
| `obs.replay.after-seq`              | Reconnect with `Last-Event-ID: N` (or `?after_sequence=N`) returns only events with sequence > N, in order, with no duplicates.                                                                                                                                 | `internal/api/core/session_stream.go:16-151`; `internal/api/core/parsers.go:28-39`                                                                                    |
| `obs.replay.cross-restart`          | Events appended before a daemon restart are still replayable after restart. SQLite `-wal`/`-shm` companion files survive recovery.                                                                                                                              | `internal/store/sessiondb/session_db.go:138-179`; `internal/CLAUDE.md` "agh-schema-migration" wal/shm note                                                              |
| `obs.transcript.replay-equivalence` | Persisted events replayed via `transcript.Assemble` produce the same `[]Message` ordering and content the SSE client originally saw — schema `agh.session.event.v1`.                                                                                            | `internal/transcript/transcript.go:17, 113-130, 174-318, 737-822`                                                                                                      |
| `obs.correlation-keys`              | Every SSE/event payload across the prompt → tool → hook → memory write → end lifecycle carries the documented keys: `session_id`, `parent_session_id`, `root_session_id`, `agent_name`, `task_id`, `run_id`, `claim_token_hash`, `lease_until`, `workflow_id`, `coordinator_session_id`, `scheduler_reason`, `hook_event`, `hook_name`, `spawn_depth`, `actor_kind`, `actor_id`, `release_reason`. | `internal/CLAUDE.md:48-50`                                                                                                                                             |
| `obs.claim-token-redaction`         | Raw `agh_claim_*` tokens never appear in logs, SSE, web responses, db rows, error payloads, settings views, or channel messages. Only `claim_token_hash` over the wire.                                                                                          | `internal/CLAUDE.md` Security Invariants (`internal/CLAUDE.md:55-56`); `internal/task/lease.go:36, 160-166`; `internal/task/manager.go:3702-3791`                       |
| `obs.secret-redaction-logs`         | Diagnostic / log text scrubs Bearer tokens, JSON-shaped secret keys, env-style assignments (`API_KEY=…`, `OPENAI_API_KEY=sk-…`), and runtime-registered secrets.                                                                                                  | `internal/diagnostics/redact.go:18-70`                                                                                                                                  |
| `obs.diagnostics.health`            | `Observer.Health` returns a typed snapshot with derived `status` ∈ {`ok`, `degraded`}; aggregates persistence, retention, failures, agent probes, bridges, tasks, activities; CLI `agh observe health -o json` and `/api/observe/health` agree byte-for-byte.    | `internal/observe/health.go:30-200`; `internal/cli/observe.go:74-121`; `internal/api/httpapi/routes.go:118`                                                            |
| `obs.logging.structured-only`       | Production code uses `slog` exclusively; no `fmt.Println`, no `log.Print*`, no `println` outside test files and ignorable `cmd/agh-daytona-sidecar` boundary code.                                                                                               | `internal/logger/logger.go:85-89`; `agh-code-guidelines` skill                                                                                                          |
| `obs.errors.no-strings-contains`    | Error propagation uses `%w` and assertions use `errors.Is/As`; no `strings.Contains(err.Error(), …)` outside the documented sqlite-error edge cases.                                                                                                              | `agh-code-guidelines`; existing exemptions cataloged in §10                                                                                                            |
| `obs.query-engine`                  | `agh observe events --session <id> --since <ts> --type <t>` returns ordered events with stable pagination; large windows stream via SSE (`agh observe events --follow`).                                                                                          | `internal/observe/query.go:14-22`; `internal/cli/observe.go:23-71, 123-173`                                                                                            |
| `obs.acp-fresh-start-fallback`      | A stale ACP session id (`Resource not found`) is classified to a fresh-start fallback and an event records the classification — never propagated as a 5xx.                                                                                                       | `internal/acp/client.go:553-567`; `internal/session/manager_lifecycle.go:77-101`                                                                                       |
| `obs.startup-pending-vs-crashed`    | A startup-pending session (in `m.pending`) is NOT marked `failed`. A crashed subprocess is. Distinct events are emitted.                                                                                                                                          | `internal/CLAUDE.md` "Forensic Bug Fixes" (`internal/CLAUDE.md:131-135`); `internal/session/manager_lifecycle.go:123-143`                                              |
| `obs.spawn-depth-cap`               | A deep lineage tree has every event at every level carrying `root_session_id`; `spawn_depth > MaxDepth` is rejected with a typed error and a `spawn.pre_create` deny event. Default `MaxDepth = 1` (`DefaultSpawnMaxDepth`).                                       | `internal/session/spawn.go:17-18, 215-237`; `internal/store/session_lineage.go:13-83, 127-140`                                                                         |
| `obs.high-rate-no-loss`             | Sustained event ingestion at ≥ 10k events/s does not block upstream callers; ledger fsync batch policy is honored; no event lost (assert via `after_seq` tail equality).                                                                                          | `internal/store/sessiondb/session_db.go:117-179, 460-505`                                                                                                              |
| `obs.retention.sweep`               | Observer retention sweep is enabled when configured; `last_sweep_status` transitions `pending` → `ok` (or `error`); deletes `event_summaries`, `token_stats`, `permission_log` rows older than `retention_days`. Per-session `events.db` is NOT swept here.       | `internal/observe/retention.go:14-100`; `internal/store/globaldb/global_db_observe.go:113-189`                                                                          |

## 3. Operating model

QA mode is **real-scenario** (per the standing directive on real-scenario
QA), not pytest-style assertions. Every scenario:

- Runs against an isolated `AGH_HOME` with unique daemon ports + tmux-bridge
  socket (per `agh-worktree-isolation`).
- Resolves provider auth from the bootstrap manifest according to each
  provider contract: bound-secret, brokered, and explicitly isolated-home
  lanes use `PROVIDER_HOME` / `PROVIDER_CODEX_HOME`, while `native_cli`
  lanes with `home_policy=operator` preserve the operator `HOME` unless the
  scenario explicitly validates isolated provider-home behavior.
- Uses real Claude Code (`claude-opus-4-8` for primary lifecycle stress;
  `claude-sonnet-5` for spawned children where indicated) as the
  subprocess driver. Cross-driver parity with OpenClaw / Hermes is already
  covered by child 03; this child does not re-run that matrix unless a
  scenario explicitly tags `provider_parity: required`.
- Emits four artifacts under `.artifacts/qa/<run-id>/obs-XX/`:
  - `obs-XX-report.md` (Worked / Failed / Blocked / Follow-up)
  - `obs-XX-summary.json` (machine-readable)
  - `obs-XX-events.json` (EventStore rows scoped to the scenario window)
  - `obs-XX-output.log` (combined daemon stdout/stderr + SSE captures)
- Asserts against EventStore rows + `events.db`/`agh.db` SQL state +
  structured log JSON, never just process exit codes.

`mock-acp` is permitted only where determinism of a write-then-read race is
the test target (e.g. OBS-04 reconnect race, OBS-13 high-frequency burst).
The surrounding daemon, observer, store, and logger remain real.

## 4. Provider matrix

| Mode               | When                                                                                                | Driver                                                                                                                       |
| ------------------ | --------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------- |
| `real-claude-code` | Default for any scenario that needs a real LLM round-trip (tool dispatch, hook fire, transcript).  | `claude-opus-4-8`; spawned children may be `claude-sonnet-5`.                                                          |
| `mock-acp` (gate)  | Determinism gate for race-sensitive scenarios (OBS-04, OBS-13, OBS-15) so the assertions are stable. | `internal/e2elane` mock ACP server. Daemon, observer, persistence, SSE, redaction, logger remain real production code paths. |

## 5. Preconditions (apply to every scenario)

- Fresh QA bootstrap via the `agh-qa-bootstrap` skill. Manifest path saved
  to `bootstrap-manifest.json`; `bootstrap.env` exported into the shell
  before any `agh` command.
- Unique `AGH_HOME` per worktree (per the worktree-isolation directive).
- Bound-secret, brokered, and explicitly isolated-home auth staged into
  `PROVIDER_HOME` / `PROVIDER_CODEX_HOME`; `native_cli` providers with
  `home_policy=operator` intentionally use the operator `HOME` / native login
  state unless the scenario explicitly validates isolated provider-home
  behavior.
- Daemon started in background. HTTP / UDS listeners reachable.
- `make verify` is green on the SUT branch before QA runs.
- `AGH_WEB_API_PROXY_TARGET` exported when the daemon is not on `:2123` so
  any web QA running in parallel can reach the isolated daemon.

Provider-specific config:

```text
AGH_HOME=$HOME/.qa/obs-15/<scenario>/agh-home
AGH_DAEMON_HTTP=127.0.0.1:<unique-port>
AGH_DAEMON_UDS=$AGH_HOME/sock/uds.sock
PROVIDER_HOME=$AGH_HOME/provider-home
PROVIDER_CODEX_HOME=$AGH_HOME/provider-codex-home
AGH_WEB_API_PROXY_TARGET=http://127.0.0.1:<unique-port>
```

## 6. Cleanup (applies to every scenario)

- `agh daemon stop` (or kill PID from manifest).
- Archive `events.db` (per session) and `agh.db` snapshots before tearing
  down the AGH_HOME — observability scenarios depend on post-mortem replay.
- If any scenario uncovered a forbidden-needle hit (raw `agh_claim_*`,
  provider keys, secret leakage), DO NOT clean — the artifacts are
  shippability evidence.
- Tear down the worktree only after evidence artifacts are written.

## 7. Mandatory scenarios

### OBS-01 — Canonical event coverage matrix audit

```yaml qa-scenario
id: obs-01-coverage-matrix-audit
title: Every documented lifecycle path emits a canonical event with the required correlation keys; an audit test fails on any gap
theme: observability.coverage_matrix
coverage:
  primary:
    - obs.canonical-event
    - obs.correlation-keys
  secondary:
    - obs.append-only-ledger
risk: critical
live: true
provider: real-claude-code
preconditions:
  - One workspace, one queued task_run, one coordinator session, one
    spawned child session, one cron-driven enqueue, one hook deny, one
    memory write, one bridge route delivery (a pre-built fixture covers
    all of these in a single 2-minute scripted run).
docs_refs:
  - /Users/pedronauck/Dev/compozy/agh/internal/CLAUDE.md
  - /Users/pedronauck/Dev/compozy/agh/.compozy/tasks/final-qa/_children/04-autonomy-kernel.md
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/internal/observe/observer.go:421-555
  - /Users/pedronauck/Dev/compozy/agh/internal/store/sessiondb/session_db.go:524-552
  - /Users/pedronauck/Dev/compozy/agh/internal/task/manager.go:20-42
  - /Users/pedronauck/Dev/compozy/agh/internal/hooks/events.go:54-130
  - /Users/pedronauck/Dev/compozy/agh/internal/session/session.go:44-45
steps:
  1. Drive the full fixture script.
  2. Stop daemon (clean shutdown — no kill -9 here).
  3. Dump every row from every `events.db` plus `agh.db.event_summaries`
     into `obs-01-events.jsonl`.
  4. Cross-reference against the table in §11 (Canonical Event Coverage
     Matrix). For every row in §11, assert at least one event of the
     listed type appears in `obs-01-events.jsonl`.
  5. For each appearing event, parse the JSON payload (`events.content`
     or `event_summaries.summary`) and assert each required correlation
     key is present and non-empty (where applicable for the event type).
expected:
  - All §11 rows have ≥1 matching event.
  - All required correlation keys are present per row's "Required
    correlation keys" column.
  - The script writes a `coverage_matrix.json` summarizing
    `{event_type, observed: bool, correlation_keys_complete: bool}`
    that is checked in as evidence.
evidence:
  - `obs-01-events.jsonl`, `coverage_matrix.json`
  - The fixture's runtime log (`obs-01-output.log`)
failure_signatures:
  - Any §11 row with `observed: false` is a release blocker — the
    lifecycle path silently does not emit its canonical event.
  - Any required correlation key missing on an emitted event is a
    blocker — observability gap that breaks downstream correlation.
cleanup:
  - Archive `events.db` per session, archive `agh.db`, stop daemon.
```

### OBS-02 — Correlation-key completeness across one full real Claude Code session

```yaml qa-scenario
id: obs-02-correlation-keys-end-to-end
title: One real Claude Code session emits SSE events that carry every documented correlation key at every stage (start → prompt → tool dispatch → hook fire → memory write → end)
theme: observability.correlation
coverage:
  primary:
    - obs.correlation-keys
    - obs.canonical-event
  secondary:
    - obs.transcript.replay-equivalence
risk: high
live: true
provider: real-claude-code
preconditions:
  - One workspace; one workspace skill seeded with a benign
    `tool.pre_call` hook (logs only, no deny) so a hook fire is captured.
  - Memory subsystem enabled; the prompt asks the agent to record one
    fact ("Remember alpha-7 is the canary id") so memory writes happen.
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/internal/observe/observer.go:478-555
  - /Users/pedronauck/Dev/compozy/agh/internal/api/core/session_stream.go:29-46
  - /Users/pedronauck/Dev/compozy/agh/internal/hooks/events.go:54-130
  - /Users/pedronauck/Dev/compozy/agh/internal/session/spawn.go:215-237
steps:
  1. Open SSE stream for the session
     (`GET /api/sessions/$S/stream`, no `Last-Event-ID`).
  2. `agh session prompt $S "Read README.md, summarize, then remember
     'alpha-7 is the canary id'"`.
  3. Wait for SSE `finish`.
  4. Capture full SSE log to `obs-02-sse.jsonl`.
  5. Walk the SSE log; for each event with one of the canonical types
     (session.created, session.started, agent_message, tool_call,
     tool_result, hook.dispatch.completed/blocked, task.run.claimed,
     task.run.completed, memory.write.recorded), assert the required
     correlation keys are present in the event payload.
expected:
  - Every event carries `session_id` (always); `agent_name` (always);
    `parent_session_id` and `root_session_id` (always for events on
    spawned children, equal-to-self for root sessions); `task_id` /
    `run_id` / `claim_token_hash` / `lease_until` (for `task.run.*`);
    `workflow_id` (for events whose run carries it in metadata, per
    `internal/task/manager.go:2099`); `hook_event` / `hook_name` (for
    `hook.dispatch.*`); `actor_kind` / `actor_id` (for events caused by
    a peer, e.g. `task.run.claimed`); `spawn_depth` (for spawn events
    and child-session events).
  - No event has `agh_claim_<raw>` text in any field.
evidence:
  - `obs-02-sse.jsonl`
  - `correlation_audit.json` summarizing per-event-type completeness
failure_signatures:
  - Any required correlation key missing from any event of a covered
    type → observability gap; release blocker.
  - Any raw `agh_claim_<>=12-char>` substring → critical security leak.
cleanup:
  - `agh session stop $S`. Archive sinks.
```

### OBS-03 — Durable-append-before-broadcast under kill -9

```yaml qa-scenario
id: obs-03-durable-before-broadcast
title: kill -9 the daemon between durable append and live broadcast — every event a client could possibly receive must be replayable after restart
theme: observability.durability
coverage:
  primary:
    - obs.durable-before-broadcast
    - obs.replay.cross-restart
    - obs.append-only-ledger
  secondary:
    - obs.sequence-monotonic
risk: critical
live: true
provider: real-claude-code
preconditions:
  - One real Claude Code session emitting a long stream of agent_message
    chunks (long-running prompt, e.g. "Print every line of
    generated_long_file.txt").
  - SSE client (curl --no-buffer) attached, capturing every event seen
    on the wire.
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/internal/store/sessiondb/session_db.go:474-552
  - /Users/pedronauck/Dev/compozy/agh/internal/session/notifier.go:5-16
  - /Users/pedronauck/Dev/compozy/agh/internal/api/core/session_stream.go:69-151
steps:
  1. Start the long-running prompt; record SSE frames into
     `obs-03-sse-pre.jsonl` until ~20 events are observed.
  2. `kill -9 $AGH_DAEMON_PID`.
  3. Restart daemon (`agh daemon start`).
  4. `sqlite3 $AGH_HOME/sessions/$S/events.db
     'SELECT id, sequence, type FROM events ORDER BY sequence'`
     → save as `obs-03-events-post.jsonl`.
  5. Reconnect SSE with `Last-Event-ID: 0`; capture into
     `obs-03-sse-post.jsonl`.
expected:
  - Every event with `id` (== sequence) recorded in
    `obs-03-sse-pre.jsonl` is present in `obs-03-events-post.jsonl`
    with the same sequence and type. (No client received an event the
    daemon could not later replay.)
  - `obs-03-sse-post.jsonl` replays the exact same prefix sequence
    (events 1..N), then the daemon either resumes the prompt (if the
    upstream provider can recover) or emits a typed `error`/`finish`
    pair.
  - Event sequences are monotonic across restart (no gaps inside the
    pre-restart window; a new sequence may start where it left off).
evidence:
  - `obs-03-sse-pre.jsonl`, `obs-03-events-post.jsonl`,
    `obs-03-sse-post.jsonl`
  - Daemon log across the kill/restart boundary
failure_signatures:
  - SSE-pre contains an event whose sequence is missing from the
    post-restart events.db dump → broadcast-before-durable-append; the
    notifier fires before the writeEvent INSERT commits.
  - Sequence numbers reset to 1 after restart → `currentMaxSequence`
    not honored; orphaned writer state.
  - Replay returns events out of order → idx_events_sequence integrity
    broken.
cleanup:
  - Stop daemon, archive events.db.
```

### OBS-04 — SSE replay with `Last-Event-ID` after mid-stream disconnect

```yaml qa-scenario
id: obs-04-sse-replay-after-seq
title: After 1000 events, disconnect at 500 — reconnect with Last-Event-ID:500 returns only events 501..1000 in order, no duplicates
theme: observability.replay
coverage:
  primary:
    - obs.replay.after-seq
    - obs.sequence-monotonic
  secondary:
    - obs.append-only-ledger
risk: high
live: false
provider: mock-acp
preconditions:
  - Mock ACP server (the deterministic dispatcher) configured to emit
    1000 `agent_message` events for one prompt turn.
  - SSE client uses `Last-Event-ID` header semantics
    (`internal/api/core/handlers.go:521`).
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/internal/api/core/session_stream.go:16-27, 69-151
  - /Users/pedronauck/Dev/compozy/agh/internal/api/core/parsers.go:28-39, 149-163
  - /Users/pedronauck/Dev/compozy/agh/internal/store/sessiondb/session_db.go:351
steps:
  1. Send the 1000-event prompt; SSE stream open.
  2. After 500 events received, hard-close the TCP socket.
  3. Re-open `GET /api/sessions/$S/stream` with header
     `Last-Event-ID: 500`.
  4. Capture into `obs-04-sse-tail.jsonl`.
expected:
  - First event in the tail has `id == 501`.
  - Last event has `id == 1000`.
  - No event has `id <= 500`.
  - No duplicate `id` across the tail.
  - Tail count exactly 500.
evidence:
  - `obs-04-sse-head.jsonl` (first 500), `obs-04-sse-tail.jsonl`
    (events 501..1000), `events_db_dump.jsonl`
failure_signatures:
  - Any `id <= 500` in the tail → `parseLastEventID` regression or
    `AfterSequence` not applied.
  - Tail starts at 502 (off-by-one) → broken `>` vs `>=` in
    `Int64Clause("sequence", ">", query.AfterSequence)`.
  - Duplicate id → idempotency broken; client could double-process.
cleanup:
  - `agh session stop $S`.
```

### OBS-05 — SSE replay across a full daemon restart

```yaml qa-scenario
id: obs-05-replay-across-restart
title: Events appended pre-restart are replayable post-restart; SQLite -wal/-shm survive cold start
theme: observability.persistence
coverage:
  primary:
    - obs.replay.cross-restart
    - obs.append-only-ledger
  secondary:
    - obs.replay.after-seq
risk: high
live: true
provider: real-claude-code
preconditions:
  - One real session with ≥ 50 persisted events (multi-tool prompt).
  - Daemon configured with default WAL settings.
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/internal/store/sessiondb/session_db.go:138-179, 723-728
  - /Users/pedronauck/Dev/compozy/agh/internal/CLAUDE.md (agh-schema-migration wal/shm)
steps:
  1. Drive prompt to completion; record `events_count_before`,
     `transcript_hash_before` (sha256 of `agh session transcript $S`).
  2. `agh daemon stop` (graceful).
  3. `sqlite3 $AGH_HOME/sessions/$S/events.db 'PRAGMA quick_check'`
     → must return `ok`.
  4. Confirm `events.db-wal` and `events.db-shm` companion files
     present (or absent if checkpointed).
  5. `agh daemon start`.
  6. Send a follow-up prompt P2 over a fresh SSE connection without
     `Last-Event-ID`.
  7. Observe full SSE replay (events 1..N + new P2 events).
  8. `agh session transcript $S` → `transcript_hash_after`.
expected:
  - quick_check == ok.
  - events_count_after >= events_count_before.
  - transcript_hash_after starts with transcript_hash_before's prefix
    (the new tail is appended; nothing rewritten).
  - SSE on the second connection returns events 1..N in original
    sequence order, then continues with the new P2 events.
  - `SELECT MAX(version_id) FROM goose_db_version_session WHERE is_applied = 1`
    matches the embedded session migration head.
evidence:
  - `events_before.jsonl`, `events_after.jsonl`,
    `quick_check.txt`, `goose_version.txt`,
    `transcript_before.json`, `transcript_after.json`
failure_signatures:
  - `quick_check` reports corruption → wal/shm recovery regression.
  - `goose_db_version_session` is missing or below the embedded head → migrations did not apply.
  - Transcript hash before != prefix of transcript hash after →
    durable rows were rewritten, violating append-only.
cleanup:
  - Stop daemon.
```

### OBS-06 — claim_token redaction grep audit across every wire surface

```yaml qa-scenario
id: obs-06-claim-token-redaction-audit
title: Across logs, SSE, web responses, db rows, error payloads, and channel/inbox messages produced during a real run, NO raw agh_claim_* appears
theme: observability.security
coverage:
  primary:
    - obs.claim-token-redaction
  secondary:
    - obs.correlation-keys
risk: critical
live: true
provider: real-claude-code
preconditions:
  - One full task_run lifecycle (claim, heartbeat, release/complete)
    over real Claude Code, with 3+ minutes of activity, plus one
    coordinator-driven spawn so synthetic prompt metadata also fires.
  - All output sinks captured: combined daemon log, SSE replay log
    (`agh observe events --follow` AND `GET /api/sessions/$S/stream`),
    every documented HTTP endpoint that returns task_run state, web
    SPA SSE if a web client is attached, `agh.db` + each `events.db`
    raw dump, settings views (`agh config show -o json`), error
    payloads from a deliberately bad request.
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/internal/CLAUDE.md (Security Invariants)
  - /Users/pedronauck/Dev/compozy/agh/internal/task/lease.go:36, 160-166
  - /Users/pedronauck/Dev/compozy/agh/internal/task/manager.go:3702-3791
  - /Users/pedronauck/Dev/compozy/agh/internal/diagnostics/redact.go:18-70
steps:
  1. Drive the full lifecycle.
  2. Capture every sink listed in preconditions into separate files
     named `obs-06-<sink>.{log,jsonl}`.
  3. Run the audit:
     `rg -n 'agh_claim_[A-Za-z0-9_-]{12,}' obs-06-*.log obs-06-*.jsonl
     <(sqlite3 agh.db .dump) <(for f in $AGH_HOME/sessions/*/events.db;
     do sqlite3 $f .dump; done)`
expected:
  - Audit output is empty.
  - The only legitimate `agh_claim_` literal anywhere is the
    placeholder `agh_claim_[REDACTED]` produced by `RedactClaimTokens`
    (`internal/task/lease.go:160-166`); the audit grep skips it because
    `[REDACTED]` does not match `[A-Za-z0-9_-]{12,}` (no underscore-or-
    hyphen-bracket-letter run of 12+).
evidence:
  - `obs-06-redaction-audit.txt` (must be empty)
  - All sink files attached so a reviewer can re-run the grep.
failure_signatures:
  - Any hit (other than the deliberate placeholder, which the regex
    skips by length): critical security violation; the run cannot ship.
    Cite `internal/CLAUDE.md` "claim_token redaction is non-negotiable".
cleanup:
  - Archive sinks; do NOT clean if any hit appears.
```

### OBS-07 — Generic secret redaction in logs (Bearer / API_KEY / OAuth)

```yaml qa-scenario
id: obs-07-secret-redaction-logs
title: Every log emission scrubs Bearer tokens, env-style secret assignments, JSON-shaped credential keys, and runtime-registered secrets
theme: observability.security
coverage:
  primary:
    - obs.secret-redaction-logs
  secondary:
    - obs.logging.structured-only
risk: critical
live: true
provider: real-claude-code
preconditions:
  - A workspace skill seeded with a hook that *intentionally* logs a
    fake secret (`OPENAI_API_KEY=sk-fake-QA-OBS07-1234567890`,
    `Authorization: Bearer fakeBearerQA0BS07-abcdefg`,
    `{"api_key":"fake-quoted-OBS07"}`) into the daemon log so we can
    prove redaction at ingest.
  - A unit-test injection harness uses `RegisterDynamicSecret`
    (`internal/diagnostics/redact.go:35-57`) to register
    `dynamic-runtime-secret-OBS07` and asserts subsequent log lines
    have it scrubbed.
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/internal/diagnostics/redact.go:18-70
  - /Users/pedronauck/Dev/compozy/agh/internal/diagnostics/redact_test.go:8, 78
  - /Users/pedronauck/Dev/compozy/agh/internal/observe/health.go:228-229 (uses RedactAndBound)
steps:
  1. Drive a normal real Claude Code prompt that triggers the
     malicious-logging hook three times (once per secret shape).
  2. Capture combined daemon log to `obs-07-daemon.log`.
  3. Run a forbidden-needle scan:
     `rg -F 'sk-fake-QA-OBS07' obs-07-daemon.log` (must be empty)
     `rg -F 'fakeBearerQA0BS07' obs-07-daemon.log` (must be empty)
     `rg -F 'fake-quoted-OBS07' obs-07-daemon.log` (must be empty)
     `rg -F 'dynamic-runtime-secret-OBS07' obs-07-daemon.log`
     (must be empty)
  4. Assert the redaction placeholder `[REDACTED]` appears in each
     expected location instead.
expected:
  - All four needle scans return zero matches.
  - The structured field is the hashed/redacted form
    (`api_key=[REDACTED]`, `Authorization: Bearer [REDACTED]`).
evidence:
  - `obs-07-daemon.log` redacted; `obs-07-needle-scan.txt`
failure_signatures:
  - Any needle match → log-redaction regression. Cite
    `internal/diagnostics/redact.go` patterns and the dynamic-secret
    registry coverage.
cleanup:
  - Remove malicious skill; rotate fake secret strings.
```

### OBS-08 — Diagnostics health probe state machine

```yaml qa-scenario
id: obs-08-diagnostics-health-state-machine
title: Under healthy / degraded / starting / unhealthy states, agh observe health -o json and /api/observe/health agree, transitions emit a state-change event
theme: observability.health
coverage:
  primary:
    - obs.diagnostics.health
  secondary:
    - obs.canonical-event
risk: high
live: true
provider: real-claude-code
preconditions:
  - Daemon configured with retention enabled (`retention_days=7`,
    sweep interval shortened to 30s for the scenario).
  - Two synthetic failures pre-seeded so `health.failures.total >= 1`
    in the degraded phase.
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/internal/observe/health.go:30-200, 162-185
  - /Users/pedronauck/Dev/compozy/agh/internal/observe/retention.go:14-100
  - /Users/pedronauck/Dev/compozy/agh/internal/cli/observe.go:74-121
  - /Users/pedronauck/Dev/compozy/agh/internal/api/httpapi/routes.go:118
steps:
  1. Phase A (starting): immediately after daemon start, before any
     session, query both surfaces. Record `health_starting.json`.
  2. Phase B (healthy): create one session, run a normal prompt,
     query both surfaces. Record `health_healthy.json`.
  3. Phase C (degraded): pre-seed two failed sessions so
     `failures.total >= 1`; force a retention sweep error (e.g. by
     making `agh.db` read-only for 1 cycle); query both surfaces.
     Record `health_degraded.json`.
  4. Phase D (unhealthy proxy): kill the upstream provider binary
     while a probe target is configured; observe agent_probes
     transition to non-OK; query both surfaces. Record
     `health_unhealthy.json`.
  5. Diff CLI vs HTTP JSON (modulo timestamp drift) for each phase.
expected:
  - CLI and HTTP responses are byte-identical except for `uptime_seconds`
    and `version` fields and per-call timestamps.
  - Phase A `status == "ok"`, but `retention.last_sweep_status ==
    "pending"`.
  - Phase B `status == "ok"`, `failures.total == 0` (or pre-existing 0).
  - Phase C `status == "degraded"` because `persistenceStatus` returns
    degraded on retention sweep error AND `failures.status ==
    "degraded"` because `failures.total > 0`.
  - Phase D `status == "degraded"` because `agentProbeStatus` flips on
    a non-OK probe.
  - Each transition emits a corresponding event in the global ledger
    (`event_summaries` rows with type `health.status_changed` or
    equivalent — verify via `agh observe events --type
    health.status_changed`).
evidence:
  - `health_starting.json`, `health_healthy.json`,
    `health_degraded.json`, `health_unhealthy.json`,
    `cli_vs_http_diff.txt`, `health_transition_events.jsonl`
failure_signatures:
  - CLI/HTTP disagree → projection drift between transports.
  - Phase D returns `status == "ok"` despite a non-OK probe →
    `agentProbeStatus` regression (`health.go:178-185`).
  - No state-change event emitted on transition → observability gap.
cleanup:
  - Restore agh.db permissions, restart provider binary, stop daemon.
```

### OBS-09 — Logging discipline static audit

```yaml qa-scenario
id: obs-09-logging-discipline-audit
title: Production code uses slog only — no fmt.Println, no log.Print, no naked println; every audit hit must be in a documented exemption list
theme: observability.logging
coverage:
  primary:
    - obs.logging.structured-only
  secondary:
    - obs.errors.no-strings-contains
risk: high
live: false
provider: static
preconditions:
  - Repo at the SUT commit, fresh checkout.
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/internal/logger/logger.go:85-89
  - /Users/pedronauck/Dev/compozy/agh/CLAUDE.md "Critical Rules"
  - .agents/skills/agh-code-guidelines/
steps:
  1. Run the static audit grepset (each must produce zero hits OR
     match the documented exemption list):
     a. `rg -n 'fmt\.Println\(' --glob '!**/*_test.go'
        --glob '!cmd/agh-daytona-sidecar/**'
        cmd/ internal/`
     b. `rg -n 'log\.Print(f|ln)?\(' --glob '!**/*_test.go'
        --glob '!cmd/agh-daytona-sidecar/**'
        cmd/ internal/`
     c. `rg -nw 'println' --glob '!**/*_test.go'
        cmd/ internal/`
     d. `rg -n 'strings\.Contains.*\.Error\(\)' --glob '!**/*_test.go'
        cmd/ internal/`
  2. For each hit, look up §10 (Logging / error-string exemption list)
     and verify it is on the list with a written justification.
  3. Save audit output to `obs-09-static-audit.txt`.
expected:
  - Hits are limited to exactly:
    - `internal/sandbox/daytona/cmd/agh-daytona-sidecar/main.go` lines
      459/491 (`log.Printf`) — sidecar bootstrap before logger is wired;
      documented in §10.
    - `internal/extension/registry.go:539` (`strings.Contains` "no
      such table") — sqlite-error-string exemption; documented in §10.
    - `internal/subprocess/transport.go:253` ("token too long") —
      bufio scanner-error sentinel; documented in §10.
    - `internal/subprocess/process.go:638` ("file already closed") —
      os.PathError sentinel; documented in §10.
    - `internal/store/globaldb/global_db_task.go:155` and
      `global_db_bridge.go:1128` ("foreign key constraint failed") —
      sqlite-error sentinel; documented in §10.
    - `internal/store/globaldb/global_db_bundles.go:45` ("no such
      table") — first-boot bootstrap sentinel; documented in §10.
  - Any hit not in §10 is a release blocker.
evidence:
  - `obs-09-static-audit.txt`
failure_signatures:
  - Any new `fmt.Println` or `log.Print*` outside §10 → discipline
    violation; cite `agh-code-guidelines` skill.
  - Any new `strings.Contains(err.Error(), …)` outside §10 → switch
    to typed error + `errors.Is/As`; cite the same skill.
cleanup:
  - None (read-only static audit).
```

### OBS-10 — Transcript replay equivalence (live SSE vs persisted-events reconstruction)

```yaml qa-scenario
id: obs-10-transcript-replay-equivalence
title: Persisted events run through transcript.Assemble produce a Message list byte-equivalent to what the SSE client originally saw
theme: observability.replay
coverage:
  primary:
    - obs.transcript.replay-equivalence
    - obs.canonical-event
  secondary:
    - obs.append-only-ledger
risk: high
live: true
provider: real-claude-code
preconditions:
  - One real Claude Code session with ≥ 3 tool turns (e.g. "Read X,
    write summary to Y, run cat Y").
  - SSE captured to `obs-10-live-sse.jsonl`.
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/internal/transcript/transcript.go:17, 113-130, 174-181, 239-318, 737-822
  - /Users/pedronauck/Dev/compozy/agh/internal/store/sessiondb/session_db.go:74-86, 788-808
steps:
  1. Drive the multi-tool prompt; capture SSE.
  2. Stop session.
  3. `agh session events $S -o jsonl > obs-10-stored.jsonl`.
  4. `agh session transcript $S -o json > obs-10-replay-messages.json`.
  5. Reconstruct from live SSE: feed each frame's `data` payload
     through `transcript.UnmarshalAgentEvent`, synthesize
     `store.SessionEvent`s in order, then `transcript.Assemble`
     → `obs-10-live-messages.json`.
  6. Diff with `jq -S . obs-10-live-messages.json` vs
     `jq -S . obs-10-replay-messages.json`.
expected:
  - Diff is empty modulo whitespace.
  - Every persisted event has `json_extract(content,'$.schema') ==
    'agh.session.event.v1'` (run as SQL audit:
    `SELECT count(*) FROM events WHERE
    json_extract(content,'$.schema') != 'agh.session.event.v1'`
    → must equal 0).
  - Every Message has matching `id`, `role`, `content`, `tool_name`,
    `tool_input`, `tool_result`, ordering, `thinking_complete`.
evidence:
  - `obs-10-live-sse.jsonl`, `obs-10-stored.jsonl`,
    `obs-10-replay-messages.json`, `obs-10-live-messages.json`,
    `obs-10-diff.txt`, `obs-10-schema-audit.txt`
failure_signatures:
  - Diff non-empty → transcript projection drifts from the wire
    contract; cite `transcript.go:174-181` (turn-change flush) or
    `transcript.go:239-318` (tool lifecycle merge).
  - Schema audit count > 0 → migration v2 regression; raw payloads
    were not stripped or new rows lack the schema marker.
cleanup:
  - `agh session delete $S`.
```

### OBS-11 — Query engine correctness and pagination stability

```yaml qa-scenario
id: obs-11-query-engine
title: agh observe events --session <id> --since <ts> --type <t> returns ordered, complete, stable rows; --follow streams new events via SSE
theme: observability.query
coverage:
  primary:
    - obs.query-engine
    - obs.canonical-event
  secondary:
    - obs.replay.after-seq
risk: high
live: true
provider: real-claude-code
preconditions:
  - Two sessions S1, S2; each with ≥ 100 events.
  - One workspace with both sessions; `since` flag pointing at S1's
    third event timestamp.
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/internal/observe/query.go:14-22
  - /Users/pedronauck/Dev/compozy/agh/internal/cli/observe.go:23-71, 123-173
  - /Users/pedronauck/Dev/compozy/agh/internal/store/types.go:272-307
steps:
  1. `agh observe events -o json` (no filters) → save A.
  2. `agh observe events --session $S1 -o json` → save B.
  3. `agh observe events --type tool_call -o json` → save C.
  4. `agh observe events --since 2026-05-02T12:00:00Z -o json` → save D.
  5. `agh observe events --last 10 -o json` → save E (must be the last
     10 strictly).
  6. `agh observe events --follow -o jsonl` (in background) while a
     fresh prompt runs; capture into F; stop after 5s.
expected:
  - A is ordered by timestamp ascending; complete (matches
    `SELECT * FROM event_summaries`).
  - B == filter A on `session_id == $S1`.
  - C == filter A on `type == "tool_call"`.
  - D == filter A on `timestamp >= $since`.
  - E is last 10 of A.
  - F has at least 1 new event with `timestamp > <follow start ts>`.
  - Pagination is stable: re-running A twice returns identical row
    order; no row drops or duplicates.
evidence:
  - A..F json files, plus `obs-11-pagination-stable.txt` showing
    sha256(A_run1) == sha256(A_run2) (excluding `summary` truncation
    differences if any).
failure_signatures:
  - Filter mismatch: query result ≠ in-memory filter on the unfiltered
    set → SQL projection bug.
  - --follow returns no new events while events exist → broken SSE
    reconnect/poll loop in `streamObserveEvents`.
  - Pagination order changes between runs → missing
    `ORDER BY timestamp, id` stability.
cleanup:
  - `agh session stop $S1; agh session stop $S2`.
```

### OBS-12 — Append-only invariant: writes that mutate event rows are forbidden

```yaml qa-scenario
id: obs-12-append-only-invariant
title: No production code path UPDATE/DELETEs the events table inside session events.db (only INSERT and the v2 raw-strip migration)
theme: observability.persistence
coverage:
  primary:
    - obs.append-only-ledger
  secondary:
    - obs.canonical-event
risk: critical
live: true
provider: real-claude-code
preconditions:
  - Repo at SUT commit (static portion); a real session with events
    persisted (runtime portion).
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/internal/store/sessiondb/session_db.go:524-552, 788-808
  - /Users/pedronauck/Dev/compozy/agh/internal/CLAUDE.md "Append-only event store"
steps:
  1. Static audit:
     a. `rg -n 'UPDATE\s+events\b' --glob 'internal/store/sessiondb/**'
        --glob '!**/*_test.go'`
        → only one expected hit:
        `internal/store/sessiondb/session_db.go:798` inside the v2
        `strip_canonical_event_raw_payloads` migration (a documented
        exception that runs once per database).
     b. `rg -n 'DELETE\s+FROM\s+events\b'
        --glob 'internal/store/sessiondb/**'
        --glob '!**/*_test.go'`
        → must be zero hits.
     c. `rg -n 'UPDATE\s+events\b|DELETE\s+FROM\s+events\b'
        --glob '!internal/store/sessiondb/**'
        --glob '!**/*_test.go'
        cmd/ internal/`
        → must be zero hits (no peer package mutates events.db).
  2. Runtime audit: drive a real session; record events.db row count
     at T0 (before stop) and T1 (after stop). T1 must be >= T0; no
     row whose `id` existed at T0 may be missing at T1.
expected:
  - Static audits match the listed hit profile exactly.
  - Runtime: row counts only grow (or stay equal across reads); no
    row id drops between T0 and T1.
evidence:
  - `obs-12-static-audit.txt`, `obs-12-rowcount-T0.txt`,
    `obs-12-rowcount-T1.txt`, `obs-12-row-ids-diff.txt`
failure_signatures:
  - Any new `UPDATE events` outside the v2 migration → append-only
    violated; cite `internal/CLAUDE.md` "Append-only event store".
  - Any `DELETE FROM events` → append-only violated.
  - Runtime row count decreases → silent destructive write; trust
    broken.
cleanup:
  - None (read-only audit).
```

### OBS-13 — High-frequency event emission (10k events/s) without loss

```yaml qa-scenario
id: obs-13-high-rate-no-loss
title: Sustained 10k events/s does not block upstream callers; fsync batch policy honored; no event lost (after_seq tail equality)
theme: observability.throughput
coverage:
  primary:
    - obs.high-rate-no-loss
    - obs.append-only-ledger
    - obs.sequence-monotonic
  secondary:
    - obs.durable-before-broadcast
risk: high
live: false
provider: mock-acp
preconditions:
  - mock-acp configured to emit synthetic `agent_message` events at
    10,000 events/s for 10 seconds (~100,000 events total).
  - Daemon running on the same host; CPU/RSS sampled every second.
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/internal/store/sessiondb/session_db.go:117-179, 460-505, 524-552
  - /Users/pedronauck/Dev/compozy/agh/internal/observe/observer.go:421-555
steps:
  1. Start mock-acp burst.
  2. Sample daemon RSS every 1s into `obs-13-rss.csv`.
  3. After burst finishes, sample `events.db` row count via
     `SELECT count(*), MAX(sequence) FROM events`.
  4. Stream all events out via `agh session events $S -o jsonl
     > obs-13-events.jsonl`.
expected:
  - row count == 100,000 (or whatever the mock emits exactly).
  - `MAX(sequence) == row count` (strict monotonic from 1).
  - No gap in sequence — assert
    `SELECT count(*) FROM (SELECT sequence,
       sequence - row_number() OVER (ORDER BY sequence) AS gap
       FROM events) WHERE gap != 0` == 0.
  - RSS growth bounded — peak RSS < 2× steady state baseline (no
    unbounded buffer).
  - Mock-acp upstream caller never blocks > 100ms on any single
    event (channel-buffered hand-off; assert via mock's own
    submit-latency histogram).
evidence:
  - `obs-13-events.jsonl`, `obs-13-rowcount.txt`,
    `obs-13-gap-audit.txt`, `obs-13-rss.csv`,
    `obs-13-mock-submit-latency.txt`
failure_signatures:
  - Row count < emitted count → events dropped; durable append broke
    under load.
  - Sequence has gaps → writer goroutine race / non-atomic increment.
  - RSS doubles → unbounded buffer; cite the writeCh size
    `defaultWriteBufferSize = 256`.
  - Submit latency p99 > 100ms → upstream caller blocked.
cleanup:
  - Stop mock-acp; archive sinks.
```

### OBS-14 — Time / sequence monotonicity under clock skew

```yaml qa-scenario
id: obs-14-sequence-monotonic-under-skew
title: Per-stream event sequence stays monotonic even when wall-clock jumps backwards (NTP correction, container time skew)
theme: observability.persistence
coverage:
  primary:
    - obs.sequence-monotonic
  secondary:
    - obs.append-only-ledger
risk: high
live: false
provider: mock-acp
preconditions:
  - mock-acp emits 1000 events spaced 1ms apart.
  - Test harness rewrites `Observer.now` (per
    `WithNow`, `internal/observe/observer.go:184-189`) to inject a
    1-second backwards jump after event 500.
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/internal/store/sessiondb/session_db.go:524-552
  - /Users/pedronauck/Dev/compozy/agh/internal/observe/observer.go:184-189
  - /Users/pedronauck/Dev/compozy/agh/internal/transcript/transcript.go:132-144
steps:
  1. Drive the burst with the rewritten clock.
  2. Dump events ordered by `sequence`.
  3. Dump events ordered by `timestamp`.
  4. Run `transcript.Assemble` and confirm message order.
expected:
  - Sequence-ordered list is strictly increasing 1..1000.
  - Timestamp-ordered list has out-of-order entries near the jump
    (expected — wall clock went back).
  - `transcript.Assemble` honors sequence-first ordering
    (`internal/transcript/transcript.go:132-144`), so the assembled
    transcript matches the sequence order, not the timestamp order.
evidence:
  - `obs-14-sequence-order.txt`, `obs-14-timestamp-order.txt`,
    `obs-14-transcript-order.json`
failure_signatures:
  - Sequence non-monotonic → writer-goroutine race (`nextSequence` not
    incremented under exclusive ownership).
  - Transcript ordering follows timestamp → broken
    `sortedTranscriptEvents` in `transcript.go:132-144`.
cleanup:
  - Restore real clock.
```

### OBS-15 — Forensic case: stale ACP id classified as fresh-start fallback (typed event)

```yaml qa-scenario
id: obs-15-stale-acp-fresh-start-fallback
title: A resume against an ACP server that no longer recognizes the session id triggers IsLoadSessionResourceMissing classification, emits a typed fallback log/event, never propagates as a 5xx
theme: observability.forensic
coverage:
  primary:
    - obs.acp-fresh-start-fallback
    - obs.canonical-event
  secondary:
    - obs.correlation-keys
risk: high
live: true
provider: real-claude-code
preconditions:
  - One session S that was created and stopped; ACP session id
    captured.
  - Provider's session storage manually wiped (e.g. delete
    `$PROVIDER_HOME/.claude/projects/<S>/...`) so the upstream agent
    no longer knows the ACP session id.
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/internal/acp/client.go:553-567
  - /Users/pedronauck/Dev/compozy/agh/internal/session/manager_lifecycle.go:77-101
  - /Users/pedronauck/Dev/compozy/agh/internal/CLAUDE.md "Stale ACP session ids must be classified, not propagated"
steps:
  1. `agh session resume $S` (which calls `Driver.loadSession` →
     `acpsdk.AgentMethodSessionLoad`).
  2. Capture daemon log + EventStore + HTTP response body.
  3. Send a follow-up prompt; capture SSE.
expected:
  - Resume completes successfully via fresh-start fallback (a new
    ACP session id is created); session row reflects new ACP session
    id but `parent_session_id`/`root_session_id` lineage unchanged.
  - Daemon log records the classification at warn level with the key
    phrase `session.resume.load_session_missing_fallback` (per
    `manager_lifecycle.go:86-90`).
  - An event is recorded in the global ledger summarizing the
    fallback (`event_summaries.type` containing "fresh_start_fallback"
    or equivalent — verify exact name from §11).
  - HTTP response is 200 OK with the resumed session payload, NOT a
    5xx with raw "Resource not found" text.
evidence:
  - `daemon.log` excerpt showing the warn line
  - `obs-15-resume.json`, `obs-15-events-after-resume.json`,
    `obs-15-followup-sse.jsonl`
failure_signatures:
  - HTTP 5xx with raw "Resource not found" → classification not
    invoked at the resume call site.
  - Lineage parent/root pointers replaced → broken lineage
    continuity.
  - No fallback event recorded → observability gap; the operator
    cannot tell whether the fallback fired.
cleanup:
  - `agh session stop $S`.
```

### OBS-16 — Forensic case: startup-pending vs crashed are distinguished

```yaml qa-scenario
id: obs-16-startup-pending-vs-crashed
title: A session in m.pending (still starting) is NOT marked failed; an uncooperative crashed subprocess IS; events distinguish the two
theme: observability.forensic
coverage:
  primary:
    - obs.startup-pending-vs-crashed
    - obs.canonical-event
  secondary:
    - obs.correlation-keys
risk: critical
live: true
provider: real-claude-code
preconditions:
  - The harness can pause the ACP subprocess between fork and the
    initialize handshake response (gdb/lldb attach, or test-only
    `AGH_QA_PAUSE_BEFORE_INITIALIZE=1` env that the test build
    honors) so the daemon sees the session as `pending` — NOT
    crashed.
  - Independently, the harness can SIGKILL a fully-active
    subprocess.
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/internal/CLAUDE.md "Inactive metadata repair must distinguish startup-pending from crashed"
  - /Users/pedronauck/Dev/compozy/agh/internal/session/manager_lifecycle.go:104-143
  - /Users/pedronauck/Dev/compozy/agh/internal/store/types.go (StopReason taxonomy)
steps:
  1. Phase A: pause subprocess pre-initialize; query
     `agh session status $S -o json`.
  2. Resume subprocess; let session reach `active`.
  3. Phase B: SIGKILL subprocess; query status.
  4. In each phase, dump
     `event_summaries` for the session.
expected:
  - Phase A: session state == `pending` or `starting`; no `failure`
    record; no `session_stopped` event yet; no `failure.kind ==
    "process_exit"`.
  - Phase B: session state transitions to `stopped` with
    `stop_reason == agent_crashed` (per
    `internal/store/types.go` `StopAgentCrashed`); failure.kind is
    `process_exit`; one `session_stopped` event with that reason.
  - Two events distinguish the two (e.g. on a `pending` session,
    inactive-metadata repair logs a different log key/event than on
    a crashed session).
evidence:
  - `phase_a_status.json`, `phase_a_events.json`,
    `phase_b_status.json`, `phase_b_events.json`
failure_signatures:
  - Phase A reports `state == "failed"` or `stop_reason ==
    "agent_crashed"` → forensic rule violated; pending was misread
    as crashed.
  - Phase B's `failure.kind` is missing or wrong → classification
    regression.
cleanup:
  - Stop both sessions.
```

### OBS-17 — Spawn depth cap enforcement + lineage at depth N

```yaml qa-scenario
id: obs-17-spawn-depth-cap-and-lineage
title: A spawn beyond MaxDepth is rejected with a typed error; up to MaxDepth, every level's event correctly carries root_session_id and spawn_depth
theme: observability.lineage
coverage:
  primary:
    - obs.spawn-depth-cap
    - obs.correlation-keys
  secondary:
    - obs.canonical-event
risk: high
live: true
provider: real-claude-code
preconditions:
  - Coordinator config with `max_depth = 1` (the default
    `DefaultSpawnMaxDepth = 1`,
    `internal/session/spawn.go:17-18`). Note: the prompt asked for
    "depth 6" but the implementation hard-caps at 1 today; this
    scenario both exercises lineage propagation at the configured
    cap and proves the cap rejects deeper spawns. If the cap is
    raised in a later release, re-tune the depth.
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/internal/session/spawn.go:17-18, 215-237
  - /Users/pedronauck/Dev/compozy/agh/internal/store/session_lineage.go:13-83, 127-140
  - /Users/pedronauck/Dev/compozy/agh/internal/coordinator/coordinator.go:204
steps:
  1. Coordinator spawns child C1 at depth 1 (allowed).
  2. C1 attempts to spawn child C2 at depth 2 → must be rejected
     with typed error; assert `spawn.pre_create` event with deny
     outcome.
  3. Capture every event from coordinator + C1 + the rejected spawn
     attempt.
  4. For each event, parse correlation keys.
expected:
  - C1 events all carry `parent_session_id == coordinator-id`,
    `root_session_id == coordinator-id`, `spawn_depth == 1`.
  - Coordinator's events carry `parent_session_id == ""`,
    `root_session_id == coordinator-id`, `spawn_depth == 0`.
  - The rejected C2 spawn produces a typed error response and a
    `spawn.pre_create` event whose payload says depth-cap; no row
    for C2 is created in `sessions`.
  - `MaxDepth` is honored exactly — no off-by-one on the boundary
    (depth 1 is allowed, depth 2 is rejected).
evidence:
  - `coordinator_events.json`, `c1_events.json`,
    `rejected_spawn.json`, `sessions_table.json`
failure_signatures:
  - C1 events missing `root_session_id` → lineage propagation
    regression in `internal/store/session_lineage.go`.
  - C2 successfully created → spawn cap bypassed; critical safety
    violation.
  - No deny event for the rejected attempt → observability gap.
cleanup:
  - Reap children, stop coordinator.
```

### OBS-18 — Real-LLM end-to-end: 10-minute Claude Code conversation, 30 tool calls, no orphan events

```yaml qa-scenario
id: obs-18-real-llm-end-to-end
title: A 10-minute multi-turn Claude Code conversation with 30 tool calls — full transcript, full event stream, every event carries required correlation keys, no orphan events, no missing pairings
theme: observability.end_to_end
coverage:
  primary:
    - obs.canonical-event
    - obs.correlation-keys
    - obs.transcript.replay-equivalence
    - obs.append-only-ledger
  secondary:
    - obs.claim-token-redaction
    - obs.secret-redaction-logs
    - obs.diagnostics.health
risk: critical
live: true
provider: real-claude-code
preconditions:
  - Workspace seeded with a real codebase to manipulate (use the AGH
    repo itself as the test corpus, read-only).
  - Cron pre-seeded so at least one cron-driven enqueue fires
    mid-conversation (cross-link to autonomy module 04 / automation
    module 09).
  - Coordinator running so spawn lineage is exercised.
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/internal/observe/observer.go:421-555
  - /Users/pedronauck/Dev/compozy/agh/internal/transcript/transcript.go:113-130
  - /Users/pedronauck/Dev/compozy/agh/internal/session/session.go:44-45
  - /Users/pedronauck/Dev/compozy/agh/internal/api/core/session_stream.go:69-151
steps:
  1. Open SSE; start a 10-minute prompt that progressively asks the
     agent to read 30 different files, write 5 summary files, run 5
     shell commands (test, ls, find, etc.), and record 5 memory
     facts.
  2. Capture full SSE log to `obs-18-sse.jsonl`.
  3. After the turn(s) finish, dump events.db (per session) into
     `obs-18-events.jsonl`, agh.db event_summaries into
     `obs-18-summaries.jsonl`.
  4. Run `agh session transcript $S -o json > obs-18-transcript.json`.
  5. Run `agh observe health -o json > obs-18-health.json` AT three
     points (start, middle, end).
  6. Run all redaction audits (claim_token, generic secrets) over
     the combined sinks.
  7. For every tool_call event, assert the matching tool_result event
     exists by `tool_call_id`.
  8. For every assistant `agent_message` chunk, assert the parent
     turn boundary is consistent.
  9. For every event, assert the required correlation keys per
     §11.
expected:
  - Tool-call/result pairings: 30 calls → 30 results, no orphans.
  - Sequence is strictly monotonic 1..N inside each session's
    events.db.
  - Cron-fire produces a `task.run.enqueued` event whose payload
    has `actor.kind == "automation"` (per `internal/observe/tasks.go:2296`)
    and that flow into a coordinator-driven `task.run.claimed` →
    real subagent prompt; every step is captured.
  - All redaction audits return zero hits.
  - `obs-18-health.json` at all three points has `status == "ok"`.
  - Transcript replay is byte-equivalent to live SSE (per OBS-10
    rules).
evidence:
  - `obs-18-sse.jsonl`, `obs-18-events.jsonl`,
    `obs-18-summaries.jsonl`, `obs-18-transcript.json`,
    `obs-18-health.json`, `obs-18-redaction-audit.txt`,
    `obs-18-tool-pairings.json`
failure_signatures:
  - Any orphan tool_call without a matching tool_result →
    transcript merge regression; cite `transcript.go:239-318`.
  - Any sequence gap → writer-goroutine race.
  - Any redaction hit → security violation; release blocker.
  - Health flips to degraded mid-run on a healthy fixture →
    spurious failure record.
cleanup:
  - `agh session stop $S`. Archive everything.
```

## 8. Optional / nice-to-have scenarios (run if time)

### OBS-19 — Retention sweep deletes old summaries without touching per-session events.db

```yaml qa-scenario
id: obs-19-retention-sweep-scope
title: Observer retention sweep deletes event_summaries / token_stats / permission_log rows older than retention_days, but per-session events.db rows are NOT touched
theme: observability.retention
coverage:
  primary:
    - obs.retention.sweep
    - obs.append-only-ledger
risk: medium
live: true
provider: real-claude-code
preconditions:
  - retention_days set to 1; sweep_interval shortened to 30s for the
    scenario; `Observer.now` rewritten 2 days into the future after
    seeding events.
code_refs:
  - /Users/pedronauck/Dev/compozy/agh/internal/observe/retention.go:14-100
  - /Users/pedronauck/Dev/compozy/agh/internal/store/globaldb/global_db_observe.go:113-189
steps:
  1. Seed 100 events; record per-session events.db row count B0
     and event_summaries count B1.
  2. Advance simulated `now` by 2 days; wait for one sweep cycle;
     record A0 (events.db) and A1 (event_summaries).
expected:
  - A1 < B1 (rows deleted by sweep).
  - A0 == B0 (per-session events.db untouched).
  - `health.retention.last_sweep_status == "ok"` and
    `last_cutoff_at > 0`.
evidence: `obs-19-counts.txt`, `obs-19-health.json`
failure_signatures:
  - A0 < B0 → sweep deletes per-session ledger; append-only violated.
  - A1 == B1 → sweep didn't fire.
cleanup: restore real clock.
```

### OBS-20 — `RegisterDynamicSecret` lifecycle (register → emit → unregister)

```yaml qa-scenario
id: obs-20-dynamic-secret-lifecycle
title: A runtime-registered secret is scrubbed while registered; after the unregister callback, new logs may include the now-rotated token without affecting earlier scrubbed lines
theme: observability.security
coverage:
  primary:
    - obs.secret-redaction-logs
risk: medium
live: false
provider: static
preconditions:
  - Test harness can call `diagnostics.RegisterDynamicSecret`
    (`internal/diagnostics/redact.go:35-57`) and observe the logger.
steps:
  1. Register secret `dyn-rotation-OBS20-A`; emit a log line with it;
     assert it is scrubbed.
  2. Call the unregister callback.
  3. Re-emit a log line with the same string; assert it is NOT
     scrubbed (to prove the registry's reference-counting is correct
     and lifecycle-bounded).
  4. Re-register the same string with two callers; release one;
     assert it is still scrubbed; release the second; assert it is
     no longer scrubbed.
expected: behavior matches the reference-counted registry.
evidence: `obs-20-log-trace.txt`
failure_signatures:
  - Reference counter not honored → secrets leak across rotations.
cleanup: drain the registry.
```

## 9. Coverage matrix (this child)

| Coverage ID                         | Scenarios                                      |
| ----------------------------------- | ---------------------------------------------- |
| `obs.canonical-event`               | OBS-01, OBS-02, OBS-08, OBS-10, OBS-15, OBS-17, OBS-18 |
| `obs.append-only-ledger`            | OBS-01, OBS-03, OBS-05, OBS-12, OBS-13, OBS-18, OBS-19 |
| `obs.durable-before-broadcast`      | OBS-03, OBS-13                                 |
| `obs.sequence-monotonic`            | OBS-03, OBS-04, OBS-13, OBS-14                 |
| `obs.replay.after-seq`              | OBS-04, OBS-11                                 |
| `obs.replay.cross-restart`          | OBS-03, OBS-05                                 |
| `obs.transcript.replay-equivalence` | OBS-02, OBS-10, OBS-18                         |
| `obs.correlation-keys`              | OBS-02, OBS-06, OBS-15, OBS-17, OBS-18         |
| `obs.claim-token-redaction`         | OBS-06, OBS-18                                 |
| `obs.secret-redaction-logs`         | OBS-07, OBS-18, OBS-20                         |
| `obs.diagnostics.health`            | OBS-08, OBS-18                                 |
| `obs.logging.structured-only`       | OBS-09                                         |
| `obs.errors.no-strings-contains`    | OBS-09                                         |
| `obs.query-engine`                  | OBS-11                                         |
| `obs.acp-fresh-start-fallback`      | OBS-15                                         |
| `obs.startup-pending-vs-crashed`    | OBS-16                                         |
| `obs.spawn-depth-cap`               | OBS-17                                         |
| `obs.high-rate-no-loss`             | OBS-13                                         |
| `obs.retention.sweep`               | OBS-19                                         |

Total: 18 mandatory + 2 optional = 20 scenarios. Every coverage ID is
exercised by at least one scenario; the security-critical IDs
(`obs.append-only-ledger`, `obs.claim-token-redaction`,
`obs.correlation-keys`) are exercised by ≥ 2.

## 10. Forbidden-needle list and audit exemptions

### Forbidden needles (must NOT appear in any sink across any scenario)

- Any literal raw `agh_claim_<>=12 random char>` (regex
  `agh_claim_[A-Za-z0-9_-]{12,}`).
- Any provider API key shape: `sk-`, `xoxb-`, `AKIA`, `ya29.`,
  `Bearer\s+\w+` other than the deliberate `[REDACTED]` placeholder.
- Pre-seeded fake secrets used by OBS-07: `sk-fake-QA-OBS07-…`,
  `fakeBearerQA0BS07-…`, `fake-quoted-OBS07`,
  `dynamic-runtime-secret-OBS07`.
- Pre-seeded redaction lifecycle test secrets used by OBS-20:
  `dyn-rotation-OBS20-A`.

A single hit across any scenario is shippability-critical.

### Logging / error-string exemptions (referenced from OBS-09)

These are the only allowed appearances of `fmt.Println` / `log.Print*` /
`strings.Contains(err.Error(), …)` in production code under `cmd/` and
`internal/`:

| Path:line                                                      | Pattern                                          | Justification                                                                                                                          |
| -------------------------------------------------------------- | ------------------------------------------------ | -------------------------------------------------------------------------------------------------------------------------------------- |
| `internal/sandbox/daytona/cmd/agh-daytona-sidecar/main.go:459` | `log.Printf("%s: %v", logMessage, err)`          | Sidecar bootstrap logging before the slog handler is wired; sidecar is a separate process binary, not the AGH daemon.                  |
| `internal/sandbox/daytona/cmd/agh-daytona-sidecar/main.go:491` | `log.Printf("write JSON response: %v", err)`     | Same — sidecar boundary.                                                                                                               |
| `internal/extension/registry.go:539`                           | `strings.Contains(strings.ToLower(err.Error()), "no such table")` | sqlite-specific error string with no exported sentinel; fallback path on first-boot when the table is absent before migrations apply. |
| `internal/subprocess/transport.go:253`                         | `strings.Contains(err.Error(), "token too long")` | bufio.Scanner returns this string only; no exported sentinel exists upstream.                                                          |
| `internal/subprocess/process.go:638`                           | `strings.Contains(err.Error(), "file already closed")` | Wraps a `*os.PathError` whose underlying message is the only signal; documented in stdlib.                                             |
| `internal/store/globaldb/global_db_task.go:155`                | `strings.Contains(strings.ToLower(err.Error()), "foreign key constraint failed")` | sqlite returns a string-only error for FK failures; mattn/go-sqlite3 has no exported sentinel.                                         |
| `internal/store/globaldb/global_db_bridge.go:1128`             | Same pattern                                     | Same sqlite reason.                                                                                                                    |
| `internal/store/globaldb/global_db_bundles.go:45`              | `strings.Contains(strings.ToLower(err.Error()), "no such table")` | First-boot bootstrap; bundles table may not exist yet if upgrading from an older version.                                              |

OBS-09 audits these exact paths; new entries require a written
justification appended to this table in the same PR that introduces
them.

## 11. Canonical Event Coverage Matrix

This is the authoritative `<lifecycle path → event(s) → required
correlation keys → emitting package → test file>` table. Any row missing
a test file or missing an event emission is flagged at the bottom of the
table. The table is built from `internal/CLAUDE.md` Observability bullets
(`internal/CLAUDE.md:48-52`) plus actual code grep against
`internal/task/manager.go:20-42`, `internal/hooks/events.go:54-130`,
`internal/session/session.go:44-45`, `internal/automation/dispatch.go`,
`internal/network/manager.go`, and `internal/coordinator/coordinator.go`.

Legend — required keys are abbreviated:
- `S` = `session_id`, `P` = `parent_session_id`, `R` = `root_session_id`,
- `A` = `agent_name`, `T` = `task_id`, `RID` = `run_id`,
- `CTH` = `claim_token_hash`, `LU` = `lease_until`, `WF` = `workflow_id`,
- `CSI` = `coordinator_session_id`, `SR` = `scheduler_reason`,
- `HE` = `hook_event`, `HN` = `hook_name`, `SD` = `spawn_depth`,
- `AK` = `actor_kind`, `AID` = `actor_id`, `RR` = `release_reason`.

| Lifecycle path                              | Canonical event type(s)                                                                                          | Required correlation keys           | Emitting package(s)                                                                                                                                              | Test file(s)                                                                                                                                                            |
| ------------------------------------------- | ---------------------------------------------------------------------------------------------------------------- | ----------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Session start (success)                     | `session_started` (impl-specific name; verify with §12 audit), token usage row + first event in events.db        | S, A, P, R, SD                       | `internal/session/manager_start.go`                                                                                                                              | `internal/session/manager_test.go`, child 03 ACP-01 — flagged: scenario for `session_started` correlation keys not yet in this child.                                  |
| Session prompt submitted                    | `user_message` (per `internal/acp/types.go:24-25`) + per-session events.db append                                | S, A, R                              | `internal/acp/`, `internal/session/manager_prompt.go`                                                                                                            | `internal/transcript/transcript_test.go`, child 03 ACP-01                                                                                                              |
| Tool dispatch invoked                       | `tool_call` (`internal/acp/types.go:32-33`)                                                                      | S, A, R, tool_call_id                | `internal/acp/`, `internal/session/manager_prompt.go`                                                                                                            | `internal/transcript/transcript_test.go`, child 03 ACP-01                                                                                                              |
| Tool dispatch completed                     | `tool_result` (`internal/acp/types.go:34-35`)                                                                    | S, A, R, tool_call_id                | Same                                                                                                                                                              | Same                                                                                                                                                                    |
| Hook deny                                   | `hook.dispatch.blocked` (`internal/hooks/dispatch.go:1210`); per-session `hook_runs` row with outcome=denied      | S, A, HE, HN, AK, AID                | `internal/hooks/`                                                                                                                                                | `internal/hooks/dispatch_test.go`, child 04 AUT-03                                                                                                                     |
| Hook completed                              | `hook.dispatch.completed` (`internal/hooks/dispatch.go:1229`)                                                    | S, A, HE, HN                          | Same                                                                                                                                                              | Same                                                                                                                                                                    |
| Hook async failure                          | `hook.dispatch.async_failed` (`internal/hooks/dispatch_async.go:106`)                                            | S, A, HE, HN                          | `internal/hooks/dispatch_async.go`                                                                                                                                | `internal/hooks/pool_test.go`                                                                                                                                           |
| Memory write                                | per-session events.db append + `memory.*` log lines from `internal/memory` (event name verified in §12 audit)    | S, A, R                              | `internal/memory/`                                                                                                                                                | `internal/memory/*_test.go` — flagged: explicit canonical-event coverage for memory writes is not currently in this child's scenarios; pulled into OBS-18.             |
| Session end / stop                          | `session_stopped` (`internal/session/session.go:44-45`)                                                          | S, A, R, stop_reason                  | `internal/session/manager_lifecycle.go`                                                                                                                          | `internal/session/manager_test.go`, child 03 ACP-06                                                                                                                     |
| Task created                                | `task.created` (`internal/task/manager.go:20`)                                                                   | T, AK, AID, WF                        | `internal/task/manager.go`                                                                                                                                       | `internal/task/manager_test.go`                                                                                                                                          |
| Task run enqueued                           | `task.run_enqueued` (`internal/task/manager.go:29`)                                                              | T, RID, AK, AID, WF                   | Same                                                                                                                                                              | Same                                                                                                                                                                    |
| Task run pre-claim                          | `task.run.pre_claim` hook (`internal/hooks/events.go:117`)                                                       | T, RID, AK, AID, WF                   | `internal/hooks/`                                                                                                                                                | `internal/hooks/dispatch_test.go`, child 04 AUT-06                                                                                                                     |
| Task run claimed                            | `task.run_claimed` (`internal/task/manager.go:30`)                                                               | T, RID, S, A, CTH, LU, AK, AID, WF    | `internal/task/lease_manager.go:14`                                                                                                                              | `internal/task/lease_test.go`, child 04 AUT-01                                                                                                                          |
| Task run lease extended                     | `task.run_lease_extended` (`internal/task/manager.go:40`)                                                        | T, RID, S, CTH, LU                    | Same                                                                                                                                                              | Same                                                                                                                                                                    |
| Task run lease expired (sweep)              | `task.run_lease_expired` (`internal/task/manager.go:41`)                                                         | T, RID, S, previous_CTH, LU           | `internal/scheduler/scheduler.go:262`                                                                                                                            | `internal/scheduler/scheduler_test.go`, child 04 AUT-02                                                                                                                |
| Task run lease recovered                    | `task.run_recovered` (`internal/task/manager.go:38`)                                                             | T, RID, S, previous_CTH               | Same                                                                                                                                                              | Same                                                                                                                                                                    |
| Task run released                           | `task.run_released` (`internal/task/manager.go:42`)                                                              | T, RID, S, RR, CTH                    | `internal/task/lease_manager.go:118-163`                                                                                                                         | Same                                                                                                                                                                    |
| Task run completed                          | `task.run_completed` (`internal/task/manager.go:34`)                                                             | T, RID, S, CTH                        | `internal/task/lease_manager.go:208`                                                                                                                             | Same                                                                                                                                                                    |
| Task run failed                             | `task.run_failed` (`internal/task/manager.go:35`)                                                                | T, RID, S, CTH, error                 | `internal/task/lease_manager.go:243`                                                                                                                             | Same                                                                                                                                                                    |
| Task canceled                               | `task.canceled` (`internal/task/manager.go:25`)                                                                  | T, RID (if a run was active)         | `internal/task/manager.go:967`                                                                                                                                   | Same                                                                                                                                                                    |
| Spawn pre-create                            | `spawn.pre_create` (`internal/hooks/events.go:126`)                                                              | P, R, SD                              | `internal/hooks/`                                                                                                                                                 | child 04 AUT-11                                                                                                                                                         |
| Spawn created                               | `spawn.created` (`internal/hooks/events.go:127`)                                                                 | S, P, R, SD                           | `internal/session/spawn.go:215-237`                                                                                                                              | `internal/session/manager_lineage_test.go`                                                                                                                              |
| Spawn parent stopped                        | `spawn.parent_stopped` (`internal/hooks/events.go:128`)                                                          | P, R                                  | `internal/session/spawn.go`                                                                                                                                       | Same                                                                                                                                                                    |
| Spawn TTL expired                           | `spawn.ttl_expired` (`internal/hooks/events.go:129`)                                                             | S, P, R                               | `internal/coordinator/`, `internal/session/`                                                                                                                      | `internal/coordinator/coordinator_test.go`                                                                                                                              |
| Spawn reaped                                | `spawn.reaped` (`internal/hooks/events.go:130`)                                                                  | S, P, R                               | Same                                                                                                                                                              | Same                                                                                                                                                                    |
| Automation cron fired                       | `automation.dispatch.delegated` log + `task.run_enqueued` event with origin=automation (`internal/observe/tasks.go:2296`) | T, RID, AK=automation, AID            | `internal/automation/dispatch.go:716`                                                                                                                            | `internal/automation/dispatch_test.go`, child 09 (automation QA)                                                                                                        |
| Automation run completed / failed           | `automation.dispatch.completed` / `automation.dispatch.failed` (`dispatch.go:746, 773`)                          | RID                                   | Same                                                                                                                                                              | Same                                                                                                                                                                    |
| Network peer joined / left                  | `network.peer.joined` / `network.peer.left` (`internal/network/manager.go:358, 552`)                             | session_id (peer)                     | `internal/network/manager.go`                                                                                                                                    | `internal/network/manager_test.go`, child 06                                                                                                                           |
| Network message sent / received / rejected  | `network.message.sent` / `received` / `rejected` (`manager.go:1130, 1152, 1176`)                                 | session_id, channel_id                | Same                                                                                                                                                              | Same                                                                                                                                                                    |
| Health status changed                       | `health.status_changed` (verify exact name in §12 audit; emitted on transition)                                  | status                                | `internal/observe/health.go`                                                                                                                                     | `internal/observe/observer_test.go`, OBS-08 — flagged: confirm the exact event type name with the SUT branch's audit; the documented behavior is "transitions emit a state-change event". |
| ACP fresh-start fallback                    | `session.resume.load_session_missing_fallback` log line + `acp.session.fresh_start_fallback` event (verify name) | S, A, previous_acp_session_id         | `internal/session/manager_lifecycle.go:77-101`                                                                                                                   | `internal/acp/client_test.go:857-858`, `internal/session/manager_test.go:585, 645`, OBS-15                                                                              |
| Bridge auth failure                         | `bridge.auth.failed` (verify name in §12 audit)                                                                  | bridge_instance_id                    | `internal/observe/bridges.go:92-`                                                                                                                                 | `internal/observe/bridges_test.go`                                                                                                                                       |

### Flagged rows (audit work for OBS-01)

The following rows in the matrix above are flagged for follow-up because
either the canonical event name is not 100% pinned in this codebase scan
or a dedicated emission test is not yet captured in this child:

1. **Memory write** — confirm canonical event name (`memory.write.recorded`
   vs `memory.fact_added` vs `memory.consolidation.applied`); pulled into
   OBS-18 as a real-LLM scenario but `memory_write_event_name_audit.json`
   from OBS-01 must record the actual emitted name and update the matrix
   in the same commit.
2. **Health status changed** — confirm whether the event is emitted as a
   row in `event_summaries` or only as a log line; OBS-08 captures the
   actual behavior and the matrix must be updated to match.
3. **ACP fresh-start fallback** — confirm whether the canonical event
   type is `acp.session.fresh_start_fallback` or another name; OBS-15
   captures the actual behavior and the matrix must be updated to match.
4. **Bridge auth failure** — confirm the canonical event type emitted by
   `RecordBridgeAuthFailure` (it currently increments a counter; whether
   that flushes a per-event row is a §12 audit question).

OBS-01's deliverable `coverage_matrix.json` is what closes those flags
for the SUT branch. Any row whose `observed: false` is a release blocker.

---

End of child 15. Reporting contract: each scenario writes the
four-artifact set required by the openclaw operator-flow pattern
(markdown report + JSON summary + observed events + combined log). The
aggregate `obs-summary.json` for this child carries the coverage matrix
from §9 alongside per-scenario `outcome ∈ {worked, failed, blocked,
follow-up}` and machine-readable timing. A child run is shippable only
when every mandatory scenario is `worked` or has an explicit accepted
follow-up; OBS-06 (claim-token redaction), OBS-07 (secret redaction),
OBS-09 (logging discipline), OBS-12 (append-only invariant), and the
forbidden-needle list are non-negotiable.
