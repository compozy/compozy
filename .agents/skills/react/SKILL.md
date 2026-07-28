---
name: react
description: Comprehensive React development guide covering component architecture, hooks, state management, TypeScript integration, useEffect patterns, and testing with Vitest. Use when creating React components, custom hooks, managing state, or any frontend React code. Essential for React 19+ development. Don't use for React Native, non-React frameworks (Vue, Svelte, Solid), or backend-only Node.js code.
allowed-tools: Read, Grep, Glob
metadata:
  author: Pedro Nauck
  github: https://github.com/pedronauck
  repository: https://github.com/pedronauck/skills
---
# React Development Guide

This skill provides comprehensive guidelines, patterns, and best practices for React development in this project.

## Quick Start

1. **Best Practices**: For component architecture, state management, and TypeScript integration, read `references/best-practices.md`
2. **Element wrappers**: If a component renders a single native element (`button`, `input`, `a`, …), extend that element’s props (`React.ComponentProps<"…">`) and spread `...props` — see **Extend native element props** below and `references/best-practices.md` → *Extending HTML Elements*.
3. **useEffect Patterns**: For understanding when to use (and avoid) useEffect, read `references/useeffect-patterns.md`
4. **Data Fetching**: For TanStack Query patterns, use the `tanstack` skill
5. **Forms**: For form handling with TanStack Form, use the `tanstack` skill

## Core Principles

- **Functional Components Only**: Use functional components exclusively - class components are legacy
- **Single Responsibility**: Keep components small and focused on a single purpose
- **Separation of Concerns**: Extract behavior logic into custom hooks, keep components focused on rendering
- **Feature-Based Organization**: Co-locate related files by feature, not by type
- **React 19+ Features**: Embrace modern React features (`use()`, Actions, `useOptimistic()`)

## Extend native element props

**Default rule for wrappers:** whenever a component’s root output is a **single native element**, its props interface MUST extend that element’s intrinsic props — same contract as shadcn/ui-generated primitives. Callers keep access to `aria-*`, `data-*`, `onClick`, `disabled`, etc., without bespoke passthrough lists.

**Do this:**

| Requirement | Detail |
| ------------- | ------ |
| Base type | `interface XProps extends React.ComponentProps<"button">` (or `"input"`, `"a"`, `"div"`, …) |
| Spreading | Destructure your custom fields, then `{...props}` (and merged `className`) onto the DOM node |
| Ref | Use `React.forwardRef` and the matching element ref type when refs are needed |

```typescript
interface TextFieldProps extends React.ComponentProps<"input"> {
  label: string;
  error?: string;
}

function TextField({ label, error, className, ...props }: TextFieldProps) {
  return (
    <label className="flex flex-col gap-1">
      <span>{label}</span>
      <input className={cn("rounded border px-2 py-1", error && "border-destructive", className)} {...props} />
      {error ? <span className="text-destructive text-sm">{error}</span> : null}
    </label>
  );
}
```

**Variants + CVA:** if you use `class-variance-authority`, combine intrinsic props with `VariantProps<typeof variants>` (often `extends React.ButtonHTMLAttributes<HTMLButtonElement>`). Follow the **`shadcn`** skill patterns.

**Deep dive:** `references/best-practices.md` → *Extending HTML Elements*.

## Quick Reference Tables

### State Management Hierarchy

| Priority | Tool | Use Case |
|----------|------|----------|
| 1 | `useState`/`useReducer` | Component-specific UI state |
| 2 | Zustand | Shared client state across components |
| 3 | TanStack Query | Server state and data synchronization |
| 4 | URL state | Shareable application state (TanStack Router) |

### useEffect Decision Tree

| Situation | DON'T | DO |
|-----------|-------|-----|
| Derived state from props/state | `useState` + `useEffect` | Calculate during render |
| Expensive calculations | `useEffect` to cache | `useMemo` |
| Reset state on prop change | `useEffect` with `setState` | `key` prop |
| User event responses | `useEffect` watching state | Event handler directly |
| Notify parent of changes | `useEffect` calling `onChange` | Call in event handler |
| Fetch data | `useEffect` without cleanup | `useEffect` with cleanup OR TanStack Query |

### When You DO Need Effects

- Synchronizing with **external systems** (non-React widgets, browser APIs)
- **Subscriptions** to external stores (use `useSyncExternalStore` when possible)
- **Analytics/logging** that runs because component displayed
- **Data fetching** with proper cleanup (or use TanStack Query)

