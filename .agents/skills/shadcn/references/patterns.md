# Shadcn/UI Patterns and Best Practices

This document provides comprehensive guidelines for working with shadcn/ui components in this project.

## Table of Contents

1. [Core Philosophy](#core-philosophy)
2. [Design Token System](#design-token-system)
3. [Component Patterns](#component-patterns)
4. [Variant System with CVA](#variant-system-with-cva)
5. [Compound Components](#compound-components)
6. [Accessibility Requirements](#accessibility-requirements)
7. [Theming and CSS Variables](#theming-and-css-variables)
8. [Component Extension Patterns](#component-extension-patterns)
9. [Common Anti-Patterns](#common-anti-patterns)

---

## Core Philosophy

Shadcn/ui follows these key principles:

- **Ownership**: You own the code - it's copy-paste, not a dependency
- **Customization**: Components are meant to be modified for your specific needs
- **Accessibility**: Built on Radix UI primitives for proper accessibility
- **Flexibility**: Styled with Tailwind CSS for easy customization
- **Composability**: Components are designed to work together seamlessly

---

## Design Token System

### Critical Rule: Always Use Design Tokens

**Never use explicit color values** like `bg-white`, `text-black`, or `border-gray-200` - they break theme switching.

### Required Background Tokens

```tsx
// Correct - uses design tokens
<div className="bg-background">Main background</div>
<div className="bg-card">Card background</div>
<div className="bg-muted">Muted/secondary background</div>
<div className="bg-popover">Popover/dropdown background</div>
<div className="bg-accent">Accent/hover state</div>

// Wrong - explicit colors
<div className="bg-white">Breaks dark mode</div>
<div className="bg-gray-100">Breaks theme switching</div>
```

### Required Text Tokens

```tsx
// Correct
<p className="text-foreground">Primary text</p>
<p className="text-muted-foreground">Secondary/muted text</p>
<p className="text-card-foreground">Text on cards</p>
<p className="text-popover-foreground">Text in popovers</p>

// Wrong
<p className="text-black">Breaks dark mode</p>
<p className="text-gray-600">Inconsistent theming</p>
```

### Required Border Tokens

```tsx
// Correct
<div className="border-border">Default border</div>
<input className="border-input" />
<div className="ring-ring">Focus ring</div>

// Wrong
<div className="border-gray-200">Breaks theming</div>
```

### Action/State Tokens

```tsx
// Primary actions
<button className="bg-primary text-primary-foreground">Primary</button>

// Secondary actions
<button className="bg-secondary text-secondary-foreground">Secondary</button>

// Destructive actions
<button className="bg-destructive text-destructive-foreground">Delete</button>

// Accent/hover states
<div className="bg-accent text-accent-foreground">Hovered item</div>
```

### Shadows and Effects

```tsx
// Use semantic shadow tokens
<div className="shadow-xs">Subtle elevation</div>
<div className="shadow-sm">Small elevation</div>

// For colored shadows, use token-based opacity
<button className="shadow-primary/24">Primary button shadow</button>
```

---

## Component Patterns

### Basic Component Structure

```tsx
import type * as React from "react";
import { cn } from "../../lib/utils";

interface CardProps extends React.ComponentProps<"div"> {
  // Add custom props here
}

function Card({ className, ...props }: CardProps) {
  return (
    <div
      className={cn(
        "rounded-md border bg-card text-card-foreground shadow-xs",
        className
      )}
      data-slot="card"
      {...props}
    />
  );
}

export { Card };
```

### Key Patterns

1. **Use `data-slot` attributes** for component identification
2. **Always spread `className` through `cn()`** for merge support
3. **Use `ComponentProps<"element">` type** for proper typing
4. **Spread remaining props** with `{...props}`

---

## Variant System with CVA

Use `class-variance-authority` for components with multiple variants.

### Basic CVA Setup

```tsx
import { cva, type VariantProps } from "class-variance-authority";
import { cn } from "../../lib/utils";

const buttonVariants = cva(
  // Base styles (always applied)
  "inline-flex items-center justify-center rounded-md font-medium transition-colors focus-visible:ring-2 focus-visible:ring-ring disabled:pointer-events-none disabled:opacity-50",
  {
    variants: {
      variant: {
        default: "bg-primary text-primary-foreground hover:bg-primary/90",
        destructive: "bg-destructive text-destructive-foreground hover:bg-destructive/90",
        outline: "border border-input bg-background hover:bg-accent",
        secondary: "bg-secondary text-secondary-foreground hover:bg-secondary/90",
        ghost: "hover:bg-accent hover:text-accent-foreground",
        link: "text-primary underline-offset-4 hover:underline",
      },
      size: {
        default: "h-9 px-4 py-2",
        sm: "h-8 px-3 text-xs",
        lg: "h-10 px-6",
        icon: "size-9",
      },
    },
    defaultVariants: {
      variant: "default",
      size: "default",
    },
  }
);

export interface ButtonProps
  extends React.ButtonHTMLAttributes<HTMLButtonElement>,
    VariantProps<typeof buttonVariants> {
  // Additional custom props
}

function Button({ className, variant, size, ...props }: ButtonProps) {
  return (
    <button
      className={cn(buttonVariants({ variant, size, className }))}
      {...props}
    />
  );
}

export { Button, buttonVariants };
```

### Compound Variants

```tsx
const inputVariants = cva("...", {
  variants: {
    variant: { default: "...", error: "..." },
    size: { sm: "...", md: "...", lg: "..." },
  },
  compoundVariants: [
    {
      variant: "error",
      size: "lg",
      className: "border-2", // Extra border for large error inputs
    },
  ],
});
```

---

## Compound Components

### Required Pattern: Separate Exports

**Always use separate exports for compound components. Never use namespaced properties.**

```tsx
// CORRECT - Separate exports
export const Card = CardRoot;
export const CardHeader = Header;
export const CardTitle = Title;
export const CardContent = Content;
export const CardFooter = Footer;

// Usage
import { Card, CardHeader, CardTitle, CardContent } from "@/components/card";

<Card>
  <CardHeader>
    <CardTitle>Title</CardTitle>
  </CardHeader>
  <CardContent>Content</CardContent>
</Card>
```

```tsx
// WRONG - Namespaced properties (do NOT use)
export const Card = Object.assign(CardRoot, {
  Header: Header,
  Title: Title,
  Content: Content,
});

// This pattern causes issues with tree-shaking and TypeScript
```

### Tech Component Extension Pattern

When creating themed variants of base components:

```tsx
import {
  Card,
  CardHeader,
  CardTitle,
  CardContent,
  CardFooter,
} from "../base/card";

function CardTechRoot({ className, children, ...props }: CardTechProps) {
  return (
    <Card className={cn("custom-tech-styles", className)} {...props}>
      {/* Add decorative elements */}
      <TechDecorations />
      {children}
    </Card>
  );
}

// Re-export base components with new names
export const CardTech = CardTechRoot;
export const CardTechHeader = CardHeader;
export const CardTechTitle = CardTitle;
export const CardTechContent = CardContent;
export const CardTechFooter = CardFooter;
```

---

## Accessibility Requirements

### Never Remove Radix UI Accessibility Attributes

Radix UI primitives provide built-in accessibility. Never remove or override these patterns:

```tsx
// Dialog with proper accessibility
<Dialog>
  <DialogTrigger asChild>
    <Button>Open Dialog</Button>
  </DialogTrigger>
  <DialogContent>
    <DialogHeader>
      {/* DialogTitle is required for screen readers */}
      <DialogTitle>Accessible Title</DialogTitle>
      {/* DialogDescription helps screen readers understand purpose */}
      <DialogDescription>
        This description provides context for the dialog.
      </DialogDescription>
    </DialogHeader>
    {/* Content */}
  </DialogContent>
</Dialog>
```

### Required Accessibility Patterns

1. **Always include `DialogTitle`** in dialogs
2. **Use `asChild` prop** when wrapping custom trigger elements
3. **Preserve keyboard navigation** - don't break tab order
4. **Include `aria-label`** on icon-only buttons:

```tsx
<Button size="icon" aria-label="Close dialog">
  <X className="size-4" />
</Button>
```

### Focus Management

```tsx
// Proper focus ring styling
<button className="focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2">
  Focusable
</button>

// For inputs
<input className="focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-1" />
```

---

## Theming and CSS Variables

### Global CSS Variables Setup

Define color variables in `globals.css`:

```css
@layer base {
  :root {
    /* Core colors */
    --background: 0 0% 100%;
    --foreground: 222.2 84% 4.9%;

    /* Card */
    --card: 0 0% 100%;
    --card-foreground: 222.2 84% 4.9%;

    /* Primary */
    --primary: 222.2 47.4% 11.2%;
    --primary-foreground: 210 40% 98%;

    /* Secondary */
    --secondary: 210 40% 96.1%;
    --secondary-foreground: 222.2 47.4% 11.2%;

    /* Muted */
    --muted: 210 40% 96.1%;
    --muted-foreground: 215.4 16.3% 46.9%;

    /* Accent */
    --accent: 210 40% 96.1%;
    --accent-foreground: 222.2 47.4% 11.2%;

    /* Destructive */
    --destructive: 0 84.2% 60.2%;
    --destructive-foreground: 210 40% 98%;

    /* Border and input */
    --border: 214.3 31.8% 91.4%;
    --input: 214.3 31.8% 91.4%;
    --ring: 222.2 84% 4.9%;

    /* Radius */
    --radius: 0.5rem;
  }

  .dark {
    --background: 222.2 84% 4.9%;
    --foreground: 210 40% 98%;
    /* ... dark mode overrides */
  }
}
```

### Adding Custom Semantic Colors

```css
@layer base {
  :root {
    /* Success states */
    --success: 142 76% 36%;
    --success-foreground: 355 100% 100%;

    /* Warning states */
    --warning: 38 92% 50%;
    --warning-foreground: 0 0% 0%;

    /* Info states */
    --info: 199 89% 48%;
    --info-foreground: 0 0% 100%;
  }

  .dark {
    --success: 142 76% 46%;
    --success-foreground: 142 76% 10%;
    /* ... */
  }
}
```

Then extend Tailwind config:

```js
// tailwind.config.js
module.exports = {
  theme: {
    extend: {
      colors: {
        success: "hsl(var(--success))",
        "success-foreground": "hsl(var(--success-foreground))",
        warning: "hsl(var(--warning))",
        "warning-foreground": "hsl(var(--warning-foreground))",
      },
    },
  },
};
```

---

## Component Extension Patterns

### Adding Loading State to Button

```tsx
import { Loader2 } from "lucide-react";

export interface ButtonProps
  extends React.ButtonHTMLAttributes<HTMLButtonElement>,
    VariantProps<typeof buttonVariants> {
  loading?: boolean;
}

function Button({
  className,
  variant,
  size,
  loading,
  disabled,
  children,
  ...props
}: ButtonProps) {
  return (
    <button
      className={cn(buttonVariants({ variant, size, className }))}
      disabled={loading || disabled}
      {...props}
    >
      {loading && <Loader2 className="mr-2 size-4 animate-spin" />}
      {children}
    </button>
  );
}
```

### Using useRender for Polymorphic Components

For components that need to render as different elements:

```tsx
import { useRender } from "@base-ui/react/use-render";
import { mergeProps } from "@base-ui/react/merge-props";

export interface ButtonProps extends useRender.ComponentProps<"button"> {
  variant?: VariantProps<typeof buttonVariants>["variant"];
  size?: VariantProps<typeof buttonVariants>["size"];
}

function Button({ className, variant, size, render, ...props }: ButtonProps) {
  const defaultProps = {
    className: cn(buttonVariants({ className, size, variant })),
    "data-slot": "button",
    ...(render ? {} : { type: "button" as const }),
  };

  return useRender({
    defaultTagName: "button",
    props: mergeProps<"button">(defaultProps, props),
    ...(render !== undefined ? { render } : {}),
  });
}
```

Usage:

```tsx
// As button (default)
<Button>Click me</Button>

// As link
<Button render={<a href="/path" />}>Navigate</Button>

// As custom component
<Button render={<RouterLink to="/path" />}>Route</Button>
```

---

## Common Anti-Patterns

### 1. Using Explicit Colors

```tsx
// BAD
<div className="bg-white text-black border-gray-200">
  Breaks theme switching
</div>

// GOOD
<div className="bg-background text-foreground border-border">
  Works with all themes
</div>
```

### 2. Namespaced Component Exports

```tsx
// BAD
export const Card = Object.assign(CardRoot, { Header, Title });

// GOOD
export const Card = CardRoot;
export const CardHeader = Header;
export const CardTitle = Title;
```

### 3. Removing Accessibility Attributes

```tsx
// BAD - Missing required elements
<Dialog>
  <DialogContent>
    <div>Content without title</div>
  </DialogContent>
</Dialog>

// GOOD - Includes required accessibility elements
<Dialog>
  <DialogContent>
    <DialogHeader>
      <DialogTitle>Title</DialogTitle>
    </DialogHeader>
    <div>Content</div>
  </DialogContent>
</Dialog>
```

### 4. Hardcoded Styles Instead of Variants

```tsx
// BAD - Hardcoded conditional styles
<button className={isLarge ? "px-6 py-3" : "px-4 py-2"}>

// GOOD - Use CVA variants
<Button size={isLarge ? "lg" : "default"}>
```

### 5. Not Using cn() for Class Merging

```tsx
// BAD - className override doesn't work properly
<div className={`base-styles ${className}`}>

// GOOD - Proper class merging with cn()
<div className={cn("base-styles", className)}>
```

### 6. Inline Styles for Themeable Values

```tsx
// BAD - Won't respond to theme changes
<div style={{ backgroundColor: "#ffffff" }}>

// GOOD - Uses CSS variables
<div className="bg-background">
```

---

## Validation Checklist

Before completing any task involving shadcn/ui components:

- [ ] All colors use design tokens, not explicit values (no `bg-white`, `text-black`, etc.)
- [ ] Accessibility attributes from Radix UI are preserved
- [ ] Keyboard navigation works correctly
- [ ] Compound components use separate exports (not namespaced)
- [ ] `cn()` is used for all className merging
- [ ] `data-slot` attributes are added for component identification
- [ ] Focus states use `focus-visible:ring-*` tokens
- [ ] Variants are implemented with CVA when multiple options exist
- [ ] Type checks pass (`pnpm run typecheck`)
- [ ] Tests pass (`pnpm run test`)
