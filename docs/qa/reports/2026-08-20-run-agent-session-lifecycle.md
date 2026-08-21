# QA Run Report — 2026-08-20 — run-agent-session-lifecycle

- **Scope:** Run-agent session lineage and terminal cleanup for session-started Loops.
- **Cadence tier:** feature
- **Build:** `9eb83ca4` plus the `fix/run-agent-session-lifecycle` working tree · **Environment:** fresh isolated local lab; CLI, HTTP/UDS, runtime, and a live Batuta provider session required
- **Started:** 2026-08-20T18:50:50-03:00 · **Status:** closed

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Bruno | Power User | desktop / wifi-fast / en-US | CH-run-agent-session-lifecycle |

## Flows in Scope

- `J-complete-partial-loop` — Author and complete a routed partial Loop (`../journeys/J-complete-partial-loop.md`).

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-run-agent-session-lifecycle | J-complete-partial-loop / LP-run-agent-session-lifecycle | Bruno | Feature Tour | Pass | #444, #445 | |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Session Debriefs

### CH-run-agent-session-lifecycle — Bruno

- **Ran:** 2026-08-20T22:11:30Z → 2026-08-20T22:17:23Z (box respected: yes)
- **Findings:** Batuta started both Loops through the native tool. Active worker reads from CLI and
  HTTP agreed on the Batuta parent and root IDs. Successful settlement stopped the worker without
  stopping Batuta. An invalid first result reused the same active worker for recovery and stopped it
  only after the schema-valid result settled.
- **Bugs filed/updated:** GitHub #444 and #445.
- **Scenarios settled:** LP-run-agent-session-lifecycle → pass.
- **Paper cuts:** The Batuta gate asked for the already stored `false` preference once before its
  structured reread; the operator answered the normal product question and dispatch continued.
- **Surprises:** The terminal worker's HTTP projection reports the stop as `user_canceled`, although
  the daemon initiated cleanup after successful Loop settlement. Session state and attachability are
  correct; the cause label is outside this fix's scope.
- **Suggested next charter:** Inspect the terminal stop-reason copy independently from lifecycle
  ownership.

## What Was Fixed

- Run-agent execution now records the nearest originating session as parent lineage without borrowing
  it.
- Successful worker settlement atomically closes its run-owned binding and queues durable cleanup.
- Retry recovery keeps the exact worker active until the cell reaches terminal settlement.

## Paper Cuts

- Batuta asked for the already stored `false` preference before continuing; dull, because the normal
  answer recovered immediately.

## Runtime Errors Observed

- No unexpected runtime errors. The deliberately invalid first result was recovered by the same
  worker session as planned.

## Human Verifications Needed

- None identified.

## Decisions for a Human

- None identified.

## Learnings

- Session lineage must be captured while the worker is active; terminal catalog views may hide stopped
  system sessions by default.
- A semantic `session wait --until stopped` provides direct evidence for cleanup without polling the
  database or internal state.

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

- **QA exit gate:** `GOFLAGS=-timeout=40m COMPOZY_GO_TEST_P=1 make gate` passed; the Go test lane completed in 1,801 seconds and recorded `.cache/gate/go-test.json`. The immutable workstream tree proceeds to `make gate-full` after this report is finalized.
- **Issues by user impact:** Blocks-Completion 0 · Data-Loss 0 · Trust-Damage 2 · Friction 0 · Cosmetic 0
- **Coverage:** 1/1 journeys walked
- **Verdict:** PASS — ready for the workstream completion gate and pull request.
