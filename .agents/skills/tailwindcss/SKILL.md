---
name: tailwindcss
description: Guide for Tailwind CSS v4 patterns and best practices. Use when styling components with Tailwind CSS, creating responsive layouts, working with Tailwind 4 features, or using tailwind-variants for component styling.
allowed-tools: Read, Grep, Glob
---

# Tailwind CSS Developer Guide

This skill provides guidelines, patterns, and best practices for working with Tailwind CSS v4 in this project.

## Quick Reference

For detailed patterns, examples, and checklists, see:
- [references/patterns.md](references/patterns.md) - Complete usage patterns, design tokens, and anti-patterns

## Core Principles

- **Utility-First**: Embrace utility-first approach and avoid custom CSS.
- **Design Tokens**: Always use design system tokens (`bg-background`, `text-foreground`) instead of explicit colors (`bg-white`, `text-black`).
- **Tailwind Variants**: Use `tailwind-variants` (tv) for component styling instead of CVA.
- **Mobile-First**: Build responsive layouts with mobile-first approach.

## Critical: Design Token Usage

To ensure theme switching works correctly:

**Always use:**
- Backgrounds: `bg-background`, `bg-card`, `bg-muted`, `bg-popover`
- Text: `text-foreground`, `text-muted-foreground`, `text-card-foreground`
- Borders: `border-border`, `border-input`, `border-ring`
- Actions: `bg-primary text-primary-foreground`, `bg-secondary text-secondary-foreground`
- States: `bg-destructive text-destructive-foreground`, `bg-accent text-accent-foreground`

**Never use:** `bg-white`, `text-black`, `border-gray-200`, `bg-blue-500`

## Common Tasks

### Component Styling with Tailwind Variants

```typescript
import { tv } from 'tailwind-variants'

const button = tv({
  base: [
    'inline-flex items-center justify-center rounded-md',
    'text-sm font-medium transition-colors',
    'focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring',
  ],
  variants: {
    variant: {
      default: 'bg-primary text-primary-foreground hover:bg-primary/90',
      outline: 'border border-input hover:bg-accent hover:text-accent-foreground',
    },
    size: {
      default: 'h-10 py-2 px-4',
      sm: 'h-9 px-3 rounded-md',
      icon: 'size-10',
    },
  },
  defaultVariants: {
    variant: 'default',
    size: 'default',
  },
})
```

### Long Class Strings

Break class strings longer than 100 characters into arrays:

```typescript
// Good: Break into logical arrays
const card = tv({
  base: [
    'relative flex flex-col rounded-xl border border-border',
    'bg-card text-card-foreground shadow-xs transition-colors duration-150',
  ],
})
```

### Responsive Design

```tsx
// Mobile-first responsive design
<div className="grid grid-cols-1 gap-4 sm:grid-cols-2 md:grid-cols-3 lg:grid-cols-4">

// Container queries (built-in in v4)
<div className="@container">
  <div className="@lg:flex @lg:items-center">
```

### Modern v4 Utilities

```tsx
<div className="size-10">          {/* Instead of w-10 h-10 */}
<div className="h-dvh">            {/* Dynamic viewport height */}
<div className="grid-cols-15">     {/* Dynamic grid columns */}
<h1 className="text-shadow-md">    {/* Text shadows */}
<div className="bg-(--custom-color)">  {/* CSS variables */}
```

## Anti-Patterns to Avoid

- Don't use `@apply` except for base styles
- Avoid inline styles when Tailwind has utilities
- Don't create utility classes that duplicate Tailwind
- Never use `!important` unless absolutely necessary
- Don't construct classes dynamically (`bg-${color}-500`)

## Validation Checklist

Before finishing a task involving Tailwind CSS:

- [ ] Using design tokens instead of explicit colors
- [ ] Long class strings broken into arrays (>100 chars)
- [ ] Using `tailwind-variants` for component styling
- [ ] Mobile-first responsive approach
- [ ] Run lint checks (`pnpm run lint`)

For detailed rules, anti-patterns, and configuration examples, see [references/patterns.md](references/patterns.md).
