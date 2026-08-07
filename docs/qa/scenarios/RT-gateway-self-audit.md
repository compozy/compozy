---
id: RT-gateway-self-audit
area: RT
title: Audit gateway posture through every structured surface
persona: Dora
journey: J-operate-daemon-schema
expected: CLI, HTTP, UDS, and the gateway native tool return the same stable severity-ranked findings or an explicit no-findings result without changing gateway state or exposing credentials.
entry_points: compozy gateway audit -o json; HTTP/UDS GET /api/gateway/audit; compozy__gateway action=audit
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: RT-gateway-local-only-boot; RT-gateway-remote-cli-profile
---

Own the operator and agent self-audit journey, including unhealthy-provider remediation, stable
finding order, route parity, mutation refusal below approve-all, and byte-level secret containment.

QA impact 2026-08-07: added for remote-gateway Task 06. Flag only; Tasks 08–09 own the walk.
