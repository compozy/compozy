# shadcn/ui Learning Guide

This guide helps you learn shadcn/ui from basics to advanced patterns.

## Learning Path

### 1. Understanding the Philosophy

shadcn/ui is different from traditional component libraries:

- **Copy-paste components**: Components are copied into your project, not installed as packages
- **Full customization**: You own the code and can modify it freely
- **Built on Radix UI**: Provides accessibility primitives
- **Styled with Tailwind**: Uses utility classes for consistent styling

### 2. Core Concepts to Master

#### Class Variance Authority (CVA)

Most components use CVA for variant management:

```tsx
const buttonVariants = cva("base-classes", {
  variants: {
    variant: {
      default: "variant-classes",
      destructive: "destructive-classes",
    },
    size: {
      default: "size-classes",
      sm: "small-classes",
    },
  },
  defaultVariants: {
    variant: "default",
    size: "default",
  },
});
```

#### cn Utility Function

The `cn` function combines classes and resolves conflicts:

```tsx
import { clsx, type ClassValue } from "clsx";
import { twMerge } from "tailwind-merge";

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}
```

### 3. Installation Checklist

- [ ] Initialize a new project (Next.js, Vite, or Remix)
- [ ] Install Tailwind CSS
- [ ] Run `npx shadcn@latest init`
- [ ] Configure CSS variables
- [ ] Install first component: `npx shadcn@latest add button`

### 4. Essential Components to Learn First

1. **Button** - Learn variants and sizes
2. **Input** - Form inputs with labels
3. **Card** - Container components
4. **Form** - Form handling with React Hook Form
5. **Dialog** - Modal windows
6. **Select** - Dropdown selections
7. **Toast** - Notifications

### 5. Common Patterns

#### Form Pattern

Every form follows this structure:

```tsx
1. Define Zod schema
2. Create form with useForm
3. Wrap with Form component
4. Add FormField for each input
5. Handle submission
```

#### Component Customization Pattern

To customize a component:

1. Copy component to your project
2. Modify the variants
3. Add new props if needed
4. Update types

### 6. Best Practices

- Always use TypeScript
- Follow the existing component structure
- Use semantic HTML when possible
- Test with screen readers for accessibility
- Keep components small and focused

### 7. Advanced Topics

- Creating custom components from scratch
- Building complex forms with validation
- Implementing dark mode
- Optimizing for performance
- Testing components

## Practice Exercises

### Exercise 1: Basic Setup

1. Create a new Next.js project
2. Set up shadcn/ui
3. Install and customize a Button component
4. Add a new variant "gradient"

### Exercise 2: Form Building

1. Create a contact form with:
   - Name input (required)
   - Email input (email validation)
   - Message textarea (min length)
   - Submit button with loading state

### Exercise 3: Component Combination

1. Build a settings page using:
   - Card for layout
   - Sheet for mobile menu
   - Select for dropdowns
   - Switch for toggles
   - Toast for notifications

### Exercise 4: Custom Component

1. Create a custom Badge component
2. Support variants: default, secondary, destructive, outline
3. Support sizes: sm, default, lg
4. Add icon support

## Resources

- [Official Documentation](https://ui.shadcn.com)
- [GitHub Repository](https://github.com/shadcn/ui)
- [Examples Gallery](https://ui.shadcn.com/examples)
- [Radix UI Primitives](https://www.radix-ui.com/primitives)
- [Tailwind CSS Documentation](https://tailwindcss.com/docs)
