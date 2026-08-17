---
id: MS-web-attention-channel-states
area: MS
title: Configure only attention channels the platform can deliver
persona: Cora
journey: J-respond-to-agent-attention
expected: Settings → Attention applies toast, sound, system, and workspace-mute changes live; system notifications show Armed, Denied, or Unavailable from real platform capability and permission state, never claim success after refusal, and preserve the complete config after reload.
entry_points: web Settings → Attention; browser notification permission prompt
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: docs/qa/reports/2026-08-16-herdr-parity.md; /Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260816-141901-835450-lab/qa-artifacts/qa/screenshots/herdr-cross-workspace-needs-you-fixed.png; /Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260816-141901-835450-lab/qa-artifacts/qa/screenshots/herdr-attention-all-quiet-cleared.png; .compozy/tasks/herdr-parity/evidence/visual/task_03
last_report: docs/qa/reports/2026-08-16-herdr-parity.md
overlaps: MS-attention-settings-roundtrip; RT-web-attention-toast-delivery
---

Walk granted, denied, unsupported, pending-write, failed-write, mute, and unmute states. Confirm
controls cannot issue overlapping full-section writes and that muted rows remain visible in the bell
while toast, sound, and system delivery stop.

QA impact 2026-08-16: Task 03 added Settings → Attention and truthful browser-channel states. Flag
only; Task 08 owns the real-user walk and evidence.

QA 2026-08-16 Herdr parity: The isolated browser journey, focused attention Playwright lane, and full Web E2E exercised cross-workspace landing, permission resolution, counts, channel suppression, task canary, catalog scope/order, finished presence clearing, and honest quiet/stale states. The lab browser exposed its real notification capability; deterministic granted and denied branches ran in the canonical browser suite.
