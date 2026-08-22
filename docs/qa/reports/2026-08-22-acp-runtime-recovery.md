# QA Run Report — 2026-08-22 — ACP runtime recovery

- **Scope:** Lossless automatic recovery for long ACP turns after provider disconnects, plus bounded-exhaustion history.
- **Cadence tier:** targeted
- **Build:** `609f2d22` plus working tree · **Environment:** isolated bootstrap lab `acp-runtime-recovery-20260822-20260822-202310-384045`
- **Started:** 2026-08-22T15:36:31-03:00 · **Status:** PASS

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Théo | Power User | desktop / flaky / en-US | CH-acp-automatic-runtime-recovery, CH-acp-recovery-exhaustion-history |

## Flows in Scope

- `J-automatic-runtime-recovery` — keep one long turn alive across a provider disconnect (`../journeys/J-automatic-runtime-recovery.md`)
- `J-dead-session-history-recovery` — preserve readable and forkable history after bounded recovery is exhausted (`../journeys/J-dead-session-history-recovery.md`)

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-acp-automatic-runtime-recovery | J-automatic-runtime-recovery / RT-acp-automatic-recovery | Théo | Network Tour | Pass | beta.19 disconnect | working tree |
| 2 | CH-acp-recovery-exhaustion-history | J-dead-session-history-recovery / RT-acp-stream-disconnect-recovery | Théo | Back-Button Tour | Fixed | beta.19 disconnect plus duplicate terminal presentation | working tree |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Session Debriefs

- `sess-23826f3159e86f8d` preserved the first partial answer, replaced the ACP runtime once, completed the interrupted turn without a second user prompt, and remained `ready` with `transition=automatic_recovery` and `generation=2` after fresh CLI, API, and web reads.
- `sess-5ca2e105c94cefa4` preserved all four partial chunks across the original process and three replacements. It recorded exactly three `runtime_recovery_started`, three `runtime_recovery_succeeded`, one `runtime_recovery_exhausted`, one terminal `error`, one provider-failure marker, and one `session_stopped` event. Refresh and repeated reads left the event cursor at 17.
- The web fork action created `sess-5b317987bad34359` in the same workspace with `parent_session_id=sess-5ca2e105c94cefa4` and `spawn_depth=1`; the original event stream stayed unchanged.

## What Was Fixed

- The process-exit finalizer no longer persists a second terminal error or marker after the prompt path already owns the fatal failure.
- Transcript presentation now hides the raw terminal error only when the same turn also has the durable `transcript_marker.provider_failure`; the marker remains visible and an unpaired raw error remains visible.

## Paper Cuts

- The first exhaustion walk exposed duplicate technical failure rows. Storage deduplication and transcript projection were corrected, then the full exhaustion path was rerun from a fresh session.

## Runtime Errors Observed

- Expected fault injection: the deterministic ACP subprocess exited with code 23 on the original prompt and each replacement process.
- No unexpected daemon, browser, CLI, API, or store errors affected the final walks.

## Human Verifications Needed

None planned.

## Decisions for a Human

None.

## Learnings

- Replacement startup succeeding is distinct from the replayed prompt surviving; both outcomes need durable events.
- A single persisted error can still duplicate user-facing diagnostics when its canonical marker is a separate transcript message, so deduplication belongs in the complete transcript projection.

## Coverage Notes

- Journeys: both the successful replacement branch and the exhausted/fork branch are planned.
- Functional: HTTP, UDS, CLI, native tools, durable transcript, status, events, and hooks are in scope.
- Experiential: recovery status visibility, preserved transcript, and one bounded wait are in scope.
- Edge/error: close-and-reopen during recovery and three consecutive replacement failures are in scope.
- Cross-cutting: web and structured-surface parity are in scope; mobile, locale, and unrelated providers are skipped because the diff does not change layout or localization and the provider fault boundary is deterministic.

## Final Status

- **Exit gate (full automated suite):** workstream gate evidence is recorded separately after the QA source freeze
- **Issues by user impact:** Blocks-Completion 0 · Data-Loss 0 · Trust-Damage 0 · Friction 1 fixed · Cosmetic 0
- **Coverage:** 2 of 2 in-scope journey sessions complete
- **Verdict:** PASS — both recovery branches, durable rereads, and the explicit fork path meet their contracts.