### When You DON'T Need Effects

1. **Transforming data for rendering** - Calculate at top level, re-runs automatically
2. **Handling user events** - Use event handlers, you know exactly what happened
3. **Deriving state** - Just compute it: `const fullName = firstName + ' ' + lastName`
4. **Chaining state updates** - Calculate all next state in the event handler

## TypeScript Integration

```typescript
// CORRECT: Type props directly (never use React.FC)
interface BrandButtonProps {
  variant: "primary" | "secondary";
  children: React.ReactNode;
}

function BrandButton({ variant, children }: BrandButtonProps) {
  return <button type="button" className={variant}>{children}</button>;
}

// When wrapping a native element, extend its props — see "Extend native element props" above
interface IconButtonProps extends React.ComponentProps<"button"> {
  icon: React.ReactNode;
}
```

## Custom Hooks Guidelines

- Extract non-visual logic into custom hooks
- Keep hooks focused on single purpose
- Use clear naming: `useXxx` pattern
- Return arrays for state-like hooks, objects for complex returns

```typescript
// State-like hook returns array
function useToggle(initial = false) {
  const [value, setValue] = useState(initial);
  const toggle = useCallback(() => setValue((v) => !v), []);
  return [value, toggle] as const;
}

// Complex hook returns object
function useUser(id: string) {
  const query = useQuery({ queryKey: ["user", id], queryFn: () => fetchUser(id) });
  return {
    user: query.data,
    isLoading: query.isLoading,
    error: query.error,
    refetch: query.refetch,
  };
}
```

## Component Architecture Pattern

```typescript
// CORRECT: Hook handles all logic, component handles rendering
function useIssueSearch(projectId: string) {
  const [query, setQuery] = useState("");
  const [filters, setFilters] = useState<Filters>({});

  const issues = useQuery({
    queryKey: ["issues", projectId, query, filters],
    queryFn: () => searchIssues(projectId, query, filters),
  });

  return {
    query,
    setQuery,
    filters,
    setFilters,
    issues: issues.data ?? [],
    isLoading: issues.isLoading,
  };
}

function IssueList({ projectId }: { projectId: string }) {
  const { query, setQuery, issues, isLoading } = useIssueSearch(projectId);

  return (
    <div>
      <SearchInput value={query} onChange={setQuery} />
      {isLoading ? <Loading /> : <IssueTable issues={issues} />}
    </div>
  );
}
```

## File Naming Conventions

| Type | Pattern | Example |
|------|---------|---------|
| Components | kebab-case.tsx | `user-avatar.tsx` |
| Hooks | use-kebab-case.ts | `use-user-data.ts` |
| Utilities | camelCase.ts | `formatDate.ts` |
| Types | types.ts | `types.ts` |
| Tests | *.test.tsx | `user-avatar.test.tsx` |

## Testing with Vitest

```typescript
import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { renderHook, act } from "@testing-library/react";

describe("MyComponent", () => {
  it("renders correctly", () => {
    render(<MyComponent />);
    expect(screen.getByText("Hello")).toBeInTheDocument();
  });
});

describe("useMyHook", () => {
  it("returns expected value", () => {
    const { result } = renderHook(() => useMyHook());
    expect(result.current.value).toBe(expected);
  });
});
```

## Validation Checklist

Before finishing a task involving React:

- [ ] Components are functional and follow single responsibility principle
- [ ] Behavior logic is extracted into custom hooks
- [ ] TypeScript props are typed directly (not using `React.FC`); native wrappers extend `React.ComponentProps<"…">` (or `ButtonHTMLAttributes` + variants per `shadcn` skill) and forward `...props`
- [ ] State management follows the hierarchy (local -> Zustand -> TanStack Query -> URL)
- [ ] useEffect is only used for external system synchronization
- [ ] Error boundaries are in place for error handling
- [ ] Loading and error states are handled
- [ ] Accessibility requirements are met (semantic HTML, keyboard navigation)
- [ ] Tests are written for components and hooks
- [ ] Run `pnpm run lint`, `pnpm run typecheck`, and `pnpm run test`

## Detailed References

For comprehensive guidance, consult these reference files:

- `references/best-practices.md` - Component architecture, TypeScript, state management, React 19+ features, testing patterns
- `references/useeffect-patterns.md` - When to use/avoid useEffect, anti-patterns, and better alternatives
