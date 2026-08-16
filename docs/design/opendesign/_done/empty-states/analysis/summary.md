# Empty states implementation research

## Research Question

How should the Tasks, Jobs, and Triggers zero-inventory redesigns translate the OpenDesign visual contract into the existing production surfaces while preserving responsive disclosure anatomy, truthful live data and actions, state precedence, runtime-owned differences, and canonical verification seams?

## Slice Map

| Slice | Focus | Finding |
| --- | --- | --- |
| 01 – visual-contracts | OpenDesign anatomy and states | All three use a quiet intro followed by a compact disclosure-row panel; shell chrome is placement context. |
| 02 – production-surfaces | Live owners and verification seams | Tasks owns a dedicated onboarding composite; Jobs/Triggers share the catalog shell, while only Jobs has a live suggestion contract. |

## Convergences

- The in-scope redesign is the intro and panel, integrated inside the existing OS windows; the prototype shell must not be rebuilt.
- `@compozy/ui` primitives remain the component owners. Domain composites should express the reference anatomy without cloning Button, Empty, Pill, or Section.
- Loading, error, filtered-empty, and unfiltered-empty must remain distinct. A redesign cannot hide a daemon error or replace clear-filter recovery.
- Tasks and Jobs can reproduce the reference with live data and existing actions. Trigger suggestions cannot be presented as live because the current daemon suggestion contract creates Jobs only.

## Divergences

- Tasks currently uses a four-card grid, while the reference requires compact disclosure rows and an optional details preview.
- Jobs already has live suggestions, but the suggestion panel and generic empty catalog are separate; the redesign should compose them as one zero-inventory experience.
- Triggers has no live suggestion resource. Its intro/anatomy can align, but sample suggestions and durable accept/dismiss behavior are an authorized prototype difference unless the runtime contract is deliberately expanded.

## Risks & Open Questions

- Do not hardcode prototype sample data or add UI-only durable Trigger suggestions.
- Preserve filtered-empty clear actions and error precedence.
- Verify whether task templates open the existing prefilled editor; keep that route-owned behavior.
- Treat post-accept navigation and undo persistence as runtime-owned, not inferred from inert prototype controls.

## Recommended Next Steps

- Implement one shared domain row anatomy for template/suggestion disclosure, then adapt Tasks and Jobs with their existing data/actions ([01](./01_analysis_visual-contracts.md), [02](./02_analysis_production-surfaces.md)).
- Keep Triggers truthful: align its intro and zero-state composition without inventing suggestion data; record the reference delta in the visual contract ([01](./01_analysis_visual-contracts.md)).
- Update the canonical Tasks empty-state and Automation catalog/suggestion suites, plus the existing route stories used for visual capture ([02](./02_analysis_production-surfaces.md)).
- Capture reference/implementation pairs at matched desktop and compact widths and review every divergence ([01](./01_analysis_visual-contracts.md)).

## Index

- [Visual contracts](./01_analysis_visual-contracts.md)
- [Production surfaces](./02_analysis_production-surfaces.md)
