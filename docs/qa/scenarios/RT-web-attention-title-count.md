---
id: RT-web-attention-title-count
area: RT
title: Keep the tab title's needs-you count exact
persona: Théo
journey: J-respond-to-agent-attention
expected: While the tab is visible or backgrounded, its title shows the exact cross-workspace needs-you summary, excludes Finished work, survives route changes, clears at zero, and never displays a stale source as current.
entry_points: browser tab title; web route navigation
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: docs/qa/reports/2026-08-16-herdr-parity.md; /Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260816-141901-835450-lab/qa-artifacts/qa/screenshots/herdr-cross-workspace-needs-you-fixed.png; /Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260816-141901-835450-lab/qa-artifacts/qa/screenshots/herdr-attention-all-quiet-cleared.png; .compozy/tasks/herdr-parity/evidence/visual/task_03
last_report: docs/qa/reports/2026-08-16-herdr-parity.md
overlaps: RT-session-attention-catalog; RT-web-attention-bell-jump
---

Exercise counts above the menubar's 9+ cap, route navigation, background updates, stream loss, and a
return to zero. The base product title must be restored without accumulating repeated count prefixes.

QA impact 2026-08-16: Task 03 added the summary-fed document title channel. Flag only; Task 08 owns
the real-user walk and evidence.

QA 2026-08-16 Herdr parity: The isolated browser journey, focused attention Playwright lane, and full Web E2E exercised cross-workspace landing, permission resolution, counts, channel suppression, task canary, catalog scope/order, finished presence clearing, and honest quiet/stale states. The lab browser exposed its real notification capability; deterministic granted and denied branches ran in the canonical browser suite.
