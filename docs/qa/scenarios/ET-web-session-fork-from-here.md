---
id: ET-web-session-fork-from-here
area: ET
title: Fork a session from the web, whole or from a message
persona: Bruno
journey: J-14
expected: A user session's row menu (sessions modal, window sidebar, agent detail) and its window overflow offer "Fork session…" right after "Continue with another agent…" (absent for archived, spawned, coordinator, and system rows). A durable user message shows "Fork from here" right before "Rewind to here" (absent for an optimistic tail, assistant messages, and read-only threads; disabled together with Rewind while the thread runs, a rewind is pending, or rewind is blocked). The dialog ("Fork session", eyebrow "Operate · Session", "Start a second session with the same agent and this conversation. This session stays unchanged.") shows the agent read-only as "{agent} · {provider}", the daemon-measured context line ("Carries over N messages" for the whole session from menus, "Carries over N of M messages" through the clicked message's turn), and "Uses the agent's own session clone." only when the preview reports `native_fork_possible`. It has no fork-point field and no Open in choice. A cut whose turn has not settled reads "That turn hasn't settled yet." and keeps Fork session disabled; a transcript that changed after opening reads "Transcript changed — reopen to fork from the current state." with Fork session disabled and Cancel reading Close; other refusals show the daemon message verbatim. Fork session shows "Starting the new session…", creates exactly one child with the same agent, and opens it in the same window, replacing the source there (a list outside any session window — sessions modal, agent detail — opens it in its own window). The child's status line shows "Forked from {source title}"; the inspector shows Origin "fork · through {message id}" (message cut) or "fork" (whole) and Seed "replay" / "native clone · loaded" / "native clone · failed — carried context used". The source keeps every turn and its `max_sequence`.
entry_points: web session window overflow (fork-menu-item); session row overflow (session-row-fork-{id}) in the sessions modal, window sidebar, and agent detail; user message action (user-message-fork); SessionForkDialog (session-fork-dialog, session-fork-agent, session-derive-preview, session-derive-preview-native, session-fork-submit, session-fork-submit-error); SessionOriginPill (session-origin-pill); inspector Origin section (ledger-origin, ledger-seed); GET …/derive/preview[?message_id=]; POST …/fork
qa_status: pass
bug_ids: BUG-20260928-message-actions-enabled-during-remote-turn; BUG-20261006-derived-child-runtime-not-selected; BUG-20261006-derived-child-nested-under-source
fix_status: fixed
retest_status: pending
fix_commits: uncommitted (task_08 part B2)
evidence: docs/qa/evidence/2026-09-29-session-continue-fork-r1-rewalk/w-fork-reopen-measuring.png; docs/qa/evidence/2026-09-28-session-continue-fork-b2/journey-log.jsonl; docs/qa/evidence/2026-09-28-session-continue-fork-b2/fork-child-empty.png; docs/qa/evidence/2026-09-28-session-continue-fork-b2/before-fork-enabled-while-running.png; .compozy/tasks/session-continue-fork/evidence/visual/task_06/
last_report: docs/qa/reports/2026-09-29-session-continue-fork-r1-rewalk.md
overlaps: ET-web-session-continue; RT-session-derive-native-fork; RT-conversation-rewind; ET-cli-session-continue; ET-web-sessions-catalog-modal
---

Planning 2026-09-28 (session-continue-fork task_06): new behavior. Walk against a lab daemon with an
agent that answers three turns (acpmock `browser_session_fork_fixture.json` `fork-web-agent`) and,
for the native line, one that advertises `session/fork` + `session/load`
(`session_fork_fixture.json` `fork-native-agent`, or real OpenCode):

1. Prompt three turns. Hover the second user message: "Fork from here" sits before "Rewind to here".
   While a fourth prompt runs, both are disabled.
2. Click "Fork from here": the dialog shows the locked agent and "Carries over N of M messages · X B";
   there is no fork-point field and no Open in choice. Cancel closes without creating anything.
3. Fork session: the same window now shows the child (same window id, no second session window);
   the pill reads "Forked from {source title}" and opens the source; `GET …/transcript` on the source
   reports the same `max_sequence` as before and all three turns remain.
4. From the window overflow → "Fork session…": the line reads "Carries over N messages"; on the native agent
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

Automated evidence 2026-09-28 (session-continue-fork task_08, part A; not a walk verdict): E2E-002 in
`web/e2e/__tests__/session-derive.spec.ts` passes against the daemon-served e2e fixture (focused
Playwright run). Storybook: every `session-fork-dialog.stories.tsx` story renders from a static build
with no page error. The walk (steps 1–8) remains with task_08.

