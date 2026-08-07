---
id: RT-gateway-public-ingress-bindings
area: RT
title: Deliver webhooks and bridge callbacks through public ingress
persona: Dora
journey: J-deliver-through-public-gateway
expected: A verified public gateway projects honest webhook and bridge callback URLs, accepts only explicitly confirmed same-workspace bindings, dispatches signed deliveries with attribution, proxies bound bridge callbacks only to their loopback adapter, rate-limits each endpoint and source, and requires reconfirmation after an address change; deleting a subject removes its binding and daemon downtime is a sender-visible failure.
entry_points: GET automation trigger; GET bridge; POST and DELETE /api/gateway/ingress-bindings over private HTTP and UDS; public webhook and bridge callback routes; gateway ingress events
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: RT-connectivity-provider-route; RT-gateway-offline-delivery-redelivery; NB-web-bridge-setup
---

Own the complete public-ingress journey for a pre-existing workspace webhook and a bridge with a
provider-owned loopback listener. Verify off, unconfirmed, live, and reconfirmation states; HMAC,
freshness, replay, disabled-trigger, body-size, browser-method, rate-limit, and workspace boundaries;
run attribution; bridge verification; external-proxy coexistence; deletion/recreation cleanup; stable
restart address; and sender-visible failure while the daemon is stopped.

QA impact 2026-08-06: added for remote-gateway Task 04. Flag only; Tasks 08–09 own the walk.
