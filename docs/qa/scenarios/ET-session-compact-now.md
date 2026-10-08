---
id: ET-session-compact-now
area: ET
title: Request the agent's own compaction from CLI, API, tool, and Web
persona: Ada
journey: J-15-operate-session-via-cli-api
expected: `compozy session compact <id>`, `POST …/sessions/{session_id}/compact` over HTTP and UDS, and `compozy__session_compact` send exactly the command the agent advertises (`/compact`, else `/compress`) as a maintenance prompt and return the accepted shape (202, `status: accepted`, `command`), the CLI reports `outcome` completed, failed, cancelled, turn_completed, or turn_failed, a running turn or a concurrent request returns 409 `session_busy`, an agent advertising neither command returns 409 `compaction_unsupported`, and the Web context meter shows Compact now only for a supporting session, disabled while a turn runs.
entry_points: compozy session compact <session-id> [-o json]; POST /api/workspaces/{workspace_id}/sessions/{session_id}/compact over HTTP and UDS; native tool compozy__session_compact (toolset compozy__sessions, risk write); Web session context meter popover → Compact now
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
   the mock received a prompt whose text is exactly `/compact`; history shows one Compaction item (with the
   summary on the Claude frames); one `session.compaction_fired` event with `trigger: "requested"`; the usage
   marker carries `trigger: "requested"`; registered `context.pre_compact` and `context.post_compact` hooks
   each ran once with `trigger: "requested"`. Human output reads `Compaction requested: /compact (prompt <id>)`
   then `Compaction completed`.
2. **Outcome classes.** A mock that finishes the turn without compaction frames → `outcome: turn_completed`;
   a failed turn → `turn_failed`; a failed or cancelled compaction update → `failed` / `cancelled` with the
   item showing `error` on failed.
3. **Command selection.** An agent advertising only `compress` receives `/compress`; one advertising both
   receives `/compact`.
4. **HTTP and UDS parity.** `POST …/compact` with body `{}` returns 202
   `{"session_id","prompt_id","command":"compact","status":"accepted"}` identically on both transports; an unknown
   or inactive session returns the existing session errors.
5. **Refusals.** While a turn runs, every surface (CLI, HTTP, UDS, tool) returns 409 `session_busy`; two
   simultaneous compact requests yield exactly one 202 and one 409 `session_busy`. An agent advertising neither
   command returns 409 `compaction_unsupported` on every surface and sends nothing to the agent.
6. **Maintenance mode.** The compact turn carries no skill expansion, augmenters, or startup instructions. On a
   session with a pending resume replay or imported context, that content rides the NEXT ordinary prompt exactly
   once, not the compact turn.
7. **Native tool.** `compozy__session_compact {"session_id": …}` returns the accepted shape, is gated as a
   `write` risk by the session's tool policy, and returns the same error codes as HTTP.
8. **Web (E2E-005, E2E-006).** Open the context meter popover on a supporting session: Compact now is present,
   enabled when idle. Choose it: the popover closes, a "Compacting context…" timeline row appears, then
   "Context compacted" with an expandable summary, and the meter lists an Agent/Requested compaction marker.
   On a session whose agent advertises nothing the action is absent. While a turn streams on a supporting
   session it is disabled. A request error (`session_busy`, `compaction_unsupported`) shows in the popover's
   inline error treatment.

QA impact 2026-10-07 (memory removal): new in this change; no prior verdict.
