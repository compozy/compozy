---
name: 03-acp-sessions
title: ACP + Sessions + Transcripts + Replay — Real-LLM QA Plan
description: Behavior-first QA scenarios for the ACP subprocess broker, session manager, transcript assembler, replay/SSE, agent identity, and `/agent/context` situation surface. Real-LLM provider lanes against Claude Code, OpenClaw, and Hermes ACP subprocesses; not glorified integration tests.
type: final-qa-child
module: acp-sessions
parent: ../_parent.md
provider_lanes: [claude-code, openclaw, hermes]
authoritative_runtime_truth: internal/CLAUDE.md
---

# 03 — ACP + Sessions + Transcripts + Replay

## 1. Module Surface

The packages under test compose the entire "live agent loop" of AGH: the daemon spawns a real ACP-compatible subprocess, brokers JSON-RPC over stdio, persists every event with correlation keys, and serves replay/SSE back to operators and other agents.

| Package                          | Responsibility (file:line refs)                                                                                                                                                                                                  |
| -------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `internal/acp`                   | `Driver` for ACP launch + JSON-RPC over stdio (`internal/acp/client.go:50-186`); `Start → initializeConnection → negotiateSession (createSession or loadSession)` flow; `Prompt` returns a typed event channel (`client.go:570-610`); `Cancel` sends `session/cancel` notification (`client.go:594-610`); `Stop` cooperative-cancel + subprocess shutdown (`client.go:641-712`); event-type constants (`internal/acp/types.go:24-52`); `IsLoadSessionResourceMissing` classification (`client.go:553-567`). |
| `internal/session`               | `Manager` with goroutine ownership, max-session reservation, lineage, finalization wait (`internal/session/manager.go:83-734`); start/lifecycle in `manager_start.go`, `manager_lifecycle.go`, `manager_stop_*`; prompt path with detached lifetime in `manager_prompt.go`; spawn lineage primitives in `spawn.go:14-200+`; transcript assembly seam in `transcript.go`; `Soul` rewrap in `soul.go`. Caller-environment injection in `manager_start.go:558-559`. |
| `internal/transcript`            | Canonical replay assembler (`internal/transcript/transcript.go:113-130`), schema constant `agh.session.event.v1` (`:17`), assistant buffer + tool-lifecycle merge (`:73-84`, `:239-318`), canonical encode/decode (`MarshalAgentEvent` / `UnmarshalAgentEvent` `:737-822`). |
| `internal/store/sessiondb`       | Per-session SQLite event store (`events.db`); schema migrations registry (`internal/store/sessiondb/session_db.go:74-86`); strict v1→v2 raw-payload strip migration; `events`, `token_usage`, `hook_runs` tables (`:27-72`).      |
| `internal/agentidentity`         | Daemon-validated caller identity (`internal/agentidentity/identity.go:18-247`); env vars `AGH_SESSION_ID`/`AGH_AGENT` (`:20-23`); UDS headers `X-AGH-Session-ID`/`X-AGH-Agent`/`X-AGH-Workspace-ID` (`:25-29`); stable error codes (`identity_required`, `identity_stale`, `identity_mismatch`, `identity_unauthorized`, `identity_lookup_unavailable`); deterministic exit codes (`:34-45`).                                  |
| `internal/situation`             | `/agent/context` and prompt-startup assembler (`internal/situation/service.go:96-308`); workspace, agent, capabilities, limits, peer roster, inbox summary; bounded section limit `DefaultSectionLimit = 8` (`:24`); deterministic provenance (`provenance` `:557-562`).                                                                                                            |
| `internal/sse`                   | Shared SSE decode helpers (`internal/sse/decode.go`).                                                                                                                                                                            |
| `internal/api/contract`          | Shared session/agent-context payloads (`internal/api/contract/agents.go`, `responses.go`, `tasks.go`), `AgentSpawnRequest`/`AgentSpawnPayload` (`agents.go:334-352`), `SessionLineagePayload` mapping.                            |
| `internal/api/core` (entry-points used by sessions) | `parseLastEventID(c.GetHeader("Last-Event-ID"), …)` for SSE replay (`internal/api/core/handlers.go:521`); `pollAndStreamSessionEvents` poll loop (`internal/api/core/session_stream.go:69-100`); after-sequence projection.                                                                |

The HTTP surface for this module is registered at `internal/api/httpapi/routes.go:66-87`:

```
POST   /api/sessions
GET    /api/sessions
GET    /api/sessions/:id
GET    /api/sessions/:id/health
GET    /api/sessions/:id/status
GET    /api/sessions/:id/inspect
POST   /api/sessions/:id/stop
POST   /api/sessions/:id/resume
POST   /api/sessions/:id/repair
POST   /api/sessions/:id/clear
POST   /api/sessions/:id/prompt
POST   /api/sessions/:id/prompt/cancel        ← detached-cancel proof
GET    /api/sessions/:id/events
GET    /api/sessions/:id/history
GET    /api/sessions/:id/transcript
GET    /api/sessions/:id/stream               ← SSE w/ Last-Event-ID replay
POST   /api/sessions/:id/approve
DELETE /api/sessions/:id
POST   /api/sessions/:id/soul/refresh
GET    /api/agent/context                      ← situation surface
POST   /api/agent/spawn                        ← bounded child session
```

The UDS surface mirrors the same routes (`internal/api/udsapi/routes.go:66-90`). The CLI shape is `agh session {new|list|status|inspect|prompt|events|history|stop|resume|repair|wait}` (`internal/cli/session.go:16-37`) and `agh spawn` for bounded child sessions (`internal/cli/spawn.go:28-76`). `agh exec --ide claude --model …` is a separate headless surface and is **not** in scope here.

ACP-supported subagent commands (the binaries that a real-LLM scenario must exercise) come from the builtin provider table at `internal/config/provider.go:124-256`:

| Provider name | Default ACP command | Display name |
|---|---|---|
| `claude` | `npx -y @agentclientprotocol/claude-agent-acp@latest` | Claude Code |
| `openclaw` | `openclaw acp` | OpenClaw |
| `hermes` | `hermes acp` | Hermes |
| `codex` | `npx -y @agentclientprotocol/codex-acp@latest` | Codex |
| `gemini` | `gemini --acp` | Gemini CLI |

Every scenario below names the provider explicitly so the QA runner picks the right binary path.

## 2. Existing Coverage (read carefully — do not duplicate)

The module already has thick unit and integration coverage. Real-LLM scenarios must NOT replicate these — they must extend behavior into the live-provider lane.

