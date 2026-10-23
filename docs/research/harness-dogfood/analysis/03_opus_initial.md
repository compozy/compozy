# Harness dogfood — independent initial analysis (Opus)

- Analyst: Claude Opus 5.5 (`claude-opus-5-5`), direct mode. The packet requested max effort; I cannot observe the launcher's effort setting myself.
- Baseline: `74f33b7d460866b8061544e7eb98b73dbe972901`. Code citations are `path:line` at baseline `HEAD`. None of the files cited for F1, F2, F4, F7 or F8 changed between `v0.3.0-beta.37` and `HEAD`, so the beta.37 and beta.38 incidents below ran the cited code.
- While I worked, the controller had uncommitted edits in this worktree (access-log levels, hook dispatch, scheduler and Loop reconciler logging, plus native-tool and session files). I used them only to avoid duplicating recommendations (F5), never as evidence.
- Data access was read-only. SQLite was opened as `file:…?mode=ro`, and no daemon, config, session or Git state was mutated. Helper scripts lived outside the repo, in `/tmp/harness-opus-scripts/`. Excerpts are sanitized to IDs, timestamps and system messages.

## 1. Inventory and sampling

| Source | Coverage | Notes |
| --- | --- | --- |
| `~/.compozy/compozy.db` | 223 sessions: 82 archived, 24 active at scan time. Created 2026-10-19T23:06Z to 2026-10-23T02:33Z | The home was freshly provisioned on 2026-10-19: `daemon-stdio.log` opens with `store.migrations.applied … applied_count=141`. No older history exists. |
| `sessions/*/events.db` | 223 of 223 ledgers read, 137,107 events, 747 MB of event content | Archived sessions were included in every aggregate scan. Deleted sessions cannot be recovered: `sess-59e7e5de132a1ca5` is still referenced by crash bundles and reply watches, but it has no row and no directory. |
| `logs/compozy*.log` | 243,770 lines, 61 MB, covering only 2026-10-22 09:48 to 23:44 (-03:00) | Retention is 10 MiB × 5 rotated files (see F5). |
| `daemon-stdio.log`, `desktop-bootstrap.jsonl`, `update-history.jsonl`, 17 crash bundles, 3 restart records | 2026-10-19 to now | Used to date the runtime versions. |
| Git | Shallow clone with 780 reachable commits | I read every October subject and walked v0.3.0-beta.35 → beta.38 → `HEAD`. |

**Session mix.**
- By type: 70 user (47 with a provenance parent, 3 continue, 1 fork), 20 system, and 133 spawned (84 subagent, 45 auto-title, 4 worker).
- By provider: codex 122, claude 100, opencode 1.
- Orchestration tables: 129 `session_subagents` rows (91 provider-native, 38 delegated), 34 subagent wakes, 111 reply watches, 309 input-queue rows and 526 `prompt_delivery` events.

**Runtime versions per window.**
- beta.36 ran from 2026-10-20T17:50Z.
- Desktop attach errors at 2026-10-21 00:52Z and 16:28–16:29Z report a running runtime versioned `0.3.0`, which is not a release build.
- The bundled beta.37 started at 16:36Z, and beta.38 at 2026-10-22T22:03:15Z.
- I don't attribute incidents from 2026-10-21 before 16:36Z to a release build.

**Method.** Full scans for counts: event types and bytes, tool outcomes, prompt spans, auto-title runtime and wake/reply states. Targeted timelines for each incident cluster. I read the current code behind every causal claim.

## 2. Current findings, ranked

### F1 (P1) — Provider rate limits turn orchestration wakes into instant retries and lose inbound results

The causal chain is verified on two days, through two separate mechanisms.

**Subagent wake, 2026-10-22, beta.37.**
- Parent: `sess-e82794a5c54126f5` (Claude Opus). Delegated child: `sub-2654490b359767cf` / `sess-bc989cfad0b3727d`.
- Wake `wake_01M5JBXVP5…` opened at 17:53:16Z. Its turn was running tool calls when the provider answered at 17:58:33Z: "You've hit your weekly limit · resets 8pm (America/Sao_Paulo)". This is event seq 1311, with `provider_failure_kind=rate_limited; next_action=handoff`.
- The successor wakes had no backoff:
  - `wake_01M5JC7HCK…` (attempts 2) opened at 17:58:34Z and failed at 17:58:42Z (seq 1316).
  - `wake_01M5JC7SW5…` (attempts 3) opened at 17:58:42Z and failed at 17:58:49Z (seq 1321).
