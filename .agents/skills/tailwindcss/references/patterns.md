# Tailwind CSS v4 — patterns and examples

Copy-ready examples and the utility/variant catalog behind [`SKILL.md`](../SKILL.md). The design-token catalog and core rules live there; this file shows how to apply them plus the utilities and variants not listed inline.

## Design tokens: explicit color → token

```tsx
// BAD — breaks theme switching
<div className="bg-white text-black border-gray-200">
<button className="bg-blue-500 hover:bg-blue-600">

// GOOD — semantic tokens, opacity modifier for shades
<div className="bg-background text-foreground border-border">
<button className="bg-primary hover:bg-primary/90">
```

## Class strings

Break strings over 100 characters into grouped arrays joined into `cn()`/`clsx()`:

```typescript
// BAD — single long string
const cardBase =
  'relative flex flex-col rounded-xl border border-border bg-card text-card-foreground shadow-xs transition-colors duration-150'

// GOOD — grouped arrays
const cardBaseClasses = [
  'relative flex flex-col rounded-xl border border-border',
  'bg-card text-card-foreground shadow-xs transition-colors duration-150',
]
```

Order utilities logically — layout, spacing, sizing, color, border, effects, then state:

```tsx
<div className={[
  'flex items-center justify-between',
  'gap-4 p-4',
  'w-full min-h-[200px]',
  'bg-card text-card-foreground',
  'rounded-lg border border-border',
  'shadow-sm',
  'hover:bg-accent transition-colors',
].join(' ')}>
```

## Responsive (mobile-first)

Start mobile, add breakpoints upward:

```tsx
<div className="grid grid-cols-1 gap-4 sm:grid-cols-2 md:grid-cols-3 lg:grid-cols-4">
<div className="flex flex-col gap-2 md:flex-row md:items-center md:justify-between">
<h1 className="text-xl font-bold md:text-2xl lg:text-3xl">
```

Container queries adapt to the parent, not the viewport:

```tsx
<div className="@container">
  <div className="@md:grid @md:grid-cols-2 @lg:flex @lg:items-center">
```

## v4 utilities

```tsx
<div className="size-10">                {/* w-10 h-10 shorthand */}
<div className="h-dvh">                  {/* dynamic viewport height; also h-svh, h-lvh */}
<div className="bg-(--brand-color)/50">  {/* CSS variable + opacity */}
<div className="grid-cols-15">           {/* arbitrary column counts */}
<div className="grid grid-cols-subgrid"> {/* subgrid */}
<h1 className="text-shadow-md">          {/* text shadows: sm / md / lg */}
<h1 className="text-balance">            {/* balanced wrapping; text-pretty avoids orphans */}
```

## Focus and state variants

```tsx
// Keyboard focus only
<button className="focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2">
<div className="focus-within:ring-2 focus-within:ring-ring">

// Group / peer
<div className="group"><span className="group-hover:text-primary" /></div>
<input className="peer" /><span className="peer-invalid:text-destructive">

// ARIA-driven
<button className="aria-disabled:opacity-50 aria-disabled:pointer-events-none">
<div className="aria-expanded:rotate-180">
```

## Dark mode

Tokens switch automatically — add a `dark:` variant only for an explicit override the token can't express.

## Anti-patterns

```tsx
// @apply — only for base element styles, never to bundle component utilities
@layer base { h1 { @apply text-2xl font-bold; } }              // OK
.btn { @apply inline-flex items-center rounded-md px-4 py-2; }  // BAD

// Inline styles — use utilities
<div style={{ padding: '16px' }}>   // BAD  →  <div className="p-4">

// Duplicate utility classes — Tailwind already ships them
.flex-center { display:flex; align-items:center; justify-content:center; }  // BAD
<div className="flex items-center justify-center">                          // GOOD

// !important — fix specificity, don't force it
<div className="text-red-500!">     // BAD  →  <div className="text-destructive">

// Dynamic class names — Tailwind can't see them at build time
<div className={`bg-${color}-500`}>                       // BAD
const map = { red: 'bg-red-500', blue: 'bg-blue-500' }    // OK
<div className="bg-primary">                              // BEST — token
```

## Tailwind merge

```typescript
import { twMerge } from 'tailwind-merge'
const cn = (...inputs: ClassValue[]) => twMerge(clsx(inputs))

<div className={cn('bg-primary', className)}>
```

## Docs

- Tailwind CSS: https://tailwindcss.com/docs
- tailwind-merge: https://github.com/dcastil/tailwind-merge
