---
id: RT-session-done-presence
area: RT
title: Keep done truthful through operator presence
persona: Théo
journey: J-11
expected: A turn settled without a live operator lease derives `done` until explicitly seen, a settle under any live lease remains `idle`, CLI and API reads never mark it seen, independent client leases cannot renew or release each other, and the result survives daemon restart.
entry_points: Web session view; POST /api/workspaces/{workspace_id}/sessions/{session_id}/presence over HTTP and UDS; compozy session list --badge done; compozy session status <session-id>
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: docs/qa/reports/2026-08-16-herdr-parity.md; /Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260816-141901-835450-lab/qa-artifacts/qa/screenshots/herdr-cross-workspace-needs-you-fixed.png; /Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260816-141901-835450-lab/qa-artifacts/qa/screenshots/herdr-attention-all-quiet-cleared.png; .compozy/tasks/herdr-parity/evidence/visual/task_03
last_report: docs/qa/reports/2026-08-16-herdr-parity.md
overlaps:
---

Settle one turn with no visible client and another while two independent presence leases overlap.
Confirm revision-based seen state, lease renewal and ownership, abandoned-lease expiry, daemon-restart
durability, catalog wake ordering, and that repeated CLI, HTTP, and UDS reads cannot clear `done`.

QA impact 2026-08-16: Task 01 added daemon-owned `done`, revision-based seen state, and per-client
presence leases. Flag only; task_08 owns execution.

QA 2026-08-16 Herdr parity: The isolated browser journey, focused attention Playwright lane, and full Web E2E exercised cross-workspace landing, permission resolution, counts, channel suppression, task canary, catalog scope/order, finished presence clearing, and honest quiet/stale states. The lab browser exposed its real notification capability; deterministic granted and denied branches ran in the canonical browser suite.
