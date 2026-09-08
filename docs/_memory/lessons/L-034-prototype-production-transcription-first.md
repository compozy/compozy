# L-034 — Prototypes of implemented surfaces are transcriptions plus deltas, never reinterpretations

**Class:** Frontend / Design system / Process
**Date discovered:** 2026-08-02
**Evidence sources:** loops `loop-node-lifecycle` design cycle 2026-08-01/02 (`docs/design/opendesign/loops/`, `DESIGN-LESSONS.md` L1–L4/L10); 4 review rounds — operator-heavy labs rejected, editor rejected for diverging from `web/src/systems/loops/components/editor/`, timeline/icons rejected against `packages/ui/src/components/custom/timeline.tsx`, approved direction consolidated.

## Context

The first loops editor artboard was seeded from an archived prototype and invented chrome the production editor does not have (invariant chip strip, 48px topbar, palette note cards, save-draft button). The run-page timeline was hand-rolled instead of copying the canonical `timeline.tsx` anatomy. Both were rejected as rollback risks for shipped code; two full review rounds were spent recovering. The pass only converged after parallel transcription-grade deep-dives (production editor, external reference, spec web scope) produced anatomy reports with exact values, and every artboard re-seeded from the approved final page of its family.

## Root cause

A prototype for an existing surface was treated as a fresh design space. Without a written production-anatomy baseline, every divergence is invisible — reviewers cannot tell proposal from drift, and implementers "fixing parity" would roll back shipped behavior. The missing discipline: production anatomy is the skeleton; the spec supplies data truth; deltas exist only as annotated proposals.

## Rule

> A redesign of an implemented surface starts from its current anatomy and shipped component owners. Record the in-scope visual structure and authorized deltas at the detail needed to prevent reinterpretation; use a full anatomy report for a substantial redesign. A small change needs only its affected region and reference. Prototypes may show user-requested proposals, clearly labeled, without silently changing implementation scope.

## Anti-pattern

- Seeding a new artboard from an archived prototype instead of production + the approved family seed.
- Hand-rolling a parallel version of an exported primitive ("close enough" timeline, selector, meter).
- Rendering controls or states the daemon does not declare, or leaving a delta un-annotated.
- Explainer cards inside product chrome standing in for point-of-use hints.
- Refusing to draw a user-requested surface because the spec marks it out of scope, instead of tagging it `new · <spec>`.

## Source

- `docs/design/opendesign/loops/DESIGN-LESSONS.md` (L1 transcription, L2 parallel deep-dives, L3 authority chain, L4 finals+labs, L10 canonical primitives) — the per-cycle evidence record.
- Encoded in: `docs/design/opendesign/design-system/GUIDE.md` §Hard rules (prototype directives promoted 2026-08-02); related: L-031 (primitive reuse gate), L-032 (fidelity binds visual language, not content).
