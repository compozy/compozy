---
id: RT-gateway-public-ingress-bindings
area: RT
title: Deliver webhooks through public ingress
persona: Dora
journey: J-deliver-through-public-gateway
expected: A verified public gateway projects honest webhook URLs, accepts only explicitly confirmed same-workspace bindings, dispatches signed deliveries with attribution, rate-limits each endpoint and source, and requires reconfirmation after an address change. Deleting a subject removes its binding; daemon downtime is a sender-visible failure.
entry_points: GET automation trigger; POST and DELETE /api/gateway/ingress-bindings over private HTTP and UDS; public webhook routes; gateway ingress events
qa_status: skipped
bug_ids: BUG-20260808-gateway-funnel-never-publishes
fix_status: fixed
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-remote-gateway-20260807-202655-957508-lab/qa-artifacts/qa/test-cases/34-signed-webhook-local-pipeline.json;/Users/pedronauck/dev/qa-labs/compozy-remote-gateway-20260807-202655-957508-lab/qa-artifacts/qa/test-cases/40-webhook-boundaries-and-projection.json;https://github.com/pedronauck/compozy-remote-gateway-e2e-20260808/actions/runs/31262366045;automation:run_wbh_bfa35e72f4aa5b0d2909a010;loop:looprun-3605ec461ab966d7
last_report: docs/qa/reports/2026-10-02-untested.md
overlaps: RT-connectivity-provider-route; RT-gateway-offline-delivery-redelivery
---

A verified public gateway projects honest webhook URLs, accepts only explicitly confirmed same-workspace bindings, dispatches signed deliveries with attribution, rate-limits each endpoint and source, and requires reconfirmation after an address change. Deleting a subject removes its binding; daemon downtime is a sender-visible failure.

Walk each listed public entry point, then reload and read the stored result independently. Exercise rejection and recovery with the same workspace and profile to confirm that unrelated state remains intact.

2026-09-27 scope update: retired product surfaces were removed from this active scenario. Historical evidence remains in the dated reports; this revised contract requires a fresh walk.

2026-10-05: Deferred from this QA cycle by the user's explicit scope reduction.
Coverage remains outstanding; this skip is not a passing result. Resume from the dated
report's session matrix in a future QA cycle.
