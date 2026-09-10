---
id: APP-deep-link-running
area: APP
title: A CompozyOS link opens the right view in the running app
persona: Théo
journey: J-desktop-link-driven
expected: Activating `compozyos://open/<path>` with the app running focuses the window and navigates to that view; a link to a deleted entity shows the product's own not-found view; a malformed or hostile payload lands safely on the default view with no dialog and no off-product navigation.
entry_points: compozyos://open/<product-path> from terminal, docs, or notification while the app runs
qa_status: pass
bug_ids:
fix_status: 
retest_status: 
fix_commits: 
evidence: docs/qa/reports/2026-08-17-electron-shell.md
last_report: docs/qa/reports/2026-08-17-electron-shell.md
overlaps:
---

PRD stories: US-010 (AC-1 focus+navigate; EC-1 not-found; EC-2 hostile payload safe; EC-3 rapid
links, last wins). BR-15/BR-16. Test IDs: E2E-008; IT-019, IT-020; UT-044–UT-047.

Per-OS evidence: macOS and Linux each walk one valid
link (focus + correct view capture), one deleted-entity link (product not-found view), and one
hostile payload (`compozyos://open/http://evil.com`-class → default view), scripted through
Playwright `_electron` where the harness supports scheme activation and otherwise manually with
recorded transcripts.

QA 2026-09-10 follow-up: The shell serializes document loads and retains the last pending deep link. Packaged E2E-004 passed three repetitions on macOS; current-head Linux verification remains required. See [release integration recovery](../reports/2026-09-10-release-integration-repair.md).
