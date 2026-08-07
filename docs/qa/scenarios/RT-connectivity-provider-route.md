---
id: RT-connectivity-provider-route
area: RT
title: Verify and supervise a connectivity provider route
persona: Dora
journey: J-operate-daemon-schema
expected: Enabling a selected provider establishes the assigned tier, publishes only an endpoint that echoes that tier's challenge under the verification policy, and on wrong-tier, unstable public endpoint, verification failure, outage, crash, or teardown leaves status degraded with no half-exposed route.
entry_points: compozy gateway provider enable; compozy status -o json; GET /api/gateway/status over HTTP and UDS; gateway provider events
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: RT-gateway-local-only-boot; RT-gateway-paired-device
---

Own the operator-facing provider lifecycle: bundled Tailscale is embedded and uses the operator
account, both tiers are independent, and status across CLI, HTTP, and UDS agrees. Walk wrong-tier,
missing nonce, HTTPS/TLS/redirect/body/deadline/SSRF rejection, public endpoint stability, outage,
bounded teardown, supervised restart, and one-provider-per-tier selection.

QA impact 2026-08-06: added for remote-gateway Task 03. Flag only; Tasks 08–09 own the walk.
