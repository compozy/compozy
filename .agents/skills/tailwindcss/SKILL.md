---
name: tailwindcss
description: "Style components with Tailwind CSS v4 and project tokens. Excludes plain CSS, CSS-in-JS, and other utility frameworks."
allowed-tools: Read, Grep, Glob
metadata:
  author: Pedro Nauck
  github: https://github.com/pedronauck
  repository: https://github.com/pedronauck/skills
---
# Tailwind CSS v4

Prefer utilities for component styling. CSS still owns theme tokens, base rules, and styles that utilities do not express clearly. Use the installed Tailwind version and existing design system.

- Use semantic tokens for theme-aware colors, borders, and states. Use names from the project's token source and design documentation; do not assume a universal shadcn vocabulary.
- Keep class names complete and statically discoverable. Use lookup maps instead of interpolated fragments such as `bg-${color}-500`.
- Group long classes for readability; no character-count gate. Use the project's `cn`/merge helper when composing variants or accepting overriding `className` values.
- Resolve specificity through the style design and class merging, avoiding `!important`. Use `@apply` only where the repository permits it.
- Build responsive layouts from content needs, preserve visible keyboard focus, and select supported v4 utilities rather than copying older idioms.
- Validate through the repository's owning gates. Use its existing root/package commands and reuse valid evidence; styling does not add a separate lint/typecheck ritual.

Read `references/patterns.md` for the utility or variant being changed. Its token names are examples; use the actual theme inventory.
