---
id: RT-migrate-memory-stream-when-disabled
area: RT
title: Migrate the shared memory stream while the memory runtime is disabled
persona: Ada
journey: J-operate-daemon-schema
expected: With memory.enabled=false on a fresh home, boot reaches readiness, global and memory schema_streams both report their embedded heads, and memory prompt, recall, and native-tool behavior remains disabled.
entry_points: config.toml memory.enabled=false; agh daemon start; agh status -o json; GET /api/status over HTTP and UDS
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report: docs/qa/reports/2026-07-12-store-redesign.md
overlaps: RT-inspect-schema-streams;MS-011
---

Peer-review round 5 added the mandatory shared-file branch: schema durability is independent of the optional memory
runtime. The next QA cycle must prove structured status parity and the absence of memory runtime behavior together.
