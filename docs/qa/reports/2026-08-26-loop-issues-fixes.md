# QA Run Report — 2026-08-26 — Loop issue fixes

- **Scope:** GitHub issues #451, #472, #480, #485, #486, and #489 — Loop recovery, failure policy, completion time, and effective config provenance
- **Cadence tier:** targeted
- **Build:** `21d420d9d` plus the current worktree diff · **Environment:** fresh isolated lab, `http://127.0.0.1:60843`, CLI/API/Web/runtime required, no provider required
- **Started:** 2026-08-26T19:25:04Z · **Status:** in-progress

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Bruno | Power User | desktop / wifi-fast / en-US | CH-loop-effective-config-truth |
| Ada | Power User | desktop / wifi-fast / en-US | CH-loop-effective-config-truth, CH-loop-terminal-time-recovery |
| Lea | New User | laptop / wifi-fast / en-US | CH-008 |

## Flows in Scope

- `J-configure-and-run-loop` — reuse reviewed settings, explain each winning value, and enforce the selected failure policy (`../journeys/J-configure-and-run-loop.md`)
- `J-loop-terminal-recovery` — preserve exact terminal time and isolate invalid persisted history during recovery (`../journeys/J-loop-terminal-recovery.md`)
- `J-02` — preview a Loop without creating work, used as the adjacent canary (`../journeys/J-02-dry-run-preview.md`)

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-loop-effective-config-truth | J-configure-and-run-loop / LP-effective-config-provenance | Bruno + Ada | Feature Tour | Pending | | |
| 2 | CH-loop-effective-config-truth | J-configure-and-run-loop / LP-halt-on-node-failure | Bruno + Ada | Feature Tour | Pending | BUG-20260826-halt-rerun-busy | |
| 3 | CH-loop-terminal-time-recovery | J-loop-terminal-recovery / LP-terminal-completion-time | Ada | Interrupt Tour | Pending | | |
| 4 | CH-loop-terminal-time-recovery | J-loop-terminal-recovery / LP-invalid-snapshot-boot-isolation | Ada | Interrupt Tour | Pending | | |
| 5 | CH-008 | J-02 / LP-006 | Lea | Garbage Tour | Pending | | |
| 6 | CH-008 | J-02 / LP-007 | Lea | Garbage Tour | Pending | | |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Session Debriefs

### CH-loop-effective-config-truth — Bruno + Ada

- **Ran:** 2026-08-26T19:26Z → fix loop in progress (box respected: yes)
- **Findings:** The configuration, source map, pinned history, and automatic halt behavior passed across CLI, HTTP, native tool, and Web. Explicit rerun failed with `rerun_busy` because a downstream node remained pending.
- **Bugs filed/updated:** BUG-20260826-halt-rerun-busy
- **Scenarios settled:** LP-effective-config-provenance pending final write-back; LP-halt-on-node-failure → fail
- **Paper cuts:** None.
- **Surprises:** The first Browser Use attempt required manual Chrome authorization; the documented agent-browser fallback completed the Web leg.
- **Suggested next charter:** Rewalk the explicit rerun from a fresh run after the governed fix.

## What Was Fixed

BUG-20260826-halt-rerun-busy is in the governed fix loop.

## Paper Cuts

- `rerun_busy` on a terminal halted run — evidence in the linked bug.

## Runtime Errors Observed

None observed yet.

## Human Verifications Needed

None identified yet.

## Decisions for a Human

None.

## Learnings

- Taxonomy coverage: functional truth, error/recovery, continuity, and cross-surface consistency are in scope. Responsive and accessibility checks are not applicable to the changed data contracts; Web layout is unchanged. Production-provider coverage is deliberately skipped because these bounded journeys require no provider.

## Final Status

Pending session execution and exit gate.
