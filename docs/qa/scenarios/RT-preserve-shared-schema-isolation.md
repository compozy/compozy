---
id: RT-preserve-shared-schema-isolation
area: RT
title: Preserve global and memory stream isolation
persona: Ada
journey: J-operate-daemon-schema
expected: Global and memory operations remain usable across restart while status reports two independent version-one streams with distinct digests and no cross-stream interference.
entry_points: compozy workspace list -o json; compozy memory list -o json; GET /api/status over HTTP and UDS; compozy status -o json
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-store-redesign-20260712-144704-069939-lab/qa-artifacts/qa/evidence/fresh-workspace-list.json;/Users/pedronauck/dev/qa-labs/compozy-store-redesign-20260712-144704-069939-lab/qa-artifacts/qa/evidence/fresh-memory-list.json;/Users/pedronauck/dev/qa-labs/compozy-store-redesign-20260712-144704-069939-lab/qa-artifacts/qa/evidence/restart-workspace-list.json;/Users/pedronauck/dev/qa-labs/compozy-store-redesign-20260712-144704-069939-lab/qa-artifacts/qa/evidence/restart-memory-list.json
last_report: docs/qa/reports/2026-07-12-store-redesign.md
overlaps: RT-inspect-schema-streams
---

Store-redesign targeted QA smoke for Safety Invariant 4. Public reads prove both domains remain usable;
the automated runtime/store suites own table-level disjointness.
