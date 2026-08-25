---
id: RT-spawn-ttl-cleanup
area: RT
title: Reap a settled child without timeout noise
persona: Ada
journey: J-15
expected: When a governed child reaches its TTL, a settled prompt is shown as a clean stopped session with the spawn origin and one stopped parent wake, while an in-flight prompt is shown as a timeout with one failure wake and one timeout marker.
entry_points: compozy spawn; POST /api/agent/spawn; compozy__session_spawn; compozy session list/status/events
qa_status: blocked-verify
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: internal/daemon/spawn_reaper_test.go; internal/session/stop_reason_test.go; internal/session/manager_lifecycle_contract_test.go
last_report: docs/qa/reports/2026-08-24-eng-147-ttl-cleanup.md
overlaps: RT-session-spawn-wake; RT-session-done-presence
---

Create a governed child through a structured surface with a short TTL. Walk one child after its
prompt settles with `done` or `end_turn`, and one while its prompt is still active when the reaper
deadline passes. Compare list, detail, status, transcript markers, parent wake reason, lease release,
and lifecycle hook records. Confirm both paths retain `spawn_reaper:ttl_expired`, process teardown,
and exactly-once `spawn.ttl_expired`, `spawn.reaped`, and session stop hooks; only the active-prompt
path may carry a timeout failure, timeout marker, or failed parent wake.

Automated lifecycle evidence covers both classifications and exact-once in-process effects. A
provider-backed CLI/HTTP/UDS walk remains blocked until a human supplies an isolated ACP provider
and confirms the settled and genuinely in-flight prompt paths through public surfaces.
