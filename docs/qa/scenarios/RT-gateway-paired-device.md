---
id: RT-gateway-paired-device
area: RT
title: Pair, manage, and revoke a remote device
persona: Iris
journey: J-expose-and-pair-gateway
expected: A one-time local pairing shown as both QR and copyable text, with a readable 6-digit manual code, admits exactly one named device; device state agrees across web, HTTP, UDS, and CLI; revocation closes live work before rejecting the credential, and removal deletes the record only after revoking an active device and appending a durable removal event.
entry_points: Web /settings/gateway; UDS POST /api/gateway/pairings; private POST /api/gateway/pairings/redeem; HTTP/UDS /api/gateway/devices; HTTP/UDS DELETE /api/gateway/devices/{id}/record; compozy device list -o json; compozy device remove
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
manual-code and artifact redeem, single-use redeem across both representations, rename, origin and
activity display, immediate revoke, self/last-device revoke, removal of an active device, removal of
a revoked device, and the empty inventory. The local daemon remains the recovery root after every
device is revoked or removed.

QA walk 2026-08-07: local pairing, rename, revoke, empty inventory, and replacement pairing passed
through product surfaces. The remote tier admission and live-stream cancellation leg remains
blocked without an authorized provider address and a second remote device.

QA regression 2026-11-03: `TestGatewayRevocationFencesInflightMutationIntegration` passed
against real HTTP and SQLite without race instrumentation. Both device and app-token revocation
roll back the in-flight mutation and return HTTP 401 with `gateway_device_revoked` or
`app_token_revoked`, even though revocation cancels the mutation context. This focused receipt
does not complete the remote-device walk above.

Backend status 2026-11-18: manual-code format, collision retry, redeem by either representation,
single use across both, expiry, grouped-input normalization, and per-source redeem rate limiting are
covered by `TestDeviceCredentialsAndPairing` and `TestGatewayRedeemRateLimiting`. Removal of an
active device (revoked, streams canceled, row deleted, event appended), removal of a revoked device,
unknown ids, and an aborted removal when the event append fails are covered by
`TestDeviceServiceLifecycle` and `TestGatewayDeviceAuditSink`. Those receipts are not a product walk:
mint, redeem, and removal still need to be exercised through the web UI and a live remote tier before
this scenario leaves `blocked-verify`.

Web status 2026-11-18: the pairing dialog leads with the grouped six-digit manual code, copies exactly
those digits, and keeps the QR, link, and long code as secondary paths. The redeem gate accepts the
typed code (with spaces or dashes) or a pasted artifact and sends exactly one credential; daemon
refusals map to plain copy. The device list offers Remove for active and revoked devices behind a
confirmation whose copy differs by state, and a 404 refreshes the inventory without an error. These
receipts are component, hook, and adapter tests in `gateway-pairing-dialog.test.tsx`,
`gateway-pairing-gate.test.tsx`, `gateway-device-list.test.tsx`, `use-gateway-mutations.test.tsx`,
and `gateway-api.test.ts`. They are not a product walk, and they do not verify the 320px and 390px
layout; a live walk at those widths still gates this scenario leaving `blocked-verify`.
