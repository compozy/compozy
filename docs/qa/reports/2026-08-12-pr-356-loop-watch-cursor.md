# QA Run Report — 2026-08-12 — PR 356 loop watch cursor remediation

- **Scope:** Targeted public-surface validation of loop watch-event parking, wake delivery, restart reconciliation, workspace isolation, and the parked read-model after replacing the ambiguous per-run cursor with a durable stream position.
- **Cadence tier:** targeted
- **Build:** `89a51fe9` plus the PR 356 remediation working tree · **Environment:** isolated local production daemon/runtime harnesses with real CLI, HTTP, UDS, and daemon-served Web surfaces; deterministic ACP fixture only for the downstream summary action.
- **Started:** 2026-08-12T16:23:32Z · **Ended:** 2026-08-12T16:46:24Z · **Status:** closed

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Bruno | Delivery Builder | desktop / wifi-fast with one deliberate daemon restart / en-US | CH-022, CH-023 |
| Ada | Autonomous Agent | desktop / wifi-fast / en-US | CH-024 |

## Flows in Scope

- `J-16-watch-events-wake` — park on a durable ledger position, wake on a matching event, preserve workspace isolation, and reconcile a committed gap after restart.

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-022 | J-16 / LP-040, LP-044 | Bruno | Feature Tour | Pass | — | working tree |
| 2 | CH-023 | J-16 / LP-041 | Bruno | Interruption Tour | Pass | — | working tree |
| 3 | CH-024 | J-16 / LP-042 | Ada | Feature Tour | Pass | — | working tree |

## Session Debriefs

### CH-022 — Bruno

- **Ran:** 2026-08-12T16:23:32Z → 2026-08-12T16:26:22Z (bounded remediation walk)
- **Findings:** The run entered `watching`; CLI and HTTP returned byte-equivalent parked subscriptions and cursors; a foreign workspace could not read the run. Matching `task.status_changed` and `task.run.completed` rows woke one coordinator round and the downstream action reached `done`.
- **Scenarios settled:** LP-040 → pass; LP-044 → pass.
- **Paper cuts:** None on the scoped flow.
- **Suggested next charter:** Keep LP-044 in the regular Loop run-detail browser matrix.

### CH-023 — Bruno

- **Ran:** 2026-08-12T16:24:09Z → 2026-08-12T16:24:11Z (restart probe)
- **Findings:** A task status row committed while the daemon was down was detected on restart, delivered once, and advanced the parked loop to `done`.
- **Scenarios settled:** LP-041 → pass.
- **Paper cuts:** None.
- **Suggested next charter:** Repeat after any future parked-output namespace change.

### CH-024 — Ada

- **Ran:** within the CH-022/CH-023 transport walk.
- **Findings:** The same run identity was read through CLI, HTTP, and UDS-backed runtime actions. Structured status preserved `watching`, subscriptions, and stream cursors; the downstream deterministic agent received both matched event kinds.
- **Scenarios settled:** LP-042 → pass.
- **Paper cuts:** None.
- **Suggested next charter:** Extend the parity walk only when a new watch-event family is added.

## What Was Fixed

- Loop watch events now use an explicit never-reused stream position while public run-event SSE keeps its per-run sequence.
- Cursor and match reads share the same workspace, parent-run, kind, and terminal-status population.
- Migration v60 preserves historical positions and atomically rearms legacy parked loop cursors at the new workspace fence.
- Public docs distinguish the watcher replay position from the per-run SSE sequence.

## Evidence

- Runtime bootstrap manifest: `/Users/pedronauck/dev/qa-labs/compozy-pr-356-loop-watch-cursor-targeted-20260812-162332-647529-lab/qa-artifacts/qa/bootstrap-manifest.json`
- Final browser bootstrap manifest: `/Users/pedronauck/dev/qa-labs/compozy-pr-356-loop-watch-cursor-final-20260812-20260812-164239-072983-lab/qa-artifacts/qa/bootstrap-manifest.json`
- Runtime, CLI, HTTP, UDS, matching wake, workspace denial, and restart reconciliation: `/Users/pedronauck/dev/qa-labs/compozy-pr-356-loop-watch-cursor-targeted-20260812-162332-647529-lab/qa-artifacts/qa/watch-events-public-e2e.jsonl`
- Behavioral Web walk: `/Users/pedronauck/dev/qa-labs/compozy-pr-356-loop-watch-cursor-final-20260812-20260812-164239-072983-lab/qa-artifacts/qa/web-watch-events-focused.json`
- Browser screenshot: `/Users/pedronauck/dev/qa-labs/compozy-pr-356-loop-watch-cursor-final-20260812-20260812-164239-072983-lab/qa-artifacts/qa/qa/screenshots/loop-run-watch-events-cursor.png`
- Clean final teardown: `/Users/pedronauck/dev/qa-labs/compozy-pr-356-loop-watch-cursor-final-20260812-20260812-164239-072983-lab/qa-artifacts/qa/teardown.json` (`clean: true`).

## Runtime Errors Observed

- None in the scoped targeted walk.
- A supplemental full Web sweep in the discarded over-broad feature-profile lab ran 137 tests successfully and skipped 3 provider/tier cases. Its first version of the new Loop case correctly rejected an empty seed cursor; the corrected seed passed twice. An unrelated session provider-selector scroll assertion remained reproducibly red and was not changed in this storage remediation.
- The final runtime test-only assertion refinement (real foreign-workspace ID plus unchanged parked state/prompt count) was formatted but not re-run after the operator's immediate close instruction. CI owns that last compile/run check together with the full gate.

## Human Verifications Needed

None.

## Decisions for a Human

None.

## Learnings

- A parked cursor can be omitted at an empty ledger fence, so the browser proof must create one eligible historical loop event before arming the watcher.
- The readable public contract is strongest when one run identity is compared across CLI, HTTP, UDS-backed runtime behavior, and Web.

## Final Status

- **Exit gate (full automated suite):** intentionally delegated to CI after the contributor-branch push. The operator explicitly prohibited local `make gate` / `make gate-full` for this remediation; focused race, migration, runtime, and browser evidence above is the local closeout contract.
- **Issues by user impact:** Blocks-Completion 0 · Data-Loss 0 · Trust-Damage 0 · Friction 0 · Cosmetic 0 in the scoped walk.
- **Coverage:** 4/4 affected scenarios passed; live matching wake, restart gap reconciliation, CLI/API parity, workspace isolation, and browser cursor visibility covered.
- **Parity:** The same parked run contract was exercised through real public CLI, HTTP, UDS/runtime, and daemon-served Web surfaces; no production service was mocked.
- **Verdict:** PASS — locally ready for the parent review; CI owns the full gate after an authorized push.
