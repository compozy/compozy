---
id: LP-agent-authored-review-run
area: LP
title: Start an agent-authored review run without a provider
persona: Bruno
journey: J-08
expected: Starting review-and-fix with task_name and optional reviewer, fixer, and auto_commit inputs launches the reviewer directly and exposes the same provider-free run through Web, CLI, HTTP, UDS, and native-tool surfaces.
entry_points: web review-and-fix run form; compozy loop run --name review-and-fix; POST /loops/review-and-fix/run; compozy__loop_run
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: LP-029;TA-081
---

Task07 2026-07-27: added for the agent-authored review hard cut; flag only, not retested.
