---
name: zustand
description: Expert guide for Zustand state management patterns, store organization, and best practices. Use when implementing client state management with Zustand, creating stores, or managing shared UI state across components.
allowed-tools: Read, Grep, Glob
---

# Zustand State Management Guide

This skill provides guidelines, patterns, and best practices for working with Zustand in this project.

## Quick Start

For detailed store patterns, middleware usage, and comprehensive examples, please refer to `references/patterns.md`.

## Core Philosophy

- **Shared Client State Only**: Use Zustand for shared client state, not server state (use TanStack Query for that).
- **Domain-Specific Stores**: Keep stores focused on specific domains.
- **Type Safety**: Leverage TypeScript for fully typed stores.
- **Simplicity**: Prefer simplicity over complex abstractions.

## Store Organization

### Essential Patterns

- Create separate stores for different domains
- Use slices pattern for large stores
- Keep stores close to features that use them
- Export typed selector hooks for better DX and performance

### Recommended Middleware Stack

Use the following middleware combination for production stores:

```typescript
import { create } from "zustand";
import { devtools, persist } from "zustand/middleware";
import { immer } from "zustand/middleware/immer";

export const useExampleStore = create<ExampleState>()(
  devtools(
    persist(
      immer((set, get) => ({
        // state and actions
      })),
      { name: "example-storage" }
    ),
    { name: "example-store" }
  )
);
```

### Store Structure Template

```typescript
interface StoreState {
  // State properties
  data: DataType | null;
  isLoading: boolean;
  // Group actions together
  actions: {
    fetchData: () => Promise<void>;
    updateData: (updates: Partial<DataType>) => void;
    reset: () => void;
  };
}
```

### Selector Hooks Pattern

Always create selector hooks for performance optimization:

```typescript
// Bad - subscribes to entire store
const { user, isLoading } = useAuthStore();

// Good - subscribes only to specific slices
export const useUser = () => useAuthStore((state) => state.user);
export const useIsLoading = () => useAuthStore((state) => state.isLoading);
export const useAuthActions = () => useAuthStore((state) => state.actions);
```

## Best Practices

1. **Keep stores focused** on specific domains
2. **Use TypeScript** for full type safety
3. **Leverage middleware** for common patterns (devtools, persist, immer)
4. **Create selector hooks** for performance
5. **Use immer** for complex nested state updates
6. **Persist only necessary state** - use `partialize` option
7. **Test stores thoroughly**
8. **Handle async operations properly** with loading/error states
9. **Implement optimistic updates** when appropriate
10. **Document store structure** and actions

## Common Tasks

### Creating a New Store

1. Define the state interface with typed actions
2. Create the store with appropriate middleware
3. Export selector hooks for each state slice
4. Add to the feature's barrel export

### Persisting State

Use the `persist` middleware with `partialize` to persist only necessary data:

```typescript
persist(
  (set) => ({ /* ... */ }),
  {
    name: "store-key",
    partialize: (state) => ({
      // Only persist these fields
      user: state.user,
      preferences: state.preferences,
    }),
  }
);
```

### Using Immer for Updates

Immer allows mutable-style updates that produce immutable state:

```typescript
immer((set) => ({
  updateNested: (id, value) => {
    set((state) => {
      const item = state.items.find((i) => i.id === id);
      if (item) {
        item.value = value; // Mutable style, but produces immutable state
      }
    });
  },
}));
```

## Validation Checklist

Before finishing a task involving Zustand:

- [ ] Store is domain-specific and focused
- [ ] TypeScript interfaces are properly defined
- [ ] Middleware is applied in correct order (devtools > persist > immer)
- [ ] Selector hooks are created for performance
- [ ] Actions are grouped in an `actions` object
- [ ] Only necessary state is persisted
- [ ] Run type checks (`pnpm run typecheck`) and tests (`pnpm run test`)

For detailed rules, examples, and anti-patterns, please consult `references/patterns.md`.
