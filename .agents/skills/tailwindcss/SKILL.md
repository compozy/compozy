---
name: tailwindcss
description: Tailwind CSS v4 conventions — semantic design tokens for theme-safe styling, mobile-first responsive layouts, and v4-first utilities. Use when styling components or writing className utilities with Tailwind. Don't use for plain CSS, CSS-in-JS (styled-components, emotion), or other utility frameworks.
allowed-tools: Read, Grep, Glob
metadata:
  author: Pedro Nauck
  github: https://github.com/pedronauck
  repository: https://github.com/pedronauck/skills
---
# Tailwind CSS v4

Prefer utilities for component styling. CSS still owns theme tokens, base rules, and styles that utilities do not express clearly. Use the installed Tailwind version and existing design system.

- Use semantic tokens for theme-aware colors, borders, and states. Names come from the project, not a universal shadcn vocabulary; Compozy uses `packages/ui/src/tokens.css` and `DESIGN.md`, including `bg-canvas`, `text-fg`, and `border-line`.
- Keep class names complete and statically discoverable. Use lookup maps instead of interpolated fragments such as `bg-${color}-500`.
- Group long classes for readability; no character-count gate. Use the project's `cn`/merge helper when composing variants or accepting overriding `className` values.
- Resolve specificity through the style design and class merging, avoiding `!important`. Use `@apply` only where the repository permits it.
- Build responsive layouts from content needs, preserve visible keyboard focus, and select supported v4 utilities rather than copying older idioms.
- Validate through the repository's owning gates. In Compozy, use root `make gate`/Turbo commands; a styling edit does not add a separate package-local lint/typecheck ritual.

Read `references/patterns.md` for the utility or variant being changed. Its token names are examples; use the actual theme inventory.
