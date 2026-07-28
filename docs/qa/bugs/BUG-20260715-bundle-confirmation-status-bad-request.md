# BUG-20260715-bundle-confirmation-status-bad-request: Missing bundle confirmation reports the wrong status

- **Status:** verified
- **Impact (user-side):** Friction
- **Severity:** Medium · **Priority:** P1
- **Persona Affected:** Bruno
- **Journey Step:** J-administer-network-live, activate a Live-requiring bundle
- **Scenarios:** ET-026
- **Found:** 2026-07-15 · **Report:** docs/qa/reports/2026-07-14-network-changes.md

## Summary

Activating a bundle with an unconfirmed Live Network requirement returned `400 Bad Request`, although the public OpenAPI and scenario contract require `409 Conflict`. Operators and agents could not distinguish a valid activation blocked on an explicit decision from malformed input.

## Reproduction

- **Charter:** CH-network-admin-lifecycle · **Tour:** Multi-Tab Tour
- **Environment:** desktop / isolated local daemon / en-US

1. Install an extension whose manifest declares a required Live Network participation block.
2. Preview its bundle and retain the requirement digest.
3. Activate the bundle without `confirm_network_requirement`.

**Expected:** The activation returns `409 Conflict` and names the missing confirmation.
**Actual:** The first real HTTP attempt returned `400 Bad Request`.

## Evidence

- `docs/qa/evidence/2026-07-14-network-changes/ch-network-admin-lifecycle.md`
- Live HTTP retest returned `409 Conflict` without creating an activation.

## Fix

- **Root cause:** `StatusForBundleError` grouped `ErrNetworkRequirementConfirmationRequired` with malformed bundle requests instead of conflict errors.
- **Fix commit:** pending final whole-diff commit.
- **Regression test:** `TestStatusForBundleErrorAndChannelHelpers/live_network_requirement_needs_confirmation` in the canonical API core mapping suite.

## Verification

- **Retested:** 2026-07-15, same persona/journey · **Report:** docs/qa/reports/2026-07-14-network-changes.md
- **Result:** HTTP reports `409`; explicit confirmation activates the bundle and records the current digest, operator, and timestamp.
