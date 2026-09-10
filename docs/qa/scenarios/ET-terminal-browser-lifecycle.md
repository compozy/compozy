---
id: ET-terminal-browser-lifecycle
area: ET
title: Use persistent terminals from the browser
persona: Marina
journey: J-operate-integrated-terminal
expected: Reload preserves terminal tabs and their processes. Window Close confirms running terminal termination before removing the window; Cancel preserves both; Stop leaves the window open; exited or missing terminals close directly; failures retain retryable windows. Group close confirms exactly its affected terminals and normal history remains retained.
entry_points: Web dock Terminal app; /terminal; /terminal/{terminal_id}
qa_status: pass
bug_ids: BUG-20260906-hidden-terminal-pane-minimum-vote
fix_status: fixed
retest_status:
fix_commits:
evidence: .compozy/tasks/sessions-stability/memory/terminal-retention-ci.md; /Users/pedronauck/dev/qa-labs/compozy-integrated-terminal-review-r2-20260902-020216-937662-lab/qa-artifacts/qa/screenshots/bruno-two-terminals-after-reload.png; /Users/pedronauck/dev/qa-labs/compozy-terminal-shared-control-20260904-204013-041114-lab/qa-artifacts/window-canary-after-reload.png; docs/qa/reports/2026-09-04-terminal-shared-control.md
last_report: docs/qa/reports/2026-09-04-terminal-shared-control.md
overlaps:
---

QA impact 2026-09-04: the Terminal app removed controller badges/actions and now accepts input as soon
as its interactive stream connects. Reset for a focused window lifecycle and shared-input re-walk.

qa-impact: 2026-09-01 deep-review round 2 changed terminal route retargeting, reconnect settlement,
window close handling, and browser recovery ownership. Reset for a focused public-surface re-walk.

2026-09-01 re-walk: passed. Bruno opened two browser terminals, executed a real shell command,
reloaded with both tabs intact, closed the window without ending either process, reopened the newest
running terminal from the dock, and closed an already-ended terminal without an error notification.
Independent `compozy terminal get|list` reads matched the rendered state throughout.

Flagged by integrated-terminal task 06; reset 2026-08-31 by the window-native
UX rework (internal tab strip removed; id-less route resolves itself; close is
idempotent). The prior pass predates that surface.

Walk:

1. Click the Terminal dock item in a project with no terminals; a working terminal opens with no launcher or empty-state click in between.
2. Use the head's New terminal; a second terminal joins the frame as an OS window tab; switch between both deck tabs and reload the browser.
3. Close a running Terminal window. Cancel the accessible confirmation and verify its process and window remain intact through CLI and refresh.
4. Confirm Close; verify the process exits before the managed window disappears. Reopen the browser and confirm it stays closed; inspect retained journal/recording history through the normal surfaces.
5. Stop a running terminal from its header; verify the window remains on its exit state, then close it without another running warning.
6. Close a group containing two terminals and an unrelated app; verify one confirmation lists only the affected running terminals. Cancel preserves the group; confirm closes the group without terminating terminals outside it.
7. Exercise the keyboard/window menu and close-other/right gestures. Pinned tabs remain protected by their existing scope rules.
8. Lose the connection while closing; verify visible failure feedback, a retained window, and a successful fresh retry after reconnect.
9. Confirm the route, title, dock badge, and another attached viewer remain truthful throughout.
10. Hold a terminal creation response, switch workspace or profile, and release it. The original
    owner's catalog retains the terminal; the destination desktop is not retargeted. Returning to
    the original scope must not revive the obsolete navigation. Its initiating tab offers
    **Open terminal**; activating it shows the process already created without adding another
    terminal. A creation without a scope change still opens its terminal normally.

QA impact 2026-09-10: issue #594 intentionally replaces the old operator view-only close behavior. Historical passes below describe the previous contract; this round is tracked in `docs/qa/reports/2026-09-10-issue-594-terminal-close.md`.

2026-09-04 targeted re-walk: passed. The Terminal app opened a second terminal, switched between OS
window tabs, survived minimize and reload with both instances restored, and closed one window without
ending the original terminal. A new browser session reattached to the running terminal with shared input.

QA re-walk 2026-09-10: PASS for the changed close contract. Production-bundle E2Es cover running cancel/confirm, grouped reload/history, disconnect feedback, Stop, and exited close. Manual isolated-browser checks cover keyboard/window-menu close, mixed-app groups, close-other/right targeting, shared viewers, and unchanged native view-only close. Scope and evidence: `docs/qa/reports/2026-09-10-issue-594-terminal-close.md`. Unchanged steps retain their earlier evidence.

Post-merge creation-scope verification is recorded in
`docs/qa/reports/2026-09-10-merged-pr-ci-review-remediation.md`; earlier browser evidence
does not independently prove the newly added pending-response step.
