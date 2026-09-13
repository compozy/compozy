# Pre-Ship Checklist

Use for a new surface, substantial redesign, or requested audit. Select only applicable checks and reuse existing evidence. The project's requirements govern release decisions; this checklist adds no approval, report, breakpoint quota, or mandatory profiling run.

## Design decisions and system fidelity

- [ ] Changed design choices follow the accepted brief, supported primitives, and canonical token/copy sources.
- [ ] New tokens or patterns have an owning source; generated documentation is updated through its existing tool.
- [ ] Optional scene, register, or visual-dial notes resolve an actual design choice rather than restating established decisions.

## Behavior and state coverage

- [ ] Affected controls perform their indicated actions through the real interface.
- [ ] Relevant default, focus, disabled, loading, empty, error, success, and recovery states were exercised; inapplicable states are omitted.
- [ ] Pending feedback settles correctly and preserves usable layout.

## Accessibility and layout

- [ ] The changed interaction satisfies its keyboard, focus, accessible-name, and announcement contract; use the matching `references/accessibility-floor.md` section.
- [ ] Rendered foreground/background combinations and non-color cues remain understandable in supported themes and contrast modes.
- [ ] Changed layout survives representative content, supported languages, and relevant widths; supported touch and reduced-motion behavior works.

## Visual and copy quality

- [ ] Hierarchy, legibility, and content emphasis suit the task; decoration does not obscure information or interaction.
- [ ] Sample content is recognizable as sample data; metrics and claims have evidence.
- [ ] Action labels and error recovery are understandable in context and follow the product's voice.
- [ ] Findings from `references/ai-slop-patterns.md` or `references/anti-defaults.md` describe observed problems, not preference scores.

## AI surfaces, when affected

- [ ] Capabilities, material limitations, sources, and uncertainty are understandable where they affect decisions.
- [ ] Users can correct, retry, dismiss, or otherwise recover through the supported flow.

## Verification and readiness

- [ ] Evidence identifies the states and inputs checked: relevant screenshots, interaction runs, accessibility checks, or existing suite results.
- [ ] Performance-sensitive changes use the existing budget and relevant measurements; see `references/performance.md`.
- [ ] Required project checks are satisfied or their gaps are explicit. Optional helper warnings are inspected for real defects rather than treated as automatic blockers.
- [ ] Remaining findings are prioritized by user impact and project policy. No arithmetic over visual preferences creates a new severity or release gate.

Fix material defects within the authorized scope. Record real unresolved decisions or limitations; no exception record is needed for an intentional design preference or inapplicable row.
