---
id: ET-web-session-continue
area: ET
title: Continue a session with another agent from the web
persona: Bruno
journey: J-14
expected: A user session's row menu (sessions modal, window sidebar, agent detail) and its window overflow offer "Continue with another agent…" after "Rename session" (absent for archived, spawned, coordinator, and system rows). The dialog ("Continue with another agent", eyebrow "Operate · Session") preselects the first agent that is not the source's, shows the daemon-measured context line ("Carries over N messages · X KiB", or "Carries over K of N messages · X KiB" plus "M earlier messages omitted to fit the context budget.", "A turn is still in progress; it will not be carried over." for a running source), keeps Continue disabled while measuring and after "Couldn't measure this session's context.", shows a Route select ("Default" + "Route n · provider · model", with an account suffix when two routes collide) only for agents that declare fallback routes and then hides Runtime (route XOR runtime), accepts an optional first message, and defaults Open in to New window. Continue shows "Starting the new session…", creates exactly one child, and opens it in a new window (This window retargets the current window instead; lists outside a session window always open a new window). The child's status line shows the neutral pill "Continued from {source agent}" linking to the source; its transcript starts with the hairline divider "Continued from {source title}" (alone above "Nothing said here yet" before the first message); the inspector shows Origin "continue · from {agent}" and Seed "replay". The source window and transcript are unchanged. A rate-limited or unauthenticated turn whose next step is handoff shows "Continue this session with another agent or route." with a "Continue with another agent…" button that opens the same dialog for that session; nothing is created until Continue. The dead-runtime banner reads "Restart in a new session" and creates a recovery child. No surface in this feature is danger-toned except refusal text.
entry_points: web session window overflow (continue-menu-item); session row overflow (session-row-continue-{id}) in the sessions modal, window sidebar, and agent detail; provider-error marker (provider-error-continue); SessionContinueDialog (session-continue-dialog, session-derive-preview, session-continue-route-select, session-derive-placement, session-continue-submit); SessionOriginPill (session-origin-pill); SessionContinueDivider (session-origin-divider); inspector Origin section (ledger-origin, ledger-seed); dead-runtime banner; GET …/derive/preview; POST …/continue
qa_status: pass
bug_ids: BUG-20260928-agent-resource-drops-fallback-chain; BUG-20260928-derive-budget-ignores-workspace-overlay; BUG-20260928-derive-retry-creates-second-child; BUG-20260928-derived-child-empty-state-hidden
fix_status: fixed
retest_status: pass
fix_commits: uncommitted (task_08 part B2)
evidence: docs/qa/evidence/2026-09-28-session-continue-fork-b2/journey-log.jsonl; docs/qa/evidence/2026-09-28-session-continue-fork-b2/continue-route-child-window.png; docs/qa/evidence/2026-09-28-session-continue-fork-b2/dead-runtime-banner.png; .compozy/tasks/session-continue-fork/evidence/visual/task_04/
last_report: docs/qa/reports/2026-09-28-session-continue-fork-exec-b2.md
overlaps: ET-cli-session-continue; RT-session-derive-retry; RT-provider-error-handoff; ET-web-session-sidebar-threads; ET-web-sessions-catalog-modal; ET-web-session-context-sidebar
---

Planning 2026-09-28 (session-continue-fork task_04): new behavior. Walk against a lab daemon with two
agents (acpmock `multi_agent_fixture.json` alpha/beta, or real Codex + Claude) and, for the route
row, an agent whose `fallback_chain` declares two routes on the same provider/model with different
commands:

1. Prompt a session on the first agent. Open its window → overflow → "Continue with another agent…".
   The dialog preselects the other agent; the context line reads "Measuring…" then
   "Carries over N messages · X KiB"; Continue enables only then.
2. Choose the routed agent: the Route select lists "Default" and "Route 1/2 · provider · model",
   the colliding route carrying "· abcd…"; choosing a route hides Runtime.
