---
id: RT-preserve-shared-schema-isolation
area: RT
title: Preserve the single global schema stream across restart
persona: Ada
journey: J-operate-daemon-schema
expected: Global operations remain usable across restart while status reports exactly one global stream with its digest, and a home upgraded from a release that had a memory stream reports no memory entry.
entry_points: compozy workspace list -o json; GET /api/status over HTTP and UDS; compozy status -o json
qa_status: untested
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

QA impact 2026-10-07 (memory removal): global migration 00128 drops the memory stream objects and its version table, and the memory stream is unregistered, so the cross-stream isolation invariant this scenario smoked in the 2026-07-12 store-redesign walk no longer has a second stream to isolate. The scenario now owns the single-stream posture: a fresh home and an upgraded home both report one global entry in `schema_streams` over HTTP, UDS, and CLI, unchanged across a daemon restart. Stale verdict reset to untested; historical evidence preserved; no QA session ran. The upgraded-home leg is walked inside RT-upgrade-memory-removal-home.