- `internal/acp/client_test.go` — JSON-RPC framing, capability negotiation, `Resource not found` classification (case `IsLoadSessionResourceMissing`, `client_test.go:857-858` injects `Code: -32002, Message: "Resource not found: sess-existing"`); cooperative cancel; permission roundtrip.
- `internal/acp/client_integration_test.go` — long-running prompt with synthetic stdio agent.
- `internal/acp/handlers_test.go`, `internal/acp/failure_probe_test.go`, `internal/acp/launcher_tool_host_test.go`, `internal/acp/process_tree_test.go` — sandbox launcher seam, ToolHost host, process-tree cleanup.
- `internal/session/manager_test.go` — Create/Resume/Stop, `AGH_SESSION_ID`/`AGH_AGENT` env injection (`:2714-2720`, `:2792-2799`), stale-resume → fresh-start fallback (`:585`, `:645`).
- `internal/session/manager_lineage_test.go`, `network_peer_test.go`, `manager_hooks_test.go`, `manager_clear_test.go`, `manager_delete_test.go` — lineage propagation, hooks, soul lock interplay.
- `internal/session/manager_integration_test.go`, `manager_stop_integration_test.go`, `provider_lifecycle_integration_test.go` — full stop/wait join pattern.
- `internal/session/transcript_test.go`, `internal/transcript/transcript_test.go` — golden persisted-events → assembled-Message round trips.
- `internal/store/sessiondb/session_db_integration_test.go`, `session_db_extra_test.go`, `hook_runs_test.go` — events.db schema invariants, append-only ordering.
- `internal/agentidentity/identity_test.go` — every error code path with mocked SessionLookup.
- `internal/situation/service_test.go` — bounded sections, soul snapshot wiring, peer/inbox sort stability.
- `internal/api/httpapi/handlers_test.go`, `internal/api/udsapi/handlers_test.go` — route registry assertions including `POST /api/sessions/:id/prompt/cancel` (`handlers_test.go:193`).

**The gap real-LLM scenarios must close**: every existing test stubs the ACP driver with a hand-rolled fake. None spawn a real Claude Code / OpenClaw / Hermes subprocess and prove the daemon behaves correctly when the upstream agent emits unpredictable token streams, real tool calls, real cancellation acknowledgements, and real subprocess exits.

## 3. Gaps (what the real-LLM lane must prove)

1. **Real tool-use turns persist intact.** Claude Code emits `tool_call` followed by `tool_result` with non-trivial `tool_input`/`tool_output` JSON; transcript reconstruction must produce a `RoleToolCall + RoleToolResult` pair without losing JSON shape (`internal/transcript/transcript.go:239-318`).
2. **SSE typed envelope parity across providers.** The same scenario shape must produce the same SSE event vocabulary (`text-delta`, `reasoning-delta`, `tool-input-start`, `tool-input-available`, `tool-output-available`, `data-agh-event`, `data-agh-permission`, `error`, `finish`) regardless of which ACP agent backs it (`internal/api/httpapi/prompt.go:251-352`).
3. **Detached-prompt invariant under real load.** `httpapi.promptSession` and `udsapi.promptSession` use `context.WithCancel(context.WithoutCancel(c.Request.Context()))` (`prompt.go:104`, `udsapi/prompt.go:33`). When the HTTP/UDS connection drops mid-stream, the subprocess MUST keep producing tokens; only `POST /api/sessions/:id/prompt/cancel` must interrupt it.
4. **Stale-ACP-id classification on resume.** `acp.IsLoadSessionResourceMissing` must convert RPC error code `-32002` "Resource not found" into a fresh-start fallback (`internal/acp/client.go:553-567`); a real provider that has restarted between sessions exercises this path naturally.
5. **`AGH_SESSION_ID` / `AGH_AGENT` reach the subprocess and are read back via UDS headers.** The agent-side CLI uses these env vars to call back into the daemon (`internal/agentidentity/identity.go:20-29`, set in `internal/session/manager_start.go:558-559`); spoofed values must be rejected by `Resolve` against the daemon's authoritative session lookup.
6. **`/agent/context` reflects daemon truth.** Workspace, capabilities (skills + agent-defined), limits, peers, inbox — all bounded to `DefaultSectionLimit = 8` (`situation/service.go:24`) — must match what `situation.Service.ContextForSession` computes from the session manager state at the moment the agent calls `/agent/context`.
7. **Goroutine ownership at shutdown.** `Manager.WaitForFinalizations` (`manager.go:704-734`) must drain; no goroutine leaks across daemon shutdown.
8. **`claim_token` redaction in this surface.** Although `claim_token` is owned by the autonomy kernel, it appears in synthetic prompt metadata (`acp.PromptSyntheticMeta`, `internal/acp/types.go:175-184`) and in any tool result that echoes a task handle. Persisted transcripts, SSE frames, and `/agent/context` MUST never carry a raw `agh_claim_*` token.
9. **Subprocess-crash classification.** A subprocess SIGKILL while a prompt is in flight must be classified `store.FailureProcess` (`store.types.go:117 StopAgentCrashed`, `internal/session/failure.go:15-91`) and surfaced as a typed `error` SSE event before the channel closes — never as a stalled stream.
10. **Replay equivalence.** A persisted events log replayed via `internal/transcript.Assemble` must produce the same `[]Message` ordering and content the live SSE stream produced for the original turn — no drift from the canonical schema (`agh.session.event.v1`).

## 4. Real-LLM Scenarios

Every scenario runs in a fresh `agh-qa-bootstrap` lab with isolated `AGH_HOME`,
daemon port, and tmux-bridge socket. Provider auth follows the resolved
provider contract: bound-secret, brokered, and explicitly isolated-home lanes
use `PROVIDER_HOME` / `PROVIDER_CODEX_HOME`, while `native_cli` lanes with
`home_policy=operator` preserve the operator `HOME` / native login state unless
the scenario explicitly validates isolated provider-home behavior. The runner
spawns the real binary listed under `provider`. Body redaction defaults on;
opt-in capture (`AGH_QA_CAPTURE_CONTENT=1`) for bug reports.

```yaml
id: ACP-01
title: Claude Code real subprocess: tool-use turn persists per-event and streams typed SSE
theme: acp.real
coverage:
  primary: [acp.tool_use, transcript.canonical, sse.typed_envelope]
  secondary: [session.lineage, identity.env_inject]
live: true
provider: claude-code
preconditions:
  - Direct `claude` provider authenticated in the effective Claude home for the lane: operator `HOME` by default, or isolated `PROVIDER_HOME` only when the scenario explicitly validates isolated native auth
  - `agh provider show claude` reports command `npx -y @agentclientprotocol/claude-agent-acp@latest`
  - daemon up via bootstrap manifest; `agh status -o json` reports `running`
preconditions_files: [internal/config/provider.go:165-173]
steps:
  1. `agh session new --agent claude --cwd $LAB/workspace -o json` → record session_id S
  2. Open `GET /api/sessions/$S/stream` SSE; record Last-Event-ID after first event
  3. `agh session prompt $S "Read README.md and tell me the title in one sentence" -o jsonl` (or `POST /api/sessions/$S/prompt {message: …}`)
  4. Wait for SSE to emit `tool-input-start` for read tool, then `tool-output-available`, then a final `text-delta` containing the title, then `finish`
  5. `agh session events $S --type tool_call -o json` and `--type tool_result -o json`
  6. `agh session transcript $S -o json`
expected_behavior:
  - SSE frames in order: `start`, `tool-input-start { toolName: "read"|"Read" }`, `tool-input-available {input.file_path: ".../README.md"}`, `tool-output-available {output.tool_result.content non-empty}`, `text-start`, ≥1 `text-delta`, `text-end`, `finish {finishReason: "stop"}`, `[DONE]`
  - events.db has at least one `tool_call` row and one `tool_result` row sharing a tool_call_id, plus ≥1 `agent_message`
  - Every event row has schema `agh.session.event.v1` (assert via direct sqlite read against `$AGH_HOME/sessions/$S/events.db` table `events.content`)
  - Transcript replay assembles Role=tool_call → Role=tool_result → Role=assistant; ToolName != ""; ToolInput.file_path matches the read file
evidence:
  - `events.json` from `agh session events $S -o jsonl`
  - `transcript.json` from `agh session transcript $S -o json`
  - `sse.log` raw SSE capture
  - `forbidden_needles_check.json`: assert raw `agh_claim_` substring count == 0 across {events.json, transcript.json, sse.log, daemon.log}
failure_signatures:
  - tool_input persisted but missing file_path → broken `internal/transcript/transcript.go:387-393`
  - tool_call without paired tool_result → broken merge in `applyToolResult` (`transcript.go:269-318`)
  - SSE emits `tool-output-available` before any `tool-input-start` → broken `ensureToolCallStarted` ordering (`prompt.go:371-395`)
cleanup:
  - `agh session stop $S`; `agh session list --all -o json` reports state stopped
```

