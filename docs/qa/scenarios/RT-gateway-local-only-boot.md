---
id: RT-gateway-local-only-boot
area: RT
title: Boot locally when gateway authentication is unavailable
persona: Ada
journey: J-expose-and-pair-gateway
expected: A fresh daemon with gateway.enabled=true but no active gateway authentication reaches local readiness, reports the refusal cause and fix, keeps provider and surface observations down or off, and advertises no remote endpoint.
entry_points: compozy daemon start --foreground; compozy status -o json; daemon log
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: MS-gateway-config-ceiling
---

This scenario owns the fail-closed boot boundary. Listener reachability, provider setup, and surface
management are covered by later remote-gateway tasks.
