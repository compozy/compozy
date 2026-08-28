---
id: RT-openclaw-provider-managed-runtime
area: RT
title: Run OpenClaw without fabricated model controls
persona: Théo
journey: J-17
expected: Public provider projections mark OpenClaw as provider_managed; the Runtime Selector shows Provider managed and disables model, Reasoning, Fast, and ACP-option controls, a session can bind with no model negotiation, and explicit unsupported overrides fail instead of being ignored.
entry_points: web session composer; GET /api/providers; workspace provider payload; settings provider payload; CLI/HTTP/UDS session prompt
qa_status: blocked-verify
bug_ids:
fix_status:
retest_status: blocked
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-acp-runtime-catalog-20260828-004625-083662-lab/qa-artifacts/qa/evidence/providers-http.json; docs/qa/reports/2026-08-27-acp-runtime-catalog.md
last_report: docs/qa/reports/2026-08-27-acp-runtime-catalog.md
overlaps: ET-web-runtime-selector-minimal-slider; RT-session-prompt-runtime-transitions
---

Added for the ACP runtime catalog rebuild. The OpenClaw Gateway owns runtime selection; CompozyOS
must expose that ownership consistently rather than inventing selectable values.

QA 2026-08-28: blocked-verify. HTTP and focused provider-strategy tests prove `provider_managed`,
disabled logical controls, bind-without-model, and rejection of explicit overrides. The isolated
provider home does not contain the OpenClaw executable, so a real Gateway handshake cannot be
claimed on this machine.
