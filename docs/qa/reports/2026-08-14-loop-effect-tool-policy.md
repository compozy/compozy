# QA Run Report — 2026-08-14 — loop-effect-tool-policy

- **Scope:** Daemon-owned terminal Loop tool effects for compozy/compozy#403.
- **Cadence tier:** targeted
- **Build:** `5873aaea`, containing the patch-equivalent daemon fix `61f3bfc2` (`b3c0087f` after rebase) · **Environment:** fresh isolated lab; local production builds; system Chrome 151 fallback because the pinned Playwright browser download was unavailable
- **Started:** 2026-08-14T22:00:58-03:00 · **Status:** closed

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Lea | Loop author and operator | laptop / wifi-fast / en-US | CH-author-loop-failure-contract |

## Flows in Scope

- `J-recover-loop-node-failure` — Author, run, repair, and finish a Loop (`../journeys/J-recover-loop-node-failure.md`)

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-author-loop-failure-contract | J-recover-loop-node-failure / LP-terminal-outcome-notification | Lea | Feature Tour | Fixed | #403 | `b3c0087f` |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Session Debriefs

### CH-author-loop-failure-contract — Lea

- **Ran:** 2026-08-14T22:04:41-03:00 → 2026-08-14T22:17:05-03:00 (box respected: yes)
- **Findings:**
  - A same-workspace daemon-owned terminal effect completed after `Done` with a durable `outcome: ok` result and delivery ID.
  - A foreign-workspace effect remained denied with `tool_denied`, while the committed Loop remained `Done`.
  - The public agent catalog did not expose a synthetic `loop-effect` agent.
  - Both durable results remained observable through the structured reads and the visual timeline after reload.
- **Bugs filed/updated:** compozy/compozy#403
- **Scenarios settled:** LP-terminal-outcome-notification → Fixed
- **Paper cuts:** Closing the browser produced a development-console stream error during transport teardown; no error appeared while the page was mounted or after reload.
- **Surprises:** The full verifier exposed an unrelated release-test dependency on the operator's global `tag.gpgSign=true`; the same two failures reproduced on clean `main`.
- **Suggested next charter:** Run a real extension delivery canary and verify its terminal callback reaches the orchestrator without polling.

## What Was Fixed

### compozy/compozy#403: Loop tool effects fail under the daemon-owned execution scope

- **Symptom:** A committed terminal tool effect failed before invocation because the synthetic `loop-effect` audit label was resolved as an authored agent.
- **Root cause:** Native policy and workspace-input authorization did not preserve daemon-owned scope semantics.
- **Fix:** `b3c0087f`
- **Regression test:** `internal/daemon/loop_effect_relay_test.go`, `internal/daemon/native_tools_test.go`, and `internal/daemon/loop_node_lifecycle_e2e_integration_test.go`
- **Retested:** Fresh isolated CLI, runtime, API/SSE, and visual journey; permitted and denied effects behaved as designed.

## Paper Cuts

| Persona | Where (journey/step) | Felt | Sharpness | Outcome |
|---|---|---|---|---|
| Lea | Browser teardown after the visual journey | Development console briefly logged a stream failure only after the browser process closed | dull | disclosed; not user-visible during the run |

## Runtime Errors Observed

- A `Loop stream failed` message was emitted only while the browser process was closing. Mounted-page and post-reload console captures were empty, and no HTTP response was `>=400`.

## Human Verifications Needed

- None currently identified.

## Decisions for a Human

- None currently identified.

## Learnings

- The daemon-owned scope can authorize a terminal native effect without making its audit label a public or authored agent.
- The visual read path used a separately stacked Web presentation fix. That code and compozy/compozy#405 are outside this branch.

## Final Status

- **Exit gate (full automated suite):** `mise exec -- make verify` — BLOCKED by two pre-existing `internal/config` release-fixture failures under global `tag.gpgSign=true`; the identical focused failures reproduce on clean `main`. All preceding frontend checks, 4,877 Bun tests, Web build, Go lint, and 22,266 other Go tests passed.
- **Issues by user impact:** Blocks-Completion 0 · Data-Loss 0 · Trust-Damage 0 · Friction 0 · Cosmetic 0
- **Coverage:** 1/1 targeted journey walked across CLI, runtime, API/SSE, and Web. The Web view was supplied by the separately stacked presentation branch; the daemon-owned tool policy and durable structured result are the verdict under test here.
- **Evidence:** `docs/qa/evidence/2026-08-14-loop-effects-combined/success-reload.png`; `docs/qa/evidence/2026-08-14-loop-effects-combined/denied-reload.png`. The isolated audit verdict was `pass`; teardown recorded `clean: true` with no surviving processes.
- **Verdict:** ready with blocked items — compozy/compozy#403 passed its targeted structured and visual acceptance checks; resolve the unrelated release-fixture signing isolation before claiming a fully green repository gate.
