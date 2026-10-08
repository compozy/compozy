---
id: ET-session-compact-now
area: ET
title: Request the agent's own compaction from CLI, API, tool, and Web
persona: Ada
journey: J-15-operate-session-via-cli-api
expected: `compozy session compact <id>`, `POST …/sessions/{session_id}/compact` over HTTP and UDS, and `compozy__session_compact` send exactly the command the agent advertises (`/compact`, else `/compress`) as a maintenance prompt and return the accepted shape (202, `status: accepted`, `command`), the CLI reports `outcome` completed, failed, cancelled, turn_completed, or turn_failed, a running turn or a concurrent request returns 409 `session_busy`, an agent advertising neither command returns 409 `compaction_unsupported` (stable `code` on the CLI, HTTP, and UDS error payloads, and the same two codes structurally on the native tool's error), the compaction request records who asked in `requested_by` (`cli`, `http`, `tool`, `web`, or `goal`), the CLI `outcome` follows the compaction's current terminal status, and the Web context rail's meter section offers a Compact now button only for a supporting active session, disabled while a turn runs or its request is in flight.
entry_points: compozy session compact <session-id> [-o json]; POST /api/workspaces/{workspace_id}/sessions/{session_id}/compact over HTTP and UDS; native tool compozy__session_compact (toolset compozy__sessions, risk mutating); Web session context rail, meter section → Compact now button (no popover)
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: RT-pressure-context-compaction; ET-web-session-context-meter; ET-cli-session-usage-context; RT-session-compact-real-adapters
---

Experimental: Compact now follows an unstable upstream ACP compaction contract. Walk it on acpmock sessions
that replay the recorded Claude compaction frames; the real-adapter walk is RT-session-compact-real-adapters.

1. **Happy path.** On an idle session whose agent advertises `compact`, run
   `compozy session compact <id> -o json`. Expect `{"session_id","prompt_id","command":"compact","outcome":"completed"}`;
   the mock received a prompt whose text is exactly `/compact`; `GET …/transcript` folds one Compaction item
   (a `data-compozy-compaction` part, with the summary on the Claude frames), while `compozy session history -o json`
   returns the raw ledger rows for the turn (the compaction snapshot rows and one `session.compaction_fired` row
   with `trigger: "requested"`) and no folded item; a `session.compaction.requested` event
   (`requested_by: cli`, see step 4 for the other values) precedes the turn; the usage marker carries `trigger: "requested"`; registered
   `context.pre_compact` and `context.post_compact` hooks each ran once with `trigger: "requested"`. Human output
   reads `Compaction requested: /compact (prompt <id>)` then `Compaction completed`; `-o json` prints only the
   four-key object, with no "requested" line.
2. **Outcome classes.** A mock that finishes the turn without compaction frames → `outcome: turn_completed`;
   a failed turn → `turn_failed`; a failed or cancelled compaction update → `failed` / `cancelled` with the
   transcript item showing `error` on failed. The outcome is the compaction's CURRENT status when the turn ends:
   a corrected update before the turn completes (completed → failed, or failed → completed) changes it, and a
   later non-terminal (vendor) status falls back to `turn_completed` / `turn_failed`. If the mock cannot replay a
   correcting update, leave that leg to the CLI wait and Goal reconstruction suites instead of claiming it.
3. **Command selection.** An agent advertising only `compress` receives `/compress`; one advertising both
   receives `/compact`.
4. **HTTP and UDS parity.** `POST …/compact` with body `{}` returns 202
   `{"session_id","prompt_id","command":"compact","status":"accepted"}` identically on both transports; an unknown
   session returns 404 (`compozy session compact <unknown-id>` exits 69), and a session that is not active
   (stopped, not resumed by the request) returns 400 with `code: session_not_promptable`. Attribution in the
   `session.compaction.requested` event (`compozy session events <id>`): a plain HTTP or UDS call records
   `requested_by: "http"`; the same call with an `X-Compozy-Client-ID` header that starts with `web-` records
   `"web"`; a `compozy-cli` User-Agent records `"cli"` and wins over a `web-` header; the native tool records
   `"tool"`. `requested_by` is telemetry, never an authorization input.
5. **Refusals.** While a turn runs, HTTP and UDS return 409 with `code: session_busy`; the CLI exits 65 and prints
   `error: session: prompt already in progress` on stderr, and `-o json` prints `{"error": …, "code":"session_busy"}`;
   the tool returns a tool error whose `code` is `session_busy` with the same message. Two simultaneous compact requests yield
   exactly one 202 and one 409 `session_busy`. An agent advertising neither command returns 409
   `compaction_unsupported` (CLI message `session: agent does not advertise a compaction command`; the tool error
   `code` is `compaction_unsupported`) and sends nothing to the agent.
6. **Maintenance mode.** The compact turn carries no skill expansion, augmenters, or startup instructions. On a
   session with a pending resume replay or imported context, that content rides the NEXT ordinary prompt exactly
   once, not the compact turn. The pending replay is a durable obligation (`pending_resume_replay` session
   metadata), so it still arrives once on the next ordinary prompt after the compact turn, a stop and daemon
   restart, or a native resume; the second ordinary prompt carries none (owned in depth by
   MS-workspace-checkpoint-continuity).
7. **Native tool.** `compozy__session_compact {"session_id": …}` returns the accepted shape without waiting, is gated
   as a `mutating` risk by the session's tool policy, and refuses with the structural tool-error codes
   `session_busy` and `compaction_unsupported` (each with the tool ID and the daemon message, not a generic
   `tool_conflict`) or `tool_invalid_input` (a session that is not active). The caller's own session is always
   mid-turn, so targeting it returns `session_busy`.
8. **Web (E2E-005, E2E-006).** Open the session's context rail from the composer context control; Compact now is
   a button in the rail's meter section (`data-testid="session-context-compact-now"`), not a popover.
   - Presence: it is present and enabled when the session is active, idle, and its agent advertises `compact` or
     `compress`; it is absent when the agent advertises nothing; it is disabled while a turn streams, while its own
     request is in flight (`aria-busy`), and when the session is not active.
   - Choosing it sends one `POST …/compact` with `{}` and gets 202; the event records `requested_by: "web"`. The
     timeline shows one Compaction row (`session-compaction-item`, `data-status`) that reads "Compacting context…"
     while in progress and then "Context compacted", with a closed "Summary" disclosure
     (`session-compaction-summary`) that expands to the agent's summary (Claude frames; Codex frames show no
     disclosure). A failed compaction reads "Context compaction failed" followed by the error, a cancelled one
     "Context compaction cancelled", and any other status is shown verbatim and never treated as finished. The row
     stays visible when its turn settles.
   - In the rail's Turns section the compaction appears as a marker (`session-context-compaction-marker`) that reads
     "Requested compaction · completed", plus the tokens before → after once the agent's next usage report makes
     the after figure known. A compaction the agent started itself reads "Agent compaction".
   - Meter: until the agent's next usage report the meter and the composer tooltip read "Context usage unknown"
     with "Context compacted. Waiting for the agent's next usage report."; before any report ever, the sentence is
     "This agent hasn't reported context usage."; after the next report the numbers return.
   - Refusal: a `session_busy` or `compaction_unsupported` response shows the daemon's message inline under the
     button (`session-context-compact-error`) and clears when the turn ends or on the next attempt; any other
     failure reads "Couldn't request compaction. Try again."
   Automated owners: E2E-005 and E2E-006 (`web/e2e/__tests__/session-hardening.spec.ts`) and the session inspector
   unit suite; this step is the real-lab walk and has not run, so the scenario stays `untested`.

QA impact 2026-10-07 (memory removal): new in this change; no prior verdict.

Review round 2 transport check: invoke `compozy__session_compact` through `POST /api/tools/{id}/invoke` and hosted MCP for busy and unsupported sessions. Both return HTTP409 with the respective structural code, tool ID and safe message (`session is busy` / `session compaction is unsupported`); backend details must not escape. The generated `tools.ErrorCode` enum includes both values. Owning automated coverage is the existing tool transport parity and hosted MCP suites; no browser walkthrough is claimed.
