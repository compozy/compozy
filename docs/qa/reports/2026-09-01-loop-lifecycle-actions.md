# QA Run Report — 2026-09-01 — Loop lifecycle actions

- **Scope:** Include lifecycle-loaded extension tools in Loop schema resolution without leaking them across workspace or Profile boundaries.
- **Cadence tier:** targeted
- **Base:** `34208e9990622ee62e9a5cf114386273ae6abfa0` · **Build:** `9ed228b6b814014f70c2752858c067b3471300a3` · **Environment:** isolated tests plus fresh combined CLI/API/UDS runtime lab
- **Started:** 2026-09-01T20:00:00Z · **Status:** closed

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Bruno | Developer/operator | Local isolated runtime / en-US | Lifecycle action scope |

## Flows in Scope

- Lifecycle action schema projection, authoring parity, execution policy, restart readback, and peer-scope denial.

## Session Matrix & Results

| # | Journey / Scenario | Persona | Status | Issue | Fix commit |
|---|---|---|---|---|---|
| 1 | `LP-extension-action-schema-scope` | Bruno | Pass | Lifecycle tool absent from Loop schema catalog | `45cf683d`, `9ed228b6` |

## Session Debriefs

- Public `ValidateLoop` accepted the lifecycle action in its acting workspace/Profile and rejected the peer workspace with `unknown_action_kind`.
- Create, patch, fork, response projection, and run-start compilation now share that scoped source.
- Real extension-provider suites passed dynamic enablement, profile runtime identity, schema, dispatch, and permission checks.
- A fresh isolated runtime validated, published, ran, and read a Loop through CLI, UDS, and HTTP; the settled run remained `done` after restart.

## What Was Fixed

- **Root cause:** Loop compilation listed tools with operator scope only, omitting workspace/Profile lifecycle providers.
- **Fix:** pass the acting scope through every Loop compile, lint, publish, fork, response, and run-start path.
- **Regressions:** `TestLoopToolSchemaSource` scoped compiler and public API cases.

## Runtime Errors Observed

- The first smoke definition declared only `cli`; UDS correctly returned `start_kind_not_allowed`. Publishing version 2 with `uds` enabled allowed the run. This was fixture correction, not a product defect.

## Human Verifications Needed

- None.

## Final Status

- **Exit gate:** `make gate` passed; focused extension/daemon race suites and strict isolated QA audit passed.
- **Issues:** Blocks-Completion 0 · Data-Loss 0 · Trust-Damage 0 · Friction 0 · Cosmetic 0
- **Coverage:** 1/1 targeted lifecycle action journey.
- **Verdict:** ready pending exact-head provider CI.
