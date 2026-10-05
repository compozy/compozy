# BUG-20261005-loop-catalog-ignores-profile: The Loop catalog shows a package disabled in the selected profile

- **Status:** verified
- **Impact (user-side):** Trust-Damage
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Bruno
- **Journey Step:** J-01, disable a developer package for one profile
- **Scenarios:** ET-052
- **Found:** 2026-10-05 · **Report:** docs/qa/reports/2026-10-02-untested.md

## Summary

Bruno disables spec-cycle in resume-editorial, but the Web catalog still offers both bundled
Loops after a fresh load. The selected-profile CLI and HTTP catalogs correctly remove them.
The screen therefore advertises capabilities that the chosen profile no longer provides.

## Reproduction

- **Charter:** CH-spec-cycle-three-loop-lifecycle · **Tour:** Feature Tour
- **Environment:** laptop, 1512x862, DPR 2, fast Wi-Fi, en-US, America/Los_Angeles

1. Select resume-editorial and open the Studio Operations Loop catalog.
2. Run compozy --profile resume-editorial extension disable spec-cycle.
3. Read the same workspace catalog over HTTP with profile=resume-editorial.
4. Reload the Web catalog and inspect its rendered rows and outgoing request.

**Expected:** The two bundled Loops leave this profile's catalog; studio keeps them.
**Actual:** The Web still shows implement-tasks and review-and-fix. Its catalog request
omits profile entirely and receives six rows instead of the selected profile's four.

## Evidence

- docs/qa/evidence/2026-10-02-untested/loops-lifecycle-disabled-web-settled.json and .png
- docs/qa/evidence/2026-10-02-untested/loops-lifecycle-disabled-web-network.json
- docs/qa/evidence/2026-10-02-untested/loops-lifecycle-disabled-http-resume-editorial-loops.json
- docs/qa/evidence/2026-10-02-untested/loops-lifecycle-disabled-http-studio-loops.json
- Scoped tool invocation correctly returns tool_not_found; unscoped operator diagnostics
  are not evidence of a profile permission leak. See loops-lifecycle-disabled-tool-scoped.json
  and loops-lifecycle-disabled-tools-workspace.json.

## Fix

The catalog hook, filter normalizers and query key omit the selected profile. Its loader
and command-palette consumer must share the same scope-preserving catalog boundary.
The extension was restored through the CLI after capture; both profile inventories match
their original identities. No provider run was started. Repair and retest are pending.


The focused repair forwards profile/all_profiles through the existing catalog owner and
keys its cache by that scope. The loader and palette use the same factory. All four red
assertions now pass, with 184 affected/adjacent tests green. Bruno's fresh browser replay
confirms 6 enabled / 4 disabled / 6 in studio / 4 on return / 6 restored after reload,
matching independent UDS reads. See loops-lifecycle-fixed-profile-return-web.json and
loops-lifecycle-fixed-restored-web.json. Required gate and fix commit are pending.

## Verified closure

- Fix commit: 1f497b4fe751696d687960c84b0f8cb5defa2be0
- Final gate: loops-lifecycle-final-delivery-gate-v2.json (all affected lanes green).
- Exact committed hashes: loops-lifecycle-committed-head.json; hook left every checked file unchanged.
- ET-052's profile leave/return walk passed, including fresh browser and independent UDS reads.
- Final compiler-compatible build: index-Cl3pDXHF.js; real-daemon E2E and manual Runs canary pass.

The pending notes above describe intermediate checkpoints; repair and retest are complete.
