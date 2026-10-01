---
id: RT-web-attention-title-count
area: RT
title: Keep the tab title's needs-you count exact
persona: Théo
journey: J-respond-to-agent-attention
expected: While the tab is visible or backgrounded, its title shows the exact cross-workspace unread needs-you notification count, excludes Finished work, survives route changes, clears at zero, and never displays a stale source as current.
entry_points: browser tab title; web route navigation
qa_status: pass
bug_ids:
fix_status:
retest_status: pass
fix_commits:
evidence: /Users/pedronauck/Dev/qa-labs/compozy-reported-issues-20260929-20260930-025651-581547-lab/qa-artifacts/qa/final-title-hidden-cleared.json; docs/qa/reports/2026-09-29-reported-issues.md
last_report: docs/qa/reports/2026-09-29-reported-issues.md
overlaps: RT-session-attention-catalog; RT-web-attention-bell-jump
---

Exercise counts above the menubar's 9+ cap, route navigation, background updates, stream loss, and a
return to zero. The base product title must be restored without accumulating repeated count prefixes.

QA impact 2026-08-16: Task 03 added the summary-fed document title channel. Flag only; Task 08 owns
the real-user walk and evidence.

QA 2026-08-16 Herdr parity: The isolated browser journey, focused attention Playwright lane, and full Web E2E exercised cross-workspace landing, permission resolution, counts, channel suppression, task canary, catalog scope/order, finished presence clearing, and honest quiet/stale states. The lab browser exposed its real notification capability; deterministic granted and denied branches ran in the canonical browser suite.


2026-09-10 issue 606: the title and bell count share the unread notification projection. Clear all
must clear both after server confirmation while runtime session badges continue to report source
attention. The attention Playwright suite now uses the real Clear all action for the zero-count
transition. Local browser execution is deferred by explicit user instruction to CI.

QA re-walk 2026-09-30: PASS for the repaired background-title contract on actual isolated Electron final index71ea3236e585a54471d994843a07ea687a1797a34badc28b6af7086f13d18b57. Public snapshot acknowledgement204 established base title CompozyOS. Genuine CUA Cmd-H made the renderer hidden and unfocused; a provider-free public Loop ask then produced exact needs_you1/finished0 and the hidden title became (1) CompozyOS. Acknowledging that exact public snapshot204 restored CompozyOS while the renderer remained hidden. The pending ask was resolved through its inspected public identity. No synthetic visibility or database writes were used. Evidence: final-title-hidden-growth.json, final-title-hidden-attention.json, final-title-hidden-ack.json and final-title-hidden-cleared.json in the canonical report lab. Broader uncapped-count/route/stream-loss behavior retains its owning historical and remote suite evidence; this walk does not claim every historical dimension was repeated.
