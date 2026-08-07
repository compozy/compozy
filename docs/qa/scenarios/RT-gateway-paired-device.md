---
id: RT-gateway-paired-device
area: RT
title: Pair and revoke a device through an isolated gateway tier
persona: Ada
journey: J-operate-daemon-schema
expected: A one-time local pairing admits exactly one device to loopback-only private routes, HTTP and UDS report the same device state, and revocation closes its live streams before rejecting the credential.
entry_points: UDS POST /api/gateway/pairings; private POST /api/gateway/pairings/redeem; private GET /api/status; HTTP/UDS /api/gateway/devices; compozy daemon status -o json
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: RT-gateway-local-only-boot
---

This scenario owns the Task 02 device-authentication boundary. Connectivity-provider setup, remote
CLI verbs, and the browser operator surface arrive in later remote-gateway tasks. The user walk is
deferred to the dedicated QA tasks at the end of the task graph.
