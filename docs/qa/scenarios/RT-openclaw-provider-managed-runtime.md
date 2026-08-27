---
id: RT-openclaw-provider-managed-runtime
area: RT
title: Run OpenClaw without fabricated model controls
persona: Théo
journey: J-17
expected: Public provider projections mark OpenClaw as provider_managed; the Runtime Selector shows Provider managed and disables model, Reasoning, Fast, and ACP-option controls, a session can bind with no model negotiation, and explicit unsupported overrides fail instead of being ignored.
entry_points: web session composer; GET /api/providers; workspace provider payload; settings provider payload; CLI/HTTP/UDS session prompt
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-web-runtime-selector-minimal-slider; RT-session-prompt-runtime-transitions
---

Added for the ACP runtime catalog rebuild. The OpenClaw Gateway owns runtime selection; CompozyOS
must expose that ownership consistently rather than inventing selectable values.