```yaml
id: ACP-02
title: OpenClaw real subprocess parity with Claude Code event vocabulary
theme: acp.real
coverage:
  primary: [acp.provider_parity, sse.typed_envelope]
  secondary: [transcript.canonical]
live: true
provider: openclaw
preconditions:
  - `openclaw` binary on PATH inside PROVIDER_HOME; `openclaw --version` ok
  - `agh provider show openclaw` reports command `openclaw acp` (`internal/config/provider.go:233-237`)
steps:
  1. `agh session new --agent openclaw --provider openclaw --cwd $LAB/workspace -o json` → S
  2. Same prompt as ACP-01 ("Read README.md …")
  3. Capture SSE; capture events.db rows
expected_behavior:
  - Identical typed-event sequence as ACP-01 (start → tool_input → tool_output → text_delta → finish), even if the OpenClaw text body differs
  - tool_call_id values across both providers are unique strings; Manager-assigned IDs are stable across the session lifetime
  - `agh session inspect $S` reports `caps.supports_load_session` matching what OpenClaw advertises in `initialize` response
evidence: same as ACP-01, plus `caps.json` from `inspect`
failure_signatures:
  - One provider emits `data-agh-event` with raw provider payload only — must NOT happen; `prompt.go:354-361` is the fallback only for unknown ACP event types
  - capability negotiation drift between providers → broken `negotiateSession` (`acp/client.go:372-468`)
cleanup: `agh session stop $S`
```

```yaml
id: ACP-03
title: Hermes real subprocess parity with Claude Code event vocabulary
theme: acp.real
coverage:
  primary: [acp.provider_parity]
  secondary: [transcript.canonical]
live: true
provider: hermes
preconditions:
  - `hermes` binary on PATH inside PROVIDER_HOME; `hermes acp --help` shows ACP transport
  - `agh provider show hermes` reports command `hermes acp` (`internal/config/provider.go:215-219`)
steps:
  1. `agh session new --agent hermes --provider hermes --cwd $LAB/workspace -o json` → S
  2. Run the same "Read README.md …" prompt
  3. Capture SSE + events.db
expected_behavior: same as ACP-02, with Hermes-specific tool-name spelling allowed
evidence: same as ACP-02
failure_signatures:
  - Hermes emits its own native event names (e.g. `agent_thought_chunk`) and the daemon does NOT remap to `EventTypeThought` → broken legacy parser (`transcript.go:401-439`)
cleanup: `agh session stop $S`
```

```yaml
id: ACP-04
title: Multi-turn conversation across SSE reconnect with Last-Event-ID replay
theme: replay.after_seq
coverage:
  primary: [sse.replay, sse.last_event_id]
  secondary: [transcript.canonical]
live: true
provider: claude-code
preconditions: ACP-01 preconditions; SSE client supports manual disconnect
steps:
  1. Create session S; send first prompt P1 ("Plan a 3-step refactor of X"); collect SSE; record last-event-id L1
  2. Send second prompt P2 ("Now do step 1") via HTTP; deliberately abort SSE GET halfway through the turn (close TCP after 50% of expected text-delta volume)
  3. Reconnect to `GET /api/sessions/$S/stream` with header `Last-Event-ID: $L1` (lib reads `c.GetHeader("Last-Event-ID")` per `internal/api/core/handlers.go:521`)
  4. Receive only events with sequence > L1; verify no duplicate text-deltas observed in the final transcript
  5. `agh session events $S --since L1 -o json` matches the resumed SSE
expected_behavior:
  - Reconnect emits exactly the missing tail of P1+P2; no event with sequence ≤ L1 is replayed
  - Final reconstructed transcript via `agh session transcript $S` is identical whether assembled from continuous-SSE or reconnect-SSE
evidence:
  - `sse_session_1.log` (initial), `sse_session_2.log` (reconnect)
  - `events_after_L1.json`
  - `transcript_diff.txt`: assert empty diff between the two reconstructions
failure_signatures:
  - reconnect replays events ≤ L1 → broken sequence comparator in `pollAndStreamSessionEvents`
  - reconnect emits zero events even though P2 produced new ones → broken poll/ticker loop (`session_stream.go:77-100`)
cleanup: `agh session stop $S`
```

```yaml
id: ACP-05
title: Cancel mid-turn — explicit cancel terminates prompt; HTTP context cancel does not
theme: detached_lifetime
coverage:
  primary: [acp.cancel, detached_lifetime]
  secondary: [sse.error_finish, transcript.cancelled]
live: true
provider: claude-code
preconditions: ACP-01 preconditions
steps:
  1. Create session S
  2. Issue a prompt that takes ≥30s to complete (e.g. "Recursively summarize every file under src/ and produce a 300-line report"); start SSE stream
  3. After ~3s, kill ONLY the SSE TCP socket (do not call `/prompt/cancel`); record wall-clock
  4. Reconnect SSE with `Last-Event-ID` from step 3; observe that text-deltas continue to arrive (proves `context.WithoutCancel` detached the prompt)
  5. Now `POST /api/sessions/$S/prompt/cancel` (`internal/api/httpapi/sessions.go:43-50`)
  6. Observe SSE emit a `data-agh-event` with `stop_reason: "canceled"` or an `error`/`finish` close-out
expected_behavior:
  - Step 3-4: token deltas continue arriving for at least 5s after socket close — proves `prompt.go:104` (`context.WithoutCancel(c.Request.Context())`) keeps the prompt alive
  - Step 5: prompt drains within 2s of explicit cancel; SSE finishes; events.db ends with a tool/agent event whose `stop_reason` is `"canceled"` (or `acp.AgentEvent.StopReason == "canceled"`); CLI exit code 0
  - `aiSDKFinishReason("canceled")` maps to `"other"` in the AI SDK envelope (`internal/api/httpapi/prompt.go:569-580`)
  - subprocess receives the ACP `session/cancel` notification (proven by stderr capture if provider logs cancellations) and continues to live (Stop is a separate operation)
evidence:
  - `sse_phase1.log`, `sse_phase2.log`, `sse_phase3.log`
  - `cancel_response.json` from `/prompt/cancel`
  - `events_tail.json` showing final stop_reason
failure_signatures:
  - Step 3-4: deltas stop after socket close → request context is still tied to prompt; `prompt.go:104` regression
  - Step 5: prompt does NOT terminate within ≥10s of explicit cancel → `Cancel` notification not delivered (`acp/client.go:594-610`)
  - SSE never emits a finish frame after cancel → broken `runPrompt` cancellation goroutine (`acp/client.go:743-792`)
cleanup: `agh session stop $S`
```

