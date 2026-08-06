# QA Run Report — 2026-08-05 — implement-tasks

- **Scope:** Hard rename the bundled dev-cycle Loop from `software-delivery` to `implement-tasks`, reduce its public contract to task implementation and collection, and update Web/docs/skill projections.
- **Cadence tier:** targeted
- **Build:** `a00a9df5` + working tree · **Environment:** fresh isolated targeted lab; runtime, CLI, API, and Web required
- **Started:** 2026-08-06T03:22:48Z · **Status:** complete

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Bruno | Power User | desktop / wifi-fast / en-US | CH-implement-tasks-first-run |
| Lea | New User | laptop / wifi-fast / en-US | CH-implement-tasks-dry-run |
| Ada | Power User | structured surfaces / wifi-fast / en-US | CH-implement-tasks-import-parity |
| Dora | Power User | desktop / wifi-fast / en-US | CH-implement-tasks-docs-truth |

## Flows in Scope

- `J-01` — Implement an authored task graph with the default dev-cycle Loop (`../journeys/J-01-arrive-and-use-run.md`).
- `J-02` — Preview the first task implementation round without side effects (`../journeys/J-02-dry-run-preview.md`).
- `J-07` — Use the extension task importer through a structured public surface (`../journeys/J-07-agent-operated-run.md`).
- `J-evaluate-compozy-beta` — Evaluate the public runnable example against the shipped artifact (`../journeys/J-evaluate-compozy-beta.md`).

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-implement-tasks-first-run | J-01 / TA-080 | Bruno | Feature Tour | Pass | | |
| 2 | CH-implement-tasks-first-run | J-01 / LP-001 | Bruno | Feature Tour | Pass | | |
| 3 | CH-implement-tasks-first-run | J-01 / LP-002 | Bruno | Feature Tour | Pass | | |
| 4 | CH-implement-tasks-first-run | J-01 / LP-003 | Bruno | Feature Tour | Pass | | |
| 5 | CH-implement-tasks-first-run | J-01 / LP-046 | Bruno | Feature Tour | Skipped | | |
| 6 | CH-implement-tasks-dry-run | J-02 / LP-006 | Lea | Garbage Tour | Pass | | |
| 7 | CH-implement-tasks-dry-run | J-02 / LP-007 | Lea | Garbage Tour | Pass | | |
| 8 | CH-implement-tasks-import-parity | J-07 / LP-045 | Ada | Feature Tour | Pass | | |
| 9 | CH-implement-tasks-docs-truth | J-evaluate-compozy-beta / ET-site-docs-examples-wave-one | Dora | Feature Tour | Pass | | |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Session Debriefs

- **Bruno:** The bundled catalog exposed only `implement-tasks` and `review-and-fix`. The generated form declared the three intended inputs. A real Codex-backed run imported one task, completed it, updated tracking, and terminated at `collect` with status `done`. LP-046 was skipped because allowed-tool narrowing is unchanged by this contract cut and remains owned by its existing specialized suites.
- **Lea:** Valid dry-runs rendered the five-node generation-one plan without persisting a run. Missing `slug` disabled the Web actions and returned a structured 422 through CLI/UDS without side effects.
- **Ada:** The public extension tool returned the same task payload consumed by `load_tasks`; empty and undeclared inputs returned `tool_invalid_input/schema_invalid` through both HTTP and CLI/UDS paths.
- **Dora:** The public example rendered under the new route with the five standard sections, a shipped maturity badge, the new artifact path, the reduced inputs, and explicit no-final-gate behavior. The retired route resolved to 404 after canonical slash handling.

## What Was Fixed

No QA-session finding has required a fix.

## Paper Cuts

None recorded yet.

## Runtime Errors Observed

None recorded yet.

## Human Verifications Needed

None recorded yet.

## Decisions for a Human

None recorded.

## Learnings

- Journey coverage uses new immutable charters because the prior charters name the retired Loop and remain historical records.
- Taxonomy: journey, functional, experiential, error, and continuity coverage ride the four sessions; viewport changes are skipped because no layout changed, and review-and-fix is the adjacent bundled-Loop canary.
- The isolated lab ended cleanly with no surviving processes: `/Users/pedronauck/dev/qa-labs/compozy-implement-tasks-20260806-032332-797220-lab/qa-artifacts/qa/teardown.json` records `clean: true`.

## Final Status

- **Exit gate (full automated suite):** pass — `make gate-full`
- **Issues by user impact:** Blocks-Completion 0 · Data-Loss 0 · Trust-Damage 0 · Friction 0 · Cosmetic 0
- **Coverage:** 4/4 journeys walked
- **Verdict:** pass.
