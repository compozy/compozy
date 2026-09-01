# QA Run Report — 2026-09-01 — Issue 506 filtered fan-out roster

- **Scope:** Correct sparse filtered/batched fan-out roster rows and their derived progress across run-read surfaces.
- **Cadence tier:** targeted
- **Build:** e96962c + QA/test working tree · **Environment:** isolated targeted lab
- **Started:** 2026-09-01T13:08:59Z · **Status:** complete; delivery CI pending

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Bruno | Power User | desktop / wifi-fast / en-US | CH-loop-graph-runtime-safety |
| Ada | Power User | desktop / flaky / en-US | CH-loop-legibility-run-read-resume |
| Dora | Power User | desktop / wifi-fast / en-US | CH-loop-legibility-operator-register |

## Flows in Scope

- `J-complete-partial-loop` — run a filtered fan-out and trust that only materialized lanes exist (`../journeys/J-complete-partial-loop.md`).
- `J-operate-loop-run-headless` — read the same roster and progress through structured surfaces (`../journeys/J-operate-loop-run-headless.md`).
- `J-diagnose-loop-run-operator` — inspect the fan-out and its server-owned rollup in Web (`../journeys/J-diagnose-loop-run-operator.md`).

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-loop-graph-runtime-safety | J-complete-partial-loop / LP-fan-out-filtering | Bruno | Feature Tour | Fixed | BUG-20260901-filtered-fanout-phantom-rows | e96962c |
| 2 | CH-loop-legibility-run-read-resume | J-operate-loop-run-headless / LP-run-read-agent-journey; LP-runs-roster-server-ordering | Ada | Network Tour | Fixed | BUG-20260901-filtered-fanout-phantom-rows | e96962c |
| 3 | CH-loop-legibility-operator-register | J-diagnose-loop-run-operator / LP-web-run-operator-register | Dora | Interrupt Tour | Fixed | BUG-20260901-filtered-fanout-phantom-rows | e96962c |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Session Debriefs

### CH-loop-graph-runtime-safety · Bruno

- Ran a real Loop whose filter retained only source index `2`.
- The terminal CLI roster contained exactly one `process_record[2]` row and no phantom pending row.
- Goal reached: yes. True end state: confirmed by a fresh terminal roster read.

### CH-loop-legibility-run-read-resume · Ada

- Ran a second Loop whose filter retained sparse indexes `2` and `4`.
- CLI, HTTP, and UDS returned those exact identities and the same `2/2` rollup; the runs list and
  `compozy__loop_runs` reported complete `3/3` progress.
- Goal reached: yes. True end state: confirmed across four public read paths.

### CH-loop-legibility-operator-register · Dora

- Opened a live sparse run in headed Chrome, selected the workspace through the public picker, and
  inspected Graph and Nodes.
- Graph showed `2 of 2 done`; the two worker record links ended in `.2` and `.4`.
- Reload preserved `2 of 2 done`, then the pending answer was submitted and the run reached Done.
- Goal reached: yes. True end state: confirmed by Web reload and a fresh CLI roster read.

## What Was Fixed

### BUG-20260901-filtered-fanout-phantom-rows: Successful filtered fan-out looks unfinished

- **Symptom:** A successful filtered fan-out reports pending rows and an unclosable progress fraction.
- **Root cause:** Roster projection expands from zero through the maximum output index instead of projecting exact output identities.
- **Fix:** e96962c
- **Regression test:** `internal/loop.TestRosterContract/Should preserve only materialized fanout item indexes` — red before, green after.
- **Retested:** all three isolated sessions passed.

## Paper Cuts

None.

## Runtime Errors Observed

None.

## Human Verifications Needed

None.

## Decisions for a Human

None.

## Learnings

- A hidden headless tab intentionally disables live Web reads, so Web evidence used headed Chrome.
  This is a test-driver condition, not a product defect; normal desktop visibility was confirmed.
- The focused Web pass covered system status, literal state words, keyboard workspace selection,
  refresh recovery, and the server-owned rollup. Responsive QA remains out of scope because Inspect
  is desktop-only by product decision.

## Final Status

- **Exit gate (scoped pre-push):** `make gate` passed — Go lint/race tests and Web lint/typecheck/tests.
- **Delivery gate:** exact-head draft PR CI pending.
- **Issues by user impact:** Blocks-Completion 0 · Data-Loss 0 · Trust-Damage 0 · Friction 0 · Cosmetic 0
- **Coverage:** 3/3 journeys walked; CLI, HTTP, UDS, native tool, and Web covered.
- **Evidence:** `/Users/pedronauck/dev/qa-labs/compozy-issue-506-filtered-fanout-roster-20260901-131013-477371-lab/qa-artifacts/qa/`.
- **Teardown:** `qa/teardown.json` reports `clean: true` with no survivors.
- **Verdict:** not ready for merge until exact-head required CI is green; local implementation and QA are ready.
