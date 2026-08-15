---
id: ET-session-attachment-model-gate
area: ET
title: Refuse attachments for a text-only model
persona: Bruno
journey: J-17
expected: When the selected ACP model lacks the required image or file capability, an attachment-bearing prompt is refused before dispatch while the draft and previews remain available for correction.
entry_points: web session composer runtime selector; prompt API
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-web-runtime-selector-minimal-slider
---

QA impact 2026-08-15: attachment lifecycle implementation added this user-visible behavior. Flag only; the orchestrator's QA tail owns the persona walk and evidence.
