---
id: RT-bounded-runtime-identity
area: RT
title: Keep desktop liveness independent from full status
persona: Ada
journey: J-operate-daemon-schema
expected: Repeated HTTP and UDS `GET /api/status/identity` reads return only the schema and daemon identity without delaying or changing the complete `GET /api/status` snapshot.
entry_points: GET /api/status/identity over HTTP; GET /api/status/identity over UDS; GET /api/status
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-bounded-runtime-identity-electron-20260817-173922-430506-lab/qa-artifacts/qa/logs/identity-http.json;/Users/pedronauck/dev/qa-labs/compozy-bounded-runtime-identity-electron-20260817-173922-430506-lab/qa-artifacts/qa/logs/identity-uds.json;/Users/pedronauck/dev/qa-labs/compozy-bounded-runtime-identity-electron-20260817-173922-430506-lab/qa-artifacts/qa/logs/identity-bursts.txt;/Users/pedronauck/dev/qa-labs/compozy-bounded-runtime-identity-electron-20260817-173922-430506-lab/qa-artifacts/qa/logs/desktop-probe-regression.log;/Users/pedronauck/dev/qa-labs/compozy-bounded-runtime-identity-electron-20260817-173922-430506-lab/qa-artifacts/qa/logs/identity-http-after-restart.json
last_report: docs/qa/reports/2026-08-17-bounded-runtime-identity-electron.md
overlaps: RT-001
---

Issue #413 impact flag: the native desktop shell now binds and monitors the daemon through the bounded identity surface instead of polling the complete runtime status aggregate.

Electron cutover impact 2026-08-17: reset because the desktop liveness transport moved from Rust to Electron; the bounded public endpoint remains unchanged.

Electron cutover replay 2026-08-17 passed: HTTP and UDS returned byte-identical bounded identity bodies, 250 reads per transport completed without failure, the full status projection stayed byte-identical before and after the burst, the Electron monitor regression passed, and a public daemon restart restored the same schema/build contract under a new PID.

Taxonomy: the journey covers the repeated-read happy path, HTTP/UDS consistency, malformed route handling, and the adjacent complete-status canary. Rendered UI, locale, and mobile dimensions are not applicable to this structured liveness contract.
