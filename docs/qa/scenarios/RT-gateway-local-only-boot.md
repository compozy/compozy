---
id: RT-gateway-local-only-boot
area: RT
title: Boot locally when gateway authentication is unavailable
persona: Ada
journey: J-expose-and-pair-gateway
expected: A fresh daemon with gateway.enabled=true but no active gateway authentication reaches local readiness, reports the refusal cause and fix, keeps provider and surface observations down or off, and advertises no remote endpoint.
entry_points: compozy daemon start --foreground; compozy status -o json; daemon log
qa_status: pass
bug_ids: BUG-20260807-gateway-provider-boot;BUG-20260812-global-workspace-gateway-config
fix_status: fixed
retest_status: pass
fix_commits:
evidence: docs/qa/reports/2026-08-12-pr-webhook-release-notes.md
last_report: docs/qa/reports/2026-08-12-pr-webhook-release-notes.md
overlaps: MS-gateway-config-ceiling
---

This scenario owns the fail-closed boot boundary. Listener reachability, provider setup, and surface
management are covered by later remote-gateway tasks.

QA walk 2026-08-07: fresh and degraded-provider restarts both reached local readiness with tiers
down, surfaces off, and no advertised address. The degraded-provider boot regression was fixed and re-walked.

QA impact 2026-08-12: reset after fixing operator-home workspace loading so a global
`gateway.enabled=true` configuration no longer blocks daemon startup.

QA walk 2026-08-12: the daemon reached local readiness from the project workspace while the
operator-home workspace and its global Gateway configuration remained active.
