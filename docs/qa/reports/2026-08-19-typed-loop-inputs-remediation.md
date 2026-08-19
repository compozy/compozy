# QA Run Report — 2026-08-19 — typed-loop-inputs-remediation

- **Scope:** Review remediation for typed Loop inputs, exact runtime selection, config defaults, and entity-annotated responses.
- **Cadence tier:** targeted
- **Build:** `56f3033` · **Environment:** fresh isolated local CLI/API/Web/runtime lab; no provider execution required
- **Started:** 2026-08-19T06:24:29-03:00 · **Status:** pass

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
| 1 | CH-typed-loop-entity-inputs | J-01 / LP-select-typed-loop-entities | Lea | Garbage Tour | Pass | | `46dd8ae` |
| 2 | CH-compozy-runtime-input-preflight | J-02 / LP-loop-input-defaults; LP-runtime-validation-preflight | Ada | Garbage Tour | Fixed | BUG-20260819-empty-runtime-default-rejected | `46dd8ae` |
| 3 | CH-typed-request-entity-answer | J-supervise-loop-request / LP-answer-typed-request-entities | Bruno | Network Tour | Fixed | BUG-20260819-composed-request-snapshot-rejected | `4e102c1` |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Session Debriefs

- **Lea:** The run form used catalog-backed enum, agent, secret, and runtime controls while leaving
  the file input as plain text. The runtime field opened the shared searchable model selector, not
  three independent inputs. A stale reviewer was rejected before a run was created.
- **Ada:** An explicit empty runtime object persisted through config storage and read back as `{}`.
  Dry-run reported the exact workspace/global origins and the API returned the same value.
- **Bruno:** The composed request created normally. A stale nested agent failed at `reviewers.0`
  without resolving the request; the exact reviewer then succeeded through CLI and Web.

## What Was Fixed

### BUG-20260819-empty-runtime-default-rejected: Empty runtime default cannot be saved

- **Symptom:** A valid `{}` runtime default failed at the public config command.
- **Root cause:** The TOML editor rejected explicit empty tables.
- **Fix:** `46dd8ae`
- **Regression test:** `internal/config/persistence_test.go` and `internal/daemon/loop_api_runs_test.go`
- **Retested:** passed through config read, API defaults, and CLI dry-run

### BUG-20260819-composed-request-snapshot-rejected: Composed request schema cannot start

- **Symptom:** A valid entity-annotated request under `allOf` / `items` / `oneOf` failed before the
  request was created.
- **Root cause:** The template-source walker treated YAML-typed schema arrays differently from the
  same arrays after JSON hydration.
- **Fix:** `4e102c1`
- **Regression test:** `internal/loop/coordinator_snapshot_test.go`
- **Retested:** passed through runtime snapshot, CLI rejection/acceptance, and Web submission

## Paper Cuts

- `table replacement requires at least one key` while saving an empty runtime default — filed as
  `BUG-20260819-empty-runtime-default-rejected`.
- `manifest key ... added during hydration` while starting a composed request — filed as
  `BUG-20260819-composed-request-snapshot-rejected`.

## Runtime Errors Observed

- The first composed-request start failed with `manifest key ... added during hydration`; this was
  the production defect fixed in `4e102c1`. No runtime errors remained in the clean re-walk.
- The Go binary's embedded Web bundle represented an older build, so Web evidence was captured from
  this worktree's Vite server against the same isolated daemon and proxy target.

## Human Verifications Needed

None.

## Decisions for a Human

None.

## Learnings

- Snapshot source manifests must traverse YAML-typed schema slices and their JSON-hydrated form
  identically.
- Empty objects are present values for partial runtime contracts; storage must not collapse them
  into missing config.
- Real Web QA must use the current worktree build when the development binary embeds a released Web
  asset package.

## Final Status

All four affected scenarios passed. The lab journey log covers CLI, API, Web, and runtime surfaces;
screenshots prove the shared runtime selector and nested agent selector. Automated close evidence is
recorded separately by `make gate-full`, and the lab's `teardown.json` records process cleanup.
