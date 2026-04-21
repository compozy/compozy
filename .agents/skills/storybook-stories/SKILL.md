---
name: storybook-stories
description: Create, update, or refactor Storybook stories following the project's standard patterns. Use this skill when adding stories for new components, updating existing stories, or fixing Storybook-related issues.
---

# Storybook Stories

This skill enforces consistent Storybook story creation patterns across the application. It ensures that all components have proper documentation, interactive examples, and follow the established project structure.

<critical_component_usage>
**MANDATORY: Always Use Base UI Components from @agh/ui**

**CRITICAL REQUIREMENTS:**

- ✅ **ALWAYS** import components from `@agh/ui` package (`packages/ui`)
- ✅ **ALWAYS** use existing base UI components instead of creating new ones from scratch
- ✅ **ALWAYS** follow design system rules from `@.cursor/rules/react.mdc` and `@.cursor/rules/shadcn.mdc`
- ✅ **ALWAYS** use design tokens (e.g., `bg-background`, `text-foreground`, `border-border`) instead of explicit colors
- ❌ **NEVER** create components from scratch when a base component exists in `@agh/ui`
- ❌ **NEVER** use explicit color values (e.g., `bg-white`, `text-black`) - always use design tokens
- ❌ **NEVER** duplicate component logic - compose from base components

**Available Base Components:**
All components from `packages/ui/src/components` are available via `@agh/ui`:

- Button, Card, Dialog, Input, Select, Badge, Avatar, Accordion, Alert, etc.
- See `packages/ui/src/index.ts` for complete list of exports

**Design System Rules:**

- Follow React best practices: `@.cursor/rules/react.mdc`
- Follow Shadcn UI patterns: `@.cursor/rules/shadcn.mdc`
- Use design tokens for theming: `bg-background`, `text-foreground`, `border-border`, etc.
  </critical_component_usage>

## Instructions

1. **File Location & Naming**
   - Place story files in a `stories/` folder within the same category folder as the component.
   - Example: `src/components/base/accordion.tsx` -> `src/components/base/stories/accordion.stories.tsx`.
   - Use the Storybook instance that matches the layer:
     - `packages/ui/.storybook` for `packages/ui/src/components/*.stories.tsx`
     - `web/.storybook` for `web/src/components/ui/**/*.stories.tsx` and `web/src/systems/**/components/stories/*.stories.tsx`

2. **Component Imports**
   - **MANDATORY**: Import base UI components from `@agh/ui`
   - Use: `import { Button, Card, Dialog } from "@agh/ui";`
   - Only import custom/domain-specific components from local files
   - Check `packages/ui/src/index.ts` to see available components before creating new ones

3. **Meta Configuration**
   - Title should follow the directory structure: `components/custom/ComponentName` or `components/ui/ComponentName`.
   - Include `component` in the meta object.
   - Set `parameters.layout` to `"centered"` by default.
   - Add `parameters.docs.description.component` to describe the component.
   - Use `decorators` if the component requires a specific container width or context.
   - **MANDATORY**: Use explicit type annotation: `const meta: Meta<typeof Component> = { ... }`
   - `web` system stories may rely on the shared QueryClient + router + MSW decorators from `web/.storybook/preview.ts`; prefer those global decorators over per-story provider duplication.

4. **Story Definition**
   - Define a helper type: `type Story = StoryObj<typeof meta>;`.
   - Export stories as named constants (PascalCase).
   - Always add JSDoc comments above each story export; these appear in the Storybook UI.
   - Use the `Default` story as the primary example.
   - **MANDATORY**: All stories must include `args` property, even if empty: `args: {}`
   - **Keep it concise**: Create only essential stories (2-5 max per component). Avoid over-engineering with excessive variations or complex scenarios.
   - For system stories, keep titles aligned to the domain surface: `systems/<name>/<ComponentName>`.

5. **Render vs Args**
   - Use `render` functions for compound components (like Accordion, Dialog, Select) that require children composition.
   - Use `args` for simple components (like Button, Badge) where props define the variation.
   - **MANDATORY**: Even when using `render`, include `args: {}` property

6. **Design System Compliance**
   - **ALWAYS** use design tokens for colors: `bg-background`, `text-foreground`, `border-border`
   - **NEVER** use explicit colors: `bg-white`, `text-black`, `border-gray-200`
   - Follow accessibility guidelines from `@.cursor/rules/shadcn.mdc`
   - Use semantic HTML elements and proper ARIA attributes

## Example Template

### Using Base UI Components from @agh/ui

