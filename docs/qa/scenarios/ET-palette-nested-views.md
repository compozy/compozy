---
id: ET-palette-nested-views
area: ET
title: Navigate nested command palette views
persona: Bruno
journey: J-operate-desktop-shell
expected: A root palette entry under Views pushes that view — the query clears, a breadcrumb names the path, and only the view's own results list. Backspace edits the query while it has text and pops exactly one level once it is empty. Escape closes the whole palette regardless of depth, and reopening starts at the root with no stale path. A pushed view with nothing to list says so in place instead of falling back to root results.
entry_points: web desktop keyboard; command palette Views group; ⌘E
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: docs/qa/reports/2026-08-16-herdr-parity.md; .compozy/tasks/herdr-parity/evidence/visual/task_06
last_report: docs/qa/reports/2026-08-16-herdr-parity.md
overlaps: ET-web-command-palette-shortcuts; ET-web-sessions-catalog-modal
---

story: As a keyboard operator, I can enter a scoped picker inside the palette and get back out one level at a time, without the palette ever showing me results from a level I am not on.

2026-08-16 qa-impact: The command palette gained a generic nested-view mechanism (ADR-003): a view
stack in the window-manager store, a left-truncating breadcrumb, backspace-on-empty pop, and
dismiss-closes-the-stack. Built-in views only in v1. Walk the depth, pop order, and reopen-at-root
behaviour, plus the destination picker (new tab → ⌘K) staying unchanged.

QA 2026-08-16 Herdr parity: The full Web E2E and inspected visual bundles covered push/pop depth, collapsed breadcrumbs, backspace and Escape semantics, reopen-at-root, Sessions filters, empty state, zero match, and one-keystroke clear.

2026-08-19 qa-impact: The shared stack now hosts List, Detail, Form, and Grid views under the same
keyboard and breadcrumb contract. Re-walk every kind, the unavailable and timeout frames, pop
order, Escape dismissal, and reopen-at-root behavior.

2026-08-20 qa-impact: Programmable-view lifecycle events and effect-failure correlation now close
the shared stack's observability contract. Keep this scenario `untested`; task 12 owns the re-walk.
