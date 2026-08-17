---
id: APP-single-command-multi-target-update
area: APP
title: Update runtime and app through one command
persona: Ada
journey: J-desktop-agent-headless
expected: One `compozy update -o json` operation applies the runtime first, then updates a running app or stages a closed app, and reports both targets from the same verified release.
entry_points: compozy update -o json; compozy update --check -o json; compozy app status -o json
qa_status: blocked-decision
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
