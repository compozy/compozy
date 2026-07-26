---
id: NB-agent-manages-participation
area: NB
title: Manage participation through structured agent surfaces
persona: Ada
journey: J-run-bounded-live-collaboration
expected: CLI, HTTP, UDS, and native tools expose the same immutable mode, source, channel, finite bounds, consumption, and actual-or-unavailable usage; invalid or unauthorized requests return stable named diagnostics without partial execution state.
entry_points: agh session/task/loop/network commands -o json; HTTP/UDS execution create/start and Network status/usage routes; agh__network_* and owner native tools; GET /api/agent/context
qa_status: blocked-verify
bug_ids: BUG-20260715-network-usage-workspace-name-empty;BUG-20260715-taskless-network-wake-run-unreadable
fix_status: fixed
retest_status: pass
fix_commits: pending local diff
evidence: docs/qa/evidence/2026-07-14-network-changes/ch-live-bounds-agent-path.md;/Users/pedronauck/dev/qa-labs/agh-network-live-bounds-20260715-061317-610983-lab/qa-artifacts/qa/qa-audit-report.json
last_report: docs/qa/reports/2026-07-14-network-changes.md
overlaps: NB-execution-participation-defaults;NB-participation-controls-serialize;RT-031;TA-049
---

Derived from US-019 and the Live flow. The structured surfaces must agree on `network_participation_unavailable`, `not_participating`, `loop_requires_live`, unknown channel, unsupported Live, authority denial, and invalid-combination behavior.

A Local agent must receive an explicit `not_participating` explanation rather than fictional Network context or silently missing controls; a child cannot widen beyond delegated authority.

QA 2026-07-15: CLI, HTTP, UDS, native-tool discovery/invocation, and agent context agreed for Local/Live status, immutable bounds, usage, unknown-channel, invalid-combination, ceiling, disabled-availability, and `not_participating` behavior after two public-read fixes. The strict charter audit still lacks a real-provider agent walk of `loop_requires_live`, unsupported-provider, and delegated-authority denial, so the complete scenario remains `blocked-verify` rather than inheriting automated-only evidence.
