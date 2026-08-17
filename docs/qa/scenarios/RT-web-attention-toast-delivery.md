---
id: RT-web-attention-toast-delivery
area: RT
title: Receive useful attention alerts without notification fatigue
persona: Cora
journey: J-respond-to-agent-attention
expected: Unfocused needs-you events deliver distinct clickable toasts, same-session flaps deduplicate for five seconds, completions coalesce for five seconds, focused and muted sessions stay silent, one sound plays per batch, and bursts show four newest needs-you toasts plus a bell ledge for the remainder.
entry_points: in-app attention toast; built-in attention sound; attention overflow ledge
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: docs/qa/reports/2026-08-16-herdr-parity.md; /Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260816-141901-835450-lab/qa-artifacts/qa/screenshots/herdr-cross-workspace-needs-you-fixed.png; /Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260816-141901-835450-lab/qa-artifacts/qa/screenshots/herdr-attention-all-quiet-cleared.png; .compozy/tasks/herdr-parity/evidence/visual/task_03
last_report: docs/qa/reports/2026-08-16-herdr-parity.md
overlaps: RT-operator-notification-delivery; RT-web-attention-bell-jump
---

Include a reconnect, mute changed during a completion window, five simultaneous needs-you events,
and a toast resolved before activation. Clicking every visible form must reach the current session or
the bell without an error, and autoplay refusal must stay silent rather than breaking delivery.

QA impact 2026-08-16: Task 03 added client delivery policy, toast variants, sound, dedup, coalescing,
focus suppression, and overflow. Flag only; Task 08 owns the real-user walk and evidence.

QA 2026-08-16 Herdr parity: The isolated browser journey, focused attention Playwright lane, and full Web E2E exercised cross-workspace landing, permission resolution, counts, channel suppression, task canary, catalog scope/order, finished presence clearing, and honest quiet/stale states. The lab browser exposed its real notification capability; deterministic granted and denied branches ran in the canonical browser suite.
