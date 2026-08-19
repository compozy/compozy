# QA Run Report — 2026-08-19 — typed-loop-inputs-remediation

- **Scope:** Review remediation for typed Loop inputs, exact runtime selection, config defaults, and entity-annotated responses.
- **Cadence tier:** targeted
- **Build:** working tree after `cd5d229` · **Environment:** fresh isolated local CLI/API/Web/runtime lab; no provider execution required
- **Started:** 2026-08-19T06:24:29-03:00 · **Status:** in-progress

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Lea | New User | desktop / wifi-fast / en-US | CH-typed-loop-entity-inputs |
| Ada | Power User | desktop / wifi-fast / en-US | CH-compozy-runtime-input-preflight |
| Bruno | Power User | desktop / flaky / en-US | CH-typed-request-entity-answer |

## Flows in Scope

- `J-01` — Start a Loop with exact typed inputs (`../journeys/J-01-arrive-and-use-run.md`)
- `J-02` — Resolve defaults and reject invalid runtime selections before execution (`../journeys/J-02-dry-run-preview.md`)
- `J-supervise-loop-request` — Answer one entity-annotated request without losing recovery (`../journeys/J-supervise-loop-request.md`)

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-typed-loop-entity-inputs | J-01 / LP-select-typed-loop-entities | Lea | Garbage Tour | Pending | | |
| 2 | CH-compozy-runtime-input-preflight | J-02 / LP-loop-input-defaults; LP-runtime-validation-preflight | Ada | Garbage Tour | Pending | | |
| 3 | CH-typed-request-entity-answer | J-supervise-loop-request / LP-answer-typed-request-entities | Bruno | Network Tour | Pending | | |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Session Debriefs

Pending.

## What Was Fixed

### BUG-20260819-empty-runtime-default-rejected: Empty runtime default cannot be saved

- **Symptom:** A valid `{}` runtime default failed at the public config command.
- **Root cause:** The TOML editor rejected explicit empty tables.
- **Fix:** pending remediation commit
- **Regression test:** `internal/config/persistence_test.go` and `internal/daemon/loop_api_runs_test.go`
- **Retested:** pending a fresh J-02 session

### BUG-20260819-composed-request-snapshot-rejected: Composed request schema cannot start

- **Symptom:** A valid entity-annotated request under `allOf` / `items` / `oneOf` failed before the
  request was created.
- **Root cause:** The template-source walker treated YAML-typed schema arrays differently from the
  same arrays after JSON hydration.
- **Fix:** pending current fix commit
- **Regression test:** `internal/loop/coordinator_snapshot_test.go`
- **Retested:** pending a fresh J-supervise-loop-request session

## Paper Cuts

- `table replacement requires at least one key` while saving an empty runtime default — filed as
  `BUG-20260819-empty-runtime-default-rejected`.
- `manifest key ... added during hydration` while starting a composed request — filed as
  `BUG-20260819-composed-request-snapshot-rejected`.

## Runtime Errors Observed

None recorded yet.

## Human Verifications Needed

None.

## Decisions for a Human

None.

## Learnings

Pending.

## Final Status

Pending the isolated walks, strict evidence audit, teardown, and full automated gate.
