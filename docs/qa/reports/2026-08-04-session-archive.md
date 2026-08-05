# QA Run Report — 2026-08-04 — session archive

- **Scope:** Durable session archive management, session-row action menus, and normal/archived catalog separation across Web and structured surfaces
- **Cadence tier:** targeted
- **Build:** working tree · **Environment:** isolated local production-parity build; manifest pending
- **Started:** 2026-08-04T22:12:17-03:00 · **Status:** in-progress

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Cora | Casual User | laptop / wifi-fast / en-US | CH-archive-session-catalog |
| Ada | Power User | desktop / wifi-fast / en-US | CH-archive-session-structured-parity |
| Nia | New User | laptop / wifi-fast / en-US | CH-session-row-open-canary |

## Flows in Scope

- `J-archive-session-without-deleting` — Hide finished work without losing its history (`../journeys/J-archive-session-without-deleting.md`)
- `J-12` — Open a session normally after the shared row gains an actions target (`../journeys/J-12-open-session-fast.md`)

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-archive-session-catalog | J-archive-session-without-deleting / RT-session-list-row-actions | Cora | Feature Tour | Pending | | |
| 2 | CH-archive-session-catalog | J-archive-session-without-deleting / RT-014 | Cora | Feature Tour | Pending | | |
| 3 | CH-archive-session-catalog | J-archive-session-without-deleting / RT-082 | Cora | Feature Tour | Pending | | |
| 4 | CH-archive-session-catalog | J-archive-session-without-deleting / ET-web-sessions-catalog-modal | Cora | Feature Tour | Pending | | |
| 5 | CH-archive-session-structured-parity | J-archive-session-without-deleting / RT-session-archive-catalog | Ada | Feature Tour | Pending | | |
| 6 | CH-archive-session-structured-parity | J-archive-session-without-deleting / RT-011 | Ada | Feature Tour | Pending | | |
| 7 | CH-session-row-open-canary | J-12 / RT-012 | Nia | Feature Tour | Pending | | |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Session Debriefs

Pending execution.

## What Was Fixed

No QA findings have entered the governed fix loop.

## Paper Cuts

| Persona | Where (journey/step) | Felt | Sharpness | Outcome |
|---|---|---|---|---|

## Runtime Errors Observed

Pending execution.

## Human Verifications Needed

None identified before execution.

## Decisions for a Human

None identified before execution.

## Learnings

Pending execution.

## Final Status

Pending execution and the full automated exit gate.