- Outcome:
  - All three wakes ended `canceled`, and the child row ended `delivery=disposed`.
  - The log recorded `WARN subagent.wake_abandoned attempts=3`, and three crash bundles were written.
- A human "continue" at 18:00:57Z, on the same provider and model, succeeded with no further errors. The limit cleared within about 2 minutes, while all three automatic attempts fell inside 16 seconds.

**Reply watch, 2026-10-21, beta.37.**
- Between 23:11:40Z and 23:19:35Z the Claude account's session limit hit five sessions: `sess-c1bb1a80395cefb6`, `sess-ae1f4187b3c68cca`, `sess-df1435e6620874cf`, the coordinator `sess-8a00287790c82242`, and the since-deleted `sess-59e7e5de132a1ca5`.
- A completed reviewer reply (`rw-1a4ad26715b8417d`) was queued as `inq-c74b908ea67714ec` at 23:19:28Z. A coordinator turn consumed it and failed at 23:19:35Z. The queue row stays `sent` with no terminal outcome, the watch stays `delivered`, and nothing redelivered it.
- The limit reset at 00:50Z, but nothing resumed until a human "continue" at 01:24:17Z, 2 h 05 min later. The coordinator then recovered by re-reading the reviewer's on-disk report.
- Three more reply watches recorded `outcome=failed` from the same limit: `rw-3559688611727b27`, `rw-ba7b0fa413bfa248` and `rw-a481c9e867c54724`.

Overall, 13 of the 17 crash bundles are rate-limit prompt failures (the others: 1 credentials, 1 handshake, 2 other).

**Code (baseline).**
- `internal/session/subagent_manager.go:64` — `drainSubagentInputEvents` returns only `(canceled, failed bool)`, so the provider failure class is discarded.
- `internal/session/manager_input_dispatch.go:224` — any failed wake turn calls `OnWakeFailed`.
- `internal/session/subagent_wake.go:200` — `failWake` retries with a `200 ms × attempts` delay (constant at line 227) and abandons at 3 attempts (line 209).
- `internal/store/globaldb/global_db_session_subagents_wakes.go:280` — at 3 attempts or more, delivery becomes `disposed`.
- `internal/store/globaldb/sqlcgen/session_subagents.sql.go:510` — `InheritSubagentWakeAttempts` carries the attempt count into each successor wake, so successors consume the same budget.
- `internal/session/reply_watch_delivery.go:58` — the watch becomes `delivered` as soon as its synthetic input row is inserted. A later turn failure has no redelivery branch.
- The classification already exists in `internal/acp/provider_failure.go:24` (`rate_limited`) and `:45` (`handoff`), but neither path consults it.

**Cause.** Wake and reply retries ignore the failure type and are compressed into seconds. Provider capacity refusals are handled like broken turns, and one failed synthetic turn consumes the reply for good. The `handoff` guidance appended to synthetic-turn errors (`compozy session continue … --agent`) has no automated consumer.

**Minimal fix.**
1. Return the terminal failure kind from `drainSubagentInputEvents`.
2. For `rate_limited`, and possibly auth and transient failures, neither increment attempts nor dispose. Keep the rows `pending` and retry after a capped backoff measured in minutes, or on the next accepted parent turn or human input.
3. When a synthetic reply turn fails on provider capacity, requeue the same input or reopen the watch instead of leaving it `sent`.
4. Stop writing crash bundles for classified `rate_limited` prompt failures; keep the error event.

**Owning invariant.** A settled child result or reply is never disposed or consumed because the provider refused capacity; it is redelivered after backoff.

**Suites.**
- `internal/session/subagent_service_test.go` — wake retry and disposal
- `internal/store/globaldb/global_db_session_subagents_test.go` — the `FailWake` threshold
- `internal/session/reply_watch_test.go` — delivery
- The classification is already covered by `internal/acp/failure_test.go`.