```yaml
id: ACP-06
title: Subprocess crash mid-prompt → typed error, no stale ACP id propagation
theme: failure.process_exit
coverage:
  primary: [failure.classification, acp.stale_session]
  secondary: [transcript.error]
live: true
provider: claude-code
preconditions:
  - ACP-01 preconditions
  - PID of the spawned ACP subprocess available (probe via `agh session inspect -o json` if exposed; otherwise `pgrep -f claude-agent-acp` inside the lab)
steps:
  1. Create session S; start a long prompt
  2. While prompt is in flight, `kill -9 <pid_of_subprocess>` (forces uncooperative exit)
  3. Observe SSE
  4. `agh session status $S -o json` and `agh session inspect $S -o json`
  5. Try to `agh session resume $S` — should fall back to fresh start, not propagate a stale ACP id
expected_behavior:
  - SSE emits a typed `error` event with non-empty `errorText`, then `finish`, then `[DONE]` — never a silently-closed channel
  - Session record transitions to `state=stopped`, `stop_reason=agent_crashed` or equivalent (`store.types.go:117`)
  - `failure.kind == "process_exit"` in the persisted Failure (`store.failure.go:20`, `session/failure.go:15-91`)
  - Resume call to a session whose former ACP session id is now unknown to a freshly respawned subprocess MUST surface as fresh-start (RPC code `-32002` mapped via `acp.IsLoadSessionResourceMissing`); raw ACP error text MUST NOT leak to the caller as a 5xx
  - Crash bundle path is set when `internal/session/crash_bundle.go:166` triggers
evidence:
  - `sse_crash.log`, `session_status.json`, `inspect.json`, `resume_response.json`
  - Bundle artifacts at `$AGH_HOME/crashes/<id>/...` listed in inspect output
failure_signatures:
  - Session reports `state=pending` instead of `crashed` → forensic rule violated (`internal/CLAUDE.md` "Inactive metadata repair must distinguish startup-pending from crashed")
  - Resume call returns 500 with raw "Resource not found" text → `IsLoadSessionResourceMissing` not invoked at the resume call site
cleanup: `agh session delete $S` (terminal cleanup)
```

```yaml
id: ACP-07
title: Stale ACP session id on cold-start resume → fresh-start fallback
theme: acp.stale_session
coverage:
  primary: [acp.stale_session, session.resume]
  secondary: [transcript.canonical]
live: true
provider: claude-code
preconditions: ACP-01 preconditions
steps:
  1. Create session S, send 1 prompt (so ACP session id is captured), `agh session stop $S`
  2. Manually wipe the upstream ACP provider's session storage (e.g. delete the effective Claude session path for the lane: `$HOME/.claude/projects/<S>/...` on operator-home runs or `$PROVIDER_HOME/.claude/projects/<S>/...` on isolated-home runs) so the upstream agent no longer knows the ACP session id
  3. `agh session resume $S` (which calls `Driver.loadSession` → `acpsdk.AgentMethodSessionLoad`)
  4. Observe behavior
expected_behavior:
  - Resume completes successfully via fresh-start fallback (a new ACP session is created); session row reflects new ACP session id
  - Daemon log records the classification at info level (no panic, no 5xx)
  - First post-resume prompt streams normally; replay-assembled transcript stitches old + new events with stable lineage (parent_session_id/root_session_id unchanged)
evidence:
  - `daemon.log` excerpt showing classification (key phrase: "ACP load failed; falling back to new session")
  - `resume.json`, `events_after_resume.json`
failure_signatures:
  - Resume fails with 5xx — classification is not invoked at this call site
  - New session id replaces lineage parent/root pointers — broken lineage continuity
cleanup: `agh session stop $S`
```

```yaml
id: ACP-08
title: Session lineage — parent agent spawns child via `/api/agent/spawn`; lineage tree visible
theme: lineage
coverage:
  primary: [session.spawn, session.lineage]
  secondary: [identity.headers, agent_context.peers]
live: true
provider: claude-code
preconditions: ACP-01 preconditions; coordinator config not in test
steps:
  1. Create parent session P (agent=claude). Record P's `AGH_SESSION_ID`/`AGH_AGENT` from the subprocess env (assert via `agh session inspect P -o json` showing the env was injected per `manager_start.go:558-559`)
  2. From inside the parent agent context, the agent calls `agh spawn --agent claude --ttl-seconds 600 --role worker -o json` (CLI passes `X-AGH-Session-ID: P` UDS header per `internal/cli/agent_kernel.go` + `internal/api/core/agent_identity.go:117`)
  3. Daemon validates identity, `Manager.Spawn` returns child C with lineage{ParentSessionID: P, RootSessionID: P, SpawnDepth: 1, SpawnRole: "worker"} (`internal/session/spawn.go:65-103`)
  4. Send a prompt to child C
  5. `agh session list --all -o json` → confirm both P and C present, lineage populated
  6. `agh session events $C -o json` — every event carries `parent_session_id=P` and `root_session_id=P` correlation keys (per CLAUDE.md observability invariant)
  7. `agh session events $P --type spawn_created -o json` — find the typed spawn event
expected_behavior:
  - Spawn returns 201 with `AgentSpawnPayload` (`contract/agents.go:348-352`); body includes `lineage.parent_session_id == P`, `lineage.root_session_id == P`, `lineage.spawn_depth == 1`
  - Child events.db rows all carry parent/root correlation in JSON event content
  - Sessions list shows P→C tree
  - `/api/agent/context` for child reports `self.session_id == C` and Soul/lineage section reflects parent
evidence:
  - `spawn.json`, `parent_events.json`, `child_events.json`, `lineage_tree.json` (from `agh session list -o json`)
failure_signatures:
  - lineage.parent_session_id missing on child events — broken normalization in `manager_start.go` or hooks dispatch
  - spawn returns 200 OK without lineage — `agentSpawnPayloadFromSession` regression (`agent_spawn.go:95+`)
cleanup: `agh session stop $C; agh session stop $P`
```

```yaml
id: ACP-09
title: Transcript replay equivalence — persisted events reconstruct the live SSE conversation byte-for-byte
theme: transcript.replay_parity
coverage:
  primary: [transcript.canonical, replay.equivalence]
  secondary: [sse.typed_envelope]
live: true
provider: claude-code
preconditions: ACP-01 preconditions
steps:
  1. Create session S; run a multi-tool prompt: "Read X, write summary to Y, run cat Y" (forces ≥3 tool turns)
  2. Capture every SSE frame in order to `live_sse.jsonl`
  3. Stop session
  4. `agh session events $S -o jsonl > stored_events.jsonl`
  5. Run `agh session transcript $S -o json > replay_messages.json`
  6. Run a transcript-from-live reconstruction: feed `live_sse.jsonl` through the same canonical schema decoder (call `transcript.UnmarshalAgentEvent` on the `data` payloads, then `transcript.Assemble` with synthesized `store.SessionEvent`s) → `live_messages.json`
expected_behavior:
  - `live_messages.json` and `replay_messages.json` are JSON-equal modulo whitespace
  - Every Message has matching `id`, `role`, `content`, `tool_name`, `tool_input`, `tool_result`, ordering
  - Schema marker preserved: every persisted `events.content` parses with `schema = "agh.session.event.v1"` (`transcript.go:17`)
evidence:
  - `live_messages.json`, `replay_messages.json`, `diff.txt`
  - `events_schema_check.json`: SQL `SELECT count(*) FROM events WHERE json_extract(content,'$.schema') != 'agh.session.event.v1'` → must equal 0
failure_signatures:
  - role drift (assistant content split where it shouldn't) → broken `flushAssistantOnTurnChange` (`transcript.go:174-181`)
  - tool_input lost between live and replay → broken canonical encode in `MarshalAgentEvent` (`transcript.go:737-787`)
  - schema mismatch → migration v2 `strip_canonical_event_raw_payloads` regression
cleanup: `agh session delete $S`
```

