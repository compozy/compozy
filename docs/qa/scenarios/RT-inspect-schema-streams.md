---
id: RT-inspect-schema-streams
area: RT
title: Inspect daemon schema streams across structured surfaces
persona: Ada
journey: J-operate-daemon-schema
expected: HTTP, UDS, and CLI JSON return deep-equal global and memory entries with stream, version, applied count, and schema digest.
entry_points: GET /api/status over HTTP; GET /api/status over UDS; agh status -o json
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/agh-store-redesign-20260712-144704-069939-lab/qa-artifacts/qa/evidence/fresh-status-cli.json;/Users/pedronauck/dev/qa-labs/agh-store-redesign-20260712-144704-069939-lab/qa-artifacts/qa/evidence/fresh-status-http.json;/Users/pedronauck/dev/qa-labs/agh-store-redesign-20260712-144704-069939-lab/qa-artifacts/qa/evidence/fresh-status-uds.json;/Users/pedronauck/dev/qa-labs/agh-store-redesign-20260712-144704-069939-lab/qa-artifacts/qa/evidence/session-summary.md
last_report: docs/qa/reports/2026-07-12-store-redesign.md
overlaps: RT-inspect-schema-streams;RT-001
---

Store-redesign QA 2026-07-12: passed. HTTP, UDS, and CLI returned the same ordered global/memory stream payload
before and after daemon restart; the normalized SHA-256 remained
`9894beca2acfb7cbda3fb607db87aa250c327173a348876c30ab3bdacb9205cf`.
