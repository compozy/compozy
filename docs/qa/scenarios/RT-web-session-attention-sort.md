---
id: RT-web-session-attention-sort
area: RT
title: Bring sessions needing attention forward without losing selection
persona: Théo
journey: J-respond-to-agent-attention
expected: Choosing Attention first uses daemon ordering to place auth, input, and failed sessions ahead of other work with stable ties, preserves keyboard selection through live transitions, renders unknown honestly, and persists the global sort without overlapping full-section writes.
entry_points: web Sessions catalog sort menu; web session-window sidebar sort menu
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: docs/qa/reports/2026-08-16-herdr-parity.md; /Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260816-141901-835450-lab/qa-artifacts/qa/screenshots/herdr-cross-workspace-needs-you-fixed.png; /Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260816-141901-835450-lab/qa-artifacts/qa/screenshots/herdr-attention-all-quiet-cleared.png; .compozy/tasks/herdr-parity/evidence/visual/task_03
last_report: docs/qa/reports/2026-08-16-herdr-parity.md
overlaps: RT-session-attention-catalog
---

Walk keyboard and pointer selection while sessions enter and leave the needs-you class, including
equal timestamps and an unreporting session. Every badge must carry a distinct glyph and accessible
label; color alone is not an acceptable signal.

QA impact 2026-08-16: Task 03 added the daemon-backed attention-first sort and unified badge
dictionary. Flag only; Task 08 owns the real-user walk and evidence.

QA 2026-08-16 Herdr parity: The isolated browser journey, focused attention Playwright lane, and full Web E2E exercised cross-workspace landing, permission resolution, counts, channel suppression, task canary, catalog scope/order, finished presence clearing, and honest quiet/stale states. The lab browser exposed its real notification capability; deterministic granted and denied branches ran in the canonical browser suite.