### F2 (P1/P2) — Codex idle-runtime release exceeds its 5 s cleanup deadline and is force-killed

**Evidence.**
- Within the 14-hour log window there were 21 `session.runtime.idle_released` successes and 9 failures logged as `ERROR session supervision failed … stop idle runtime … context deadline exceeded`.
- All nine failures are Codex `gpt-6-astra` subagents: `sess-98e6d780d0c0d8b3`, `sess-ec723009c40deb11`, `sess-65bccb156a087456`, `sess-9e6e6da983b2b5fb`, `sess-b63e75b942f06ac8`, `sess-1e7700116d57de3c`, `sess-bf129e97d669ce79`, `sess-e914b48c909850a1` and `sess-a5f772874b5b2790`.
- The last two failed at 02:37–02:38Z on 2026-10-23, so the failure reproduces on beta.38.
- Among released sessions still in the DB, Codex had 9 successes and 9 failures; Claude had 6 successes and 0 failures.

**Code.**
- `internal/session/manager_idle_release.go:137` stops the process through `stopReplacedRuntime`.
- That uses `m.lifecycleCleanupContext()` (`internal/session/manager_prompt_runtime.go:285`), a 5 s timeout: `defaultLifecycleTimeout`, defined in `internal/session/manager.go:20` and applied in `internal/session/manager_context.go:5`.
- `acp.Driver.Stop` budgets 5 s for stop plus 3 s for drain (`internal/acp/client_control.go:147`, constants in `internal/acp/client.go:23-24`). If the caller's context expires first, it force-kills the process.

**Cause.** The caller's deadline is shorter than the callee's stop budget. A Codex cooperative shutdown often takes more than 5 s, so the release turns into a forced kill and is recorded as failed (`release.finish(err)`). The next rebind then has to stop the process again (`waitRuntimeRelease`).

**Minimal fix.**
- Give `stopReplacedRuntime` callers a deadline derived from the driver budget (stop + drain + margin), or let the driver own its deadline under `context.WithoutCancel`.
- Log each release outcome with provider and elapsed time.
- Log rebind and `session/load` outcomes; today the only trace is `acp.start.stage session_load`, which appears 6 times.

**Invariant.** An idle release of a healthy agent completes cooperatively within the driver's stop budget.

**Suites.** The nearest existing suite is `internal/session/manager_stop_integration_test.go`; I found no suite named for idle release.

**Unverified.** Whether a forced kill ever lost Codex thread state on a later `session/load`. No rebind logs exist to check.

### F3 (P2) — Native tool input rejections carry no actionable detail

This is current on beta.38.

**Evidence.**
- `compozy__monitor_create` (Codex) failed 11 of 102 calls with `schema_invalid: invalid tool request`, including after the beta.38 boundary; the latest was at 2026-10-23T02:52Z.
- The structured error is only `{"code":"tool_invalid_input","message":"invalid tool request","reason_codes":["schema_invalid"],"tool_id":"compozy__monitor_create"}` (sample from `sess-0f9708577ea53a95`).
- The same opaque class appears for `compozy__session_prompt` (3, Claude), `compozy__skill_view` (4, Codex), `compozy__terminal_wait` (1) and `compozy__terminal_signal` (1).
- With no field path, the agent can only retry by guesswork.

**Fix.** Return the failing JSON pointer, the expected type or enum, and the allowed fields, bounded and redacted. This matches the controller's "safe schema diagnostics".

**Suites.** `internal/tools/registry_test.go`, `internal/tools/mcp_test.go`.

### F4 (P2, cost and performance) — Auto-title runs a full default-model agent session for every title

**Evidence.**
- There were 45 `spawn_role=auto-title` sessions in about 3.5 days, 20% of all sessions. All ran `claude-opus-5-5`.
- From prompt to `done`: median (p50) 16.7 s, max 44.8 s. Median lifetime: 21 s.
- Per title (median): 40,745 cache-write plus 10,483 cache-read tokens to produce about 31 output tokens.
- The provider-reported cost totals $16.57, about $0.37 per title.
- Compozy's own prompt is only about 3.4 KB (the `system_prompt` span), so most of the cost is the provider CLI's own system and tool preamble.

