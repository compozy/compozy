# QA Run Report — 2026-08-20 — run-agent-terminal-cleanup

- **Scope:** Review remediation for run-agent session cleanup after final failure, node-lane cancellation, and Loop terminalization, with retry reuse as the adjacent canary.
- **Cadence tier:** targeted
- **Build:** `8ac88e33` plus the `fix/run-agent-session-lifecycle` working tree · **Environment:** fresh isolated CLI/runtime/provider lab at `/home/francisross/dev/qa-labs/compozy-run-agent-terminal-cleanup-20260821-010622-057996-lab`; browser unavailable and outside this CLI/runtime charter
- **Started:** 2026-08-20T22:07:24-03:00 · **Status:** closed

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Bruno | Power User | desktop / wifi-fast / en-US | CH-run-agent-terminal-cleanup |

## Flows in Scope

- `J-complete-partial-loop` — Author and complete a routed partial Loop (`../journeys/J-complete-partial-loop.md`).

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-run-agent-terminal-cleanup | J-complete-partial-loop / LP-run-agent-session-lifecycle | Bruno | Feature Tour | Pass | #444, #445 | |
| 2 | CH-run-agent-terminal-cleanup | J-complete-partial-loop / LP-run-agent-output-ownership | Bruno | Feature Tour | Pass | | |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Session Debriefs

### CH-run-agent-terminal-cleanup — Bruno

- **Ran:** 2026-08-21T01:12:08Z → 2026-08-21T01:16:13Z (box respected: yes)
- **Findings:** A live Codex worker produced schema-valid output, and the resulting session became
  stopped after successful settlement. A bounded failure advanced through its configured retry and
  the final escalated attempt's worker became stopped. Canceling an active node lane changed its
  output to canceled and stopped the captured worker. Fresh CLI run reads and independent HTTP
  session reads agreed on each terminal state.
- **Bugs filed/updated:** GitHub #444 and #445.
- **Scenarios settled:** LP-run-agent-session-lifecycle → pass;
  LP-run-agent-output-ownership → pass.
- **Paper cuts:** The first fixture publish omitted the required start surface and the CLI correctly
  refused the run until the definition was republished.
- **Surprises:** Terminal cleanup reports `user_canceled` as its stop reason even after successful or
  failed settlement. State and attachability are correct; cause labeling remains outside this fix.
- **Suggested next charter:** Audit terminal stop-reason semantics separately from binding cleanup.

## What Was Fixed

- Final failed coordinator dispositions close only their run-owned worker binding and queue durable
  stop cleanup.
- Terminal Loop settlement closes live run-agent bindings left by failure or recovery paths.
- Exact node-lane cancellation captures and closes the worker binding before clearing task ownership.
- Retry dispositions remain excluded from terminal cleanup.

## Paper Cuts

- Initial fixture publication omitted a start surface. The public CLI returned
  `start_kind_not_allowed`; publishing version 2 with CLI, UDS, and HTTP starts resolved it.

## Runtime Errors Observed

- No unexpected daemon error occurred. Deliberate worker timeout and node cancellation produced the
  expected terminal dispositions.

## Human Verifications Needed

None identified.

## Decisions for a Human

None identified.

## Learnings

- Final failure proof must use the disposition from the exhausted attempt, not the intermediate retry.
- Node cancellation must capture its task-owned session before clearing the output's `task_run_id`.
- Independent HTTP reads provide a fresh-state check after structured CLI control operations.

## Evidence

- Success: `qa/logs/success-terminal-session.json` — run
  `looprun-7435332441516c1f`, session `sess_3c4fa1984696a96f90fc02fa41f6ec47`.
- Final failure: `qa/logs/failure-terminal-session.json` — run
  `looprun-3a90358907d0edb2`, session `sess_ecba95fd6a065a0f720d0f3895eb27ec`.
- Node cancellation: `qa/logs/canceled-terminal-session.json` — run
  `looprun-b3b122ffd0f31668`, session `sess_b54f0de2b480d51bf6425f01a9ded0e7`.
- Bootstrap manifest: `qa/bootstrap-manifest.json`.
- Teardown: `qa/teardown.json` (`clean: true`).

## Official Skill Audit

Only `skills/compozy/references/loops.md` changed. Every writing-skills checklist item was rechecked:

| Item | Verdict | Item | Verdict |
|---|---|---|---|
| A1 Invocation earned | Pass | A1 Leading word front-loaded | Pass |
| A1 One trigger per branch | Pass | A1 Triggers only | Pass |
| A2 Content typed | Pass | A2 Completion criteria | Pass |
| A2 Disclosure by branch | Pass | A2 Pointers worded for when | Pass |
| A2 Co-location | Pass | A3 Single source of truth | Pass |
| A3 Relevance | Pass | A3 No-op hunt | Pass |
| A3 Negation | Pass | A3 Leading words | Pass |
| B1 Naming | Pass | B1 Description length | Pass |
| B1 Trigger coverage | Pass | B1 Third-person tone | Pass |
| B2 Standard folders | Pass | B2 No human docs | Pass |
| B2 Forward slashes | Pass | B2 Explicit helper paths | Pass |
| B2 No orphans | Pass | B3 Lean body | Pass |
| B3 Imperative mood | Pass | B3 Domain-native terms | Pass |
| B3 CLI design | Pass | B3 Helper roles | Pass |
| B3 Failure states | Pass |  |  |

## Final Status

- **QA exit gate:** Focused public CLI/runtime/provider walk passed. The immutable workstream tree
  proceeds to `make gate-full`; its current evidence is recorded separately by `make gate-status`.
- **Issues by user impact:** Blocks-Completion 0 · Data-Loss 0 · Trust-Damage 2 · Friction 0 · Cosmetic 0
- **Coverage:** 1/1 journeys walked
- **Verdict:** PASS — ready for the workstream completion gate and pull request update.
