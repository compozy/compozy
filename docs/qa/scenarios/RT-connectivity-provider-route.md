---
id: RT-connectivity-provider-route
area: RT
title: Verify and supervise a connectivity provider route
persona: Dora
journey: J-expose-and-pair-gateway
expected: Enabling a selected provider establishes the assigned tier, publishes only an endpoint that echoes that tier's challenge under the verification policy, and on wrong-tier, unstable public endpoint, verification failure, outage, crash, or teardown leaves status degraded with no half-exposed route.
entry_points: compozy gateway provider enable; compozy status -o json; GET /api/gateway/status over HTTP and UDS; gateway provider events
qa_status: blocked-verify
bug_ids: BUG-20260807-gateway-provider-cause;BUG-20260807-gateway-provider-boot
fix_status: fixed
retest_status: pass
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-remote-gateway-20260807-202655-957508-lab/qa-artifacts/qa/test-cases/12-private-provider-degraded-status.json;/Users/pedronauck/dev/qa-labs/compozy-remote-gateway-20260807-202655-957508-lab/qa-artifacts/qa/test-cases/33-provider-restart-rewalk.json
last_report: docs/qa/reports/2026-08-07-remote-gateway.md
overlaps: RT-gateway-local-only-boot; RT-gateway-paired-device
---

Own the operator-facing provider lifecycle: bundled Tailscale is embedded and uses the operator
account, both tiers are independent, and status across CLI, HTTP, and UDS agrees. Walk wrong-tier,
missing nonce, HTTPS/TLS/redirect/body/deadline/SSRF rejection, public endpoint stability, outage,
bounded teardown, supervised restart, and one-provider-per-tier selection.

QA impact 2026-08-06: added for remote-gateway Task 03. Flag only; Tasks 08–09 own the walk.

QA walk 2026-08-07: missing authorization degraded safely, retained no address, exposed an
actionable value-free cause, and boot recovered local-only after the fix. Real route establishment,
challenge verification, and external teardown remain blocked without an authorized Tailscale account.

QA impact 2026-08-08: bundled provider renamed to `tailscale` (extension ID and gateway provider
name); `compozy gateway provider enable` and `/api/gateway/providers/{name}` take the new name.
Walk attempt: rename covered mechanically by the Go suite in the full gate; real route establishment
stays blocked on the same missing authorized Tailscale account, so the scenario remains blocked-verify.
