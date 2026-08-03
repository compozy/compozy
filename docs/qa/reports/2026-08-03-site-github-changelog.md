# QA Run Report — 2026-08-03 — site-github-changelog

- **Scope:** GitHub-backed changelog index, version detail, evidence, feed, and discovery surfaces
- **Cadence tier:** targeted
- **Build:** `530a78df` + working tree · **Environment:** optimized Next.js build at `http://127.0.0.1:3417`, live GitHub API with authenticated GraphQL
- **Started:** 2026-08-03T17:02:12Z · **Status:** pass

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Bruno | Power User | desktop + 320px / wifi-fast / en-US | CH-site-changelog-release-evidence |

## Flows in Scope

- `J-evaluate-compozy-beta` — a release reader can understand a published beta and trace its evidence (`../journeys/J-evaluate-compozy-beta.md`)

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-site-changelog-release-evidence | J-evaluate-compozy-beta / SITE-changelog-release-receipts | Bruno | Feature Tour | Pass | | |

## Session Debriefs

### CH-site-changelog-release-evidence — Bruno

- **Ran:** 2026-08-03T17:04:00Z → 2026-08-03T17:11:00Z (box respected: yes)
- **Findings:** The formal re-walk passed after a preflight accessibility correction. The index listed exactly `v0.3.0-beta.3`, `.2`, and `.1`; the latest detail preserved its GitHub categories and complete Release Notes, linked 13 merged PRs, attributed three unique human contributors, and exposed 23 assets plus source archives.
- **Bugs filed/updated:** none
- **Scenarios settled:** SITE-changelog-release-receipts → pass
- **Paper cuts:** none in the changed changelog surface
- **Surprises:** the release body uses headings as deep as Markdown level five; the renderer now normalizes these to sequential HTML levels while preserving the structure.
- **Suggested next charter:** re-run after the next GitHub release to measure the five-minute freshness boundary against a real publication timestamp.

Evidence:

- `docs/qa/evidence/2026-08-03-site-github-changelog/index-desktop.png`
- `docs/qa/evidence/2026-08-03-site-github-changelog/detail-desktop.png`
- `docs/qa/evidence/2026-08-03-site-github-changelog/index-mobile.png`
- `docs/qa/evidence/2026-08-03-site-github-changelog/detail-mobile.png`
- `docs/qa/evidence/2026-08-03-site-github-changelog/detail-evidence-full.png`

Edge probes attempted and clean: deep-link refresh; pre-cutoff version URL; 320 px viewport; keyboard
tab order; GitHub PR and contributor destinations; collapsed asset disclosure; RSS; site search; and
the public not-found recovery link.

## What Was Fixed

The first exploratory pass exposed a skipped heading level in deep release notes. The production
renderer now maps GitHub Markdown levels three through six to sequential HTML levels two through
five. `components/blog/__tests__/changelog-components.test.tsx` owns the regression invariant and
passed before the clean re-walk.

## Paper Cuts

None.

## Runtime Errors Observed

Only the expected local absence of Vercel Analytics and Speed Insights scripts; no application,
React, network, or browser errors were observed.

## Human Verifications Needed

None.

## Decisions for a Human

None.

## Learnings

- The GitHub release itself is sufficient as the changelog content system; keeping a second MDX
  receipt made freshness depend on an unrelated post-publication workflow step.
- Production-parity deviation: Chrome was exercised at desktop and 320 px; Safari and Firefox were
  not part of this targeted local pass. The data source was the real GitHub API, not a mock.

## Final Status

- **Exit gate (full automated suite):** tracked by the workstream's content-keyed `make gate-status` record; outside this targeted browser report
- **Issues by user impact:** Blocks-Completion 0 · Data-Loss 0 · Trust-Damage 0 · Friction 0 · Cosmetic 0
- **Coverage:** 1/1 in-scope journey walked; index and detail re-walked after the preflight correction
- **Verdict:** PASS
