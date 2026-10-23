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


Review remediation 2026-10-18: include ACP `terminal/create` commands in the
priority inspection. Agent-owned groups use the configured niceness, while human
and system terminal groups retain inherited priority. Exercise an existing Goal
session through work, continuation, and compaction: each waits before a durable
submission under pressure, and cancellation releases the wait without dispatch.
Enqueue more than three full Goal outbox batches, send one nudge, and verify that
all ready events and session cleanups drain before the next fallback sweep. A
failed acknowledgement or Stop remains durable and is retried on a later wake.
The terminal manager uses real process groups for the priority regression; managed
input and relay suites own cancellation, exact dispatch counts, and batch retry
boundaries. The full multi-session CPU saturation walk above remains unverified.


Hosted MCP fan-out regression: with multiple bound peers in the same profile/workspace and
identical effective policies and native availability, one epoch change builds one shared
projection. Each stream retains its own digest suppression and receives a private copy of
changed tools. Repeat with different lineage grants, agent allowlists (including an explicitly
empty grant), session-specific availability, profiles, and workspaces; views must remain isolated.
Release or cancel a peer during a build and confirm other peers continue. Unversioned state
still refreshes at the existing fallback interval; providers without a declared projection scope
retain per-bind isolation.

Focused evidence: `TestHostedServiceProjectionGenerationCache` covers fan-out, policy and
availability isolation, caller mutation, epoch changes, cancellation, and release;
`TestHostedMCPProjectionStreamGenerationCache` and `TestHostedMCPProjectionStreamPush` exercise
the existing UDS SSE path. `BenchmarkHostedStreamProjectionFanout` compares 1 and 16 streams
with and without sharing. These checks do not certify the CPU-saturation journey above;
its full walk remains unverified because local full/E2E execution is prohibited for this change.
