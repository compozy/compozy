---
id: RT-gateway-local-only-boot
area: RT
title: Boot locally when gateway authentication is unavailable
persona: Ada
journey: J-expose-and-pair-gateway
expected: A fresh daemon with gateway.enabled=true but no active gateway authentication reaches local readiness, reports the refusal cause and fix, keeps provider and surface observations down or off, and advertises no remote endpoint.
entry_points: compozy daemon start --foreground; compozy status -o json; daemon log
qa_status: pass
bug_ids: BUG-20260807-gateway-provider-boot
fix_status: fixed
retest_status: pass
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-remote-gateway-20260807-202655-957508-lab/qa-artifacts/qa/test-cases/33-provider-restart-rewalk.json
last_report: docs/qa/reports/2026-08-07-remote-gateway.md
overlaps: MS-gateway-config-ceiling
---

This scenario owns the fail-closed boot boundary. Listener reachability, provider setup, and surface
management are covered by later remote-gateway tasks.

QA walk 2026-08-07: fresh and degraded-provider restarts both reached local readiness with tiers
down, surfaces off, and no advertised address. The degraded-provider boot regression was fixed and re-walked.
