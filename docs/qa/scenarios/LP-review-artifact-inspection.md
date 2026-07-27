---
id: LP-review-artifact-inspection
area: LP
title: Inspect deterministic review artifacts before remediation
persona: Bruno
journey: J-08
expected: A non-empty reviewer result creates the next exclusive reviews-NNN directory with one deterministic issue file per finding, preserved source fields, and no files outside the authenticated task workspace.
entry_points: workspace .compozy/tasks/<task>/reviews-NNN; loop run status; ext__dev_cycle__write_review_artifacts
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: LP-029;LP-agent-authored-review-run
---

Task07 2026-07-27: added for on-disk artifact inspection; flag only, not retested.