```yaml
id: ACP-10
title: Caller identity — `AGH_SESSION_ID` env present, absent, and spoofed
theme: identity
coverage:
  primary: [identity.env_inject, identity.spoof_reject]
  secondary: [identity.exit_codes]
live: true
provider: claude-code
preconditions: ACP-01 preconditions
steps:
  Subtest A — happy path:
    1. Create session S
    2. From inside the subprocess (via `agh session prompt S "exec: agh agent context -o json"` or by attaching to the spawned subprocess shell), run `agh agent context -o json`
    3. Daemon validates `X-AGH-Session-ID: S`, `X-AGH-Agent: claude` and returns a populated `AgentContextPayload`
  Subtest B — missing identity:
    1. From a shell *outside* any AGH session (env vars absent), run `agh agent context -o json`
    2. CLI exits with code 64 (`ExitIdentityRequired`, `agentidentity/identity.go:38`); JSON error has `code: "identity_required"`, message contains "AGH_SESSION_ID is required"
  Subtest C — spoofed identity:
    1. Set `AGH_SESSION_ID=sess-stale-12345`, `AGH_AGENT=claude` in a shell that has no such session
    2. `agh agent context -o json`
    3. CLI exits 65 (`ExitIdentityInvalid`); JSON error code `identity_stale`
  Subtest D — workspace mismatch:
    1. Create two workspaces W1, W2; create session S in W1
    2. Use S's identity but call `/api/agent/context?workspace=W2`
    3. CLI exits 77 (`ExitUnauthorized`); JSON error code `identity_unauthorized`
expected_behavior:
  - Each subtest returns the exit code listed above (`agentidentity/identity.go:34-45`)
  - JSON error envelope shape matches `ErrorPayloadFor` (`identity.go:306-329`)
  - No raw session id leak in error message text
evidence:
  - `subtest_*.json` per case; `exit_codes.txt`
failure_signatures:
  - Spoofed id returns 200 OK — bypassed `lookupSessionSnapshot` validation (`identity.go:200-246`)
  - Spoofed id matches but for a different agent — `snapshot.AgentName != creds.AgentName` check missing (`identity.go:237-244`)
cleanup: stop both sessions
```

```yaml
id: ACP-11
title: `/agent/context` situation surface returns the agent-correct view bounded by section limits
theme: situation
coverage:
  primary: [situation.context]
  secondary: [situation.bounded_sections, identity.headers]
live: true
provider: claude-code
preconditions:
  - ACP-01 preconditions
  - 12 skills enabled in workspace (forces Section.Truncated=true at limit=8)
steps:
  1. Create session S in workspace W (with 12 skills)
  2. Call `GET /api/agent/context` with X-AGH-Session-ID: S
  3. Inspect payload sections
  4. Send a prompt that uses a skill so the situation surface should reflect the skill capability
  5. Re-call `/agent/context`; capabilities still bounded
expected_behavior:
  - `self.session_id == S`, `self.agent_name == "claude"`, `self.provider == "claude"`
  - `workspace.id == W.id`, `workspace.root_dir == W.root_dir`
  - `capabilities.section.limit == 8`, `capabilities.section.returned == 8`, `capabilities.section.truncated == true`, `len(capabilities.capabilities) == 8` (`situation/service.go:24, 1053-1063`)
  - `limits.max_children`, `limits.max_spawn_depth==1`, `limits.max_active_task_leases==1`, `limits.context_section_limit==8` (`service.go:446-460`)
  - `provenance.source == "daemon.situation"` (`service.go:25, 558-562`); `provenance.generated_at` is RFC3339Nano UTC
  - `peer_roster.section.limit == 8`, `inbox_summary.section.limit == 8`
evidence:
  - `agent_context_phase1.json`, `agent_context_phase2.json`
failure_signatures:
  - capabilities count exceeds 8 — `boundedCapabilities` regression
  - provenance source missing — `provenance` constant changed silently
cleanup: `agh session stop $S`
```

```yaml
id: ACP-12
title: Manager goroutine ownership — clean shutdown with `WaitForFinalizations` join
theme: concurrency
coverage:
  primary: [session.lifecycle, concurrency.no_leak]
  secondary: [daemon.shutdown]
live: true
provider: claude-code
preconditions: ACP-01 preconditions; daemon launched in supervised mode that exposes goleak hook in test build (`-tags goleak_check`)
steps:
  1. Spawn 5 sessions concurrently; send 1 prompt to each; let them run to completion
  2. `agh daemon stop` (graceful)
  3. Daemon shutdown invokes `Manager.WaitForFinalizations(ctx)` (`session/manager.go:704-734`)
  4. With the goleak-instrumented build, capture `goroutine_dump.txt` after `WaitForFinalizations` returns and before process exit
expected_behavior:
  - All 5 finalizations complete within the daemon shutdown timeout
  - goroutine_dump shows zero goroutines owned by `internal/session` package after WaitForFinalizations returns (assert via `runtime.Stack` regex against `pedronauck/agh/internal/session\.`)
  - Daemon exits 0; `daemon.log` records every session as final-state stopped
evidence:
  - `goroutine_dump.txt`, `daemon_shutdown.log`, `final_session_states.json`
failure_signatures:
  - any session goroutine remains in `goroutine_dump.txt` after shutdown — broken WG join in `manager_*.go`
  - shutdown blocks indefinitely → finalization channel not closed (`manager.go:672-700`)
cleanup: ensure no zombie subprocesses (`pgrep -f claude-agent-acp` returns empty)
```

```yaml
id: ACP-13
title: Detached prompt with deadline re-attached — explicit deadline still fires
theme: detached_lifetime
coverage:
  primary: [detached_lifetime, context.deadline]
  secondary: [acp.cancel]
live: true
provider: claude-code
preconditions: ACP-01 preconditions; daemon configured with prompt deadline override (e.g. `SessionSupervisionConfig.PromptDeadline = 8s` in this isolated lab)
steps:
  1. Create session S
  2. Send a prompt expected to take ≥30s
  3. Watch SSE; daemon must enforce its own 8s deadline because `context.WithoutCancel` does NOT preserve deadlines (CLAUDE.md invariant)
  4. After ~8s, daemon must cancel the prompt itself and emit a `error` SSE event
expected_behavior:
  - At ~8s, SSE emits `runtime_warning` (`acp/types.go:46-47`) and then `error` with deadline message; `finish`; `[DONE]`
  - Subprocess remains alive (no Stop)
  - `agh session status $S` shows session still active (only the prompt was deadlined, not the session)
evidence:
  - `sse_deadline.log`, `session_status_after_deadline.json`
failure_signatures:
  - Prompt runs past 30s with no daemon-side cancel — re-attached deadline missing or `context.WithoutCancel` used without re-deadlining (regression of CLAUDE.md "Detached execution lifetime")
cleanup: `agh session stop $S`
```

