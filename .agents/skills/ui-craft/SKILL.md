---
name: ui-craft
description: "Design, build, review, or refactor visible UI, including pages, components, forms, loading/error states, and AI chat/streaming interfaces. Excludes backend, infrastructure, CLI/TUI, and docs-only work."
metadata:
  author: Pedro Nauck
  github: https://github.com/pedronauck
  repository: https://github.com/pedronauck/skills
---

# UI Craft

Use the project's visual and copy authorities, shipped primitive inventory, and real runtime behavior. Preserve keyboard/focus operation, supported states, meaningful labels, contrast, reduced motion, and token ownership.

For a small edit, inspect the affected component/state and reuse existing decisions. A new surface or substantial redesign may need a short user job, scene, register, or visual dials when those resolve an actual design choice; they are not required artifacts for every tweak.

- Read the relevant design/token/copy contract. When generated documentation and runtime tokens disagree, follow the project's canonical-source rule and regenerate through its owning tool.
- Reuse existing primitives and supported states. Do not invent controls, metrics, helper copy, brand variants, or host chrome to satisfy a prototype.
- Match verification to changed interaction/visual behavior: render the affected states, check keyboard/focus where interaction changes, and compare a named reference when required. Reuse current evidence; whole-surface matrices and all-breakpoint sweeps need corresponding scope.
- Review actual hierarchy, overflow, token drift, misleading interaction, and accessibility defects. Avoid a fixed scorecard that turns subjective preferences into universal merge blockers.
- Update design authorities only for an actual token/pattern change, through their generator when owned by codegen. A missing optional reference does not justify scaffolding a new design system.

## References by Need

Use the matching section and its necessary dependencies. The table is a lookup, not a list to load in full. `assets/state-matrix.md`, `assets/pre-ship-checklist.md`, and `assets/ui-audit-template.md` are optional working formats for substantial design/audit tasks; ordinary changes can use concise evidence.

| When                                                            | File                                    | Covers                                                              |
| --------------------------------------------------------------- | --------------------------------------- | ------------------------------------------------------------------ |
| Designing a novel surface / auditing usability                  | `references/usability-foundations.md`   | Nielsen 10, Laws of UX, progressive disclosure, mental models      |
| Any interactive widget (dialog, menu, combobox, tabs, slider)   | `references/accessibility-floor.md`     | WCAG 2.2 AA non-negotiables, ARIA patterns, verification recipes   |
| Buttons/inputs/dialogs/nav/tables/cards/dropdowns/tabs/toasts   | `references/component-patterns.md`      | States, keyboard contract, a11y, and slop per component            |
| Writing or reviewing user-visible text                          | `references/microcopy-quality.md`       | Banned vocabulary, per-surface tone, error/empty/CTA templates     |
| Auditing AI-generated UI                                        | `references/ai-slop-patterns.md`        | Before/after + remediation for the 14 patterns, detection prompts  |
| A choice feels like a category-average default                  | `references/anti-defaults.md`           | 17 literal artifacts to refuse on sight                            |
| Building an AI/agent surface                                    | `references/human-ai-ux.md`             | Microsoft 18 + IBM 4 pillars, streaming/citation/confidence UI     |
| Reading/extending DESIGN.md, tokens, shadcn/Radix, codegen rules| `references/design-system-integration.md`| DESIGN.md sections, token discipline, Figma/Code Connect          |
| No token system, or extending a scale                           | `references/visual-craft.md`            | Type/color/spacing/radius/elevation/motion defaults                |
| Adding or reviewing animation                                   | `references/motion-patterns.md`         | Duration bands, easing, CSS/Framer/`@starting-style`, reduced-motion|
| Implementing or reviewing dark mode                             | `references/dark-mode.md`               | Surface-lightness elevation, accent desaturation, dangerous pairs  |
| Unprofiled ship, CWV regress, heavy dep on a hot route          | `references/performance.md`             | CWV floors, 80ms threshold, font zero-CLS, pre-ship gate           |
| "We need a [Linear/Notion]-like X"                              | `references/archetypes.md`              | 7 archetypes, each a six-slot signature contract                   |

## Helpers

Resolve these from the actual skill directory (`<ui-craft-dir>`); run only for the question in scope:

- `node <ui-craft-dir>/scripts/check-contrast.mjs` — read-only contrast calculation for changed rendered colors.
- `node <ui-craft-dir>/scripts/detect-token-drift.mjs` — read-only drift scan; use project lint/codegen evidence when it already owns the invariant.
- `python3 <ui-craft-dir>/scripts/validate-metadata.py` — authoring metadata validation.

Follow explicit user decisions and project authorities for trade-offs. Record material limitations honestly; do not force a scene interview, design audit, or another approval solely because this skill was used.