```tsx
import type { Meta, StoryObj } from "@storybook/react";
import { Button, Card, CardHeader, CardTitle, CardContent } from "@agh/ui";
import { MyCustomComponent } from "./my-custom-component";

const meta: Meta<typeof MyCustomComponent> = {
  title: "components/custom/MyCustomComponent",
  component: MyCustomComponent,
  parameters: {
    layout: "centered",
    docs: {
      description: {
        component: "A custom component that composes base UI components from @agh/ui.",
      },
    },
  },
  // Optional decorator using design tokens
  decorators: [
    Story => (
      <div className="w-[400px] p-4 bg-background border border-border rounded-lg">
        <Story />
      </div>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof meta>;

/**
 * Default usage showing the standard behavior
 * Uses base Button and Card components from @agh/ui
 */
export const Default: Story = {
  args: {},
  render: () => (
    <Card>
      <CardHeader>
        <CardTitle>My Custom Component</CardTitle>
      </CardHeader>
      <CardContent>
        <MyCustomComponent>
          <Button variant="default">Action</Button>
        </MyCustomComponent>
      </CardContent>
    </Card>
  ),
};

/**
 * Variation with specific props
 * All styling uses design tokens (bg-background, text-foreground, etc.)
 */
export const WithVariant: Story = {
  args: {},
  render: () => (
    <div className="bg-card border border-border rounded-lg p-4">
      <MyCustomComponent variant="secondary">
        <Button variant="outline">Secondary Action</Button>
      </MyCustomComponent>
    </div>
  ),
};
```

### Story for Base UI Component (from @agh/ui)

```tsx
import type { Meta, StoryObj } from "@storybook/react";
import { Button } from "@agh/ui";

const meta: Meta<typeof Button> = {
  title: "components/ui/Button",
  component: Button,
  parameters: {
    layout: "centered",
    docs: {
      description: {
        component: "A button component with multiple variants and sizes.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

/**
 * Default button with standard styling
 */
export const Default: Story = {
  args: {
    children: "Button",
    variant: "default",
    size: "default",
  },
};

/**
 * All variants using design tokens
 */
export const AllVariants: Story = {
  args: {},
  render: () => (
    <div className="flex flex-wrap gap-4 bg-background p-4 rounded-lg">
      <Button variant="default">Default</Button>
      <Button variant="secondary">Secondary</Button>
      <Button variant="outline">Outline</Button>
      <Button variant="ghost">Ghost</Button>
      <Button variant="muted">Muted</Button>
    </div>
  ),
};
```

## Best Practices

### Autodocs Policy

Use `tags: ["autodocs"]` only for public `packages/ui` primitives or stories that intentionally document configuration-heavy public APIs. Do not add it by default to `web/src/components/ui` or `web/src/systems/**` stories; that policy comes from ADR-003 and keeps domain-story docs noise low.

### Conciseness & Simplicity

<critical>
**MANDATORY: Keep Stories Concise and Focused**

- ✅ **DO**: Create only essential stories that demonstrate core functionality
- ✅ **DO**: Use minimal, realistic examples that show the component's purpose
- ✅ **DO**: Focus on one concept per story
- ❌ **DON'T**: Create excessive variations that don't add value
- ❌ **DON'T**: Over-engineer with complex mock data or elaborate scenarios
- ❌ **DON'T**: Create stories for every possible prop combination
- ❌ **DON'T**: Add unnecessary decorators or wrappers unless required

**Story Count Guidelines:**

- Simple components (Button, Badge): 2-3 stories max (Default + 1-2 key variations)
- Medium components (Card, Dialog): 3-4 stories max (Default + key use cases)
- Complex components: 4-5 stories max (Default + essential scenarios)
- **Never create more than 5 stories per component** unless absolutely necessary
  </critical>

### Component Usage

- **Base Components First**: Always check `@agh/ui` for existing components before creating new ones
- **Composition Over Creation**: Compose complex components from base UI components
- **Compound Components**: Always demonstrate the full structure (Parent + Children) when using compound components from `@agh/ui`
- **Design Tokens**: Always use design tokens (`bg-background`, `text-foreground`, etc.) instead of explicit colors

### Story Structure

- **Mock Data**: Keep mock data minimal and realistic - only include what's necessary to demonstrate the component
- **Interactivity**: For components like Accordion or Dialog, ensure the `render` function sets up the component in a way that allows interaction (e.g., not force-controlled unless necessary)
- **Cleanliness**: Remove unused imports and generic "template" comments
- **Type Safety**: Always use explicit type annotation for `meta`: `const meta: Meta<typeof Component>`
- **Args Property**: Always include `args: {}` in all stories, even when using custom `render` functions
- **One Story Per Concept**: Each story should demonstrate one clear use case or variation, not multiple concepts

