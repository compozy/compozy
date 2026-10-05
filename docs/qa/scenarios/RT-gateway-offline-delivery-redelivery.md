---
id: RT-gateway-offline-delivery-redelivery
area: RT
title: Recover an offline public delivery through sender redelivery
persona: Bruno
journey: J-deliver-through-public-gateway
expected: A delivery attempted while the daemon is offline fails visibly at the sender and creates no hidden Compozy work; after ingress is healthy, one sender-side redelivery creates exactly one attributed Loop run.
entry_points: External sender delivery log and redelivery action; public webhook URL; Compozy run detail
qa_status: skipped
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-remote-gateway-20260807-202655-957508-lab/qa-artifacts/qa/test-cases/34-signed-webhook-local-pipeline.json;/Users/pedronauck/dev/qa-labs/compozy-remote-gateway-20260807-202655-957508-lab/qa-artifacts/qa/test-cases/40-webhook-boundaries-and-projection.json
last_report: docs/qa/reports/2026-10-02-untested.md
overlaps: RT-gateway-public-ingress-bindings; TA-060
---

A delivery attempted while the daemon is offline fails visibly at the sender and creates no hidden Compozy work; after ingress is healthy, one sender-side redelivery creates exactly one attributed Loop run.

Walk each listed public entry point, then reload and read the stored result independently. Exercise rejection and recovery with the same workspace and profile to confirm that unrelated state remains intact.

2026-09-27 scope update: retired product surfaces were removed from this active scenario. Historical evidence remains in the dated reports; this revised contract requires a fresh walk.

2026-10-05: Deferred from this QA cycle by the user's explicit scope reduction.
Coverage remains outstanding; this skip is not a passing result. Resume from the dated
report's session matrix in a future QA cycle.
