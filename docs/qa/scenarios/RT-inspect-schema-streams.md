---
id: RT-inspect-schema-streams
area: RT
title: Inspect daemon schema streams across structured surfaces
persona: Ada
journey: J-operate-daemon-schema
expected: HTTP, UDS, and CLI JSON return a deep-equal `daemon.schema_streams` array in the status payload whose single entry is the global stream with `stream`, `version`, `applied_count`, and `sum_digest`.
entry_points: GET /api/status over HTTP; GET /api/status over UDS; compozy status -o json
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-store-redesign-20260712-144704-069939-lab/qa-artifacts/qa/evidence/fresh-status-cli.json;/Users/pedronauck/dev/qa-labs/compozy-store-redesign-20260712-144704-069939-lab/qa-artifacts/qa/evidence/fresh-status-http.json;/Users/pedronauck/dev/qa-labs/compozy-store-redesign-20260712-144704-069939-lab/qa-artifacts/qa/evidence/fresh-status-uds.json;/Users/pedronauck/dev/qa-labs/compozy-store-redesign-20260712-144704-069939-lab/qa-artifacts/qa/evidence/session-summary.md
last_report: docs/qa/reports/2026-07-12-store-redesign.md
overlaps: RT-inspect-schema-streams; RT-001
---

Store-redesign QA 2026-07-12: passed. HTTP, UDS, and CLI returned the same ordered global/memory stream payload
before and after daemon restart; the normalized SHA-256 remained
`9894beca2acfb7cbda3fb607db87aa250c327173a348876c30ab3bdacb9205cf`.

QA impact 2026-10-07 (memory removal): the memory stream is unregistered by global migration 00128, so `daemon.schema_streams` carries one global entry (the 2026-07-12 walk compared two); a fresh home reports `{stream: "global", version: 128, applied_count: 128, sum_digest}` identically over HTTP, UDS, and CLI. `compozy status` also reports the top-level `schema_version` `2026-10-07` and has no `memory` object. Stale verdict reset to untested; historical evidence preserved; no QA session ran.
