# BUG-20260901-site-search-catalog-files-missing: Readers cannot find a release through site search

- **Status:** open
- **Impact (user-side):** Friction
- **Severity:** Medium · **Priority:** P2
- **Persona Affected:** Bruno
- **Journey Step:** J-evaluate-compozy-beta, find the latest release from the public site
- **Scenarios:** SITE-changelog-release-receipts
- **Found:** 2026-09-01 · **Report:** docs/qa/reports/2026-09-01-site-changelog-beta22.md

## Summary

Bruno could read `v0.3.0-beta.22` from the changelog index and dedicated page, but entering the same
version in the public site search returned no result because the search request failed.

## Reproduction

- **Charter:** CH-site-changelog-release-evidence · **Tour:** Feature Tour
- **Environment:** production site, desktop, wifi-fast, en-US

1. Open `https://www.compozy.com/changelog/`.
2. Open `v0.3.0-beta.22` and confirm its dedicated page renders.
3. Enter `v0.3.0-beta.22` in the header search.

**Expected:** Search offers the dedicated beta.22 changelog page.
**Actual:** Search shows no result and its public `/api/search/` request returns HTTP 500.

## Evidence

- `docs/qa/evidence/2026-09-01-site-changelog-beta22/search-error.png`
- A fresh public HTTP read of `/api/search/?query=v0.3.0-beta.22` returned HTTP 500.
- Vercel request logs reported missing `extensions/bridges` and
  `extensions/spec-cycle/extension.json` files in the serverless function.

## Fix

- **Root cause:** The dynamic search route reads repository-owned extension and Skill manifests, but
  Next.js output tracing used the site package as its root and omitted those monorepo files from the
  deployed function.
- **Fix commit:** pending
- **Regression test:** The Next production build trace now contains all 8 bridge manifests, the 16
  Spec Cycle inventory files, and all 10 bundled Skill manifests, with no Spec Cycle Go sources; the
  existing public search and marketplace catalog suites pass. Public replay is pending deployment.

## Verification

- **Retested:** pending deployment
- **Result:** pending
