---
name: shadcn
description: Best practices for building UI components with shadcn/ui. Use when creating, customizing, or styling components with shadcn, working with Radix UI primitives, implementing design tokens, or following compound component patterns.
allowed-tools: Read, Grep, Glob
---

# Shadcn UI Developer Guide

This skill provides guidelines, patterns, and best practices for working with shadcn/ui components in this project.

## Quick Start

1. **Core Patterns**: Read `references/patterns.md` for comprehensive styling, theming, and component patterns.
2. **Tailwind Integration**: For Tailwind CSS patterns, use the `tailwindcss` skill.

## Core Philosophy

- **Ownership**: Shadcn is copy-paste, not a dependency - you own the code.
- **Customization**: Components are meant to be modified for your needs.
- **Accessibility**: Built on Radix UI primitives for proper accessibility.
- **Flexibility**: Styled with Tailwind CSS for easy customization.

## Critical: Design Token Usage

**Always use design system tokens for theme switching compatibility:**

**Required tokens:**
- Backgrounds: `bg-background`, `bg-card`, `bg-muted`, `bg-popover`
- Text: `text-foreground`, `text-muted-foreground`, `text-card-foreground`
- Borders: `border-border`, `border-input`, `border-ring`
- Actions: `bg-primary text-primary-foreground`, `bg-secondary text-secondary-foreground`
- States: `bg-destructive text-destructive-foreground`, `bg-accent text-accent-foreground`

**Never use explicit colors** like `bg-white`, `text-black`, `border-gray-200` - they break theme switching.

## Common Tasks

### Adding Components

Use the CLI to add components, then customize as needed:

```bash
npx shadcn-ui@latest add button card dialog
```

### Extending Components

Add custom props and variants while preserving accessibility:

```typescript
export interface ButtonProps
  extends React.ButtonHTMLAttributes<HTMLButtonElement>,
    VariantProps<typeof buttonVariants> {
  loading?: boolean
}

const Button = React.forwardRef<HTMLButtonElement, ButtonProps>(
  ({ className, variant, size, loading, children, ...props }, ref) => {
    return (
      <button
        className={cn(buttonVariants({ variant, size, className }))}
        ref={ref}
        disabled={loading || props.disabled}
        {...props}
      >
        {loading && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
        {children}
      </button>
    )
  }
)
```

### Compound Components

**Always use separate exports, NOT namespaced properties:**

```tsx
// CORRECT
export const CardTech = CardTechRoot;
export const CardTechHeader = CardHeader;
export const CardTechTitle = CardTitle;
export const CardTechContent = CardContent;

// WRONG - Do not use
export const CardTech = Object.assign(CardTechRoot, {
  Header: CardHeader,
  Content: CardContent,
});
```

### Preserving Accessibility

Never remove Radix UI's accessibility attributes:

```tsx
<Dialog>
  <DialogTrigger asChild>
    <Button>Open Dialog</Button>
  </DialogTrigger>
  <DialogContent>
    <DialogHeader>
      <DialogTitle>Accessible Title</DialogTitle>
      <DialogDescription>
        This description helps screen readers understand the dialog's purpose
      </DialogDescription>
    </DialogHeader>
  </DialogContent>
</Dialog>
```

## Theming

Use CSS variables for all color values in `globals.css`:

```css
@layer base {
  :root {
    --success: 142 76% 36%;
    --success-foreground: 355 100% 100%;
  }

  .dark {
    --success: 142 76% 46%;
    --success-foreground: 142 76% 10%;
  }
}
```

## Validation Checklist

Before finishing a task involving shadcn/ui:

- [ ] Verify all colors use design tokens, not explicit values.
- [ ] Ensure accessibility attributes from Radix UI are preserved.
- [ ] Test keyboard navigation works correctly.
- [ ] Use separate exports for compound components (not namespaced).
- [ ] Run type checks (`pnpm run typecheck`) and tests (`pnpm run test`).

For comprehensive patterns, component examples, and detailed guidelines, read `references/patterns.md`.
