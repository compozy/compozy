---
id: APP-single-command-multi-target-update
area: APP
title: Update runtime and app through one command
persona: Ada
journey: J-desktop-agent-headless
expected: One `compozy update -o json` operation applies the runtime first, then updates a running app or stages a closed app, and reports both targets from the same verified release.
entry_points: compozy update; compozy update --check -o json; compozy app status -o json
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: REL-beta-self-update; APP-agent-cli-app-verbs
---

Added 2026-08-16 for the Electron shell update-operation cutover. Task_07 owns the walk against a
fixture release with running-app, closed-app, headless, and managed-runtime branches.
