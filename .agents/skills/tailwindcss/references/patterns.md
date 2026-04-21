# Tailwind CSS v4 Patterns and Best Practices

This document contains comprehensive patterns and best practices for using Tailwind CSS v4 in this project.

## Core Principles

- **Utility-First**: Embrace utility-first approach and avoid custom CSS
- **Design Tokens**: Always use design system tokens instead of explicit colors
- **Tailwind Variants**: Use `tailwind-variants` (tv) for component styling instead of CVA
- **Mobile-First**: Build responsive layouts with mobile-first approach
- **No @apply**: Avoid `@apply` except for base styles

## Critical: Design Token Usage

To ensure theme switching works correctly, always use design system tokens:

### Required Tokens

**Backgrounds:**
- `bg-background` - Main page background
- `bg-card` - Card surfaces
- `bg-muted` - Muted/subtle backgrounds
- `bg-popover` - Popover/dropdown backgrounds

**Text:**
- `text-foreground` - Primary text
- `text-muted-foreground` - Secondary/subtle text
- `text-card-foreground` - Text on card surfaces

**Borders:**
- `border-border` - Standard borders
- `border-input` - Form input borders
- `border-ring` - Focus ring borders

**Actions:**
- `bg-primary text-primary-foreground` - Primary actions
- `bg-secondary text-secondary-foreground` - Secondary actions

**States:**
- `bg-destructive text-destructive-foreground` - Destructive actions
- `bg-accent text-accent-foreground` - Accent/highlight states

### Never Use Explicit Colors

```tsx
// BAD - Breaks theme switching
<div className="bg-white text-black border-gray-200">

// GOOD - Uses design tokens
<div className="bg-background text-foreground border-border">

// BAD - Explicit color values
<button className="bg-blue-500 hover:bg-blue-600">

// GOOD - Semantic tokens
<button className="bg-primary hover:bg-primary/90">
```

## Component Styling with Tailwind Variants

Use `tailwind-variants` for component styling:

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
      destructive: 'bg-destructive text-destructive-foreground hover:bg-destructive/90',
      outline: 'border border-input hover:bg-accent hover:text-accent-foreground',
      secondary: 'bg-secondary text-secondary-foreground hover:bg-secondary/80',
      ghost: 'hover:bg-accent hover:text-accent-foreground',
      link: 'underline-offset-4 hover:underline text-primary',
    },
    size: {
      default: 'h-10 py-2 px-4',
      sm: 'h-9 px-3 rounded-md',
      lg: 'h-11 px-8 rounded-md',
      icon: 'size-10',
    },
  },
  defaultVariants: {
    variant: 'default',
    size: 'default',
  },
})

// Usage
<button className={button({ variant: 'outline', size: 'sm' })}>
```

### Compound Variants

```typescript
const card = tv({
  base: 'rounded-lg border bg-card text-card-foreground shadow-sm',
  variants: {
    interactive: {
      true: 'cursor-pointer',
    },
    elevated: {
      true: 'shadow-md',
    },
  },
  compoundVariants: [
    {
      interactive: true,
      className: 'hover:bg-accent transition-colors',
    },
  ],
})
```

## Long Class Strings

Break class strings longer than 100 characters into arrays:

```typescript
// BAD - Single long string
const card = tv({
  base: 'relative flex flex-col rounded-xl border border-border bg-card text-card-foreground shadow-xs transition-colors duration-150',
})

// GOOD - Broken into logical arrays
const card = tv({
  base: [
    'relative flex flex-col rounded-xl border border-border',
    'bg-card text-card-foreground shadow-xs transition-colors duration-150',
  ],
})
```

## Responsive Design

### Mobile-First Approach

Always build mobile-first and add breakpoints for larger screens:

```tsx
// Mobile-first responsive grid
<div className="grid grid-cols-1 gap-4 sm:grid-cols-2 md:grid-cols-3 lg:grid-cols-4">

// Mobile-first layout
<div className="flex flex-col gap-2 md:flex-row md:items-center md:justify-between">

// Mobile-first typography
<h1 className="text-xl font-bold md:text-2xl lg:text-3xl">
```

### Container Queries (v4 Feature)

```tsx
// Define container
<div className="@container">
  <div className="@lg:flex @lg:items-center @md:grid @md:grid-cols-2">
    Content adapts to container size
  </div>
</div>
```

### Breakpoint Reference

| Prefix | Min Width | Common Use |
|--------|-----------|------------|
| `sm:` | 640px | Small tablets |
| `md:` | 768px | Tablets |
| `lg:` | 1024px | Laptops |
| `xl:` | 1280px | Desktops |
| `2xl:` | 1536px | Large screens |

## Tailwind CSS v4 Modern Utilities

### Size Shorthand

```tsx
// Instead of w-10 h-10
<div className="size-10">

// Useful for icons and avatars
<Avatar className="size-8" />
```

### Dynamic Viewport Units

```tsx
// Dynamic viewport height (accounts for mobile browser chrome)
<div className="h-dvh">

// Small viewport height
<div className="h-svh">

