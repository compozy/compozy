# QA Run Report — 2026-08-15 — PR 409 empty states

- **Scope:** PR #409 remediation for Tasks, Jobs, and Triggers zero-inventory states
- **Cadence tier:** targeted
- **Build:** working tree at `4598eb67` · **Environment:** isolated live daemon + production-parity Web; manifest `/Users/pedronauck/dev/qa-labs/compozy-pr409-empty-states-20260815-050745-398663-lab/qa-artifacts/qa/bootstrap-manifest.json`
- **Started:** 2026-08-15T02:08:02-03:00 · **Status:** passed

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Bruno | Power User | desktop / wifi-fast / en-US | CH-empty-catalog-first-use |

## Flows in Scope

- `J-start-from-empty-catalogs` — understand every empty catalog and reach only real next actions (`../journeys/J-start-from-empty-catalogs.md`)

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-empty-catalog-first-use | J-start-from-empty-catalogs / TA-web-tasks-zero-inventory-templates | Bruno | Feature Tour | Pass | | |
| 2 | CH-empty-catalog-first-use | J-start-from-empty-catalogs / TA-web-jobs-zero-inventory-suggestions | Bruno | Feature Tour | Pass | | |
| 3 | CH-empty-catalog-first-use | J-start-from-empty-catalogs / TA-web-triggers-zero-inventory-intro | Bruno | Feature Tour | Pass | | |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Session Debriefs

Bruno walked the three catalogs in a fresh isolated workspace. Tasks exposed the four real templates, expanded by pointer and keyboard, opened the existing editor, and kept filtered, loading, and error states distinct. Jobs rendered four live suggestions only while the catalog was empty, kept pointer and keyboard disclosure usable, persisted Dismiss and Create job through the daemon, suppressed suggestions after the accepted job survived refresh, and did not render suggestions in Global scope. Triggers rendered no invented suggestions, opened the existing editor, and preserved filtered, loading, and error states.

The accepted Job and the remaining pending suggestions were independently read through the public HTTP API and structured CLI output. Evidence is indexed in the isolated lab journey log and under `docs/qa/evidence/2026-08-15-pr409-empty-states/`.

## What Was Fixed

- Corrected the Tasks CLI handoff to the real `compozy task create` command.
- Preserved zero-inventory precedence while preventing filtered or populated Jobs catalogs from preloading or rendering suggestions.
- Kept suggestion fixtures and handlers isolated by workspace and status.
- Reused the shared disclosure primitive and moved template facts into the template contract.
- Added interaction coverage for pointer and keyboard disclosure plus real CTA routing.

## Paper Cuts

None.

## Runtime Errors Observed

The daemon was deliberately stopped inside the isolated lab to verify loading and error behavior. The resulting `502` responses were expected, stayed confined to the lab, and recovered after the daemon restarted. No unexpected browser or daemon failure was observed during the normal flows.

## Human Verifications Needed

None planned.

## Decisions for a Human

None planned.

## Learnings

Zero-inventory suggestions must be subordinate to the unfiltered Jobs inventory result. Treating the Jobs query as the gate avoids both suggestion flashes and unnecessary suggestion requests on filtered or populated routes.

## Final Status

- **Exit gate (full automated suite):** `make gate-full` (`make verify`) — PASS
- **Gate evidence:** `/Users/pedronauck/dev/qa-labs/compozy-pr409-empty-states-20260815-050745-398663-lab/qa-artifacts/qa/evidence/final-make-verify.log`
- **Issues by user impact:** Blocks-Completion 0 · Data-Loss 0 · Trust-Damage 0 · Friction 0 · Cosmetic 0
- **Coverage:** 1/1 journeys walked · 3/3 scenarios passed
- **Verdict:** PASS — fresh isolated Web/API/CLI evidence confirms the repaired empty-state behavior.
