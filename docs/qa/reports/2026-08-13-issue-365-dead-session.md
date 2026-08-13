# QA Run Report — 2026-08-13 — issue 365 dead session

- **Scope:** Issue #365: a terminal ACP session remains readable and can be forked from the web UI.
- **Cadence tier:** targeted
- **Build:** `codex/issue-365-dead-session` working tree · **Environment:** isolated targeted lab `compozy-issue-365-dead-session-attach-20260813-220424-298966-lab`
- **Started:** 2026-08-13T21:22:02-03:00 · **Status:** pass

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Théo | Power User | desktop / wifi-fast / en-US | CH-dead-session-history-recovery |

## Flows in Scope

- `J-dead-session-history-recovery` — read persisted failure history and deliberately fork follow-up work (`../journeys/J-dead-session-history-recovery.md`)

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-dead-session-history-recovery | J-dead-session-history-recovery / RT-acp-stream-disconnect-recovery | Théo | Back-Button Tour | Pass | #365 | local working tree |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Session Debriefs

The deterministic ACP fixture streamed `partial before crash`, then exited with code 23. The
original session became `stopped`, `dead`, not attachable, and not eligible for wake.

Repeated CLI recap reads returned the same persisted history; only the recap snapshot's generated-at
timestamp changed. Explicit HTTP attach and prompt both returned 409, while UDS `session resume`
and `session prompt` were recoverably refused as not attachable. The ACP diagnostics recorded zero
subsequent `session/load` calls.

The web session window displayed the original transcript, `process_exit` failure details, a
disabled prompt, and the read-only recovery action. Agent-browser forked a child session; the child
was in the same workspace and retained the original session as `parent_session_id`. It then opened
a fresh direct link to the original session and reloaded it; its history and fork action remained
available and the `session/load` count stayed zero.

## What Was Fixed

The runtime now rejects terminal `process_exit` sessions before an ACP resume attempt. Session
projection reads remain independent of runtime attachment, and the session window makes the
history-only state explicit while offering an intentional child-session fork.

## Paper Cuts

The new targeted lab initially lacked its ACP fixture executable and standard log/evidence
directories. Building the fixture from the current worktree and creating those lab directories
corrected the envelope only; no product code was changed for the QA setup.

## Runtime Errors Observed

The intended ACP fixture process exit was observed. No unexpected runtime errors were found.

## Human Verifications Needed

None recorded yet.

## Decisions for a Human

None recorded yet.

## Learnings

Persisted session projection is sufficient for history and recap after a provider dies; runtime
attachment is neither needed nor safe for those reads. Recovery should create an explicit child
session rather than replaying the failed turn in the original.

## Final Status

PASS — the targeted runtime, CLI, HTTP, UDS, and web journey completed with a deterministic ACP
subprocess fixture. No credentialed provider was required for this provider-process failure path.
The final automated exit gate is collected after this report's last tracked update; its current
evidence is recorded in the isolated QA lab journey log during completion.
