# UIUX Template

Structure for `_uiux.md` — the UI change map written only for UI-bearing features (screens, controls, or interaction states). It freezes the operator-facing surface the way `_dx.md` freezes the developer-facing one. Its presence marks the feature UI-bearing, so the task suite carries browser e2e coverage. Downstream consumers: the project's design/review process consumes it as its inventory, implementation tasks cite its component plan, and `_tests.md` derives browser E2E journeys from its surface map.

## Rules

- **Inventory, not design**: this document names every touched surface, its states, and its production mapping; the visual language itself belongs to the design pass and its artboards under the project's agreed design-artifact directory. Name an artboard only when the task has a visual reference; existing-surface adjustments may use `none — no named visual reference`.
- **Grammar**: use the target project's design system and token sources. Identify the actual source files; when none exist, record the proposed visual constraints and their approval owner. Propose semantic mappings for signals; do not invent project-specific token paths.
- **Reuse before create**: check the target project's component inventory before naming any new component; justify every new primitive against it. Use its existing locations for shared primitives and domain composites.
- **Truthful UI**: unknown renders as unknown, pending ≠ ready, affordances absent (not disabled) when the runtime cannot support them.
- **Anchored in code**: every modified surface cites its today-state with `file:line` anchors; states are enumerated from `_user_stories.md` ACs/ECs, cited by ID.
- **Copy**: follow the target project's copy guidelines and established UI patterns; add supporting prose only when it helps users make a decision or avoid an error.

## Document Skeleton

```markdown
# UI/UX Change Map: [Feature Name]

Every UI surface this feature touches: where it lives today, what changes,
which states change, and any named reference artboard. New visual references
land under the project's agreed design-artifact directory; only named references become
visual contracts. Existing-surface or copy-only work needs no new artboard.

Companions: `_spec.md` Part I (behavior authority), `_user_stories.md`
(states come from ACs/ECs), `_dx.md` (the non-UI half of the surface).

## Design constraints (apply to every artboard)

The Rules above, made feature-specific: the proposed signal-palette semantic
mapping, truthful-UI lines for this feature's unknowns, nesting/keyboard
notes, reusable copy sources.

## Surface map

| #   | Surface   | Kind         | Core change       | Stories        |
| --- | --------- | ------------ | ----------------- | -------------- |
| S1  | [surface] | [new/modify] | [one-line change] | US-NNN, US-NNN |

### S1. [Surface name]

- **Today**: current behavior with `file:line` anchors (omit for new surfaces).
- **Change**: what this feature does to it.
- **States to design**: every state, from the cited stories' ACs/ECs —
  empty, pending, error, permission, and scale states included.
- **Artboard**: `[slug]-[surface].html` or `none — no named visual reference`.

## Component plan (design → production mapping)

The implementation contract: compose from what exists; artboard CSS is a
visual contract, never a stylesheet to import.

### Rules

Feature-specific composition rules.

### New shared primitives

Each one justified against the export inventory — or "none".

### New domain components

Per target-project domain/module: component name, composed from which
primitives, used by which surfaces.

### Signal & state mapping

Design glyph/state → existing primitive + token, one row per state chip or
signal used in the artboards.
```
