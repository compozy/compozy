---
id: ET-session-compact-now
area: ET
title: Request the agent's own compaction from CLI, API, tool, and Web
persona: Ada
journey: J-15-operate-session-via-cli-api
expected: `compozy session compact <id>`, `POST …/sessions/{session_id}/compact` over HTTP and UDS, and `compozy__session_compact` send exactly the command the agent advertises (`/compact`, else `/compress`) as a maintenance prompt and return the accepted shape (202, `status: accepted`, `command`), the CLI reports `outcome` completed, failed, cancelled, turn_completed, or turn_failed, a running turn or a concurrent request returns 409 `session_busy`, an agent advertising neither command returns 409 `compaction_unsupported` (stable `code` on the CLI, HTTP, and UDS error payloads; the native tool surfaces the same refusals as a `tool_conflict` tool error), and the session context meter offers Compact now only for a supporting session, disabled while a turn runs once the Web action ships.
entry_points: compozy session compact <session-id> [-o json]; POST /api/workspaces/{workspace_id}/sessions/{session_id}/compact over HTTP and UDS; native tool compozy__session_compact (toolset compozy__sessions, risk mutating); Web session context meter → Compact now (Web action not shipped on the integration branch yet)
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
   (`requested_by: cli`) precedes the turn; the usage marker carries `trigger: "requested"`; registered
   `context.pre_compact` and `context.post_compact` hooks each ran once with `trigger: "requested"`. Human output
   reads `Compaction requested: /compact (prompt <id>)` then `Compaction completed`; `-o json` prints only the
   four-key object, with no "requested" line.
2. **Outcome classes.** A mock that finishes the turn without compaction frames → `outcome: turn_completed`;
   a failed turn → `turn_failed`; a failed or cancelled compaction update → `failed` / `cancelled` with the
   transcript item showing `error` on failed.
3. **Command selection.** An agent advertising only `compress` receives `/compress`; one advertising both
   receives `/compact`.
4. **HTTP and UDS parity.** `POST …/compact` with body `{}` returns 202
   `{"session_id","prompt_id","command":"compact","status":"accepted"}` identically on both transports; an unknown
   session returns 404 (`compozy session compact <unknown-id>` exits 69), and a session that is not active
   (stopped, not resumed by the request) returns 400 with `code: session_not_promptable`.
5. **Refusals.** While a turn runs, HTTP and UDS return 409 with `code: session_busy`; the CLI exits 65 and prints
   `error: session: prompt already in progress` on stderr, and `-o json` prints `{"error": …, "code":"session_busy"}`;
   the tool returns a `tool_conflict` tool error with the same message. Two simultaneous compact requests yield
   exactly one 202 and one 409 `session_busy`. An agent advertising neither command returns 409
   `compaction_unsupported` (CLI message `session: agent does not advertise a compaction command`) and sends nothing
   to the agent.
6. **Maintenance mode.** The compact turn carries no skill expansion, augmenters, or startup instructions. On a
   session with a pending resume replay or imported context, that content rides the NEXT ordinary prompt exactly
   once, not the compact turn.
7. **Native tool.** `compozy__session_compact {"session_id": …}` returns the accepted shape without waiting, is gated
   as a `mutating` risk by the session's tool policy, and refuses with `tool_conflict` (409 causes) or
   `tool_invalid_input` (a session that is not active); the `session_busy` and `compaction_unsupported` codes
   appear only on the CLI, HTTP, and UDS payloads.
8. **Web (E2E-005, E2E-006).** Contract per `_uiux.md` S17/S18; the Compact now action and the Compaction timeline
   row are not shipped on the integration branch yet, so record this step as blocked (not shipped) until
   `git grep -i "compact now" web/src` finds the action, then re-check its placement and copy before walking it.
   From the session context meter on a supporting session, Compact now is present and enabled when idle. Choosing
   it shows a "Compacting context…" timeline row, then "Context compacted" with an expandable summary, and the
   meter lists an Agent/Requested compaction marker. On a session whose agent advertises nothing the action is
   absent. While a turn streams on a supporting session it is disabled. A request error (`session_busy`,
   `compaction_unsupported`) shows with the control's existing inline error treatment.

QA impact 2026-10-07 (memory removal): new in this change; no prior verdict.