**Code.**
- `internal/config/roles.go:161` sets `AutoTitle: RoleConfig{Enabled: true}` with no provider or model.
- `internal/daemon/auto_title_generator.go:91` makes the role inherit the requester's agent.
- At line 102 it spawns a full session and a synthetic turn for each title.

**Fix.** When the role inherits, pick a small or fast model for that provider from the catalog, or use a title path that doesn't need a tool-enabled agent. Show the effective route in Settings.

**Compatibility.** This changes the default behaviour of a public config key without changing its shape (SD-013 public surface), so it needs a release note.

**Suites.** `internal/daemon/auto_title_role_integration_test.go`, `internal/daemon/auto_title_runtime_test.go`, and the config role-default tests.

### F5 (P2, observability) — Logs keep about 14 hours, crowded out by access lines and repeated warnings

Part of this is already in progress in the controller's worktree.

**Evidence.**
- 61 MB of logs covers only 2026-10-22 09:48 to 23:44 (-03:00), at the default 10 MiB × 5 rotation (`internal/config/defaults.go:108`). No structured logs survive for the 2026-10-21 incidents.
- 83.0% of the bytes are `httpapi: request` lines; 79.7% of the bytes are successful GETs, which baseline logs at INFO (`internal/api/httpapi/middleware.go:49`).
- `hook.dispatch.*` lines are another 13.0%.

The same window has deterministic WARN and ERROR repetition:

| Message | Count | Unique causes |
| --- | --- | --- |
| `skills: verification warning` | 465 | 4 (skill, pattern) pairs |
| `skills: overriding skill` | 438 | 10 path pairs |
| `daemon.model_catalog.refresh_failed` | 348 | 6 providers × 58. Every one is a configuration failure: missing executable, CLI login, missing secret (3 providers), no discovery path. Each is retried about every 14.5 min (`internal/daemon/model_catalog.go:322`; sweep at `internal/daemon/model_catalog_refresh_loop.go:15`). |
| `session.spawn_wake.suppressed` | 246 | Expected: every subagent and auto-title session has `notify_creator=0`. |
| `workspace.resolve.error` / `cmd palette request failed` (ERROR) | 129 / 24 | Client `context canceled`. |

**Traceability gaps.**
- The UDS API has no request log at all. Only the HTTP server registers `requestLoggingMiddleware` (`internal/api/httpapi/server_setup.go:124`).
- 101 of 110 HTTP `POST …/prompt` requests are `client_id=unidentified`, because the Go CLI never sets `X-Compozy-Client-ID`.
- For those requests, access-log latency is the whole turn (median 10 s, max 724 s), so admission latency can't be observed.

**Already in the controller's tree.** Demotion of GET/HEAD/OPTIONS access logs, hook dispatch, scheduler sweeps and Loop reconciler cycles.

**Additional small fixes.**
- Log each skill verification or override once per (path, content digest).
- Classify deterministic catalog failures, log only on state change, and back them off further.
- Demote the expected `spawn_wake.suppressed`.
- Log client cancellations at DEBUG.
- Optionally, add a low-volume UDS log for mutations that records the calling session.

**Suites.** `internal/api/httpapi/middleware_refac_test.go`, `internal/skills/registry_diagnostics_test.go`, `internal/daemon/model_catalog_test.go`, `internal/session/spawn_wake_test.go`.

### F6 (P2, storage) — Inline image payloads are stored twice

The storage impact is verified; the performance impact is not.

**Evidence.**
- `tool_result` events hold 611 MB of the 747 MB of event content; 439 MB of that is inline base64 of 4 KB or more.
- The transcript projection (`transcript_entries`) holds another 692 MB, 518 MB of it containing base64.
- 35 sessions are affected.
- The worst is an archived design session, `sess-271c8af963f77be9`, whose `events.db` is 660 MB. It contains 1,288 Claude `Read` results of images: 292 MB in the ledger, plus about 321 MB copied into the projection.
- Separately, `available_commands_update` events average 63 KB each (17.8 MB over 281 events).

