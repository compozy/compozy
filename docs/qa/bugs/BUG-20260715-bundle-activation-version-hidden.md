# BUG-20260715-bundle-activation-version-hidden: Bundle confirmation cannot reject stale operators

- **Status:** verified
- **Impact (user-side):** Trust-Damage
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Bruno
- **Journey Step:** J-administer-network-live, confirm a changed bundle requirement
- **Scenarios:** ET-027; ET-028
- **Found:** 2026-07-15 · **Report:** docs/qa/reports/2026-07-14-network-changes.md

## Summary

The `bundle.activation` resource store had optimistic versions internally, but activation reads omitted `version` and confirmation updates accepted no `expected_version`. The store then reread the newest version immediately before every write, so an operator acting on stale confirmation evidence could never receive the required conflict.

## Reproduction

- **Charter:** CH-network-admin-lifecycle · **Tour:** Multi-Tab Tour
- **Environment:** desktop / isolated local daemon / en-US

1. Read one confirmed bundle activation from two independent operator contexts.
2. Confirm the requirement in the first context.
3. Repeat from the stale second context.

**Expected:** Reads expose the activation version; the first update advances it and the second returns `409 Conflict`.
**Actual:** The original payload exposed no version and both confirmation requests used the store's newest version implicitly.

## Evidence

- `docs/qa/evidence/2026-07-14-network-changes/ch-network-admin-lifecycle.md`
- Live retest: version `1` confirmed to version `2`; a second CLI update at expected version `1` exited 65, and raw HTTP returned `409 Conflict` with `resources: conflict: expected version 1`.

## Fix

- **Root cause:** `resources.Record.Version` was dropped when converting an activation, and `UpdateBundleActivation` performed a fresh read instead of using caller-observed state.
- **Fix commit:** pending final whole-diff commit.
- **Regression tests:** the canonical resource-store suite rejects stale activation writes; the Network-requirement suite proves reconcile bumps a changed digest, clears confirmation, rejects the stale version, and accepts the current version; API/CLI suites cover the public fields and error mapping.

## Verification

- **Retested:** 2026-07-15, same persona/journey · **Report:** docs/qa/reports/2026-07-14-network-changes.md
- **Result:** list/get/preview/activate payloads expose `version`; confirmation PATCH requires `expected_version`; changed digests reconcile to an unconfirmed newer version; stale writers receive `409` without overwriting current evidence.