```yaml
id: ACP-14
title: Subprocess managed-stop respects ctx.Done() between Shutdown and Wait
theme: subprocess.lifecycle
coverage:
  primary: [subprocess.shutdown, ctx.respect]
  secondary: [process_group]
live: true
provider: claude-code
preconditions: ACP-01 preconditions
steps:
  1. Create session S
  2. Issue prompt
  3. While prompt in flight, `agh session stop $S` with a tight deadline (e.g. `--timeout 2s`)
  4. Subprocess receives SIGTERM (Unix process-group signaling per `internal/procutil`); if it doesn't exit within 2s, daemon escalates to SIGKILL (per `acp/client.go:714-741 stopExecCommand`)
expected_behavior:
  - Stop returns within 2s + small grace; CLI exit 0
  - Subprocess gone (no orphan in process tree); `pgrep -f claude-agent-acp` returns empty
  - SSE for the in-flight prompt closes with `error` or final `done` carrying `stop_reason=session_stopped`
  - `proc.Wait()` is wrapped in `select { case <-proc.Done(): case <-ctx.Done(): }` per CLAUDE.md "Subprocess managed-stop"
evidence:
  - `process_tree_before.txt`, `process_tree_after.txt` (`ps -ef -o pid,pgid,command`)
  - `sse_stop.log`
failure_signatures:
  - Orphan subprocess survives → broken `stopAgentProcessAndWait` (`acp/client.go:697-712`) or broken `terminateManagedProcess` (`client.go:714-741`)
  - Stop blocks past deadline → `select` not honoring ctx
cleanup: ensure no orphans
```

```yaml
id: ACP-15
title: Concurrent two prompts on same session serialize — no interleaved JSON-RPC
theme: acp.serialization
coverage:
  primary: [acp.prompt_queue]
  secondary: [transcript.canonical]
live: true
provider: claude-code
preconditions: ACP-01 preconditions
steps:
  1. Create session S
  2. From two concurrent goroutines/processes, both call `POST /api/sessions/$S/prompt` with prompts P1 and P2
  3. Capture the active turn IDs via SSE; capture full transcript
expected_behavior:
  - Daemon serializes the two prompts: only one `acpsdk.AgentMethodSessionPrompt` JSON-RPC call is in flight at a time (`internal/session/manager_prompt.go` enforces a per-session prompt mutex via `proc.beginPrompt` (`acp/client.go:584-587`))
  - The second caller either (a) waits and then runs after the first, or (b) is rejected with a typed error — pick whichever the implementation guarantees, then assert that exactly
  - Final transcript shows two complete turns (no half-merged assistant content)
  - Each turn has its own turn_id; transcript role boundaries respect turn changes (`transcript.go:174-181`)
evidence:
  - `prompt_a_sse.log`, `prompt_b_sse.log`, `transcript_final.json`
failure_signatures:
  - text-deltas of P1 and P2 interleave in the same turn_id → broken serialization
  - `proc.beginPrompt` returned an active state for two concurrent calls
cleanup: `agh session stop $S`
```

```yaml
id: ACP-16
title: Very large output (>1MB streamed) — backpressure works, transcript persists, no OOM
theme: backpressure
coverage:
  primary: [acp.stream_backpressure, persistence.large_event]
  secondary: [transcript.canonical]
live: true
provider: claude-code
preconditions: ACP-01 preconditions; daemon log threshold raised to capture WARN if backpressure triggers
steps:
  1. Create session S
  2. Issue a prompt that forces a large output (e.g. "Print the contents of generated_long_file.txt verbatim" where the file is ~2MB)
  3. Stream SSE; observe daemon RSS during the stream (sample every second)
  4. After completion, `agh session transcript $S` and assert the assistant message content size matches the original
expected_behavior:
  - Daemon RSS growth bounded (no unbounded buffering — `acp.Driver.WithPromptBufferSize` defaults to 128 events `acp/client.go:25`)
  - Every text-delta is flushed (assert SSE writer flush semantics in `core.WriteSSE`)
  - events.db rows for `agent_message` chunks add up to the full content; sequence is monotonic
  - No `runtime_warning` for stalled writer
evidence:
  - `rss_samples.csv`, `transcript_size_bytes.txt`, `events_count.txt`
failure_signatures:
  - RSS grows to >2× expected → unbounded buffer
  - SSE pauses for seconds at a time → blocked flush
cleanup: `agh session stop $S`
```

```yaml
id: ACP-17
title: ACP version mismatch — daemon refuses or downgrades cleanly with stable error
theme: acp.version
coverage:
  primary: [acp.protocol_negotiation]
  secondary: [error.stable_envelope]
live: true
provider: claude-code
preconditions:
  - PROVIDER_HOME pinned to an older `@agentclientprotocol/claude-agent-acp` major that doesn't speak `acpsdk.ProtocolVersionNumber` (`internal/acp/client.go:339`)
steps:
  1. `agh session new --agent claude --cwd $LAB/workspace -o json`
  2. Daemon attempts `acpsdk.AgentMethodInitialize` (`acp/client.go:337-369`); upstream returns version-mismatch error
expected_behavior:
  - Session creation fails with `store.FailureHandshake` and a typed error envelope; CLI exit non-zero with stable code
  - Error message identifies "ACP initialize handshake failed" and includes the upstream agent name (`acp/client.go:357-364`)
  - No subprocess orphan (cleanup invoked via `cleanupFailedStart`, `acp/client.go:543-551`)
  - Daemon log records the version mismatch at WARN/ERROR but does not panic
evidence: `create_response.json`, `daemon.log` excerpt, `process_tree.txt`
failure_signatures:
  - daemon panics on version mismatch — handshake error not classified
  - subprocess remains alive after failed start — `cleanupFailedStart` regression
cleanup: nothing (start failed cleanly)
```

