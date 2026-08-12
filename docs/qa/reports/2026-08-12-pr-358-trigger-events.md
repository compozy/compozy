# QA Run Report — 2026-08-12 — pr-358-trigger-events

- **Scope:** PR #358 trigger grammar, config preflight, public creation/update boundaries, and the real memory consolidation producer
- **Cadence tier:** targeted
- **Build:** `6e9b9c6f` + remediation worktree · **Environment:** isolated targeted lab `pr-358-trigger-events-20260812-161024-582368`
- **Started:** 2026-08-12T16:09:37Z · **Stopped:** 2026-08-12T16:46:15Z · **Status:** blocked-verify

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Bruno | Power User | desktop / wifi-fast / en-US | CH-producer-backed-trigger-events |

## Flows in Scope

- `J-create-and-activate-trigger` — reject impossible event names before persistence and prove accepted events activate from real producers (`../journeys/J-create-and-activate-trigger.md`)
- Adjacent canary: dynamic trigger CRUD remains readable and mutable across public surfaces (`TA-056`)

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-producer-backed-trigger-events | J-create-and-activate-trigger / TA-057 | Bruno | Garbage Tour | Blocked (needs human verify) | Successful public memory consolidation was not reached before the operator stopped the run. | |
| 2 | CH-producer-backed-trigger-events | J-create-and-activate-trigger / TA-056 canary | Bruno | Garbage Tour | Pass | | |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Session Debriefs

- CLI create rejected `loop.terminal`, whole-event whitespace, both hook-delimiter padding forms, and bare `ext.` with exit 65 and the canonical validator message.
- HTTP create returned 400 for `hook. release.completed`; UDS update returned 400 for ` ext.next`. A second CLI read confirmed the invalid update did not alter stored `ext. release`.
- Valid public definitions preserved `ext. release` and `hook.release review.v2.completed`. Create and update help exposed the same producer-backed grammar.
- `config validate` rejected `loop.terminal`; changing the same definition to `ext. release` returned `status: valid`.
- Stopping public session `sess-91e01fab1c2c2105` produced recorded runs for trigger `trg-2c2ab42578bf7f0b`, proving the lifecycle producer. The trigger was then disabled to stop a filterless session-stop chain.
- A public workspace memory write and a real Claude-backed session completed. The subsequent dream request returned `triggered: false`, `candidate_count: 0`, so this run does not claim successful `memory.consolidated` activation.
- The operator explicitly ended local QA before further attempts and delegated the full automated gate to CI after the authorized push.

## What Was Fixed

No production mutations were made during the QA session.

## Paper Cuts

- A filterless `session.stopped` trigger also matched automation-created sessions. The configured fire limit bounded the chain; QA disabled the trigger after observing three runs. This behavior predates the remediation and was not expanded under the stop-now instruction.

## Runtime Errors Observed

- One daemon restart tried to load an operator workspace outside the lab. The run removed only that empty workspace registration from the isolated QA database, restarted with the lab as `HOME`, and did not touch operator files.

## Human Verifications Needed

- Re-run the public memory dream journey with a qualifying recall signal and confirm exactly one workspace-scoped run for trigger `trg-8427dbab87e9738f`.

## Decisions for a Human

None recorded yet.

## Learnings

- Stored trigger definitions are not proof of activation; independent run history exposed both the lifecycle producer and the skipped memory producer attempt.
- Whole-event whitespace is rejected on mutation/fire surfaces, while list filters remain convenience-normalized and `ext. release` remains producer-aligned.

## Final Status

- **Exit gate (full automated suite):** intentionally not run locally by explicit operator instruction; CI after the authorized push owns the full gate.
- **Focused automation:** Go model/config/API/manager/store/memory suites passed; Web trigger preview/form passed. The last focused post-refactor race run passed `internal/automation` and `internal/memory/consolidation`.
- **Issues by user impact:** no validation regression observed; successful public memory activation remains unverified in this stopped run.
- **Coverage:** CLI, HTTP, UDS, config preflight, daemon restart/migration v60, session lifecycle producer, and attempted memory producer.
- **Teardown:** `/Users/pedronauck/dev/qa-labs/compozy-pr-358-trigger-events-20260812-161024-582368-lab/qa-artifacts/qa/teardown.json` — `clean: true`, `survivors: []`.
- **Verdict:** `blocked-verify` for the remaining public memory producer leg; local full-gate absence is not a blocker because the operator delegated it to CI.
