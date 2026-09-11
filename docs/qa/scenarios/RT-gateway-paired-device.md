---
id: RT-gateway-paired-device
area: RT
title: Pair, manage, and revoke a remote device
persona: Iris
journey: J-expose-and-pair-gateway
expected: A one-time local pairing shown as both QR and copyable text admits exactly one named device, device state agrees across web, HTTP, UDS, and CLI, and revocation closes live work before rejecting the credential.
entry_points: Web /settings/gateway; UDS POST /api/gateway/pairings; private POST /api/gateway/pairings/redeem; HTTP/UDS /api/gateway/devices; compozy device list -o json
qa_status: pass
bug_ids: BUG-20260911-private-ui-origin-rejected-on-forwarded-tier;BUG-20260911-private-tier-loopback-guard-inert
fix_status: fixed
retest_status: pass
fix_commits: 17304fe8f
evidence: docs/qa/evidence/2026-09-11-mobile-surface-truth/ts2-04-pairing-qr-link.png;docs/qa/evidence/2026-09-11-mobile-surface-truth/ts2-05-redeem-link-landing.png;docs/qa/evidence/2026-09-11-mobile-surface-truth/ts2-07-paired-session.png
last_report: docs/qa/reports/2026-09-11-mobile-surface-truth.md
overlaps: RT-gateway-local-only-boot
---

This scenario owns the device lifecycle across the operator surface and structured planes: mint,
single-use redeem, rename, origin and activity display, immediate revoke, self/last-device revoke,
and the empty inventory. The local daemon remains the recovery root after every device is revoked.

QA walk 2026-08-07: local pairing, rename, revoke, empty inventory, and replacement pairing passed
through product surfaces. The remote tier admission and live-stream cancellation leg remains
blocked without an authorized provider address and a second remote device.

Re-opened as untested 2026-09-11: the mobile-surface-truth change (branch `mobile-surface-truth`)
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

Remote-leg walk 2026-09-11 with the authorized provider (lab
`compozy-mobile-surface-truth-remote-20260911-125112-264736`): the private tier established —
provider `up/healthy`, verified live address advertised (`live: true`), operator_ui surface `on`,
and the Web card shows "Reachable — Verified: this address was proven to reach this machine". The
pairing dialog now shows QR + copyable link backed by that address. Over the private HTTPS endpoint
the lifecycle verified through the documented API: redeem (`actor_kind=operator_device`) → 200 with
`Secure; HttpOnly; SameSite=Lax` device cookie; device-authenticated reads latch
`X-Compozy-Gateway-Tier: private` (401 `gateway_device_unauthenticated` without); artifact reuse
refused `409 gateway_pairing_spent`; inventory agreed across Web/HTTP/UDS/CLI and the
private-endpoint read; revocation closed live work server-side (`canceled: 1` — an open SSE
catalog-stream was closed mid-flight) before the credential started refusing (401). FAILS on the
product's own pairing UX: opening the pairing link on the second device renders a blank page —
every ES-module asset is refused `403 {"error":"origin not allowed"}` on the forwarded private tier
(BUG-20260911-private-ui-origin-rejected-on-forwarded-tier), so pairing cannot be completed through
the QR/link flow. Also linked: the paired device's credential executes guarded daemon mutations
over the private tier (BUG-20260911-private-tier-loopback-guard-inert). Verdict `fail` — fix both
bugs, then re-walk the link-open pairing flow and the paired session.

Verification walk 2026-09-11 after the fixes (commit `17304fe8f`, lab
`compozy-mobile-surface-truth-verify2-20260911-141455-792161`, fresh authorized key): the
link-open pairing flow now works end to end through the product's own UI — the paired private-tier
SPA boots (`/assets/*` clean; only the expected 401s on unauthenticated `/api` calls), renders the
"Pair this device" gate pre-filled from the `#pair=` fragment, and redeeming lands the full paired
operator session with the `Secure; HttpOnly; SameSite=Lax` device cookie. Single-use semantics
re-verified (artifact reuse → 409; stream-ticket reuse → 401 `gateway_stream_ticket_invalid`;
re-mint per connect), inventory agreed across Web/HTTP/UDS/CLI and the private read, and revocation
again closed a live SSE stream mid-flight (`canceled: 1`) before the credential refused. With both
defects fixed and every observable confirmed, the scenario settles **pass**; the walk key was
deleted after bring-up and the lab runtime purged (zero key-material occurrences by byte-scan).
