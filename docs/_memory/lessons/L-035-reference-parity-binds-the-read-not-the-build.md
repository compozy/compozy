# L-035 — Reference parity binds the read, not the build

**Class:** Frontend / Design system / Process
**Date discovered:** 2026-08-11
**Evidence sources:** operator-reported implementation incidents (agents replacing shipped `@compozy/ui` usage with hand-rolled recreations of prototype markup during Visual Contract work); worktree design-set audit of 2026-08-11 (`docs/design/opendesign/worktree/` — artboards approximate `Dialog`, `Button`, `ScrollArea`, `Input`, `Eyebrow`, `IntensityMeter` as local CSS classes like `.dialog`/`.icon-button`/`.scroll`/`.im`, and redraw menubar, session, task, and loop chrome around the worktree piece, e.g. `worktree-menubar-menu.html` rebuilding the full `.menubar`/`.ws-trigger` shell).

## Context

L-032 scoped visual references as lossy on content, data, copy, and marks — but the doctrine still listed "component anatomy" and "chrome geometry" as normative axes, and `visual-contract.md` + `cy-final-verify` classified "substituted component anatomy" and "wrong shell" as blocking divergences. Prototypes are HTML approximations by construction: they hand-roll shipped primitives as local CSS because they can't import `@compozy/ui`, and they redraw the host surface (menubar, session shell, task/loop views) purely to show where a new piece lands. Under those blockers, the cheapest all-gates-green path for an embedded feature (worktree inside tasks, loops, sessions) is forking primitives to match prototype DOM and rebuilding or deleting live host UI to match the redrawn shell — the same gate-crossing as L-032, on the next pair of axes.

## Root cause

Authority scope was completed for content and marks but not for build: nothing distinguished what a surface *reads* like (normative) from what it is *built* from (owned by `@compozy/ui` and existing composites), and nothing bounded the contract to the named piece — so a prototype's redrawn placement context was judged like spec, and a shipped component's standard internals were judged like divergence.

## Rule

> A visual reference binds what the in-scope piece reads like — layout, hierarchy, spacing, tokens, on-screen anatomy — never what it is built from or what surrounds it. Component identity is owned by the `@compozy/ui` inventory and existing domain composites: map every in-scope region to a shipped component before code and express the reference's read through its variants, props, and tokens. Host chrome a prototype redraws to situate the piece is placement context owned by the live surface: integrate the piece into it; divergence from the redraw is authorized by default. The blocking divergence is the hand-rolled replica where a shipped component exists — not the substitution of one.

## Anti-pattern

- Rebuilding an exported primitive as bespoke markup/CSS because the artboard hand-rolled it.
- Deleting, restyling, or reconstructing live host UI (nav, menubar, session shell, task/loop surfaces) to match a prototype's simplified redraw of it.
- Chasing pixel-diff ratio driven by placement context instead of reading it through the scope boundary.
- Treating a shipped component's standard internals (focus ring, padding, DOM) as parity divergences from the mock.

## Source

- Operator directive of 2026-08-11 (worktree design set flagged as embedding risk across tasks/loops/sessions before implementation started).
- Encoded in: `.agents/skills/eng/eng-ui-screenshot/references/visual-contract.md` (scope boundary + component map steps, divergence taxonomy), `.agents/skills/eng/eng-design/SKILL.md` (§Named visual contracts + Error Handling, precedence over `impeccable` comp reproduction), `.agents/skills/cy-final-verify/SKILL.md` §Visual Contract Parity (+ `extensions/dev-cycle/skills/cy-final-verify/` mirror), root `CLAUDE.md` §Design System, SD-007.
