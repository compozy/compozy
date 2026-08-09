---
id: RT-gateway-self-audit
area: RT
title: Audit gateway posture through every structured surface
persona: Dora
journey: J-audit-and-teardown-gateway
expected: Web, CLI, HTTP, UDS, and the gateway native tool return the same stable severity-ranked findings or an explicit no-findings result, and following a finding's remediation clears it without exposing credentials.
entry_points: Web /settings/gateway; compozy gateway audit -o json; HTTP/UDS GET /api/gateway/audit; compozy__gateway action=audit
qa_status: pass
bug_ids: BUG-20260807-gateway-native-tool-wiring
fix_status: fixed
retest_status: pass
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-remote-gateway-20260807-202655-957508-lab/qa-artifacts/qa/test-cases/15-private-audit-provider-finding.json;/Users/pedronauck/dev/qa-labs/compozy-remote-gateway-20260807-202655-957508-lab/qa-artifacts/qa/test-cases/17-private-audit-remediated.json;/Users/pedronauck/dev/qa-labs/compozy-remote-gateway-20260807-202655-957508-lab/qa-artifacts/qa/test-cases/38-native-audit-parity.json;/Users/pedronauck/dev/qa-labs/compozy-remote-gateway-toolmeta-remediation-20260808-060444-758800-lab/qa-artifacts/qa/test-cases/05-gateway-audit-canary.json
last_report: docs/qa/reports/2026-08-08-remote-gateway-toolmeta-remediation.md
overlaps: RT-gateway-local-only-boot; RT-gateway-remote-cli-profile
---

Own the operator and agent self-audit journey, including unhealthy-provider remediation, stable
finding order, route parity, mutation refusal below approve-all, and byte-level secret containment.

QA impact 2026-08-07: added for remote-gateway Task 06. Flag only; Tasks 08–09 own the walk.

QA walk 2026-08-07: provider degradation produced a finding, its own remediation cleared it, and
CLI, HTTP, UDS, Web, and the native gateway tool agreed on explicit local-only no-findings output.
The native tool composition panic was fixed and re-walked.

QA canary 2026-08-08: a fresh isolated daemon returned the same local-only no-findings payload through CLI, HTTP, UDS, and `compozy__gateway`; the native result was completed and redacted.