**Fix.**
- Move binary and image content blocks above a size threshold into the existing content-addressed tool artifacts (`~/.compozy/tool-artifacts`, read via `compozy__tool_artifact_read`), and store references in both the ledger and the projection.
- At minimum, stop copying the blobs into `transcript_entries`.
- Existing ledgers need an SD-013 plan: the projection can be rebuilt, but the ledger is authoritative.

**Suites.** `internal/transcript/transcript_ui_projection_test.go`, `internal/transcript/transcript_codec_contract_test.go`.

### F7 (P3, measured risk) — The skills catalog always fills its 16K budget, so every description is cut

**Evidence.**
- In all 162 observed first turns the `skills` span sits at the cap (median 15,984 B, max 16,384 B).
- The whole first-turn prefix is 32–36 KB:
  - runtime identity 1.7 KB
  - situation 2.7 KB
  - tools manual 11.7 KB
  - skills 16 KB

**Code.**
- The budget is 16,000 runes (`internal/daemon/prompt_sections.go:32`), applied at `internal/daemon/prompt_skills.go:139`.
- `internal/skills/catalog.go:211` binary-searches a single description length shared by all skills. If no length fits, it returns an empty catalog (`bounded := ""`, line 242).

**Estimate.** This repo exposes about 93 workspace, 17 user and 13 extension skills plus the bundled one. At that size the per-description limit is roughly 75 runes, which cuts most "Use when / Do not use" clauses.

**Not established.** I found no case in the ledgers where an agent actually picked the wrong skill because of this. It relates to the controller's scoped inline skill routing.

**Suites.** `internal/skills/catalog_test.go`, `internal/daemon/prompt_skills_test.go`.

### F8 (P3) — Normal idle stops are recorded as escalations, and the stop cause is serialized as an integer

**Evidence.**
- There are 66 `session.stop_escalated` events: 59 `forced`, 7 `killed`.
- The sampled `forced` events fire 19–35 ms after the stop request, on sessions whose last turn ended minutes earlier (`sess-005569896fddc031` seq 535→536, `sess-08dca6da87f86d8c` seq 843→844).
- 55 of the 123 user-canceled stops have `stop_escalated=1` saved on the session.
- The payload carries `"cause":3`, which means `user_requested`.

**Code.**
- The cooperative phase only runs for `completed` and `owner_released` causes (`internal/session/stop_operation.go:171`).
- `StopCause` is an `int` with only `String()` (`internal/session/stop_cause.go:4`) and is embedded as-is (`internal/session/stop_events.go:22`). Turn-stop payloads, by contrast, use strings (`internal/session/turn_stop_events.go:20`).
- The `Escalated` flag reaches agents through `compozy__session_stop` (`internal/daemon/native_tool_session_stop.go:42`).

**Fix.**
- Don't emit or save an escalation when the stop policy goes straight to forced for an idle session.
- Serialize causes as stable string tokens, and have readers accept the historical integers.

**Suites.** `internal/session/stop_reason_test.go`, `internal/session/manager_stop_integration_test.go`.

**Minor (single sample).** Codex's `session_info_update` titles its thread with Compozy's injected preamble ("Session instructions (treat as system guidance…): <compozy-runtime-context>…"). Compozy then records it as a `system` event whose `tool_name` holds that title (`sess-34a3c21d5d160cf7` seq 240).

## 3. Historical incidents excluded as fixed