// Large viewport height
<div className="h-lvh">
```

### CSS Variables

```tsx
// Direct CSS variable usage
<div className="bg-(--custom-color)">

// With opacity modifier
<div className="bg-(--brand-color)/50">
```

### Modern Grid

```tsx
// Dynamic grid columns (v4 feature)
<div className="grid-cols-15">

// Subgrid support
<div className="grid grid-cols-subgrid">
```

### Text Shadows

```tsx
<h1 className="text-shadow-sm">
<h1 className="text-shadow-md">
<h1 className="text-shadow-lg">
```

### Balance and Wrap Utilities

```tsx
// Balanced text wrapping
<h1 className="text-balance">

// Prevent orphans
<p className="text-pretty">
```

## Animation and Transitions

### Built-in Animations

```tsx
<div className="animate-spin">
<div className="animate-ping">
<div className="animate-pulse">
<div className="animate-bounce">
```

### Custom Transitions

```tsx
// Standard transition
<button className="transition-colors duration-150">

// Multiple properties
<div className="transition-all duration-300 ease-in-out">

// Hover states
<div className="hover:scale-105 transition-transform">
```

## Focus States

### Focus Visible (Keyboard Only)

```tsx
// Keyboard focus only (accessibility best practice)
<button className="focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2">

// Focus within for containers
<div className="focus-within:ring-2 focus-within:ring-ring">
```

## State Variants

### Peer and Group

```tsx
// Group hover
<div className="group">
  <span className="group-hover:text-primary">
</div>

// Peer states (for form validation)
<input className="peer" />
<span className="peer-invalid:text-destructive">
```

### Aria States

```tsx
<button className="aria-disabled:opacity-50 aria-disabled:pointer-events-none">
<div className="aria-expanded:rotate-180">
```

## Dark Mode

```tsx
// Dark mode variants
<div className="bg-background dark:bg-background">
<span className="text-foreground dark:text-foreground">

// Note: Design tokens handle this automatically
// Only use dark: prefix when you need explicit overrides
```

## Anti-Patterns to Avoid

### DON'T: Use @apply Excessively

```css
/* BAD - Defeats the purpose of utility classes */
.btn {
  @apply inline-flex items-center justify-center rounded-md px-4 py-2;
}

/* OK - Only for base styles that truly need extraction */
@layer base {
  h1 {
    @apply text-2xl font-bold;
  }
}
```

### DON'T: Use Inline Styles

```tsx
// BAD
<div style={{ padding: '16px' }}>

// GOOD
<div className="p-4">
```

### DON'T: Create Duplicate Utility Classes

```css
/* BAD - Tailwind already has these */
.flex-center {
  display: flex;
  align-items: center;
  justify-content: center;
}

/* GOOD - Use Tailwind directly */
<div className="flex items-center justify-center">
```

### DON'T: Use !important

```tsx
// BAD
<div className="!text-red-500">

// GOOD - Fix the specificity issue properly
<div className="text-destructive">
```

### DON'T: Construct Classes Dynamically

```tsx
// BAD - Tailwind can't detect these at build time
<div className={`bg-${color}-500`}>

// GOOD - Use complete class names
const colorClasses = {
  red: 'bg-red-500',
  blue: 'bg-blue-500',
}
<div className={colorClasses[color]}>

// BETTER - Use design tokens
<div className="bg-primary">
```

## Utility Organization

When multiple utilities are needed, organize them logically:

```tsx
<div className={[
  // Layout
  'flex items-center justify-between',
  // Spacing
  'gap-4 p-4',
  // Sizing
  'w-full min-h-[200px]',
  // Colors (using design tokens)
  'bg-card text-card-foreground',
  // Borders
  'rounded-lg border border-border',
  // Effects
  'shadow-sm',
  // States
  'hover:bg-accent transition-colors',
].join(' ')}>
```

## Tailwind Merge

When composing classes with potential conflicts, use `tailwind-merge`:

```typescript
import { twMerge } from 'tailwind-merge'

const cn = (...inputs: ClassValue[]) => twMerge(clsx(inputs))

// Usage
<div className={cn('bg-primary', className)}>
```

## Validation Checklist

Before finishing a task involving Tailwind CSS:

- [ ] Using design tokens instead of explicit colors (`bg-background`, not `bg-white`)
- [ ] Long class strings broken into arrays (>100 chars)
- [ ] Using `tailwind-variants` for component styling with variants
- [ ] Mobile-first responsive approach (`sm:` `md:` `lg:` prefixes for larger screens)
- [ ] Focus states use `focus-visible:` for keyboard accessibility
- [ ] No `@apply` except for base styles
- [ ] No inline styles when Tailwind utilities exist
- [ ] No dynamically constructed class names
- [ ] No `!important` usage
- [ ] Run lint checks (`pnpm run lint`)
- [ ] Run type checks (`pnpm run typecheck`)

## Documentation

- **Official Docs**: https://tailwindcss.com/docs
- **Tailwind Variants**: https://www.tailwind-variants.org/
- **Tailwind Merge**: https://github.com/dcastil/tailwind-merge
