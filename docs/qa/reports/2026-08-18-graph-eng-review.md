# QA Run Report — 2026-08-18 — graph-eng review

- **Scope:** Request generation identity, request-context recovery, and route-editor draft coherence
- **Cadence tier:** targeted
- **Build:** `cc07d284` plus the current remediation diff · **Environment:** isolated local browser/runtime lane; request-card scenario uses the production component with Storybook transport fixtures
- **Started:** 2026-08-18T13:50:19Z · **Status:** closed

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Bruno | Power User / Operator | Desktop / wifi-fast / en-US | CH-loop-request-lifecycle, CH-loop-editor-completion |

## Flows in Scope

- `J-supervise-loop-request` — resolve the intended Loop request exactly once (`../journeys/J-supervise-loop-request.md`)
- `J-complete-partial-loop` — author and run a graph whose routed state remains truthful (`../journeys/J-complete-partial-loop.md`)

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-loop-request-lifecycle | J-supervise-loop-request / LP-web-request-answer-card | Bruno | Network Tour | Pass | | |
| 2 | CH-loop-editor-completion | J-complete-partial-loop / LP-web-editor-route-ask | Bruno | Feature Tour | Pass | | |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Session Debriefs

### CH-loop-request-lifecycle — Bruno

- **Ran:** 2026-08-18T14:22:00Z → 2026-08-18T14:55:00Z (box respected: yes)
- **Findings:** none. Two same-node requests from generations 2 and 3 remained separate, and only generation 3 exposed the context retry.
- **Bugs filed/updated:** none.
- **Scenarios settled:** LP-web-request-answer-card → pass.
- **Paper cuts:** none.
- **Surprises:** none.
- **Suggested next charter:** repeat the same identity walk against two live pending generations when the runtime seed exposes that state.

Evidence: `/Users/pedronauck/dev/qa-labs/compozy-graph-eng-review-20260818-141718-102629-lab/qa-artifacts/qa/screenshots/loop-request-repeated-generation.png`.

### CH-loop-editor-completion — Bruno

- **Ran:** 2026-08-18T14:22:00Z → 2026-08-18T14:55:00Z (box respected: yes)
- **Findings:** none. Playwright covered route authoring, connection drop, graph deletion, quick-add, publish, reload, and a daemon-served run.
- **Bugs filed/updated:** none.
- **Scenarios settled:** LP-web-editor-route-ask → pass.
- **Paper cuts:** none.
- **Surprises:** none.
- **Suggested next charter:** retain the existing multi-tab editor charter for concurrent-update coverage.

Evidence: `/Users/pedronauck/dev/qa-labs/compozy-graph-eng-review-20260818-141718-102629-lab/qa-artifacts/qa/screenshots/loop-editor-authored-published.png`.

## What Was Fixed

No defects were discovered during this QA run.

## Paper Cuts

None recorded.

## Runtime Errors Observed

None recorded. The browser console check for the repeated-generation story was clean.

## Human Verifications Needed

None.

## Decisions for a Human

None.

## Learnings

- Request identity must remain the full `(run, generation, node, item)` tuple at every cache, form, URL, and retry boundary.
- Route declarations and displayed edges are one editor contract; every mutation path must reconcile both.
- Production-parity deviation: the repeated-generation request card used the production component and transport adapter with Storybook fixtures; the editor and adjacent run journey also passed against daemon-served Playwright runtime.

## Final Status

- **Exit gate (full automated suite):** `make gate-full`; the fresh `.cache/gate` record is authoritative for release readiness.
- **Issues by user impact:** Blocks-Completion 0 · Data-Loss 0 · Trust-Damage 0 · Friction 0 · Cosmetic 0
- **Coverage:** 2/2 changed journeys walked; adjacent daemon-served Loop run and editor canaries included; Playwright 202 passed, 3 provider/tier-only legs skipped, 0 failed.
- **Verdict:** ready, provided the final `make gate-full` record is green.