- **H1 — Claude rejected hosted tool results that didn't match the declared output schema.** The rejections read `data/session/runtime must NOT have additional properties`, `data must have required property 'truncated'` and `data/skill/source must be string`.
  - Before beta.38 (2026-10-22T22:03:15Z): `session_create` failed 24/24, `session_archive` 20/22, `session_status` 4/4, `skill_view` 17/82.
  - After beta.38: `session_create` passed 9/9 and `skill_view` 18/18.
  - Fixed by `00dcc3da2` (#885, shared `sessionRuntimeOutputSchema` in `internal/tools/builtin/sessions_create_schemas.go:30`) and `13aeecc40` (#892). Both first shipped in v0.3.0-beta.38.
  - The guards are `internal/tools/builtin/builtin_test.go` and `testdata/native-tool-catalog.json`. I did not verify that every tool's real output is checked against its schema with a strict validator.
- **H2 — Older incident classes known only from Git.** These predate the data window, and I did not re-verify them against live data:
  - incremental transcript fold (`88534944a`, #715)
  - false Goal attention and idle CPU (`fe5cd3d21`, #770)
  - idle warnings/stops and idle release (`eb6113595` #787, `e7df69bab` #789)
  - load mitigations (`a32d21bc3`, #797)
  - busy-target message delivery and hop model (`bf5fc3c38`)
  - tool call crossing a Goal turn (`928848ba4`, #803)
  - native monitor wakes (`5f36f0a54`, #860)
  - stale retry loops (`8234b2f3b`, #875)
  - deletion deadlock (`64d4f5afa`, #880)
  - schema-valid budgeted results (`cbd984bd0`)
- **H3 — Live backlog, not a defect.** Five queued synthetic inputs on the active orchestrator `sess-1f21a9fed7f33635` (02:38–02:48Z) are normal pending work, not stuck rows.

## 4. Uncertain hypotheses (need reproduction)

- **U1 — Supervision warnings on turns that had already finished.**
  - On 2026-10-21, `sess-34a3c21d5d160cf7` and `sess-bbe56ae2ae893582` (reviewer subagents) recorded `done end_turn` at 15:52:05Z and 15:54:05Z.
  - 31 minutes later both got `session.supervision_warning cause=inactivity` for the same turn ID, with `quiet_since` about 1 min after `done`.
  - Current code warns only when `turnAwaitsAgentLocked()` is true (`internal/session/work_supervision.go:56`, `internal/session/session_prompt_state.go:109`). That implies `currentTurnID` was still set after `done`.
  - This happened on the non-release runtime, and its logs are gone. Reproduce against baseline before acting.
- **U2 — Restart classification.** The same two idle sessions became `agent_crashed` / `process_exit` ("daemon crashed while session active") at 16:26:56Z, when the desktop started a new runtime. Whether sessions with no turn in flight should land in failure attention after a restart is a policy question.
- **U3 — Stale `sessions.last_update_at`.** The global DB value goes stale for live sessions. For the orchestrator `sess-bbf023e40086b881` the DB says 22:04:15Z while `meta.json` liveness says 02:40:24Z. I don't know whether catalog sorting overlays live values; SQL-based diagnostics are misled either way.
- **U4 — Single occurrences worth a look.**
  - `query transcript projection … sql: transaction has already been committed or rolled back` (`sess-2889ac05f749143e`)
  - hosted MCP bind `nonce_invalid` (`sess-4d95738fd898691c`)
  - `tool_unknown` returned for `skill_view` (7 times)
  - a Codex `initialize` deadline (2026-10-21 20:23Z, `sess-5f50d3a245829df3`)
- **U5 — F6 performance impact not measured.** I did not measure transcript load or SSE cost for very large sessions. Across the log window, transcript GET was 26 ms median and 3.3 s max.
- **U6 — F4 cost figures** come from the provider-reported `token_usage.cost_amount`.
- **U7 — Unidentified HTTP prompt clients.** I did not establish whether they are the CLI over HTTP or external controllers.

## 5. Relation to the controller's in-progress decisions

- **Safe schema diagnostics:** supported by F3, which keeps recurring after beta.38 through `monitor_create`.
- **Lower log noise:** supported by F5. I'd add the deterministic WARN sources, client cancellation logged as ERROR, and caller attribution for UDS requests.
- **Scoped inline skill routing:** related to F7 (budget saturation), but the ledgers show no actual wrong-skill incident.
- **Derived-user delegation ceilings, and reusing the `history type=agent_message` filter:** I collected no direct evidence for or against either.
- **Not on the controller's list; I recommend these next:**
  - F1 has the largest correctness and usability impact: it repeated on two days, lost results and stalled orchestrations for hours.
  - F2 is current and affects about half of Codex idle releases.
