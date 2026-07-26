---
id: NB-network-availability-toggle
area: NB
title: Disable and re-enable Network without collateral damage
persona: Bruno
journey: J-administer-network-live
expected: Disabling Network rejects new Live participation, cancels and truthfully settles active wakes, preserves conversation and usage data read-only, and leaves Local work unaffected; re-enabling advances availability without replaying or duplicating old wake sources.
entry_points: web /settings/network; PATCH /api/settings/network over HTTP/UDS; agh config set network.enabled; agh config reload; agh network status -o json
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: docs/qa/evidence/2026-07-14-network-changes/ch-network-admin-lifecycle.md
last_report: docs/qa/reports/2026-07-14-network-changes.md
overlaps: NB-001;NB-002;NB-run-bounded-live-collaboration
---

Derived from the administration flow and ADR-001/008. This scenario owns the availability transition itself; settings form mechanics stay with `NB-002`, and wake-level accounting stays with `NB-run-bounded-live-collaboration`.

Execution must compare one active Live owner with a Local control, re-read preserved history and usage after disable/re-enable, and prove the same accepted source cannot wake twice.
