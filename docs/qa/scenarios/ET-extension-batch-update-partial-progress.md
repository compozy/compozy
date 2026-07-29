---
id: ET-extension-batch-update-partial-progress
area: ET
title: Preserve per-extension progress when a batch update partially fails
persona: Bruno
journey: J-extension-distribution
expected: One `extension update --all` request updates every eligible extension it can, returns ordered structured outcomes for both successes and failures, and preserves completed updates when another source is unavailable or one artifact fails verification.
entry_points: `compozy extension update --all`; `POST /api/extensions/update`; `compozy__extensions_update`
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-019; ET-023
---

Added by ext-improvs Task 05. Planning flag only; no QA session ran.

Seed at least two extensions with distinct source outcomes. Assert HTTP and UDS return the same ordered
`updated` and `failed` items from one daemon batch call, with stable redacted diagnostics for failures.
Repeat in check-only mode and verify it mutates neither registry rows nor runtime activation state.
