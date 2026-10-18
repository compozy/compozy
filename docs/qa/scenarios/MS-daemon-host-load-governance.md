---
id: MS-daemon-host-load-governance
area: MS
title: Keep the host responsive while many agent sessions run
persona: Dora
journey: J-drain-daemon-safely
expected: Agent ACP process groups and every tool they spawn run at `daemon.load.agent_niceness` (default 10, `ps -o nice` shows it on the agent tree); with normalized 1-minute load above `daemon.load.admission_threshold`, scheduled/trigger automation runs, Loop sessions/prompts, and scheduler wakes wait up to `daemon.load.admission_max_wait` while user prompts, user-created sessions, subagent spawns, and manual automation runs start immediately; `compozy doctor -o json` reports the `runtime.load` item (pressured as warning); threshold `0` disables sampling and niceness `0` keeps normal priority. An idle daemon with bound hosted-MCP clients stays near zero CPU (no 100ms projection polling, no 30s model-catalog full hydration for fresh sources).
entry_points: config.toml [daemon.load]; compozy config get|set daemon.load.*; HTTP/UDS GET /api/doctor; compozy doctor -o json; ps/top on the agent process tree
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: MS-daemon-memory-reporting
---

Added 2026-10-18 for branch `perf/daemon-load-mitigations`. The walk starts ≥5 managed sessions in an
isolated home, saturates the host (e.g. parallel `yes > /dev/null`), and proves the niceness of each
agent tree, the background-only admission hold (a scheduled automation waits, a user prompt does not),
the doctor item, both disable switches after restart, and idle daemon CPU with sessions bound.
