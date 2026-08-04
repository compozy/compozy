# QA Run Report — 2026-08-04 — durable-acp-sessions

- **Scope:** Stopped-session prompt continuity, durable per-session runtime selection, focused assistant message actions, and clean running-window teardown.
- **Cadence tier:** targeted
- **Build:** working tree · **Environment:** isolated local daemon and Web lab; real provider where available
- **Started:** 2026-08-04T13:00:00-03:00 · **Status:** acceptance passed

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Théo | Power User | desktop / wifi-fast / en-US | CH-stopped-session-prompt-continuity, CH-runtime-selection-restart-continuity, CH-running-session-window-close |
| Rafa | Casual User | desktop / wifi-fast / en-US | CH-message-actions-copy-timestamp |
| Dora | New User | desktop / wifi-fast / en-US | CH-durable-session-docs-continuity |

## Flows in Scope

- `J-13` — Follow, stop, and continue one durable ACP conversation (`../journeys/J-13-follow-a-live-run.md`).
- `J-17` — Persist and apply the session's next-prompt runtime (`../journeys/J-17-session-create-unified-selector.md`).
- `J-11` — Leave and return to live background work (`../journeys/J-11-return-to-running-session.md`).
- `J-14` — Review a finished transcript with focused message actions (`../journeys/J-14-read-a-finished-transcript.md`).
- `J-evaluate-compozy-beta` — Read the migration and session guides as one stopped-session lifecycle.

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-stopped-session-prompt-continuity | J-13 / RT-018 | Théo | Interrupt Tour | Fixed | BUG-20260804-stopped-prompt-restart-panic | working tree |
| 2 | CH-runtime-selection-restart-continuity | J-17 / RT-session-runtime-selection-continuity | Théo | Interrupt Tour | Pass | | |
| 3 | CH-running-session-window-close | J-11 / ET-running-session-window-close-clean | Théo | Interrupt Tour | Pass | | |
| 4 | CH-message-actions-copy-timestamp | J-14 / RT-053 | Rafa | Feature Tour | Pass | | |
| 5 | CH-durable-session-docs-continuity | J-evaluate-compozy-beta / ET-site-docs-first-session | Dora | Feature Tour | Pass | | |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Session Debriefs

- **Stopped session:** the composer remained enabled in `stopped`; after daemon restart the first attempt exposed a 500. The production admission key was corrected, the canonical regression passed, and the live replay retained the same session id and recalled the first answer.
- **Runtime continuity:** Claude Code / Claude Fable 5 / Max persisted immediately, remained visible while stopped, survived daemon restart, and matched both HTTP and UDS CLI readback before the next prompt.
- **Window teardown:** a session window closed while a real Claude turn was running. The session kept running, reopened with current output, and produced no thread teardown error or error boundary.
- **Message actions:** settled messages exposed Copy only. `/goal status` remained available through the composer command path.
- **Documentation continuity:** the production-built site consistently taught that a normal prompt restarts stopped work on the same session and history, while `session resume` only attaches to a live process. Migration, Web UI, lifecycle, resume, and generated runtime set/clear pages all survived direct navigation and reload. Browser error collection was empty; the local-only Vercel Analytics and Speed Insights notices remained informational console logs.

## What Was Fixed

- Fixed `BUG-20260804-stopped-prompt-restart-panic`: durable stopped-session prompt admission now serializes on the target session id instead of dereferencing an absent active session.
- Added the regression to the canonical `TestPromptErrorPaths` suite; no duplicate standalone suite was created.

## Paper Cuts

None recorded.

## Runtime Errors Observed

- One pre-fix HTTP 500 and Go nil dereference occurred on the first post-restart prompt; it is recorded in the bug above and passed the same live retest after the production fix.
- After the fix, browser error collection was empty. Expected SSE `cleanup` and `sse_open` debug events remained.

## Human Verifications Needed

None identified.

## Decisions for a Human

None identified.

## Learnings

- Stopped is a durable lifecycle state, not a terminal transcript state. Admission code must therefore be keyed by durable request identity before an in-memory `Session` exists.
- Selected runtime intent and effective runtime evidence remained distinct through stop and resume, which made restart behavior both stable and inspectable.
- The migration and lifecycle guides need to describe stopped sessions from the same durable-session model as the runtime; calling stop terminal would make correct UI behavior appear broken.

## Visual Evidence

- [Migration guide](../evidence/2026-08-04-durable-acp-sessions-docs/CH-durable-session-docs-continuity-migration.png)
- [Session lifecycle](../evidence/2026-08-04-durable-acp-sessions-docs/CH-durable-session-docs-continuity-lifecycle.png)

## Final Status

- **Exit gate (full automated suite):** owned by the final fingerprint-cached gate record cited at handoff
- **Issues by user impact:** Blocks-Completion 1 found / 1 fixed · Data-Loss 0 · Trust-Damage 0 · Friction 0 · Cosmetic 0
- **Coverage:** 5/5 journeys walked
- **Verdict:** acceptance behavior and documentation passed; workstream closure requires current full-gate evidence and clean teardown.
