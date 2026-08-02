# QA Run Report — 2026-08-02 — Task terminal recovery

- **Scope:** Targeted branch QA for active-session Task run completion, failure, cancellation, conflict handling, and daemon-restart recovery, plus attach-session canary coverage.
- **Cadence tier:** targeted
- **Build:** `741d3563` with the current `refac-go` working tree · **Environment:** fresh isolated lab at `http://127.0.0.1:59791`; CLI/HTTP/UDS use the manifest-owned runtime; Browser mode `browser-use`
- **Started:** 2026-08-02T18:41:19-03:00 · **Status:** behavior-pass / evidence-blocked

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Bruno | Power User | desktop / wifi-fast / en-US | CH-task-run-terminal-recovery |

## Flows in Scope

- `J-finish-task-run` — finish an active run once across concurrent commands and restart (`../journeys/J-finish-task-run.md`)

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-task-run-terminal-recovery | J-finish-task-run / TA-027 | Bruno | Interrupt Tour | Pass | | |
| 2 | CH-task-run-terminal-recovery | J-finish-task-run / TA-028 | Bruno | Interrupt Tour | Pass | | |
| 3 | CH-task-run-terminal-recovery | J-finish-task-run / TA-029 | Bruno | Interrupt Tour | Pass | | |
| 4 | CH-task-run-terminal-recovery | J-finish-task-run / TA-026 | Bruno | Interrupt Tour | Pass | | |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Session Debriefs

### TA-026 canary and TA-027 completion branch

- Through the isolated UDS-backed CLI, Bruno created `qa-terminal-complete`, enqueued `run-de974bd1f618c944`, attached the existing Codex-backed session `sess-010c952dc11a4a83`, and advanced the run from `queued` to `starting` to `running` without a worker lease. This passes the TA-026 attach-session canary.
- The provider session was independently visible as `prompting`, `healthy`, and `active_prompt: true` before terminalization. Completion won a near-concurrent failure request; the competing CLI received the durable-terminal-intent conflict and could not replace the result.
- Completion returned only after the provider session stopped. Fresh CLI and HTTP reads agreed on run/task `completed`, session `stopped`, result `{\"summary\":\"terminal completion kept\"}`, and exactly one `task.run_completed` event (`evt-03e928b6e51f013b`).
- Evidence: `qa/notes/ta-026-ta-027.json` and the correlated `qa/journey-log.jsonl` entries in the isolated lab. The restart section below closes TA-027's daemon-interruption and raw HTTP 409 branches.

### TA-028 direct failure branch

- Bruno attached the active Codex-backed session `sess-b6721f9ee0c9f8da` to running run `run-6594c475679a9a12` and failed it with the durable error `provider analysis stopped before completion` plus structured metadata.
- A competing HTTP cancellation received `409 Conflict`. It arrived after the failure had already settled, so this proves terminal-state refusal but does not yet prove the in-progress durable-command conflict.
- Fresh task and session reads found the run `failed`, the retryable task `ready`, the session `stopped`, and exactly one `task.run_failed` event (`evt-2fcf20c906d098e6`).
- Evidence: `qa/notes/ta-028-base.json`. This direct branch was pending at that point; the restart section below closes TA-028's in-progress conflict and recovery branches.

### Terminal-command restart recovery

- For completion, failure, and cancellation, Bruno admitted the chosen HTTP terminal command while a Codex-backed session was active. A different concurrent HTTP terminal action received `409 Conflict` with `terminal run command in progress`; the winning client then lost its reply when only the isolated daemon was interrupted.
- Each same-home restart logged the affected session as `agent_crashed` with `stop did not complete`, followed by `daemon: task boot recovery complete` with `settled_terminal_commands: 1`. This is direct runtime evidence that boot recovery, rather than ordinary active-run recovery, settled the recorded command.
- Fresh public reads preserved the original completion result, failure error/metadata, and cancellation reason/metadata. Each run had one matching canonical terminal event; no competing outcome appeared. Public `task inspect` showed every bound session stopped with `stop_reason: agent_crashed`.
- The final cancellation probe additionally froze the exact lab daemon in process state `Tl` before `SIGKILL`; its next boot again settled one terminal command. No non-lab process or default runtime home was signaled.
- Evidence: `qa/notes/ta-027-ta-029-restart.json`, the public task/session reads recorded in `qa/journey-log.jsonl`, and the manifest-owned operational log. TA-027 and TA-028 pass. TA-029 behavior passes through CLI/API/restart and remains Pending only for the available Web cancel affordance.