```yaml
id: ACP-18
title: claim_token redaction — raw `agh_claim_*` never appears in transcripts, SSE, web, or logs
theme: security.redaction
coverage:
  primary: [security.claim_token_redaction]
  secondary: [transcript.canonical, sse.typed_envelope]
live: true
provider: claude-code
preconditions:
  - ACP-01 preconditions
  - Test fixture: a synthetic prompt overlay or task wake event whose `PromptSyntheticMeta` carries a fake claim_token (`acp/types.go:175-184`)
  - The fake token has prefix `agh_claim_FAKE_QA_` so it's distinguishable from real ones
steps:
  1. Create session S; trigger a synthetic prompt that has a `PromptSyntheticMeta` with a `claim_token_hash` field set (raw token never sent over the wire — fixture asserts this at the call site)
  2. The synthetic prompt is dispatched by the daemon (e.g. via the autonomy/synthetic prompt path that already calls `manager.dispatchSyntheticPrompt`)
  3. Capture the full SSE stream and persisted events; capture daemon logs for the duration of the run; capture the web SPA SSE if a web client is attached
  4. Run a forbidden-needle scan: `grep -c 'agh_claim_FAKE_QA_' <file>` across {events.db dump, sse.log, daemon.log, web_sse.log, transcript.json, agent_context.json}
expected_behavior:
  - All grep counts == 0 (token never appears raw)
  - `claim_token_hash` field is present in the canonical event payload only as a hash (no raw bytes)
  - CLAUDE.md invariant "Raw `claim_token` (`agh_claim_*`) MUST NEVER appear in logs, status APIs, settings views, error payloads, channel messages, SSE, web UI, or memory" is upheld
evidence:
  - `forbidden_needles.json`: counts per file; all zero
  - `events_dump.sql.txt` (raw rows), `daemon.log`, `transcript.json`
failure_signatures:
  - Any non-zero count → raw token leak; this is a release blocker (cite CLAUDE.md "claim_token redaction is non-negotiable")
cleanup: `agh session stop $S`
```

```yaml
id: ACP-19
title: Resume after daemon restart — events.db survives, transcript stays canonical, lineage preserved
theme: persistence.restart
coverage:
  primary: [persistence.events_db, session.resume]
  secondary: [transcript.canonical, session.lineage]
live: true
provider: claude-code
preconditions: ACP-01 preconditions; ability to restart daemon (`agh daemon restart`)
steps:
  1. Create session S; run a multi-tool prompt to completion; record events_count_before and transcript_hash_before
  2. `agh daemon stop`; verify `$AGH_HOME/sessions/$S/events.db` and `events.db-wal`/`-shm` are not corrupted (sqlite `pragma quick_check` returns OK)
  3. `agh daemon start`
  4. `agh session resume $S`; send a follow-up prompt
  5. `agh session events $S -o jsonl > events_after.jsonl`; transcript again
expected_behavior:
  - Events count after >= count before (new events appended); the first-N events are bit-identical to the saved snapshot before restart
  - Transcript_hash_before matches the prefix of the new transcript
  - `goose_db_version_session` reports exactly the embedded session-stream head as applied
  - lineage fields untouched
evidence:
  - `events_before.jsonl`, `events_after.jsonl`, `goose_version.txt`, `quick_check.txt`
failure_signatures:
  - events.db corrupted on cold start → wal/shm recovery regression (CLAUDE.md "agh-schema-migration: -wal / -shm companion handling on recovery")
  - schema_version mismatch — migration didn't apply or applied twice
cleanup: `agh session stop $S`
```

## 5. Edge Cases

- **Empty prompt body via `extractPromptMessage`** — POST with no message and no parts MUST return 400 "message is required" (`internal/api/httpapi/prompt.go:213-244`).
- **`X-AGH-Workspace-ID` header narrows scope** — request to `/api/agent/context` with header for a workspace the session does not belong to → 403/identity_unauthorized.
- **Session reaches max-sessions cap** — `Manager.reserve` returns `ErrMaxSessionsReached` (`manager.go:33, 571-589`); CLI surfaces the typed error; no orphan reservation left in `m.pending` (assert via `agh session list`).
- **`negotiateSession` fails after handshake succeeds** — subprocess must be cleaned up via `cleanupFailedStart`; no zombie subprocess.
- **`session/load` returns non-`-32002` error** — must NOT trigger fresh-start fallback; bubbled as typed error.
- **Last-Event-ID with non-numeric value** — `parseLastEventID` returns wrapped error (`internal/api/core/session_stream.go:16-27`); HTTP returns 400.
- **Tool result with empty `tool_result.content` AND non-empty `tool_result.error`** — `transcript.cloneToolResult` preserves both; `ToolError` flag is true (`transcript.go:268-318`).
- **Duplicate `tool_call` events** — `applyToolCall` merges into the existing message via `mergeToolCallMessage` (`transcript.go:239-267`); no duplicate row in transcript.
- **Permission prompt timeout** — `defaultPermissionWait = 5*time.Minute` (`acp/client.go:27`); if the user/agent does not approve, prompt fails with typed `permission_timeout` error.
- **`/api/sessions/:id/clear`** wipes conversation history but preserves session metadata; subsequent prompts start fresh.
- **`AGH_AGENT_NAME` legacy env** — `manager_test.go:2720` proves both `AGH_AGENT` and `AGH_AGENT_NAME` are set; agentidentity reads only `AGH_AGENT`. The legacy alias must not be relied upon by the CLI.

## 6. Integration Surfaces

| Surface | Kind | File:line refs |
|---|---|---|
| `POST /api/sessions/:id/prompt` (HTTP) | SSE stream — typed AI-SDK envelope | `internal/api/httpapi/prompt.go:90-156` |
| `POST /api/sessions/:id/prompt` (UDS) | SSE stream — JSON envelope | `internal/api/udsapi/prompt.go:22-74` |
| `POST /api/sessions/:id/prompt/cancel` | HTTP + UDS | `internal/api/httpapi/sessions.go:43-50`, `internal/api/udsapi/sessions.go:20-…`, route reg `httpapi/routes.go:81`, `udsapi/routes.go:80` |
| `GET /api/sessions/:id/stream` | SSE replay | `internal/api/core/session_stream.go:69-100`, `internal/api/core/handlers.go:521` |
| `GET /api/sessions/:id/events`, `/history`, `/transcript` | JSON projection | `internal/api/core/handlers.go` |
| `GET /api/agent/context` | situation surface | `internal/situation/service.go:191-252`, route `httpapi/routes.go:91` |
| `POST /api/agent/spawn` | bounded child session | `internal/api/core/agent_spawn.go:25-103`, route `httpapi/routes.go` (in agent group) |
| CLI `agh session …` | wraps HTTP/UDS | `internal/cli/session.go:16-378` |
| CLI `agh spawn` | calls `/api/agent/spawn` | `internal/cli/spawn.go:28-168` |
| CLI `agh agent context` | calls `/api/agent/context` | `internal/cli/agent_kernel.go` (caller side via UDS headers `agentidentity/identity.go:24-29`) |

## 7. DX Cliffs

- **Detached prompt vs. request lifetime is invisible to operators.** The fact that closing an SSE stream does not stop the LLM is a feature; documentation must be loud about the explicit `prompt/cancel` requirement.
- **Last-Event-ID semantics.** Many SSE clients default to using the `id` field as a cursor, but the AI-SDK envelope (used by the web UI) uses opaque message ids. The HTTP handler reads `Last-Event-ID` from the request header for replay (`handlers.go:521`). Operators using `curl --no-buffer` need to know to send the header back.
- **Stale session id error from upstream is invisible by design.** The "Resource not found" is silently re-classified as fresh-start (`acp/client.go:553-567`); operators see no error but they DO see a new ACP session id in `inspect`. Document this.
- **`AGH_AGENT_NAME` vs `AGH_AGENT`.** Legacy alias still emitted in env; only `AGH_AGENT` is authoritative.
- **`/agent/context` is bounded; the agent must page to see beyond the limit.** Section limit defaults to 8; an agent that needs more must call `/api/skills`, `/api/network/peers`, `/api/network/inbox` directly.

## 8. Failure Modes

