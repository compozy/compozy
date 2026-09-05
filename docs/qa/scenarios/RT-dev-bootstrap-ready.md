---
id: RT-dev-bootstrap-ready
area: RT
title: Start the development UI only after daemon readiness
persona: Ada
journey: J-operate-daemon-schema
expected: From a fresh isolated home, make dev completes daemon startup before exposing the Vite UI, so the first status, workspace, task, and stream requests reach the current daemon without connection-refused failures.
entry_points: make dev; http://localhost:<web-port>; GET /api/status
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: .compozy/tasks/tests-cleanup/performance/dev-optimized-events.json; .compozy/tasks/tests-cleanup/performance/dev-optimized-browser.txt; .compozy/tasks/tests-cleanup/performance/dev-browser-api-refresh.log; .compozy/tasks/tests-cleanup/performance/dev-build-recovery.json
last_report: docs/qa/reports/2026-09-05-dev-performance.md
overlaps: RT-001;RT-inspect-schema-streams
---

QA impact 2026-07-24: new developer-supervisor readiness contract. The current Air run publishes one daemon-ready event carrying its built binary identity; stale runs and repeated rebuilds cannot unlock Vite startup. Planning flag only; automated process integration and implementation validation do not settle the real-user tracker verdict.

QA 2026-09-05: the isolated development supervisor reached Vite in 5.488 seconds with current generation evidence and unchanged build inputs. Daemon readiness preceded config resolution and Vite. The real browser reached onboarding and survived refresh; status, workspaces, tasks, and stream requests reached the isolated daemon. An invalid Go edit preserved HTTP 200 from the current daemon; restoring the source rebuilt successfully without restarting identical bytes. Final gate and teardown evidence are recorded in the linked report.
