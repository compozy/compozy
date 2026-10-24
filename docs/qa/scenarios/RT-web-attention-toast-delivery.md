---
id: RT-web-attention-toast-delivery
area: RT
title: Receive useful attention alerts without notification fatigue
persona: Cora
journey: J-respond-to-agent-attention
expected: Unfocused needs-you events deliver distinct clickable toasts, same-session flaps deduplicate for five seconds, completions coalesce for five seconds, focused and muted sessions stay silent, one sound plays per batch, and bursts show four newest needs-you toasts plus a bell ledge for the remainder.
entry_points: in-app attention toast; built-in attention sound; attention overflow ledge
qa_status: blocked-verify
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: docs/qa/evidence/2026-08-18-request-questionnaire/attention-toast-single.png; docs/qa/evidence/2026-08-18-request-questionnaire/attention-toast-stacked.png; docs/qa/reports/2026-08-16-herdr-parity.md; /Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260816-141901-835450-lab/qa-artifacts/qa/screenshots/herdr-cross-workspace-needs-you-fixed.png; /Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260816-141901-835450-lab/qa-artifacts/qa/screenshots/herdr-attention-all-quiet-cleared.png; .compozy/tasks/herdr-parity/evidence/visual/task_03
last_report: docs/qa/reports/2026-08-16-herdr-parity.md
overlaps: RT-operator-notification-delivery; RT-web-attention-bell-jump
---

Include a reconnect, mute changed during a completion window, five simultaneous needs-you events,
and a toast resolved before activation. Clicking every visible form must reach the current session or
the bell without an error, and autoplay refusal must stay silent rather than breaking delivery.

QA impact 2026-08-16: Task 03 added client delivery policy, toast variants, sound, dedup, coalescing,
focus suppression, and overflow. Flag only; Task 08 owns the real-user walk and evidence.

QA 2026-08-16 Herdr parity: The isolated browser journey, focused attention Playwright lane, and full Web E2E exercised cross-workspace landing, permission resolution, counts, channel suppression, task canary, catalog scope/order, finished presence clearing, and honest quiet/stale states. The lab browser exposed its real notification capability; deterministic granted and denied branches ran in the canonical browser suite.

Bug 2026-08-18 (operator report, live run): attention toasts rendered transparent — sonner `toast.custom` applies no chrome and `ToastFrame` carried no surface, so window content behind (a run's Done status pill) bled through the toast text. The story masked it by wrapping the toast in its own surface div. Fixed: `ToastFrame` and the overflow ledge now self-carry the floating surface (`border-line` + `bg-canvas-soft` + `shadow-overlay`), and the story renders the bare component. Evidence: attention-toast-single/stacked captures. qa_status reset to blocked-verify — the /qa-execution walk is operator-invoked and pending re-walk over real window content.

QA impact 2026-10-23 (Sessions Kanban, sk-web-chip; walk pending): an idle session that still waits on children, a reply, a Loop run, a task lease or a monitor now reads `waiting` and fires no finished toast; the toast fires once when its last waiting signal closes. A single finished toast for a session with an open PR reads "#N ready for review" in its body line (title, meta and click-to-open unchanged); grouped toasts are unchanged. The PR number comes from the session read model the client already holds, so a session never read by this client keeps "Finished while you were away". Automated: `attention.spec.ts` Sessions Kanban E2E-004 (child live → no toast; child stopped → one toast) and `attention-delivery.test.ts` (UT-065, UT-066).

Monitor retry regression: let a timer expire while wake admission temporarily fails. The
session must remain waiting while the monitor is delivering, even after its original watch
deadline; no finished transition is justified by that expired deadline. Cancel or finish the
delivery and verify the signal disappears. Backend coverage: `TestSessionWorkSourcesMonitors`
exercises the real monitor service with deterministic time and a refusing wake boundary.
