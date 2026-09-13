# Performance

Use this reference for an observed slowdown, a performance-sensitive change, or a requested performance review. Start with the affected route or interaction and existing evidence. An ordinary UI edit does not introduce a full performance audit or new CI gate.

## Measurement and targets

Use the project's budgets, supported devices, and release policy. Where no budget exists, Core Web Vitals provide useful reference targets: LCP at most 2.5 seconds, INP at most 200 milliseconds, and CLS at most 0.1, assessed at the 75th percentile with mobile and desktop segmented. These are Google's good-experience thresholds, not automatic merge rules supplied by this skill. [Definitions and measurement guidance](https://web.dev/articles/vitals).

Record the build, route, interaction, device/browser, network/cache conditions, and measurement method needed to interpret a result. Compare equivalent conditions. Lab measurements diagnose and reproduce regressions; field data describes actual users. A Lighthouse page-load score or TBT measurement does not establish field INP. [Lab and field measurement](https://web.dev/articles/vitals#lab_tools_to_measure_core_web_vitals).

Use current field data when available and the existing profiler or browser tools for the affected path. Missing telemetry is an uncertainty to report, not a reason to fabricate a percentile or install monitoring for a small edit.

## Loading feedback

Choose feedback from the operation and observed wait:

| Situation | Useful starting point |
| --- | --- |
| Known layout loading | Skeleton or reserved space that approximates the final structure. |
| Mutation with reliable reconciliation and rollback | Optimistic state when the product contract supports it; otherwise clear pending feedback. |
| Unknown duration | An understandable pending state; delay decorative indicators only when needed to avoid flicker. |
| Progress can be measured | A progress indicator tied to real work. |
| Long or failure-prone operation | Explain the state and offer supported cancellation, retry, or recovery. |

Reuse the existing loading pattern and tune timing from the interaction; there is no universal 80ms threshold or mandatory spinner delay. Ensure completion, failure, and cancellation all settle the visible state. Verify that loading feedback does not shift controls or block unrelated work.

## Fonts and images

- Reserve media space with dimensions or aspect ratios appropriate to the responsive layout. Inspect shifts with representative content.
- Use the project's responsive-image tooling and choose formats, quality, and sizes from supported engines and actual asset needs. Prioritize the critical image and defer noncritical media where appropriate.
- Reuse the existing font strategy. Match fallback metrics when font swaps cause layout shifts; generate values for the actual font pair instead of copying arbitrary percentages.
- Preload only demonstrated critical assets. Compare transfer size, subsets, weights, and caching before adding font files; family counts are a heuristic, not a gate.

## Motion and rendering

Prefer transform and opacity for effects they can express. When layout or paint animation is needed, profile the affected elements and reduce costly area, overlap, or frequency. Do not replace required behavior just to satisfy a property blacklist.

Honor reduced-motion preferences while preserving state communication. Use `will-change` only for a demonstrated rendering need and remove it when no longer needed. Consult the relevant `motion-patterns.md` section for implementation details.

## Dependencies and delivery

Inspect the actual bundle when a dependency affects the critical path. Reuse the framework's code splitting and import conventions. Defer expensive optional features when that improves the relevant journey; an editor that is the main product may belong in the initial view.

Package names and import syntax alone do not establish shipped bytes or tree-shaking behavior. Use the installed bundler's output before replacing a library, bypassing a public barrel, or adding a dependency. Keep alternatives within the requested scope.

## Server and data paths

Use traces to distinguish server latency, network transfer, script execution, and rendering. Apply caching only with the owning freshness, authorization, and invalidation contract. Stream or defer work when the product can expose a useful partial result. Do not assume a database query or React render is the bottleneck without evidence.

## Verification and readiness

Replay the affected interaction under comparable conditions after a fix and run the project's applicable checks. Reuse unchanged valid results. Record improvements, regressions, and untested conditions honestly; no repeated full audit is required because a surface uses this skill.

A failed required project budget or a demonstrated unacceptable regression keeps that readiness claim open. Optional measurements and design preferences do not become new release blockers. A focused performance diagnosis is complete when it explains the measured issue and its evidence; implementation work also needs the requested repair and validation.
