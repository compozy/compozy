# QA Run Report — 2026-08-15 — triggers-ui

- **Scope:** Rebased Jobs/Triggers catalogs and redesigned trigger detail rule page, including workspace-safe Loop destinations and review remediations.
- **Cadence tier:** targeted
- **Build:** rebased review-remediation commit containing this report on `9296ed7e` · **Environment:** final fresh isolated daemon at `127.0.0.1:64831`; Web at `localhost:4177`; Chromium; no mocked product data
- **Started:** 2026-08-15T17:12:05Z · **Status:** passed

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Bruno | Power User | desktop / wifi-fast / en-US | CH-trigger-detail-rule-page |

## Flows in Scope

- `J-24` — Find, inspect, and manage automation while catalog and detail state remain truthful (`../journeys/J-24-triage-work-at-scale.md`).

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-trigger-detail-rule-page | J-24 / ET-web-trigger-detail-rule-page | Bruno | Feature Tour | Fixed | BUG-20260815-trigger-detail-duplicate-key | This report's commit |
| 2 | CH-trigger-detail-rule-page | J-24 / ET-web-jobs-triggers-catalog | Bruno | Feature Tour | Pass | | |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Session Debriefs

### CH-trigger-detail-rule-page — Bruno (first walk)

- **Ran:** 2026-08-15T17:18:06Z → 2026-08-15T17:23:19Z (box respected: yes)
- **Findings:** The catalog, webhook rule, diagnostics, sample envelope, and persisted enable switch worked. The detail emitted a duplicate-key reconciliation warning on every rerender (Cosmetic; no visible omission in this walk).
- **Bugs filed/updated:** BUG-20260815-trigger-detail-duplicate-key
- **Scenarios settled:** ET-web-trigger-detail-rule-page → fail pending governed fix; catalog walk remains open
- **Paper cuts:** none
- **Surprises:** Clearing the search with a synthetic empty fill did not emit an input event; real keyboard input correctly cleared `?q=Deploy`, so this was a driver artifact, not a product finding.
- **Suggested next charter:** Re-run this charter from a fresh browser after the bounded fix, including managed trigger and Jobs canary.

### CH-trigger-detail-rule-page — Bruno (fresh-session retest)

- **Ran:** 2026-08-15T17:25:08Z → 2026-08-15T17:34:10Z (box respected: yes)
- **Findings:** The fixed detail rendered without application console errors. Dynamic and managed enable switches persisted after reload; managed actions stayed hidden; Inspect, diagnostics, sample envelope, search clear, Jobs canary, and route history behaved as specified.
- **Bugs filed/updated:** BUG-20260815-trigger-detail-duplicate-key → verified
- **Scenarios settled:** ET-web-trigger-detail-rule-page → pass; ET-web-jobs-triggers-catalog → pass
- **Edge probes:** Refresh, direct deep link, back/forward, malformed trigger id, keyboard/Escape, and 320x800 viewport passed. The browser driver did not expose an observable zoom change, so the compact viewport supplied the stronger constrained-layout probe.
- **Six-lens result:** Usability, accessibility quick pass, performance responsiveness, recoverability, and local production-data parity passed. Compatibility covered current Chromium at desktop and compact widths; Safari and Firefox were outside this targeted run.

### CH-trigger-detail-rule-page — Bruno (post-rebase canary)

- **Ran:** 2026-08-15T18:00:28Z → 2026-08-15T18:03:10Z (box respected: yes)
- **Findings:** A new isolated lab bootstrapped the final schema head, registered a workspace, and created a webhook trigger plus Job through the real public API. The catalog, trigger detail, and Inspect sheet rendered on Web port 4177 without application console errors.
- **Scenarios confirmed:** ET-web-trigger-detail-rule-page → pass; ET-web-jobs-triggers-catalog → pass
- **Isolation:** The predecessor lab was torn down with `clean: true` before this final lab started; no process used port 3000.

## What Was Fixed

### BUG-20260815-trigger-detail-duplicate-key: Trigger detail reports duplicate UI identity

- **Symptom:** React reports duplicate sibling identity on trigger detail rerenders.
- **Root cause:** Delete and Inspect overlays used the same raw trigger id as sibling keys.
- **Fix:** Delete and Inspect overlays now have distinct React keys in the commit containing this report.
- **Regression test:** `web/src/systems/automation/components/__tests__/trigger-detail-panel.test.tsx`
- **Retested:** Pass in a fresh browser session on Web port 4177; no duplicate-key or other application console errors.

## Paper Cuts

None recorded.

## Runtime Errors Observed

- First walk: React duplicate-key warning for `trg-684b2d3ed23e7196`, filed as BUG-20260815-trigger-detail-duplicate-key.
- Fresh-session retest: no application console errors; only the development-only React DevTools information message appeared.

## Human Verifications Needed

None.

## Decisions for a Human

None.

## Learnings

- Real keyboard input must be used when checking search-query clearing; synthetic empty fill did not emit the same input sequence.
- Keeping the Web lab on a dedicated port avoids interfering with the operator's process on port 3000.
- A post-rebase QA pass must use a new bootstrap when the schema head changes; reusing a database bootstrapped at an older alpha head is outside the greenfield state contract.

## Final Status

- **Exit gate (full automated suite):** `make gate-full` (`make verify`) — **PASS**; evidence: `/Users/pedronauck/dev/qa-labs/compozy-triggers-ui-post-rebase-20260815-175926-780764-lab/qa-artifacts/qa/logs/final-make-verify.log`; current fingerprint recorded by `make gate-status`.
- **Issues by user impact:** Blocks-Completion 0 · Data-Loss 0 · Trust-Damage 0 · Friction 0 · Cosmetic 1 found / 1 fixed / 1 verified
- **Coverage:** 1/1 journeys walked; 2/2 scenarios settled
- **Verdict:** **PASS** — the isolated user walk, governed fix loop, fresh post-rebase retest, and full automated gate are green.
