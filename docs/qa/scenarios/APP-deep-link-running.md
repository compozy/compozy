---
id: APP-deep-link-running
area: APP
title: A CompozyOS link opens the right view in the running app
persona: Théo
journey: J-desktop-link-driven
expected: Activating `compozyos://open/<path>` with the app running focuses the window and navigates to that view; a link to a deleted entity shows the product's own not-found view; a malformed or hostile payload lands safely on the default view with no dialog and no off-product navigation.
entry_points: compozyos://open/<product-path> from terminal, docs, or notification while the app runs
qa_status: blocked-verify
bug_ids:
fix_status:
retest_status: blocked-verify
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-desktop-app-release-20260810-110811-513872-lab/qa-artifacts/qa/app-control-product.txt; /Users/pedronauck/dev/qa-labs/compozy-desktop-app-release-20260810-110811-513872-lab/qa-artifacts/qa/platform-capability-blockers.txt
last_report: docs/qa/reports/2026-08-10-desktop-app-release.md
overlaps:
---

PRD stories: US-010 (AC-1 focus+navigate; EC-1 not-found; EC-2 hostile payload safe; EC-3 rapid
links, last wins). BR-15/BR-16. Test IDs: E2E-008; IT-019, IT-020; UT-044–UT-047.

Per-OS evidence (N-004): each OS registers the scheme differently — all three OSes walk one valid
link (focus + correct view capture), one deleted-entity link (product not-found view), and one
hostile payload (`compozyos://open/http://evil.com`-class → default view). macOS scripted-manual;
Windows/Linux scripted via tauri-driver where the harness supports scheme activation, otherwise
manual activation with recorded transcripts.
