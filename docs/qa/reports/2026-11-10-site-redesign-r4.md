# QA Run Report — 2026-11-10 — site redesign R4 walk

- **Scope:** compozy.com redesign on PR #1408 after review fixes R1–R3 and R4 (leftovers, walk fixes), branch `feat/brand-identity-v1-2` (walked on the executor branch `site/fix-r4`, squashed as `d2695346a`).
- **Cadence tier:** targeted
- **Build:** `d2695346a` · **Environment:** `bunx turbo run build --filter=./packages/site`, then `next start -p 4615` with `COMPOZY_SITE_GITHUB_TOKEN`; Chrome through Playwright at 1440×900 and 390×844, dark, reduced motion and JavaScript-off legs where the scenario asks. GitHub-down legs ran on a throwaway copy of the same build on port 4616 with `api.github.com` fetches failing and the prerendered pages aged so ISR regenerated them.
- **Started:** 2026-11-10T13:40Z · **Status:** closed

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Dora | evaluating the beta | desktop 1440 and phone 390, en-US | landing, download, marketplace, changelog, blog, docs |
| Bruno | following releases | desktop 1440 and phone 390 | changelog |
| Ana | building an external app | desktop 1440 and phone 390 | SDK reference |
| Cora | judging the OS claim | desktop 1440 | landing proof |

## Flows in Scope

- `J-evaluate-compozy-beta` — a visitor reads the site, downloads, and finds the docs.
- `J-build-external-app` — a developer learns the client in one language and looks up one method.

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | — | J-evaluate-compozy-beta / ET-site-landing | Dora | draft parity, live numbers | Blocked (human decision) | comparison table content | d2695346a |
| 2 | — | J-evaluate-compozy-beta / ET-site-download | Dora | platforms, copy, GitHub down | Pass | | |
| 3 | — | J-evaluate-compozy-beta / ET-site-marketplace-catalog | Dora | states, jumps, routes | Fixed | catalog jump offset | d2695346a |
| 4 | — | J-evaluate-compozy-beta / ET-site-changelog | Bruno | states, URL state, GitHub down | Fixed | pulse bars overlap | d2695346a |
| 5 | — | J-evaluate-compozy-beta / ET-site-blog | Dora | filters, rail, reading | Fixed | anchor offset, prose leading | d2695346a |
| 6 | — | J-evaluate-compozy-beta / ET-site-docs-shell | Dora | shell, rail, palette | Fixed | palette selection, caption | d2695346a |
| 7 | — | J-evaluate-compozy-beta / ET-site-docs-reference-pages | Dora | CLI and API templates | Pass | | |
| 8 | — | J-build-external-app / ET-site-docs-sdk-reference | Ana | language, filter, deep link | Pass | | |
| 9 | — | J-evaluate-compozy-beta / ET-site-404-og-feeds-llms | Dora | 404, OG, feeds, llms, sweep | Pass | | |
| 10 | — | J-evaluate-compozy-beta / ET-site-docs-search-context | Dora | duplicate titles | Pass | | |
| 11 | — | J-evaluate-compozy-beta / ET-site-docs-api-reference-ui | Dora | API tags, Bundle absent | Pass | | |
| 12 | — | J-evaluate-compozy-beta / REL-os-landing-proof | Cora | claim | Blocked (human decision) | comparison table content | |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

Evidence for every row lives under `.compozy/tasks/site-redesign/qa/R4/<scenario-id>/` (captures as `<state>-<width>-site.jpg` and `-draft.jpg`, and `walk.jsonl` with every measured value, console log and request failure). The regression sweep is `.compozy/tasks/site-redesign/qa/R4/regression-sweep.json`.

## What Was Fixed

### Release count baked as a dash by local production builds
- **Symptom:** the landing's releases stat served "—" after `bunx turbo run build`, until ISR regenerated the page.
- **Root cause:** Turbo's strict environment mode does not pass `COMPOZY_SITE_GITHUB_TOKEN` to the site build, so the build called GitHub anonymously and got 403. Vercel runs `next build` and was not affected.
- **Fix:** `packages/site/turbo.json` passes the token through to `build`; the count now logs why it gives up.
- **Regression test:** none; a rebuild from an empty fetch cache bakes 78 into `index.html`.
- **Retested:** landing at 1440 and 390.

### Pulse bars opened the wrong release
- **Symptom:** at 390 a click on the v0.5.5 bar opened v0.5.4; releases hours apart covered each other at every width.
- **Root cause:** each bar had a fixed 12px hit area and later bars sat on top.
- **Fix:** each hit area stops halfway to its neighbours (`pulseSlots`).
- **Regression test:** `lib/__tests__/changelog-timeline.test.ts` (`pulseSlots`).
- **Retested:** v0.5.5, v0.5.4, v0.5.2, v0.5.1 open themselves at 1440 and 390.

### Anchors and catalog jumps landed 88px below the draft
- **Symptom:** blog headings, changelog sections and the marketplace catalog landed with a gap under the header.
- **Root cause:** the page's `scroll-padding-top` (88px) added to the draft's own scroll margins.
- **Fix:** element margins keep only the offset the draft sets beyond the header.
- **Regression test:** none (CSS offsets); measured against the draft in the browser.
- **Retested:** every marketplace jump equals the draft's position; blog and changelog anchors land at 112px like the draft.

### Docs palette lost its selection after a section filter
- **Symptom:** after choosing a section, Enter did nothing until an arrow key was pressed.
- **Root cause:** cmdk moves its selection only when the query text changes; Chrome detaches the removed row before cmdk can reselect.
- **Fix:** the palette controls the selection and falls back to the first visible result.
- **Regression test:** not possible in jsdom (its cmdk reselects where Chrome does not); browser walk 6 of 6 runs.
- **Retested:** 1440 and 390.

### Smaller draft deviations
- Blog body text used 1.75 line height where the draft renders 1.7; face chips on the landing were 5px taller than the draft; one Mermaid caption showed literal backticks.

## Runtime Errors Observed

- `/_vercel/insights` and `/_vercel/speed-insights` 404 outside Vercel — documented noise.
- Anonymous `api.github.com` 403 for the star count, with its console warning — documented noise; the count hides itself.
- Chrome warns that CSS preloaded for prefetched routes was not used within a few seconds — Next's `<Link>` prefetch, a warning, not an error.
- GitHub-down release page: React's production server-render error is logged; this is the intended throw that keeps a cached release page from being replaced.

## Human Verifications Needed

- [ ] The landing comparison table: replace the "to verify" cells and the `<date>` placeholder, then walk the comparison step of ET-site-landing and REL-os-landing-proof (rows 1 and 12).

## Learnings

- A page-level `scroll-padding-top` and a draft's per-element `scroll-mt` add up; port drafts with the page padding subtracted.
- `turbo run build` in strict mode drops undeclared secrets; a local production build can differ from Vercel's `next build`.

## Final Status

- **Exit gate (focused):** `bunx vitest run` in `packages/site` — 51 files, 364 tests passed; `oxfmt --check`, `oxlint --deny-warnings`, `tsc --noEmit`, react-doctor 100/100, `make source-size`, `check-product-language.sh` all clean.
- **Issues by user impact:** Blocks-Completion 0 · Data-Loss 0 · Trust-Damage 1 (release count) · Friction 2 (pulse bars, palette Enter) · Cosmetic 4
- **Coverage:** 12 of 12 scenarios walked; 10 pass, 2 partial on the owner's comparison-table content.
- **Verdict:** ready with blocked items — the comparison table content waits on the owner.
