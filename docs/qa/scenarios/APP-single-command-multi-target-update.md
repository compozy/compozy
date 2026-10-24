---
id: APP-single-command-multi-target-update
area: APP
title: Update runtime and app through one command
persona: Ada
journey: J-desktop-agent-headless
expected: One `compozy update -o json` operation uses the app bundle for a desktop-owned runtime when both targets are available. It updates a running app or stages a closed app, then reports observed runtime readiness after app restart. Standalone and explicit runtime-only requests preserve their runtime update path.
entry_points: compozy update -o json; compozy update --check -o json; compozy app status -o json
qa_status: untested
bug_ids:
fix_status: 
retest_status: 
fix_commits: 
evidence: docs/qa/reports/2026-08-17-electron-shell.md
last_report: docs/qa/reports/2026-08-17-electron-shell.md
overlaps: REL-beta-self-update; APP-agent-cli-app-verbs
---

Added 2026-08-16 for the Electron shell update-operation cutover. Task 07 owns the walk against a
mock GitHub release/channel fixture with running-app, closed-app, headless, and managed-runtime branches.

2026-10-21 targeted implementation verification: [update-flow report](../reports/2026-10-21-desktop-update-flow.md). The current full scenario is flagged
`untested`; native fixture and owning-suite results do not replace published-release or
cross-platform acceptance. Prior dated reports retain their historical evidence.
