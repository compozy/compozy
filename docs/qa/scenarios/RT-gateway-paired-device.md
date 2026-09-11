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
evidence: docs/qa/evidence/2026-09-11-mobile-surface-truth/i-13-pairing-code-minted.png;docs/qa/evidence/2026-09-11-mobile-surface-truth/i-14-devices-listed.png;docs/qa/evidence/2026-09-11-mobile-surface-truth/i-18-after-revoke.png;docs/qa/evidence/2026-09-11-mobile-surface-truth/i-19-after-revoke-web.png
last_report: docs/qa/reports/2026-09-11-mobile-surface-truth.md
overlaps: RT-gateway-local-only-boot
---

This scenario owns the device lifecycle across the operator surface and structured planes: mint,
single-use redeem, rename, origin and activity display, immediate revoke, self/last-device revoke,
and the empty inventory. The local daemon remains the recovery root after every device is revoked.

QA walk 2026-08-07: local pairing, rename, revoke, empty inventory, and replacement pairing passed
through product surfaces. The remote tier admission and live-stream cancellation leg remains
blocked without an authorized provider address and a second remote device.

Re-opened as untested 2026-09-11: the mobile-surface-truth change (branch `mobile-browser-pwa`)
altered the post-pairing operator surface (capability gating), so the paired-device experience is
re-walked under CH-gateway-paired-operator-surface; the real-address admission leg remains
externally blocked pending an authorized TS_AUTHKEY.

QA walk 2026-09-11 (CH-gateway-paired-operator-surface, lab
`compozy-mobile-surface-truth-20260911-041346-834544`): local legs re-passed on this build — Web
pairing dialog mints a one-time code with truthful copy ("There is no verified private address yet,
so the other device has nowhere to open" — no QR, no dead URL); HTTP mint + documented redeem
(`POST /api/gateway/pairings/redeem`, `actor_kind=operator_device`) returned a `Secure; HttpOnly;
SameSite=Lax` device cookie; inventory agreed across Web (1 paired → 0 after revoke), HTTP, UDS,
and `compozy device list -o json`; CLI rename landed; revoke bumped `revoke_epoch` with
`revoked_at` set, structured planes retain the marked record while the Web shows 0 paired devices.
The `compozy pair redeem` CLI profile path truthfully refuses a non-HTTPS origin. Blocked-verify
remains for the remote-tier admission legs: the private-tier listener never binds without an
authorized provider (reconciler preflight/establish fails-closed before binding — recovery retries
keep the tier `down`, surface `off`), so the redeemed device credential cannot be exercised against
a gateway listener in-lab (named blocker: user-provisioned authorized `TS_AUTHKEY`).
