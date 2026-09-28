---
id: RT-authored-context-lifecycle
area: RT
title: Preserve agent Soul and Heartbeat through managed authoring
persona: Ada
journey: J-manage-agent-authored-context
expected: SOUL.md and HEARTBEAT.md remain authorable, validated, revisioned, scope-isolated, and available to managed sessions; Heartbeat wake decisions retain their policy and audit without creating a Task or bypassing Task claim authority.
entry_points: compozy agent soul; compozy agent heartbeat; compozy session soul refresh; /api/agents/:name/soul and /heartbeat over HTTP and UDS; /api/workspaces/:workspace_id/sessions/:session_id/soul/refresh; compozy__agent_heartbeat_status; compozy__agent_heartbeat_wake; compozy__session_health; compozy hooks list
qa_status: skipped
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: docs/qa/evidence/2026-09-27-pkgs-cleanup-real-runtime; /Users/pedronauck/dev/qa-labs/compozy-pkgs-cleanup-hardcut-20260927-223700-515958-lab/qa-artifacts/qa/journey-log.jsonl
last_report: docs/qa/reports/2026-09-27-pkgs-cleanup-real-runtime.md
overlaps: RT-031; ET-044; TA-agent-knowledge-refresh-on-wake
---

In an isolated lab, create an agent in each of two workspaces and profiles. Read, validate, write,
and independently re-read distinctive SOUL.md and HEARTBEAT.md bodies through CLI, HTTP, and UDS.
Confirm the returned digest, revision, source, and scope agree. A stale expected_digest must refuse
without mutation. Exercise history, rollback, and deletion; package-owned sidecars remain read-only.

Start a managed session and inspect its resolved authored context. Refresh Soul, then read it again
from that session; restarting the daemon must preserve authored bytes and revision history. A foreign
workspace or profile must not expose those bodies or operate on the session.

Read session health and Heartbeat status through the native tools and structured transports. Perform
a dry-run and one eligible manual wake, then read the retained wake audit. An ineligible session
returns a typed refusal. The wake must remain advisory and must not enqueue a Task run or acquire a
Task claim. Inspect the five agent.soul / agent.heartbeat hook declarations and their provenance;
observation hooks cannot replace authored bytes or expand session authority.

Automated owners (distinct from a real walkthrough):

- internal/daemon/authored_context_transport_test.go: TestAuthoredContextTransportParity and TestAuthoredContextCoreRouteBehavior own HTTP/UDS payload and error parity.
- internal/api/core/authored_context_test.go owns registry workspace resolution, profile isolation, package-owned mutation refusal, and foreign-session wake denial.
- internal/cli/authored_context_test.go owns Soul, Heartbeat, and session command routing.
- internal/daemon/heartbeat_wake_runtime_test.go owns scheduler and harness wake integration and shutdown behavior.
- internal/hooks/authored_context_bypass_test.go and matcher_authored_context_test.go own authored-context hook authority and matcher validation.

The 2026-09-27 local walkthrough verified managed authoring and persistence. It does not claim a provider-backed session or eligible wake verdict. Automated suite results remain separate evidence.

## 2026-09-27 bounded real runtime walk

The two local charters passed through a real isolated daemon, production Web, CLI, HTTP, and UDS.
The full scenario remains skipped in this provider-free cycle; the linked report lists exact observed
steps, retained receipts, and excluded upgrade/provider/runtime legs. This is partial scenario coverage,
not a full-scenario pass.
