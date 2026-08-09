---
id: RT-gateway-public-ingress-bindings
area: RT
title: Deliver webhooks and bridge callbacks through public ingress
persona: Dora
journey: J-deliver-through-public-gateway
expected: A verified public gateway projects honest webhook and bridge callback URLs, accepts only explicitly confirmed same-workspace bindings, dispatches signed deliveries with attribution, proxies bound bridge callbacks only to their loopback adapter, rate-limits each endpoint and source, and requires reconfirmation after an address change; deleting a subject removes its binding and daemon downtime is a sender-visible failure.
entry_points: GET automation trigger; GET bridge; POST and DELETE /api/gateway/ingress-bindings over private HTTP and UDS; public webhook and bridge callback routes; gateway ingress events
qa_status: pass
bug_ids: BUG-20260808-gateway-funnel-never-publishes
fix_status: fixed
retest_status: pass
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-remote-gateway-20260807-202655-957508-lab/qa-artifacts/qa/test-cases/34-signed-webhook-local-pipeline.json;/Users/pedronauck/dev/qa-labs/compozy-remote-gateway-20260807-202655-957508-lab/qa-artifacts/qa/test-cases/40-webhook-boundaries-and-projection.json;https://github.com/pedronauck/compozy-remote-gateway-e2e-20260808/actions/runs/31262366045;automation:run_wbh_bfa35e72f4aa5b0d2909a010;loop:looprun-3605ec461ab966d7
last_report: docs/qa/reports/2026-08-08-remote-gateway-tailscale-github.md
overlaps: RT-connectivity-provider-route; RT-gateway-offline-delivery-redelivery; NB-web-bridge-setup
---

Own the complete public-ingress journey for a pre-existing workspace webhook and a bridge with a
provider-owned loopback listener. Verify off, unconfirmed, live, and reconfirmation states; HMAC,
freshness, replay, disabled-trigger, body-size, browser-method, rate-limit, and workspace boundaries;
run attribution; bridge verification; external-proxy coexistence; deletion/recreation cleanup; stable
restart address; and sender-visible failure while the daemon is stopped.

QA impact 2026-08-06: added for remote-gateway Task 04. Flag only; Tasks 08–09 own the walk.

QA walk 2026-08-07: a real signed delivery reached one Loop with attribution; invalid signature,
stale timestamp, replay, disabled trigger, and oversized body were distinct and side-effect free.
Public binding, bridge callback, address change, and downtime legs remain blocked without a provider.

QA impact 2026-08-08: initial public-provider activation now keeps an unadvertised Funnel staged
while public DNS converges and resolves the proof through an authenticated DNS-over-TLS server.
Status reset to untested for the real Tailscale and GitHub delivery walk.

QA walk 2026-08-08: Tailscale issued a valid certificate, public proof reached the staged listener,
and the public tier became advertised. GitHub push `1b56e60` started Actions run `31262366045`; its
signed delivery `github-31262366045-1` was accepted once and produced one workspace-attributed Loop
run, `looprun-3605ec461ab966d7`, which finished `done`. Gateway audit returned no findings.