QA walk 2026-09-28 (task_08 part B2): steps 1–8 walked on `fork-web-agent`/`fork-native-agent`. Step 1: Fork from here sits before Rewind. While an API-started turn runs, both stayed enabled (BUG-20260928-message-actions-enabled-during-remote-turn); after the fix both are disabled. Step 2: "Through “Second step”", "Carries over 4 of 6 messages · 636 B"; Change closes with nothing created. Step 3: child `sess-49e0cef5531c6e1a` in a new window, pill "Forked from Ledger migration steps", source `max_sequence` 16 unchanged. Step 4: "Whole session"; the native line shows only on an idle bound native source (a stopped one reports `native_fork_possible: false`). Step 5: unsettled cut via a fulfilled preview → "That turn hasn't settled yet.", Fork session disabled. Step 6: CLI prompt with the dialog open → "Transcript changed — reopen…", Fork disabled, Cancel reads Close, no child. Step 7: catalog row Fork opens over the catalog, and the child opens in its own window. Step 8: stopped fork child inspector Origin "fork · through <msg id>", Seed "replay". Bundles VC-04, 05, 11..15, 17, 23 PASS. Verdict: pass (after fix). Report: `docs/qa/reports/2026-09-28-session-continue-fork-exec-b2.md`.

Review round 1 2026-09-28 (W3, web; retest owed on the next walk): closing a measured Fork dialog and reopening it while the source changed keeps the context line on "Measuring…" and Fork session disabled until this open's preview answers, so the obsolete fences of the previous open are never submitted. Regression: `session-fork-dialog.test.tsx` "Should keep a reopened dialog measuring until its own preview answers" (failed before the fix with `data-state="ready"`).

## 2026-09-29 re-walk (review round 1) — PASS

Bruno, lab `…-r1-rewalk-…`. "Fork from here" on "Second step" measured "Carries over 4 of 6 messages · 636 B", then Cancel. The source was prompted from the CLI (`max_sequence` 18) and the dialog reopened with the preview held. It read `Measuring…` with Fork session disabled; after release it read "Carries over 4 of 8 messages". The fork POST carried the fresh fences (`expected_max_sequence: 18`) and created the child. The source kept `max_sequence` 18 and 10 entries. The busy gate (Fork/Rewind disabled while an API-started turn runs, enabled after `prompt-cancel`) is recorded in RT-conversation-rewind. VC-15 (fence conflict) was re-rendered after the shared refusal wrapper change: same layout and copy, bundle unchanged. Evidence: `docs/qa/evidence/2026-09-29-session-continue-fork-r1-rewalk/w-fork-reopen-measuring.png`, `docs/qa/evidence/2026-09-29-session-continue-fork-r1-rewalk/fork-src-before.txt`, `docs/qa/evidence/2026-09-29-session-continue-fork-r1-rewalk/VC-15-implementation.png`.

qa-impact: 2026-09-30 shell rail Q2. At half width the child window keeps its full title: the head's time drops first, then the "Forked from" pill truncates its subject, and the agent name ellipsizes last; the actions never shrink. E2E-002 now asserts the child title is not truncated.

qa-impact: 2026-09-30 shell-rail polish P6 — with the new default `new_window_policy = tab`, Open in › New window now asks for a split explicitly so the child still opens in its own window beside the source (unchanged from the walked behavior; session-derive E2E-002 covers it). Flag only.

Fix 2026-10-06 (BUG-20261006-derived-child-runtime-not-selected; BUG-20261006-derived-child-nested-under-source): a fork child now starts with the
source's runtime as its selected runtime, so its composer shows the source model (not the agent
default) before the first message, and the child is a top-level row in every session list instead of
nesting under its source. Walk: fork a session whose runtime differs from its agent's default, check
the child's composer model and that the source row shows no thread count for it. Reset to untested.
Regressions: `TestForkSession` (selected runtime), `session-hierarchy.test.ts`.

## Deleted root ancestor regression

1. Create and prompt session A, fork A into B, then fork B into C.
2. Delete A while keeping B and C.
3. On C, choose **Fork session** and submit (choose a different agent for Continue).
4. Confirm the child opens successfully and carries C's conversation. Its lineage parent and root
   both identify C; its kind matches the operation and its historical depth is C's depth plus one.
5. Confirm C's stored lineage and transcript remain unchanged. Repeat after restarting the isolated
   lab daemon to cover persisted sources.

The session-manager regression is `TestDeriveLineageReadModel/Should_derive_*_after_the_root_ancestor_was_deleted`.
The inherited-root workspace rejection remains owned by `TestCreateUserSessionRecordsProvenanceLineage`.
The deleted-root UI walk and restart persistence checks passed on 2026-10-21; see
[the targeted QA report](../reports/2026-10-21-issue-883-deleted-root.md).

## 2026-10-29 issue #1024 — simplified dialog, same-window landing

The dialog dropped the Fork point field (and its Change link) and the Open in choice; a fork always
opens in the window it was started from. Walked in an isolated lab (`fork-web-agent`, branch build
served from `web/dist`): "Fork from here" on "Second step" showed only Agent, "Carries over 4 of 6
messages · 636 B", the "From" note, and Cancel / Fork session (light and dark). Submitting retargeted
the source window to the child (same window id, one session window), the child's lineage named the
clicked message, and the source kept `max_sequence` 14. The sidebar row menu → "Fork session…"
("Carries over 6 messages") landed the child in that same window too. Steps 5–8 are unchanged by this
change. Regressions: `session-fork-dialog.test.tsx`, `session-thread.test.tsx`, E2E-002 in
`web/e2e/__tests__/session-derive.spec.ts`. Evidence: the PR for #1024.
