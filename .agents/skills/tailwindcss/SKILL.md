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

Style with utility classes only, and prefer v4 utilities over v3 idioms and hand-written CSS.

## Design tokens (theme-safe)

Every color, background, and border class uses a semantic token so theme and dark-mode switching stays automatic. Reach for a token, never a literal like `bg-white` or `bg-blue-500`:

- Backgrounds — `bg-background`, `bg-card`, `bg-muted`, `bg-popover`
- Text — `text-foreground`, `text-muted-foreground`, `text-card-foreground`
- Borders — `border-border`, `border-input`, `border-ring`
- Actions — `bg-primary text-primary-foreground`, `bg-secondary text-secondary-foreground`
- States — `bg-destructive text-destructive-foreground`, `bg-accent text-accent-foreground`
- Shades — tune with the opacity modifier (`bg-primary/90`), not a new color.

## Class conventions

- Break class strings over 100 characters into a logical array joined into `cn()`/`clsx()`.
- Compose conflicting classes through `tailwind-merge` so the last value wins.
- Use complete, static class names — a lookup map for variants, never an interpolated `bg-${color}-500`.
- Extract shared styles by composing utilities; reserve `@apply` for element defaults in `@layer base`.
- Resolve specificity through class order and merging, not `!important`.

## Reach for v4

Default to v4 utilities: `size-10` over `w-10 h-10`, `h-dvh`/`h-svh` for viewport height, `@container` queries, `bg-(--var)` for CSS variables, `text-shadow-*`, `text-balance`/`text-pretty`. Build layouts mobile-first, adding `sm:`/`md:`/`lg:` for larger screens.

## Before finishing

- Colors, backgrounds, and borders all use semantic tokens.
- Long class strings broken into arrays; class names static; `@apply` only in the base layer; no `!important`.
- `focus-visible:` used for keyboard focus rings.
- `pnpm run lint` and `pnpm run typecheck` pass.

For copy-ready examples and the full utility/variant catalog — extended v4 utilities, focus/peer/group/ARIA variants, dark mode, and anti-pattern examples — see [references/patterns.md](references/patterns.md); when a utility or variant you need is not listed above, read it before choosing classes.
