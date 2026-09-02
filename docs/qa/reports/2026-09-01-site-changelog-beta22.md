# QA Run Report — 2026-09-01 — Site changelog beta.22

- **Scope:** Restore the published `v0.3.0-beta.22` release notes on GitHub, make its public changelog detail route resilient to a stale cached release collection, and preserve the same notes in the next automated release PR.
- **Cadence tier:** targeted
- **Build:** `2fd4de39b515b55bf1dff32187ed914e73524ab3` · **Environment:** `https://www.compozy.com`, production Vercel deployment; public GitHub Release and release PR
- **Started:** 2026-09-01T23:08:00-03:00 · **Status:** in-progress

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Bruno | Public production site | Desktop and 320 px / wifi-fast / en-US | CH-site-changelog-release-evidence |
| Dora | Public GitHub and release workflow outputs | Desktop / wifi-fast / en-US | CH-release-note-signal |

## Flows in Scope

- `J-evaluate-compozy-beta` — understand the latest beta through its public release evidence (`../journeys/J-evaluate-compozy-beta.md`)
- `J-approve-compozy-beta-candidate` — confirm the release signal survives in public and automated release surfaces without publishing another release (`../journeys/J-approve-compozy-beta-candidate.md`)

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-site-changelog-release-evidence | J-evaluate-compozy-beta / SITE-changelog-release-receipts | Bruno | Feature Tour | Pending | | `2fd4de39b` |
| 2 | CH-release-note-signal | J-approve-compozy-beta-candidate / REL-release-note-signal | Dora | Feature Tour | Pending | | `2fd4de39b` |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Session Debriefs

### CH-site-changelog-release-evidence — Bruno

- **Ran:** 2026-09-01T23:08:00-03:00 → 2026-09-01T23:14:00-03:00 (box respected: yes)
- **Findings:** The index first showed its deployment-time unavailable state, then regenerated within
  the declared five-minute window. A fresh session found beta.22 first, opened the complete detail,
  survived reload, followed PR #498, contributor `pedronauck`, the beta.21 compare, and a macOS asset.
  Header search failed with HTTP 500, so the session cannot pass until deployment and replay.
- **Bugs filed/updated:** BUG-20260901-site-search-catalog-files-missing;
  BUG-20260901-changelog-deploy-prerender-unavailable
- **Scenarios settled:** SITE-changelog-release-receipts → fail, pending governed fix
- **Paper cuts:** None beyond the registered search failure.
- **Surprises:** The static changelog error state observed during deployment was replaced inside the
  promised refresh window; it was not reproducible after regeneration.
- **Suggested next charter:** Repeat the same charter from a fresh session after the tracing fix is live.

## What Was Fixed

### Published beta.22 changelog returned a false not-found page

- **Symptom:** The changelog index listed `v0.3.0-beta.22`, but its dedicated public page rendered “Release not found.”
- **Root cause:** The index and detail route consumed different cached GitHub release snapshots; the oversized 100-release response also exceeded the framework data-cache limit.
- **Fix:** `2fd4de39b` limits the cached collection to 25 releases and performs one bounded, coalesced recent-release refresh when a detail lookup misses the cached collection.
- **Regression test:** `packages/site/lib/__tests__/github-changelog.test.ts` and `packages/site/components/blog/__tests__/changelog-components.test.tsx` failed before the fix and pass after it.
- **Retested:** Pending public persona sessions.

## Paper Cuts

None recorded yet.

## Runtime Errors Observed

- Public `/api/search/?query=v0.3.0-beta.22` returned HTTP 500 because the deployed function lacked
  repository-owned extension catalog files. Registered as
  `BUG-20260901-site-search-catalog-files-missing`.
- Both production deployments initially published the changelog's unavailable state and recovered
  only at the first five-minute regeneration. Registered as
  `BUG-20260901-changelog-deploy-prerender-unavailable`.

## Human Verifications Needed

None.

## Decisions for a Human

None.

## Learnings

- The five-minute publication promise cannot be measured retroactively for beta.22; this run can prove the current public state and records that the next publication must measure the elapsed refresh time live.

## Final Status

- **Exit gate (full automated suite):** Pending final tracked QA write-back.
- **Issues by user impact:** Blocks-Completion 0 · Data-Loss 0 · Trust-Damage 0 · Friction 0 · Cosmetic 0
- **Coverage:** Pending 2 of 2 targeted sessions.
- **Verdict:** not ready — public sessions and exact-head delivery checks are still running.
