---
id: NB-coordination-invitation-future-runs
area: NB
title: Coordination invitation accepts for future runs only
persona: Bruno
journey: J-enable-coordinated-conversations
expected: On an active multi-agent run with coordination off and Network available, the invitation is visible, states that acceptance does not change the active run, accept enables workspace coordination for future runs, and dismiss persists via daemon invitation GET across reload.
entry_points: web task run detail and kanban invitation; GET/PUT /api/workspaces/:id/network-coordination over HTTP/UDS; PUT /api/workspaces/:id/network-coordination/invitation; agh network coordination and invitation commands
qa_status: untested
bug_ids: BUG-20260715-task-run-designation-hidden
fix_status: fixed
retest_status: pass
fix_commits: pending final whole-diff commit
evidence: docs/qa/evidence/2026-07-14-network-changes/ch-coordination-future-runs.md;/Users/pedronauck/dev/qa-labs/agh-network-coordination-future-runs-20260715-081644-086405-lab/qa-artifacts/qa/teardown.json
last_report: docs/qa/reports/2026-07-14-network-changes.md
overlaps: NB-execution-participation-defaults
---

Planning flag for Task 05 invitation UX. Next QA cycle should prove the visibility matrix, double-accept idempotency, daemon-backed dismiss, and future-runs-only copy.

QA 2026-07-15: the real coordinator payload initially hid its designation and suppressed the invitation. After the owning conversion fix, daemon-served Web proved eligible rendering, persistent daemon-backed dismissal, idempotent task-scoped acceptance, immutable existing Local runs, future Live resolution, and an unrelated Local control. Single-agent and terminal direct routes stayed ineligible; the existing Web suite owns the Network-disabled eligibility branch.

2026-07-21: qa_status reset to untested — the opendesign redesigns restructured this scenario's web entry surface (task detail/run detail 3-tab IA, settings takeover shell, or providers page); the pass verdict predates that surface.