### Design System Compliance

- **Follow React Rules**: Adhere to patterns in `@.cursor/rules/react.mdc`
- **Follow Shadcn Rules**: Adhere to patterns in `@.cursor/rules/shadcn.mdc`
- **Accessibility**: Use semantic HTML and proper ARIA attributes
- **Theme Support**: All stories should work in both light and dark themes using design tokens

## Common Mistakes to Avoid

❌ **Creating components from scratch when base components exist:**

```tsx
// ❌ BAD: Creating a button from scratch
export const Bad: Story = {
  render: () => <button className="bg-blue-500 text-white px-4 py-2 rounded">Click me</button>,
};

// ✅ GOOD: Using base Button from @agh/ui
import { Button } from "@agh/ui";
export const Good: Story = {
  args: {},
  render: () => <Button variant="default">Click me</Button>,
};
```

## References

- TechSpec: [`../../../.compozy/tasks/storybook-stories/_techspec.md`](../../../.compozy/tasks/storybook-stories/_techspec.md)
- ADR-001: [`../../../.compozy/tasks/storybook-stories/adrs/adr-001.md`](../../../.compozy/tasks/storybook-stories/adrs/adr-001.md)
- ADR-002: [`../../../.compozy/tasks/storybook-stories/adrs/adr-002.md`](../../../.compozy/tasks/storybook-stories/adrs/adr-002.md)
- ADR-003: [`../../../.compozy/tasks/storybook-stories/adrs/adr-003.md`](../../../.compozy/tasks/storybook-stories/adrs/adr-003.md)
- ADR-004: [`../../../.compozy/tasks/storybook-stories/adrs/adr-004.md`](../../../.compozy/tasks/storybook-stories/adrs/adr-004.md)

❌ **Using explicit colors instead of design tokens:**

```tsx
// ❌ BAD: Using explicit colors
<div className="bg-white text-black border-gray-200">

// ✅ GOOD: Using design tokens
<div className="bg-background text-foreground border-border">
```

❌ **Missing args property:**

```tsx
// ❌ BAD: Missing args property
export const Bad: Story = {
  render: () => <Button>Click</Button>,
};

// ✅ GOOD: Including args property
export const Good: Story = {
  args: {},
  render: () => <Button>Click</Button>,
};
```

❌ **Missing explicit type annotation:**

```tsx
// ❌ BAD: Type inference
const meta = {
  title: "Components/Button",
  component: Button,
} satisfies Meta<typeof Button>;

// ✅ GOOD: Explicit type annotation
const meta: Meta<typeof Button> = {
  title: "Components/Button",
  component: Button,
};
```

❌ **Over-engineering stories with unnecessary examples:**

```tsx
// ❌ BAD: Too many stories with similar variations
export const Default: Story = { ... };
export const WithIcon: Story = { ... };
export const WithIconLeft: Story = { ... };
export const WithIconRight: Story = { ... };
export const WithLongText: Story = { ... };
export const WithShortText: Story = { ... };
export const Disabled: Story = { ... };
export const Loading: Story = { ... };
export const WithTooltip: Story = { ... };
export const InCard: Story = { ... };
export const InDialog: Story = { ... };
// ... 10+ stories for a simple button

// ✅ GOOD: Concise, focused stories
export const Default: Story = {
  args: {
    children: "Button",
    variant: "default",
  },
};

export const Variants: Story = {
  args: {},
  render: () => (
    <div className="flex gap-2">
      <Button variant="default">Default</Button>
      <Button variant="secondary">Secondary</Button>
      <Button variant="outline">Outline</Button>
    </div>
  ),
};

export const Disabled: Story = {
  args: {
    children: "Disabled",
    disabled: true,
  },
};
// Only 3 stories covering essential use cases
```

❌ **Over-complicated mock data or scenarios:**

```tsx
// ❌ BAD: Unnecessarily complex mock data
const mockUsers = [
  { id: "1", name: "John Doe", email: "john@example.com", role: "admin", avatar: "...", lastLogin: "...", permissions: [...], metadata: {...} },
  { id: "2", name: "Jane Smith", email: "jane@example.com", role: "user", avatar: "...", lastLogin: "...", permissions: [...], metadata: {...} },
  // ... 10 more users with full data
];

// ✅ GOOD: Minimal, realistic data
const mockUsers = [
  { id: "1", name: "John Doe", email: "john@example.com" },
  { id: "2", name: "Jane Smith", email: "jane@example.com" },
];
```
