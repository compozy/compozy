---
id: ET-palette-sessions-view-switch
area: ET
title: Switch sessions from the palette Sessions view
persona: Bruno
journey: J-operate-desktop-shell
expected: ⌘E opens the palette already inside the Sessions view, and the root Views entry pushes the same view. Sessions list attention-first with their exact state word; typing narrows by title or agent; the needs-you / working / finished / idle chips narrow by state class with truthful counts, and a chip that matches nothing names its filter and clears with one Backspace. The globe toggle widens the list through the operator's persisted session-list breadth — the sessions sidebar follows, and `compozy config get shell.sessions.scope` reports the same value. Enter focuses the session window, restoring it when it was closed and switching workspace first when the session is foreign; landing on a done session clears its finished marker.
entry_points: web desktop keyboard; ⌘E; command palette Views group; sessions sidebar globe; compozy config get/set shell.sessions.scope
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: docs/qa/reports/2026-08-16-herdr-parity.md; .compozy/tasks/herdr-parity/evidence/visual/task_06
last_report: docs/qa/reports/2026-08-16-herdr-parity.md
overlaps: ET-web-command-palette-shortcuts; ET-web-sessions-catalog-modal; ET-keyboard-navigation-actions
---

story: As an operator with sessions spread across workspaces, I can filter to the ones that need me and land on one in two keystrokes, from wherever I already am.

2026-08-16 qa-impact: The Sessions view shipped as the first registered palette view (US-030/US-031).
It consumes the shared badge dictionary, attention-first ordering, and the shared land-on-session
path rather than deriving its own. Show all is the existing `shell.sessions.scope` setting, so the
walk must confirm the round trip across the palette, the sidebar, and the CLI. Rows render bounded
with an honest "showing N of M" note — confirm the note appears only when matches exceed the bound.

QA 2026-08-16 Herdr parity: The full Web E2E and inspected visual bundles covered push/pop depth, collapsed breadcrumbs, backspace and Escape semantics, reopen-at-root, Sessions filters, empty state, zero match, and one-keystroke clear.

QA impact 2026-08-17: The labelled "All workspaces" pill became the same icon-only globe the
sessions sidebar uses (`aria-pressed`, accessible name "All workspaces"). Off now writes
`workspace`, not `all`. Re-walk the round trip across the palette, the sidebar, and the CLI.

Walk this cycle: blocked-verify — the web unit suites (5211 passing) and the rewritten Playwright
legs cover the globe's pressed states, the daemon round trip, and workspace-group isolation, but an
isolated QA lab with a live daemon was not started, so a persona walk through public entry points
could not meet the qa-execution evidence standard.

2026-08-19 qa-impact: Sessions was re-registered through the generalized domain-view registry, and
the same grammar now serves every list-bearing domain. Re-walk Sessions selection survival,
attention order, state filters, truthful counts, zero-match clearing, and scope persistence.

2026-08-20 qa-impact: Root fallback assembly and default-agent session creation now share the
session landing path. Keep this scenario `untested`; task 12 owns the isolated re-walk.
