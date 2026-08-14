# QA Run Report — 2026-08-14 — worktree-scroll-area

- **Scope:** Bounded, independently scrolling worktree submenus with XState-owned interaction state
- **Cadence tier:** targeted
- **Build:** current working tree · **Environment:** isolated daemon and Web dev server from `worktree-scroll-area-20260814-184457-553088`
- **Started:** 2026-08-14T18:44:57-03:00 · **Status:** pass

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Bruno | Power User | desktop / wifi-fast / en-US | CH-worktree-large-catalog-navigation |

## Flows in Scope

- `J-worktree-management` — Create, select, and manage isolated work without losing history (`../journeys/J-worktree-management.md`)

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-worktree-large-catalog-navigation | J-worktree-management / RT-worktree-web-nested-navigation | Bruno | Feature Tour | Pass | | |
| 2 | CH-worktree-large-catalog-navigation | J-worktree-management / RT-worktree-web-create-adopt | Bruno | Feature Tour | Pass | | |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Session Debriefs

Bruno opened Workspaces through the normal Go menu against an isolated daemon containing one
workspace and 18 real linked worktrees. The visible Web catalog matched the structured CLI list.
The submenu viewport was 380px high for 864px of content and moved from `scrollTop=0` to
`scrollTop=484`; document scroll stayed at zero and the New worktree footer kept the same bounds.
Pointer hover, ArrowRight entry, ArrowLeft focus return, dialog open/cancel, and a cold page refresh
all passed.

## What Was Fixed

None during QA.

## Paper Cuts

None.

## Runtime Errors Observed

None.

## Human Verifications Needed

None.

## Decisions for a Human

None.

## Learnings

The shared `ScrollArea` needs its viewport to own the maximum height. A max-height on the outer
flex container alone leaves a percentage-height viewport with an indefinite containing block.
The interaction state remained in the XState store; React refs are limited to real DOM anchors.

## Final Status

- **Exit gate (full automated suite):** `make gate-full` is the final command after this report is frozen; completion handoff records its result
- **Issues by user impact:** Blocks-Completion 0 · Data-Loss 0 · Trust-Damage 0 · Friction 0 · Cosmetic 0
- **Coverage:** 1/1 journeys walked
- **Verdict:** pass
