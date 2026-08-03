# QA Run Report — 2026-08-03 — Loop node lifecycle Task 08

- **Scope:** Task 08 adds daemon-backed node lifecycle states, controls, quarantine inspection, and workspace node inventories to the Loop run Web surface.
- **Cadence tier:** targeted
- **Build:** Task 07 checkpoint `bccb038d` plus the Task 08 working tree · **Environment:** isolated lab `loop-operator-lifecycle-ui-20260803-044343-123901`
- **Started:** 2026-08-03T04:12:30Z · **Status:** pass

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Bruno | Power User | desktop / wifi-fast / en-US | CH-operator-lifecycle-ui |
| Cora | Non-technical Owner | desktop / wifi-fast / en-US | CH-compozy-run-plain-language |

## Flows in Scope

- `J-04` — pause, resume, cancel, or kill a running Loop (`../journeys/J-04-operator-pause-resume.md`)
- `J-01` — adjacent canary: reopen the exact persisted run and understand its durable truth (`../journeys/J-01-arrive-and-use-run.md`)

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-operator-lifecycle-ui | J-04 / LP-operator-lifecycle-ui | Bruno | Feature Tour | Pass | | |
| 2 | CH-compozy-run-plain-language | J-01 / LP-loop-run-deep-link | Cora | Feature Tour | Pass | | |
| 3 | CH-operator-lifecycle-ui | J-improve-loop-with-feedback / LP-wait-event-catalog-validation | Bruno | Error Tour | Fixed | BUG-20260803-wait-event-rejected-too-late | Task 08 checkpoint |
| 4 | CH-operator-lifecycle-ui | J-04 / LP-live-run-survives-extension-disable | Bruno | Feature Tour | Fixed | BUG-20260803-disabled-extension-blocks-wait-resume | Task 08 checkpoint |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Session Debriefs

### Bruno — operator lifecycle

- Entered from the Loop run list and worked only through public CLI, HTTP, and Web surfaces.
- Resumed a real timer wait with a payload and confirmed the fresh run read reached `done`.
- Inspected a real quarantine entry: hint first, attempts grouped by episode, target, input reference, and requeue confirmation all matched daemon state. Requeue created a fresh generation and removed the stale action.
- Created a real timeout retry. The run page showed attempt, failure class, and next-attempt time; drain pause recorded the human actor and safe point; resume restored the retry schedule; the workspace retrying inventory agreed with the detail page.
- Killed a second run through the sole run-level overflow. The fresh terminal page was calm, explained why it stopped, and removed Happening Now.
- Walked all four inventories, loop filters, state-age sort, truthful empty states, a 768px viewport, keyboard-only overflow access, and an injected inventory request failure followed by Retry recovery.

### Cora — durable deep-link canary

- Reopened the exact persisted run URL after the desktop layout stream synchronized.
- The fresh page matched the requested run id and durable terminal story; no stale neighboring run or optimistic state remained.

## What Was Fixed

- Unsupported wait event kinds are now rejected by `compozy loop validate` before publication or execution (`BUG-20260803-wait-event-rejected-too-late`).
- Resuming an in-flight wait now reads the run's pinned executed definition instead of recompiling the mutable extension catalog (`BUG-20260803-disabled-extension-blocks-wait-resume`).
- Inventory load failures now keep the last truthful boundary, render a recoverable error, and Retry performs a fresh request.
- Wait resume chooses the currently open wait entry, and lifecycle dialogs reset their form state between targets and reopenings.
- Quarantine attempts now use one canonical `snake_case` JSON shape end to end; the Web reader and stories no longer carry a dual-spelling compatibility path.

## Paper Cuts

- The browser driver did not expose native browser zoom as a controllable value. The run page was checked at a 768px CSS viewport and separately at device scale factor 2; both preserved controls and readable state. This is an evidence-tool limitation, not an observed product failure.
- A strict `eng-real-scenario-qa` audit was attempted, then discarded because its startup-playbook contract requires four agents, three channels, and a live provider session. This targeted `qa-execution` walk does not claim that unrelated release-grade startup coverage.

## Runtime Errors Observed

- Before repair, `qa.acknowledged` passed authoring and failed only at runtime with an executed watch-event contract error.
- Before repair, disabling `dev-cycle` caused the unrelated wait Resume request to return HTTP 500 and leave the wait open.
- Both errors were reproduced, fixed in production code, covered by canonical regression suites, and re-walked through public surfaces.

## Human Verifications Needed

None.

## Decisions for a Human

None.

## Learnings

- Lifecycle control copy was easiest to trust when the page named both the current state and what the selected verb would do at the next scheduling boundary.
- A live extension catalog is mutable, while an accepted run is immutable. Controls on an in-flight run must use the run's definition digest for config decisions.
- Validation must use the same closed event catalog as execution; accepting an impossible event moves an authoring error into a much more expensive runtime failure.
- The final wire hard cut was regenerated with `make codegen`, passed `make codegen-check`, and the daemon-served `make test-e2e-web` lane was rerun after the mutation.

## Final Status

- **Exit gate:** targeted QA pass; mandatory teardown is recorded at `/Users/pedronauck/dev/qa-labs/compozy-loop-operator-lifecycle-ui-20260803-044343-123901-lab/qa-artifacts/qa/teardown.json` with `clean: true` and no survivors. The Task 08 code gate is recorded separately by `make gate`.
- **Issues by user impact:** 2 Blocks-Completion findings found, fixed, and verified; 0 open product bugs; 1 evidence-tool limitation.
- **Coverage:** 4 of 4 matrix rows walked; 30 evidence artifacts cover lifecycle, recovery, durability, responsive, keyboard, and validation paths.
- **Verdict:** Pass. The operator can see and control real lifecycle state without the Web surface inventing verbs, totals, or recovery outcomes.
