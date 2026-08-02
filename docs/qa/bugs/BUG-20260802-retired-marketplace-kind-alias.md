# BUG-20260802-retired-marketplace-kind-alias: Retired Marketplace kind silently opened Skills

- **Status:** verified
- **Impact (user-side):** Trust-Damage
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Ada
- **Journey Step:** J-bundle-product-boundary, probe retired Web routes
- **Scenarios:** ET-bundle-product-surface-absent; ET-web-marketplace-kind-navigation; ET-web-catalog-navigation
- **Found:** 2026-08-02 · **Report:** docs/qa/reports/2026-08-02-bundles-removal.md
- **Origin:** Task 06 real-user QA

## Summary

Opening `/marketplace/bundles` kept the retired URL visible but rendered the Skills catalog. The
silent alias made the removed Bundle product appear to remain routable and gave no truthful
not-found signal.

## Reproduction

1. Open the daemon-served Web UI.
2. Navigate directly to `/marketplace/bundles`.
3. Compare the address, rendered Marketplace kind, and OS notification.

**Expected:** The URL remains unchanged and the OS reports that no app owns the retired path.
**Actual:** The URL remains `/marketplace/bundles`, but Marketplace silently renders Skills.

## Evidence

- Before fix: `/Users/pedronauck/dev/qa-labs/compozy-devtool-oss-launch-20260802-195112-911343-lab/qa-artifacts/qa/browser-marketplace-bundles-absent.png`.
- Final visual verification: `/Users/pedronauck/dev/qa-labs/compozy-devtool-oss-launch-20260802-195112-911343-lab/qa-artifacts/qa/marketplace-bundles-not-found.png`.
- The new Playwright regression failed before the production change at
  `web/e2e/__tests__/marketplace.spec.ts:158` because the not-found message was absent.

## Fix

- **Root cause:** The Marketplace parent route treated an unknown one-segment child as its default
  Skills view.
- **Correction:** A non-parenting TanStack route for `/$kind_` now throws the OS not-found result.
  The trailing underscore preserves valid `/$kind/$entryId` detail routes as siblings.
- **Fix commit:** `7701a3f`
- **Regression test:** The canonical real-daemon Marketplace Playwright suite requires the retired
  path to keep its URL and show `Nothing lives at this address`.

## Verification

- Manual browser replay showed the OS not-found posture, then confirmed that
  `/marketplace/extension/bundles-removal-static-kit` still loaded a valid detail route.
- `make test-e2e-web` passed all 133 tests, including the new retired-kind regression.

