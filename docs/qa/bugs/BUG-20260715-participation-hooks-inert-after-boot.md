# BUG-20260715-participation-hooks-inert-after-boot: Participation hooks never run after daemon boot

- **Status:** verified
- **Impact (user-side):** Trust-Damage
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Ada
- **Journey Step:** J-administer-network-live, enforce extension participation policy
- **Scenarios:** ET-network-participation-hooks
- **Found:** 2026-07-15 · **Report:** docs/qa/reports/2026-07-14-network-changes.md

## Summary

A real installed `network.participation.pre_resolve` hook appeared in the public hook catalog but never received participation resolutions. Live requests bypassed the declared extension policy even though the hook was enabled and healthy.

## Reproduction

- **Charter:** CH-network-admin-lifecycle · **Tour:** Multi-Tab Tour
- **Environment:** desktop / isolated local daemon / en-US

1. Install an extension with a synchronous `network.participation.pre_resolve` hook.
2. Make the hook deny a Live session request.
3. Create the session through the public CLI and inspect hook events and the resulting participation snapshot.

**Expected:** The hook denies the request before any participation snapshot persists.
**Actual:** The session resolved Live and no hook dispatch was recorded.

## Evidence

- `docs/qa/evidence/2026-07-14-network-changes/ch-network-admin-lifecycle.md`
- Real retests cover allow, deny, Live-to-Local narrowing, and forbidden Local-to-Live widening with structured hook logs.

## Fix

- **Root cause:** The daemon created and injected the shared participation resolver before `bootHooks`; the wrapper captured a nil hook runtime permanently and was never reattached after hook boot.
- **Fix commit:** pending final whole-diff commit.
- **Regression test:** the canonical daemon boot-hooks suite requires the shared resolver to receive the booted hook runtime.

## Verification

- **Retested:** 2026-07-15, same persona/journey · **Report:** docs/qa/reports/2026-07-14-network-changes.md
- **Result:** deny returns exit 69 with the hook reason; narrow persists Local; widening is rejected; allow persists Live. Dispatch logs name the installed hook on every branch.
