# QA Run Report — 2026-08-14 — review-handoff-spec-cycle

- **Scope:** Targeted verification of the spec-cycle three-Loop lifecycle, the orchestrate-tasks public example, and the bundled Marketplace inventory after one deep-review remediation round.
- **Cadence tier:** targeted
- **Build:** `583a7f66` plus the current remediation working tree · **Environment:** fresh isolated lab at `http://127.0.0.1:58645`; local daemon, Web, and site processes; no provider-backed run in scope.
- **Started:** 2026-08-14T22:40:28Z · **Status:** behavioral-pass

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Bruno | Power User | desktop / wifi-fast / en-US | CH-spec-cycle-three-loop-lifecycle |
| Dora | Runtime Administrator | desktop / wifi-fast / en-US | CH-spec-cycle-public-inventory |

## Flows in Scope

- `J-01` — Disable and restore the bundled extension as one runtime unit (`../journeys/J-01-arrive-and-use-run.md`).
- `J-evaluate-compozy-beta` — Follow public documentation and Marketplace truth (`../journeys/J-evaluate-compozy-beta.md`).

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-spec-cycle-three-loop-lifecycle | J-01 / ET-052 | Bruno | Feature Tour | Pass | | |
| 2 | CH-spec-cycle-public-inventory | J-evaluate-compozy-beta / ET-site-docs-examples-wave-one | Dora | Feature Tour | Pass | | |
| 3 | CH-spec-cycle-public-inventory | J-evaluate-compozy-beta / ET-site-marketplace-bridges-bundled | Dora | Feature Tour | Pass | | |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Session Debriefs

### Bruno — bundled extension lifecycle

- Disabled and re-enabled `spec-cycle` through the public CLI, then read independent CLI and HTTP
  catalogs at each boundary.
- All three Loops, three bundled agents, and three extension tools left and returned together. The
  Web Loops window showed the disabled empty state, then all three restored entries after refresh and
  full-page reload.
- No provider session was started; the charter tests catalog lifecycle, not Loop execution.
- Lens pass: lifecycle state, catalog consistency, reload persistence, empty state, runtime errors,
  and basic desktop interaction all passed in Chromium.

### Dora — public Examples and bundled Marketplace inventory

- Opened the six-entry Examples index and followed the public orchestrate-tasks card. The rendered
  artifact remained copyable and showed both strict task-frontmatter parsing and terminal worker-state
  checks after reload.
- Followed the walkthrough's Marketplace link. The detail showed three Loops, nine skills, three
  agents, three tools, provenance, and `compozy extension status spec-cycle` without an install claim.
- Public search resolved “Spec Cycle” to the same detail route. Refresh, back/forward navigation, and
  an unknown bundled slug behaved correctly.
- Lens pass: information accuracy, navigation, recovery, copy affordance, empty/error route behavior,
  and desktop Chromium compatibility passed. Safari, Firefox, and mobile were outside this targeted run.

## What Was Fixed

No in-session defects were found or fixed.

## Paper Cuts

None recorded in the target flows.

## Runtime Errors Observed

None recorded in the target flows.

## Human Verifications Needed

None.

## Decisions for a Human

None.

## Learnings

- The bundled package behaves as one lifecycle unit across CLI, HTTP, and Web projections.
- The shared bundled detail path is reflected consistently by walkthrough links, public search, reload,
  and browser history.

## Final Status

Behavioral verdict: **PASS**. All three sessions passed and the isolated runtime was torn down with
`teardown.json` reporting `"clean": true` and no survivors. Workstream completion additionally requires
a current passing `make gate-full` evidence record for this exact tree.
