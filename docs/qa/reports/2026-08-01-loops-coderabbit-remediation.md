# QA Run Report — 2026-08-01 — Loops CodeRabbit Remediation

- **Scope:** Durable Loop verdict diagnostics, ratchet provenance validation, generation contracts, and nested JSON/YAML Loop configuration.
- **Cadence tier:** targeted
- **Build:** b52929b plus the uncommitted CodeRabbit remediation · **Environment:** fresh isolated lab from `bootstrap-manifest.json`, public HTTP/CLI/Web runtime, and live Codex provider
- **Started:** 2026-08-02T01:39:11Z · **Status:** PASS

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Bruno | Power User | desktop / wifi-fast / en-US | CH-loop-ratchet-truth, CH-loop-config-file-overrides |

## Flows in Scope

- `J-improve-loop-with-feedback` — Preserve and explain the accepted best candidate after regression (`../journeys/J-improve-loop-with-feedback.md`).
- `J-configure-and-run-loop` — Reuse strict JSON and YAML settings without losing nested values (`../journeys/J-configure-and-run-loop.md`).

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-loop-ratchet-truth | J-improve-loop-with-feedback / LP-ratchet-climb-restore | Bruno | Feature Tour | Fixed | in-session recovery validation regression | uncommitted remediation |
| 2 | CH-loop-config-file-overrides | J-configure-and-run-loop / LP-loop-config-file-snake-case | Bruno | Feature Tour | Pass | | |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Session Debriefs

### Ratchet truth

- **Start:** fresh isolated daemon and the canonical sequential generation-feedback E2E.
- **Actions:** exercised climb, restore, definition-of-done repair, revise, exhaustion, and revision-cap outcomes; then ran a provider-free transform Loop and inspected the same run through CLI, HTTP, SSE, and web.
- **Finding:** an initial interpretation of a review comment required route-cause output status `succeeded`, but rejected gate outputs are valid terminal `failed` records. That change incorrectly terminalized restore and exhaustion runs.
- **Fix and retest:** recovery now accepts terminal `succeeded` or `failed` records with a non-empty verdict, rejects non-terminal records, and all six E2E cases passed.
- **Evidence:** `/Users/pedronauck/dev/qa-labs/compozy-loops-coderabbit-remediation-20260802-013626-435207-lab/qa-artifacts/qa/observed-results.md` and `loop-config-smoke-run.png`.

### Config-file overrides

- **Start:** reviewed JSON and YAML files with nested enabled checks in the fresh lab.
- **Actions:** previewed JSON, persisted YAML, read the effective config again, submitted an unknown field, ran a real provider-free Loop, and asked a live Terra/high session to summarize the reused YAML.
- **Verdict:** nested groups and command values survived both decode paths; scalar limits matched; invalid input exited `1` without mutation; the live session reported iteration cap `5`, `lint`, `unit`, and `integration`.
- **Evidence:** `/Users/pedronauck/dev/qa-labs/compozy-loops-coderabbit-remediation-20260802-013626-435207-lab/qa-artifacts/qa/observed-results.md`.

## What Was Fixed

- Preserved valid rejected gate verdicts during persisted route-cause recovery while retaining validation for pending, running, missing, and malformed outputs.
- Completed the remaining valid CodeRabbit contract, cleanup, diagnostics, ordering, validation, test-safety, generated-contract, and public-schema remediations.

## Paper Cuts

- Vite emitted its existing config-loader warning about a native import without an extension. It did not affect the Loop page or proxy behavior and is outside this remediation.

## Runtime Errors Observed

- The first feedback E2E exposed the route-cause recovery regression described above. Production code was fixed and the complete scenario passed on rerun.
- No daemon, HTTP, SSE, CLI, provider, or browser runtime error remained after the fix.

## Human Verifications Needed

None identified.

## Decisions for a Human

None identified.

## Learnings

- A rejected gate output is stored with terminal status `failed`; recovery must validate terminality and verdict presence, not equate validity with successful outcome.
- YAML must be normalized to JSON-compatible values before strict JSON decoding when a field intentionally carries nested JSON-shaped configuration.

## Compozy Impact Audit

- **Native tools:** Loop run status/list output schemas now require non-empty run IDs; the generated catalog and rejection tests cover the changed descriptors. No tool ID, risk flag, capability gate, or fallback changed.
- **Extensibility and hooks:** Loop gate hook payloads now expose the existing `outcome` contract without the unused `decision`; pre-hook denial precedence is deterministic. No extension registry, bundle, resource, MCP sidecar, or config lifecycle other than Loop file decoding changed.
- **Workspace data isolation:** changed Loop data remains workspace-scoped through the existing CLI/HTTP/UDS/store/SSE paths. The same workspace-bound run ID matched across CLI, HTTP, SSE, and web; no cache or event scope changed.
- **Official Compozy skill:** no impact; checked Loop tool IDs, CLI paths, hook event names, and capability semantics. The public behavior and management paths documented by `skills/compozy/` are unchanged.

## Final Status

PASS. Both affected scenarios pass, the strict evidence audit reports zero blockers, and the broad
iteration `make verify` passed with log `.cache/gate/logs/full-1785636902.log`. The isolated runtime
was closed through the manifest teardown command; `qa-artifacts/qa/teardown.json` records
`"clean": true`. The authoritative workstream-close gate is the current `make gate-status` record
produced after this report's last edit.
