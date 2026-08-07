---
id: RT-gateway-offline-delivery-redelivery
area: RT
title: Recover an offline public delivery through sender redelivery
persona: Bruno
journey: J-deliver-through-public-gateway
expected: A delivery attempted while the daemon is offline fails visibly at the sender and creates no hidden Compozy work; after ingress is healthy, one sender-side redelivery creates exactly one attributed Loop run or bridge effect.
entry_points: External sender delivery log and redelivery action; public webhook or bridge callback URL; Compozy run or bridge detail
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: RT-gateway-public-ingress-bindings; TA-060
---

Expected limitation, not an open defect: this feature has no store-and-forward broker. Daemon uptime
bounds delivery reachability, and the external sender owns retry or redelivery. The operator runbook,
trigger surface, and bridge surface must state this boundary wherever they present a delivery URL.

Record the failed sender receipt, prove no run or callback effect appeared during downtime, restore
the daemon, confirm the binding is still live or explicitly needs reconfirmation, and use the sender's
own redelivery path. Duplicate delivery identities must remain deduplicated.

