---
id: RT-gateway-paired-device
area: RT
title: Pair, manage, and revoke a remote device
persona: Iris
journey: J-expose-and-pair-gateway
expected: A one-time local pairing shown as both QR and copyable text admits exactly one named device, device state agrees across web, HTTP, UDS, and CLI, and revocation closes live work before rejecting the credential.
entry_points: Web /settings/gateway; UDS POST /api/gateway/pairings; private POST /api/gateway/pairings/redeem; HTTP/UDS /api/gateway/devices; compozy device list -o json
qa_status: blocked-verify
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-remote-gateway-20260807-202655-957508-lab/qa-artifacts/qa/screenshots/06-paired-devices.png;/Users/pedronauck/dev/qa-labs/compozy-remote-gateway-20260807-202655-957508-lab/qa-artifacts/qa/screenshots/08-revoked-browser-device.png;/Users/pedronauck/dev/qa-labs/compozy-remote-gateway-20260807-202655-957508-lab/qa-artifacts/qa/test-cases/22-device-rename.json
last_report: docs/qa/reports/2026-08-07-remote-gateway.md
overlaps: RT-gateway-local-only-boot
---

This scenario owns the device lifecycle across the operator surface and structured planes: mint,
single-use redeem, rename, origin and activity display, immediate revoke, self/last-device revoke,
and the empty inventory. The local daemon remains the recovery root after every device is revoked.

QA walk 2026-08-07: local pairing, rename, revoke, empty inventory, and replacement pairing passed
through product surfaces. The remote tier admission and live-stream cancellation leg remains
blocked without an authorized provider address and a second remote device.