3. Continue with a first message and New window: a second window opens on the child; the pill reads
   "Continued from {source agent}" and opens the source; the divider sits above the child's first
   message; the source window still shows its turns and `GET …/transcript` on it reports the same
   `max_sequence` as before.
4. Continue again with This window: the current window retargets to the new child; no extra window.
5. Continue without a message: the child shows the divider alone above "Nothing said here yet".
6. From the sessions modal row menu and agent detail row menu: the item is present for user rows,
   absent for archived and spawned rows; the child always opens in its own window.
7. Rate-limit a user session's turn (RT-provider-error-handoff): the marker offers
   "Continue with another agent…"; the dialog opens for that session; `GET /api/sessions` lists no new
   session until Continue.
8. A dead runtime shows "Restart in a new session"; it creates a child with lineage kind `recovery`.
9. Stop a continued child and open its inspector: Origin "continue · from {agent}", Seed "replay".

Automated evidence at authoring time: `session-continue-dialog.test.tsx`, `use-session-derive.test.ts`,
`runtime-activity-notice.test.tsx`, `session-status-line.test.tsx`, `session-thread.test.tsx`,
`session-inspector.test.tsx`, `sessions-modal.test.tsx`, `use-session-topbar-slot.test.tsx`,
`session-window-content.test.tsx`. Web E2E `web/e2e/__tests__/session-derive.spec.ts` (E2E-001 written,
E2E-004 `fixme` until acpmock can rate-limit a prompt) — not run by task_04. task_08 owns the walk,
the E2E run, and the visual-contract bundles (VC-01..03, VC-06..10, VC-16, VC-18..22).

Automated evidence 2026-09-28 (session-continue-fork task_08, part A; not a walk verdict): E2E-001 and
E2E-004 in `web/e2e/__tests__/session-derive.spec.ts` pass against the daemon-served e2e fixture
(focused Playwright run with the lane's `COMPOZY_TEST_DAEMON_BIN`/`COMPOZY_TEST_ACPMOCK_DRIVER_BIN`/
`COMPOZY_WEB_DIST_DIR`). E2E-004 is no longer `fixme`: it uses `handoff-agent` from
`internal/testutil/acpmock/testdata/provider_error_fixture.json`, whose `fail_prompt` driver_control
step fails the second prompt with "429 rate limit exceeded". Storybook: every
`session-continue-dialog.stories.tsx` and `session-origin.stories.tsx` story renders from a static
build with no page error. The walk (steps 1–9) remains with task_08.

QA walk 2026-09-28 (task_08 part B2, lab daemon + headless Chromium, acpmock): steps 1–9 walked. Step 1: overflow → dialog, beta preselected, preview measured, Continue enabled. Step 2: route-agent Route 1/2 with `· 541a…`/`· 4dc4…` suffixes; choosing a route hides Runtime. Before the fix, submitting any route failed with `route_not_found` (BUG-20260928-agent-resource-drops-fallback-chain). Step 3: New window + first message → child `sess-b946f53737c279ea`, pill "Continued from alpha" focuses the source, divider above the first message, source `max_sequence` 6 unchanged. Step 4: This window retargets the window in place. Step 5: no message → divider + "Nothing said here yet" (only after BUG-20260928-derived-child-empty-state-hidden). Step 6: sessions modal and agent-detail row menus offer both verbs on user rows, the archived row has neither, Open in is hidden, and the child opens in its own window. Step 7: see RT-provider-error-handoff. Step 8: "Restart in a new session" → `lineage.kind: recovery`. Step 9: inspector Origin/Seed read correctly. Found and fixed along the way: the workspace overlay budget was ignored (BUG-20260928-derive-budget-ignores-workspace-overlay), and a retry after a post-commit 422 duplicated the child (BUG-20260928-derive-retry-creates-second-child). Visual bundles VC-01..03, 06..10, 16, 18..22 PASS. Verdict: pass (after fixes). Report: `docs/qa/reports/2026-09-28-session-continue-fork-exec-b2.md`.
