# QA Run Report — 2026-08-12 — Issue 348 Loop Cancel Restart

- **Scope:** Targeted branch QA for missing-session Loop cancellation, canceled coordinator repair, daemon restart readiness, and workspace-read canary behavior.
- **Cadence tier:** targeted
- **Build:** `v0.3.0-beta.13-3-g7fc534a0-dirty` · **Environment:** fresh isolated daemon at `http://127.0.0.1:63558`, isolated UDS and runtime home, deterministic ACP provider
- **Started:** 2026-08-12T03:51:29Z · **Status:** complete

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Bruno | Power User | desktop / wifi-fast / en-US | CH-loop-cancel-restart |

## Flows in Scope

- `J-recover-loop-node-failure` — interrupt Loop work, recover its coordinator, and keep the daemon usable after restart (`../journeys/J-recover-loop-node-failure.md`)
- Adjacent canary: list the affected and a foreign workspace after each restart so readiness and isolation are independently observable.

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-loop-cancel-restart | J-recover-loop-node-failure / LP-cancel-restart-recovers | Bruno | Interrupt Tour | Pass | #348 | this PR |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Session Debriefs

### CH-loop-cancel-restart — Bruno

- **Ran:** 2026-08-12T03:51:29Z → 2026-08-12T03:57:25Z (box respected: yes)
- **Findings:** The fixed build accepted cancellation after one bound ACP session was removed, repaired a canceled coordinator on boot, and preserved one open recovery run across an immediate second restart. Repeating the coordinator cancellation created one new causal recovery run and remained idempotent on the next restart.
- **Bugs filed/updated:** Existing GitHub issue #348; no new QA registry bug.
- **Scenarios settled:** LP-cancel-restart-recovers → pass
- **Paper cuts:** `task inspect` rejects deterministic `loop.*` task ids, but `task get` and the documented HTTP task route expose the required state; recorded as an adjacent observation, not a failure of this journey.
- **Surprises:** The CLI start path identifies as UDS, so the lab Loop needed to declare `start: uds`. The public task catalog presents a repaired task with an open queued run as `ready`; its event history exposes the persisted `canceled → in_progress` repair.
- **Suggested next charter:** Re-run the broad `CH-003` Cancel/Kill UI charter when a Web surface changes; this backend-only fix did not change Web controls or payloads.

## What Was Fixed

### GitHub issue #348: daemon boot fails on canceled Loop coordinator

- **Symptom:** Missing cancellation sessions left a nonterminal Loop with a canceled reusable coordinator task, causing every daemon start to fail before readiness.
- **Root cause:** Canceled coordinator pulses were projected as task cancellation, boot recovery reused a terminal reservation identity, and the daemon treated a missing session as failed delivery.
- **Fix:** This PR repairs only nonterminal Loop coordinator tasks, reserves causally distinct recovery runs, and treats only `session.ErrSessionNotFound` as already stopped.
- **Regression test:** Canonical task-policy, task-catalog, Loop cancellation/boot, and daemon adapter suites; focused race-enabled runs passed before this session.
- **Retested:** `J-recover-loop-node-failure` through public CLI, HTTP, UDS, runtime, scheduler, task, session, Loop, and ACP subprocess surfaces.

## Paper Cuts

None.

## Runtime Errors Observed

The attempted `task inspect loop.LOOP_RUN_ID.coordinator` returned the expected diagnostic that `inspect` accepts only `task_*`/`task-*` and `run_*`/`run-*` identifiers. The session continued through the public `task get` and HTTP task routes; no daemon error occurred.

## Human Verifications Needed

None.

## Decisions for a Human

None.

## Learnings

- Journey and functional coverage are owned by the restart walk and fresh structured reads.
- The Interrupt Tour covers absent sessions, repeated restart, repeated coordinator cancellation, and recovery continuity.
- Workspace isolation is checked with affected and foreign workspace reads; browser, mobile, locale, and visual compatibility are deliberately out of scope because this diff changes no Web or copy surface.
- Public evidence: `/Users/pedronauck/dev/qa-labs/compozy-issue-348-loop-cancel-restart-20260812-034715-623464-lab/qa-artifacts/qa/behavioral-evidence.md`.
- Teardown evidence: `/Users/pedronauck/dev/qa-labs/compozy-issue-348-loop-cancel-restart-20260812-034715-623464-lab/qa-artifacts/qa/teardown.json` reports `clean: true` with no survivors.
- Production-parity qualification: the provider is the repository's deterministic ACP mock. All affected runtime paths are real; external model output is not part of the cancellation/boot contract.

## Final Status

- **Exit gate (full automated suite):** `make gate` escalates to the full `make verify` suite for the SQL change; PASS evidence is copied to `/Users/pedronauck/dev/qa-labs/compozy-issue-348-loop-cancel-restart-20260812-034715-623464-lab/qa-artifacts/qa/final-make-verify.log`, and the content-keyed record is reported by `make gate-status`.
- **Issues by user impact:** Blocks-Completion 0 · Data-Loss 0 · Trust-Damage 0 · Friction 0 · Cosmetic 0
- **Coverage:** 1/1 journeys walked; affected and foreign workspace reads passed
- **Verdict:** PASS — the missing-session cancellation and repeated restart walk passed, teardown is clean, and the full automated suite passed for the final tracked content.
