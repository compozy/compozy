---
id: NB-run-conversation-bounds-usage
area: NB
title: Run detail shows conversation, bounds, and truthful usage
persona: Bruno
journey: J-enable-coordinated-conversations
expected: Coordinated run detail shows the participation chip, an empty conversation explaining silence when no messages exist, paginated history when messages exist, and workspace-scoped usage labeled actual or usage_unavailable with no fabricated totals.
entry_points: web task run detail conversation and usage panels; GET /api/network/usage over HTTP/UDS; compozy network usage -o json; run conversation SSE and paginated history projections
qa_status: blocked-verify
bug_ids: BUG-20260715-run-conversation-hardcoded-empty;BUG-20260715-designated-fanout-conversation-split;BUG-20260715-shared-button-focus-invisible
fix_status: fixed
retest_status: pass
fix_commits: 8eeb8a38
evidence: docs/qa/evidence/2026-07-14-network-changes/ch-coordination-future-runs.md;/Users/pedronauck/dev/qa-labs/compozy-network-coordination-future-runs-20260715-081644-086405-lab/qa-artifacts/qa/teardown.json;/Users/pedronauck/dev/qa-labs/compozy-qa-misc-network-goal-release-site-20260730-060405-932516-lab/qa-artifacts/qa
last_report: docs/qa/reports/2026-07-28-untested-full.md
overlaps: NB-run-bounded-live-collaboration
---

Planning flag for Task 05 run conversation/usage surfaces.

QA 2026-07-15: designated siblings initially split into per-run channels and the run route hardcoded empty history/zero usage. After the owning fixes, one real three-run group shared one conversation while an independent group stayed isolated; empty silence, a live SSE update, 120+5 pagination, run-fenced bounds/usage, refresh, Back/Forward, and task authority passed. The shared keyboard focus defect was fixed at the design-token owner and verified at 375/768/1280.

2026-07-21: qa_status reset to untested — the opendesign redesigns restructured this scenario's web entry surface (task detail/run detail 3-tab IA, settings takeover shell, or providers page); the pass verdict predates that surface.
