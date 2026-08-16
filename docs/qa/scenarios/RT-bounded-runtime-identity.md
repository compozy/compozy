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
evidence: /Users/pedronauck/dev/qa-labs/compozy-issue-413-bounded-liveness-20260816-055839-132323-lab/qa-artifacts/qa/logs/identity-http.json;/Users/pedronauck/dev/qa-labs/compozy-issue-413-bounded-liveness-20260816-055839-132323-lab/qa-artifacts/qa/logs/identity-uds.json;/Users/pedronauck/dev/qa-labs/compozy-issue-413-bounded-liveness-20260816-055839-132323-lab/qa-artifacts/qa/logs/identity-http-burst.tsv;/Users/pedronauck/dev/qa-labs/compozy-issue-413-bounded-liveness-20260816-055839-132323-lab/qa-artifacts/qa/logs/identity-uds-burst.tsv;/Users/pedronauck/dev/qa-labs/compozy-issue-413-bounded-liveness-20260816-055839-132323-lab/qa-artifacts/qa/logs/desktop-probe-regression.log
last_report: docs/qa/reports/2026-08-16-issue-413-bounded-liveness.md
overlaps: RT-001
---

Issue #413 impact flag: the native desktop shell now binds and monitors the daemon through the bounded identity surface instead of polling the complete runtime status aggregate.

Taxonomy: the journey covers the repeated-read happy path, HTTP/UDS consistency, malformed route handling, and the adjacent complete-status canary. Rendered UI, locale, and mobile dimensions are not applicable to this structured liveness contract.
