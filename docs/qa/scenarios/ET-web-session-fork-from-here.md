---
id: ET-web-session-fork-from-here
area: ET
title: Fork a session from the web, whole or from a message
persona: Bruno
journey: J-14
expected: A user session's row menu (sessions modal, window sidebar, agent detail) and its window overflow offer "Fork session…" right after "Continue with another agent…" (absent for archived, spawned, coordinator, and system rows). A durable user message shows "Fork from here" right before "Rewind to here" (absent for an optimistic tail, assistant messages, and read-only threads; disabled together with Rewind while the thread runs, a rewind is pending, or rewind is blocked). The dialog ("Fork session", eyebrow "Operate · Session", "Start a second session with the same agent and this conversation. This session stays unchanged.") shows the agent read-only as "{agent} · {provider}", the fork point "Whole session" (menus) or "Through “{first 60 characters}”" with a "Change" link that closes the dialog (message), the daemon-measured context line, "Uses the agent's own session clone." only when the preview reports `native_fork_possible`, and Open in (New window default). A cut whose turn has not settled reads "That turn hasn't settled yet." and keeps Fork session disabled; a transcript that changed after opening reads "Transcript changed — reopen to fork from the current state." with Fork session disabled and Cancel reading Close; other refusals show the daemon message verbatim. Fork session shows "Starting the new session…", creates exactly one child with the same agent, and opens it per Open in. The child's status line shows "Forked from {source title}"; the inspector shows Origin "fork · through {message id}" (message cut) or "fork" (whole) and Seed "replay" / "native clone · loaded" / "native clone · failed — carried context used". The source keeps every turn and its `max_sequence`.
entry_points: web session window overflow (fork-menu-item); session row overflow (session-row-fork-{id}) in the sessions modal, window sidebar, and agent detail; user message action (user-message-fork); SessionForkDialog (session-fork-dialog, session-fork-agent, session-fork-point, session-fork-point-change, session-derive-preview, session-derive-preview-native, session-derive-placement, session-fork-submit, session-fork-submit-error); SessionOriginPill (session-origin-pill); inspector Origin section (ledger-origin, ledger-seed); GET …/derive/preview[?message_id=]; POST …/fork
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-web-session-continue, RT-session-derive-native-fork, RT-conversation-rewind, ET-cli-session-continue, ET-web-sessions-catalog-modal
---

Planning 2026-09-28 (session-continue-fork task_06): new behavior. Walk against a lab daemon with an
agent that answers three turns (acpmock `browser_session_fork_fixture.json` `fork-web-agent`) and,
for the native line, one that advertises `session/fork` + `session/load`
(`session_fork_fixture.json` `fork-native-agent`, or real OpenCode):

1. Prompt three turns. Hover the second user message: "Fork from here" sits before "Rewind to here".
   While a fourth prompt runs, both are disabled.
2. Click "Fork from here": the dialog shows the locked agent, "Through “…”" quoting that message,
   "Carries over N messages · X KiB"; Change closes the dialog without creating anything.
3. Fork session with New window: a second window opens on the child; the pill reads
   "Forked from {source title}" and opens the source; `GET …/transcript` on the source reports the
   same `max_sequence` as before and all three turns remain.
4. From the window overflow → "Fork session…": the dialog reads "Whole session"; on the native agent
   (idle, bound) the line adds "Uses the agent's own session clone."; on a replay-only agent it does not.
5. Open the dialog from a message whose turn is still running: "That turn hasn't settled yet.", Fork
   session disabled.
6. Open the dialog, send another prompt to the source from the CLI, then submit: "Transcript changed —
   reopen to fork from the current state.", Fork session disabled, Cancel reads Close; reopening forks.
7. From the sessions modal and agent detail row menus: present for user rows, absent for archived and
   spawned rows; the child always opens in its own window.
8. Stop a forked child and open its inspector: Origin "fork · through msg_…" (or "fork"), Seed per the
   daemon's derivation.

Automated evidence at authoring time: `session-fork-dialog.test.tsx`, `session-thread.test.tsx`,
`sessions-modal.test.tsx`, `use-session-topbar-slot.test.tsx`, `session-inspector.test.tsx`. Web E2E
`web/e2e/__tests__/session-derive.spec.ts` E2E-002 written, not run by task_06. task_08 owns the walk,
the E2E run, and the visual-contract bundles (VC-04, VC-05, VC-11..15, VC-17, VC-23).
