# BUG-20260901-changelog-deploy-prerender-unavailable: A fresh deploy temporarily replaces the changelog with an error

- **Status:** open
- **Impact (user-side):** Trust-Damage
- **Severity:** Medium · **Priority:** P2
- **Persona Affected:** Bruno
- **Journey Step:** J-evaluate-compozy-beta, open the public release archive
- **Scenarios:** SITE-changelog-release-receipts
- **Found:** 2026-09-01 · **Report:** docs/qa/reports/2026-09-01-site-changelog-beta22.md

## Summary

Immediately after each production deploy, Bruno saw “The release archive could not be loaded” in
place of every release. The index recovered at its five-minute regeneration boundary, but every
unrelated site deployment could publish the same misleading state again.

## Reproduction

- **Charter:** CH-site-changelog-release-evidence · **Tour:** Feature Tour
- **Environment:** production site, desktop, wifi-fast, en-US

1. Deploy the site to production.
2. Open `https://www.compozy.com/changelog/` before the first five-minute regeneration.
3. Open the same URL after regeneration.

**Expected:** The first request after deployment reads the cached GitHub data at runtime and lists
the published releases.
**Actual:** The deployment serves a build-time error page until the first regeneration, then lists
the same releases correctly.

## Evidence

- `docs/qa/evidence/2026-09-01-site-changelog-beta22/index-deploy-unavailable-reproduced.png`
- The behavior reproduced on both production deployments in this run and recovered at the next ISR
  regeneration.

## Fix

- **Root cause:** The route exported a five-minute page revalidation value, so Next.js prerendered
  the GitHub-backed index during the build and shipped the transient unavailable result as static
  HTML.
- **Fix commit:** pending
- **Regression test:** The production build must classify `/changelog` as dynamic while the GitHub
  fetch keeps its explicit five-minute data cache. Public first-request replay is pending deployment.

## Verification

- **Retested:** pending deployment
- **Result:** pending
