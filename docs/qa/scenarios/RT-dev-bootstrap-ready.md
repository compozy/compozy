---
id: RT-dev-bootstrap-ready
area: RT
title: Start the development UI only after daemon readiness
persona: Ada
journey: J-operate-daemon-schema
expected: From a fresh isolated home, make dev completes daemon startup before exposing the Vite UI, so the first status, workspace, task, and stream requests reach the current daemon without connection-refused failures.
entry_points: make dev; http://localhost:<web-port>; GET /api/status
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: RT-001;RT-inspect-schema-streams
---

QA impact 2026-07-24: new developer-supervisor readiness contract. The current Air run publishes one daemon-ready event carrying its built binary identity; stale runs and repeated rebuilds cannot unlock Vite startup. Planning flag only; automated process integration and implementation validation do not settle the real-user tracker verdict.
