---
id: ET-model-source-five-rate-pricing
area: ET
title: Extension model source preserves five-rate pricing
persona: Ada
journey: J-20
expected: An extension model.source row with distinct input, output, cache-read, cache-write, and reasoning rates validates, persists across daemon reopen, and returns unchanged through Host API models/list; negative or non-finite rates fail the source and method/capability IDs remain unchanged.
entry_points: extension model.source models/list; Host API models/list; model catalog readback
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: MS-042;MS-055;MS-056
---

Register a local extension with `model.source` and `model.read`. Return one model row whose five rates
are distinct, refresh it into the catalog, restart AGH, and compare native catalog and Host API
`models/list` payloads. Then return a negative or non-finite rate and require a failed, redacted source
status without corrupting the last good row. Confirm `model.source`, `models/list`, `model.read`, and
`model.write` identifiers and grants did not change.