### TA-029 Web cancellation affordance

- Bruno opened active run `run-16a5baecc646c7da` for task `qa-web-cancel` in the Web run page. The More actions menu exposed `Cancel run` while Codex-backed session `sess-4553afae63b836e4` was active.
- Choosing `Cancel run` moved the visible attempt to `Canceled`. Fresh CLI reads independently found run/task `canceled`, session `stopped` and `dead`, and exactly one `task.run_canceled` event (`evt-597df5c6e244f32c`) with HTTP origin `tasks.cancel_run`.
- The Web action does not collect a cancellation reason. This branch proves the Web affordance and public settlement; the explicit cancellation reason and metadata are proved by the API restart branch above.
- Evidence: `qa/notes/ta-029-web.json`; `qa/screenshots/ta-029-web-cancel-before.png`; `qa/screenshots/ta-029-web-cancel-after.png`; and the official `eng-ui-screenshot` capture `qa/screenshots/ta-029-web-route.png` in the isolated lab.

### Strict evidence audit

- The strict auditor ran against the manifest-owned feature contract and all captured CLI/API/Web/runtime/provider evidence.
- Functional surface coverage is present, but the release-grade evidence contract remains blocked: three distinct actors were used against a minimum of four; the generated behavioral charter does not define three differentiated roles or three channels; and no produced artifact was consumed by another agent. These requirements were not fabricated or weakened for a single-operator terminal lifecycle walk.
- The auditor also correctly reports that final full-gate evidence is pending because repository QA tracker/report mutations occurred after the last green full record. Evidence: `qa/qa-audit-report.json` and `qa/qa-audit-report.md` in the isolated lab.
- Mandatory teardown completed after the last browser read. `qa/teardown.json` records `"clean": true`, the two registered lab processes were reaped, and no survivors remained.

## What Was Fixed

None during QA so far.

## Paper Cuts

None recorded so far.

## Runtime Errors Observed

None recorded so far.

## Human Verifications Needed

- The Task terminal behaviors need no human verification. The broader release-grade scenario contract needs a deliberate multi-agent charter rather than retroactive evidence decoration.

## Decisions for a Human

None identified so far.

## Learnings

- Planning coverage: the journey and functional dimensions are owned by TA-027/028/029; interruption, conflict, and abandonment/restart are exercised by the Interrupt Tour; TA-026 is the adjacent canary. Visual compatibility and mobile coverage are not primary claims for the CLI/HTTP lifecycle, while the Web cancel affordance remains in scope when reachable.
- Automated precondition: `make gate` passed at fingerprint `6e1fed4875a1ce4fac1c26852f74b1b990f9117e` before the first QA interaction (`.cache/gate/logs/full-1785705707.log`).

## Final Status

- **Exit gate (full automated suite):** Pending after the last repository mutation.
- **Issues by user impact:** Blocks-Completion 0 · Data-Loss 0 · Trust-Damage 0 · Friction 0 · Cosmetic 0
- **Coverage:** 1/1 targeted journeys fully walked; TA-026, TA-027, TA-028, and TA-029 passed their behavior contracts through public surfaces.
- **Behavior verdict:** PASS.
- **Release-grade evidence verdict:** BLOCKED — the strict feature-profile audit requires a real four-agent, three-channel charter with downstream artifact reuse, and the final full gate is still pending. Laboratory teardown is clean.
