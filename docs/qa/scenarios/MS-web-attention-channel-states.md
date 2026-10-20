---
id: MS-web-attention-channel-states
area: MS
title: Configure only attention channels the platform can deliver
persona: Cora
journey: J-respond-to-agent-attention
expected: Settings → Attention applies toast, sound, system, and project-mute changes live; system notifications show Allowed, Blocked, or Unavailable from real platform capability and permission state, never claim success after refusal, and preserve the complete config after reload.
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

Desktop notification permission regression (#819): launch an isolated desktop instance and enable
system notifications from Settings. The product's main frame must receive notification permission
and deliver a notification while the app is in the background. Foreign origins and subframes must
remain denied, including same-origin subframes. With no product origin configured, all permissions
remain denied. A blocked state must not claim the operating system caused the refusal; in-app
notifications remain available. Record OS notification delivery separately for each tested platform.

2026-10-19: isolated Electron/macOS policy checks passed for the product main frame and denied
same-origin subframes, foreign origins, missing product origin, and geolocation. Constructing a
notification did not throw, but no show event arrived within five seconds; OS delivery and the
complete Settings journey remain pending. See `docs/qa/reports/2026-10-19-issues-818-819-828.md`.

QA impact 2026-08-16: Task 03 added Settings → Attention and truthful browser-channel states. Flag
only; Task 08 owns the real-user walk and evidence.

QA 2026-08-16 Herdr parity: The isolated browser journey, focused attention Playwright lane, and full Web E2E exercised cross-workspace landing, permission resolution, counts, channel suppression, task canary, catalog scope/order, finished presence clearing, and honest quiet/stale states. The lab browser exposed its real notification capability; deterministic granted and denied branches ran in the canonical browser suite.