| Mode | Surface | Detection |
|---|---|---|
| Subprocess crash | events.db tail + session state | ACP-06; assert `failure.kind == "process_exit"` |
| Stale ACP session id | resume call | ACP-07; classification via `IsLoadSessionResourceMissing` |
| ACP version mismatch | handshake | ACP-17; `store.FailureHandshake` |
| Permission timeout | prompt path | scenario in §5; `ApprovePermission` returns `defaultPermissionWait` exceeded |
| Goroutine leak | shutdown | ACP-12; `goleak.VerifyNone` post `WaitForFinalizations` |
| Unbounded buffer | large output | ACP-16; RSS sampling |
| Duplicate replay | SSE reconnect | ACP-04; sequence comparator |
| Detached cancel ignored | prompt path | ACP-05; SSE continues after explicit cancel = blocker |
| Lineage drop on restart | persistence | ACP-19; lineage assertions post-restart |
| Raw `agh_claim_*` leak | any | ACP-18; needle scan |
| Identity spoof accepted | `/agent/*` | ACP-10/C |

## 9. Fixtures

- **Bootstrap manifest**: produced by `agh-qa-bootstrap` skill; includes unique `AGH_HOME`, daemon ports, tmux-bridge socket, `PROVIDER_HOME`/`PROVIDER_CODEX_HOME` paths, `AGH_WEB_API_PROXY_TARGET` (when web QA also runs).
- **Workspace seed**: `$LAB/workspace/` with a `README.md` (≥3 paragraphs), `src/file_a.go`, `src/file_b.go`, and a `generated_long_file.txt` (~2MB) for ACP-16.
- **Skill seed**: 12 enabled skills under `$AGH_HOME/skills/` for ACP-11 (truncation proof).
- **Provider auth**: direct `claude` uses native Claude CLI auth from the effective Claude home for the lane (operator `HOME` by default; isolated `PROVIDER_HOME` only for explicit isolated-home scenarios). OpenClaw, Hermes, wrapped providers, and brokered credentials follow their own contract and may stage auth into `PROVIDER_HOME` / `PROVIDER_CODEX_HOME` when the lane is bound-secret or explicitly isolated.
- **Forbidden needles**: `["agh_claim_FAKE_QA_", "agh_claim_TESTONLY_"]` for ACP-18; runner sweeps SSE/events/log files for these and asserts count == 0.
- **goleak build tag**: `//go:build goleak_check` for ACP-12 to avoid hot-path overhead in production builds.

## 10. Citations

- Repo-wide rules: `/Users/pedronauck/Dev/compozy/agh/CLAUDE.md` (Critical Rules; Workflow; Skill Dispatch; CI/Release).
- Backend invariants: `/Users/pedronauck/Dev/compozy/agh/internal/CLAUDE.md` — Architecture (lines 9-49), Concurrency (29-37), Observability (47-52), Security Invariants (55-62), Forensic Bug Fixes (130-135).
- ACP driver:
  - `/Users/pedronauck/Dev/compozy/agh/internal/acp/client.go:23-31` (defaults), `:50-186` (Driver/Start), `:337-369` (initialize), `:372-468` (negotiate), `:553-567` (`IsLoadSessionResourceMissing`), `:570-610` (Prompt+Cancel), `:641-741` (Stop).
  - `/Users/pedronauck/Dev/compozy/agh/internal/acp/types.go:24-52` (event-type constants), `:175-184` (`PromptSyntheticMeta`).
  - `/Users/pedronauck/Dev/compozy/agh/internal/acp/launcher.go` (sandbox launcher seam).
- Sessions:
  - `/Users/pedronauck/Dev/compozy/agh/internal/session/manager.go:83-734` (Manager core); `:558-559` env injection in `manager_start.go`.
  - `/Users/pedronauck/Dev/compozy/agh/internal/session/spawn.go:14-200+` (Spawn caps, lineage, permission narrowing).
  - `/Users/pedronauck/Dev/compozy/agh/internal/session/failure.go:15-91`, `crash_bundle.go:166`, `liveness.go:32-39`.
- Transcript:
  - `/Users/pedronauck/Dev/compozy/agh/internal/transcript/transcript.go:17` (`agh.session.event.v1`), `:113-130` (Assemble entry), `:174-181` (turn-change flush), `:239-318` (tool lifecycle), `:737-822` (Marshal/Unmarshal canonical).
- Persistence:
  - `/Users/pedronauck/Dev/compozy/agh/internal/store/sessiondb/session_db.go:27-72` (schema), `:74-86` (migrations registry).
  - `/Users/pedronauck/Dev/compozy/agh/internal/store/types.go:117` (`StopAgentCrashed`).
  - `/Users/pedronauck/Dev/compozy/agh/internal/store/failure.go:20` (`FailureProcess`).
- Identity:
  - `/Users/pedronauck/Dev/compozy/agh/internal/agentidentity/identity.go:18-247` (full Resolve flow + error envelopes), `:34-45` (exit codes).
- Situation:
  - `/Users/pedronauck/Dev/compozy/agh/internal/situation/service.go:24` (`DefaultSectionLimit`), `:96-144` (Service ctor), `:191-252` (`ContextForSession`), `:446-460` (limits), `:557-562` (provenance), `:1053-1063` (`sectionMeta`).
- API surfaces:
  - `/Users/pedronauck/Dev/compozy/agh/internal/api/httpapi/routes.go:66-87` (session route registration).
  - `/Users/pedronauck/Dev/compozy/agh/internal/api/udsapi/routes.go:66-90`.
  - `/Users/pedronauck/Dev/compozy/agh/internal/api/httpapi/prompt.go:90-156` (HTTP SSE entry; detached lifetime at `:104`); `:251-580` (typed envelope state machine).
  - `/Users/pedronauck/Dev/compozy/agh/internal/api/udsapi/prompt.go:22-74`.
  - `/Users/pedronauck/Dev/compozy/agh/internal/api/httpapi/sessions.go:43-50` (`cancelSessionPrompt`).
  - `/Users/pedronauck/Dev/compozy/agh/internal/api/core/session_stream.go:69-100` (`pollAndStreamSessionEvents`).
  - `/Users/pedronauck/Dev/compozy/agh/internal/api/core/handlers.go:521` (`Last-Event-ID`).
  - `/Users/pedronauck/Dev/compozy/agh/internal/api/core/agent_spawn.go:25-103`.
- CLI:
  - `/Users/pedronauck/Dev/compozy/agh/internal/cli/session.go:16-378` (session subtree).
  - `/Users/pedronauck/Dev/compozy/agh/internal/cli/spawn.go:28-168` (spawn cmd).
- Provider matrix:
  - `/Users/pedronauck/Dev/compozy/agh/internal/config/provider.go:124-256` (claude / openclaw / hermes / codex / gemini commands).
- QA framework references:
  - `/Users/pedronauck/Dev/compozy/agh/.compozy/tasks/final-qa/_references/openclaw-qa-patterns.md` (scenario shape, provider-mode tri-state, evidence-as-pass-criterion).
  - `/Users/pedronauck/Dev/compozy/agh/.compozy/tasks/final-qa/_references/hermes-qa-patterns.md` (hermetic env shield, async/cancel rigor, subprocess HOME isolation, cancellation ≤2s assertions).
