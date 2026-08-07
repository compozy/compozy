---
id: RT-gateway-no-device-recovery
area: RT
title: Recover after every paired device is unavailable
persona: Iris
journey: J-expose-and-pair-gateway
expected: When the last device is revoked, expired, or lost, remote views reveal no product data and the daemon host can mint a fresh QR plus copyable pairing artifact that restores access without reviving any old credential.
entry_points: Revoked private or public Gateway view; local Web /settings/gateway; compozy pair mint
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: RT-gateway-paired-device; RT-gateway-public-ui-consent
---

This is the shipped recovery boundary: the local daemon UI or CLI is the root of trust. There is no
remote password reset, account recovery service, or public pairing route. Test an empty inventory,
self-revoke, last-device revoke, expired artifact, consumed artifact, and replacement pairing; every
old credential stays revoked.

