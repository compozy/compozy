# BUG-20260911-private-tier-loopback-guard-inert: A paired remote device can change settings and drain the daemon — the "paired devices can read, not change" enforcement is absent on the private tier

- **Status:** open
- **Impact (user-side):** Trust-Damage
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Iris
- **Journey Step:** J-expose-and-pair-gateway, paired operator session (Truthful Surface Tour)
- **Scenarios:** RT-gateway-operator-surface-truth; RT-gateway-paired-device
- **Found:** 2026-09-11 · **Report:** docs/qa/reports/2026-09-11-mobile-surface-truth.md

## Summary

The product promises paired devices read-only operator access — the UI hides
every write affordance on remote tiers and the loopback-only strip says
"Paired devices can read, not change settings." The server does not enforce
that on the private tier. With only a paired device credential (the HttpOnly
cookie the pairing flow issues), a remote client successfully executed
daemon-authority mutations over the verified private address: `POST /api/drain`
returned 200 and actually drained the daemon (`admission_closed: true`), and a
guarded settings mutation (`PATCH /api/settings/general`) passed the loopback
mutation guard and reached handler validation instead of being refused with
`loopback_mutation_required`. The UI-side hiding (BR-1) is presentation only;
the server-side enforcement the same feature promises (BR-6) is missing.

Root cause direction: the loopback mutation guard is decided once from the
listener's bound host (`handlers.boundHost`), and the private-tier listener is
by design bound to `127.0.0.1` (the provider fronts it), so the guard evaluates
"loopback" and allows every guarded mutation from any paired remote client.

## Reproduction

- **Charter:** CH-gateway-paired-operator-surface · **Tour:** Truthful Surface Tour
- **Environment:** authorized tailscale provider, private tier live; documented pairing redeem issued the device credential; requests sent over the verified private HTTPS address

1. Pair a device (redeem a minted artifact with `actor_kind=operator_device` over the private address) and keep the issued device cookie.
2. `POST https://<private-address>/api/drain` with the device cookie.
3. Observe `200` with `{"state":"draining","admission_closed":true,…}` — the daemon drained.
4. Independently confirm on the local surface that the daemon state changed, then `POST /api/undrain` remotely to restore (also 200).
5. `PATCH https://<private-address>/api/settings/general` with the device cookie — observe `400 settings validation error` (handler validation), not `403 loopback_mutation_required`.

**Expected:** guarded mutations from a paired remote device are refused with
`403 loopback_mutation_required` (the truthful server-side backstop behind the
read-only UI).
**Actual:** guarded mutations execute with the paired device credential; the
enforcement boundary is absent on the private tier.

## Evidence

- Walk transcript reproduced in docs/qa/reports/2026-09-11-mobile-surface-truth.md ("remote-leg walk with authorized provider" addendum, enforcement-gap section)
- Independent read path: local `GET /api/status` confirmed the drained state transition and the restore after remote `undrain`
- Route/composition facts: private-tier server registered with `WithHost("127.0.0.1")`; `loopbackMutationGuard` allows when the bound host is loopback

## Fix

<!-- filled when status moves to fixed -->

## Verification

<!-- filled when status moves to verified -->
