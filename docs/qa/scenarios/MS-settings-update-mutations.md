---
id: MS-settings-update-mutations
area: MS
title: Apply and cancel an update through settings
persona: Dora
journey: J-desktop-update-moment
expected: One shared settings action applies every eligible target in runtime→app order; one-target and managed/no-action states remain truthful; invalid or legacy target sets return 400; HTTP and UDS expose the same typed result and refusal states.
entry_points: GET /api/settings/update; POST /api/settings/update/apply; POST /api/settings/update/cancel over HTTP and UDS
qa_status: blocked-verify
bug_ids:
fix_status: 
retest_status: 
fix_commits: 
evidence: docs/qa/reports/2026-08-24-eng-144.md
last_report: docs/qa/reports/2026-08-24-eng-144.md
overlaps: MS-034; APP-cancel-dormant-update
---

Added 2026-08-16 for the asynchronous settings update contract. ENG-144 extends the walk to the
single combined action, runtime→app ordering, one-target eligibility, managed/live no-action states,
invalid target-set refusal, and HTTP/UDS parity. The browser walk is blocked in this worktree because
the focused Playwright command is not registered in Turbo and the permitted web E2E lane is broader
than this task; the canonical unit and transport-contract suites remain green.
