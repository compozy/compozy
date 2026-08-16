---
id: RT-web-attention-toast-delivery
area: RT
title: Receive useful attention alerts without notification fatigue
persona: Cora
journey: J-respond-to-agent-attention
expected: Unfocused needs-you events deliver distinct clickable toasts, same-session flaps deduplicate for five seconds, completions coalesce for five seconds, focused and muted sessions stay silent, one sound plays per batch, and bursts show four newest needs-you toasts plus a bell ledge for the remainder.
entry_points: in-app attention toast; built-in attention sound; attention overflow ledge
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: RT-operator-notification-delivery; RT-web-attention-bell-jump
---

Include a reconnect, mute changed during a completion window, five simultaneous needs-you events,
and a toast resolved before activation. Clicking every visible form must reach the current session or
the bell without an error, and autoplay refusal must stay silent rather than breaking delivery.

QA impact 2026-08-16: Task 03 added client delivery policy, toast variants, sound, dedup, coalescing,
focus suppression, and overflow. Flag only; Task 08 owns the real-user walk and evidence.
