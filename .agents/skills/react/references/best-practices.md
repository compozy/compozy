# React Development Patterns and Best Practices

This document contains comprehensive guidelines for React development in this project.

## Table of Contents

- [Core Principles](#core-principles)
- [Component Architecture](#component-architecture)
- [TypeScript Integration](#typescript-integration)
- [State Management](#state-management)
- [React 19+ Features](#react-19-features)
- [Custom Hooks](#custom-hooks)
- [Data Fetching](#data-fetching)
- [Error Handling](#error-handling)
- [Performance Optimization](#performance-optimization)
- [Accessibility](#accessibility)
- [Testing with Vitest](#testing-with-vitest)
- [Code Organization](#code-organization)
- [Anti-Patterns to Avoid](#anti-patterns-to-avoid)

---

## Core Principles

### Functional Components Only

Use functional components exclusively. Class components are legacy and should not be used:

```typescript
// CORRECT: Functional component
function UserProfile({ userId }: { userId: string }) {
  const user = useUser(userId);
  return <div>{user.name}</div>;
}

// INCORRECT: Class component (legacy)
class UserProfile extends React.Component { /* ... */ }
```

### Single Responsibility Principle

Keep components small and focused on a single purpose. If a component is doing too much, split it:

```typescript
// CORRECT: Focused components
function UserAvatar({ src, alt }: UserAvatarProps) {
  return <img src={src} alt={alt} className="avatar" />;
}

function UserInfo({ name, email }: UserInfoProps) {
  return (
    <div>
      <h3>{name}</h3>
      <p>{email}</p>
    </div>
  );
}

function UserCard({ user }: { user: User }) {
  return (
    <div className="user-card">
      <UserAvatar src={user.avatarUrl} alt={user.name} />
      <UserInfo name={user.name} email={user.email} />
    </div>
  );
}
```

### Separation of Concerns

Never mix behavior logic with visual rendering in the same component. Extract behavior into focused custom hooks:

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

### Feature-Based Organization

Co-locate related files by feature, not by type:

```
/src
└── /features
    ├── /products
    │   ├── /components
    │   │   ├── product-card.tsx
    │   │   ├── product-list.tsx
    │   │   └── product-details.tsx
    │   ├── /hooks
    │   │   ├── use-product.ts
    │   │   └── use-product-search.ts
    │   ├── /api
    │   │   └── product-api.ts
    │   ├── /types
    │   │   └── product.ts
    │   └── index.ts  // Public API
    └── /auth
        ├── /components
        ├── /hooks
        └── index.ts
```

---

## Component Architecture

### Essential Component Patterns

```typescript
// 1. Simple presentational component
interface ButtonProps {
  variant: "primary" | "secondary";
  children: React.ReactNode;
  onClick?: () => void;
}

function Button({ variant, children, onClick }: ButtonProps) {
  return (
    <button className={`btn btn-${variant}`} onClick={onClick}>
      {children}
    </button>
  );
}

// 2. Component with composition
interface CardProps {
  children: React.ReactNode;
  className?: string;
}

function Card({ children, className }: CardProps) {
  return <div className={cn("card", className)}>{children}</div>;
}

function CardHeader({ children }: { children: React.ReactNode }) {
  return <div className="card-header">{children}</div>;
}

function CardContent({ children }: { children: React.ReactNode }) {
  return <div className="card-content">{children}</div>;
}

// Usage
<Card>
  <CardHeader>Title</CardHeader>
  <CardContent>Content</CardContent>
</Card>

// 3. Component with render props pattern
interface DataListProps<T> {
  items: T[];
  renderItem: (item: T, index: number) => React.ReactNode;
  keyExtractor: (item: T) => string;
}

function DataList<T>({ items, renderItem, keyExtractor }: DataListProps<T>) {
  return (
    <ul>
      {items.map((item, index) => (
        <li key={keyExtractor(item)}>{renderItem(item, index)}</li>
      ))}
    </ul>
  );
}
```

### Component File Structure

Follow kebab-case for component filenames:

```
/components
├── user-avatar.tsx       // Component
├── user-avatar.test.tsx  // Tests
├── user-avatar.stories.tsx // Storybook stories
└── use-user-avatar.ts    // Related hook (if needed)
```

---

## TypeScript Integration

### Props Typing

Do NOT use `React.FC`. Type props directly on the function:

```typescript
// CORRECT: Type props directly
interface UserCardProps {
  user: User;
  onSelect?: (user: User) => void;
}

function UserCard({ user, onSelect }: UserCardProps) {
  return (
    <div onClick={() => onSelect?.(user)}>
      {user.name}
    </div>
  );
}

// INCORRECT: Using React.FC
const UserCard: React.FC<UserCardProps> = ({ user }) => { /* ... */ };
```

### Extending HTML Elements

Use `React.ComponentProps<'element'>` for extending native HTML elements:

```typescript
interface InputProps extends React.ComponentProps<"input"> {
  label: string;
  error?: string;
}

function Input({ label, error, className, ...props }: InputProps) {
  return (
    <div className="input-wrapper">
      <label>{label}</label>
      <input className={cn("input", error && "input-error", className)} {...props} />
      {error && <span className="error">{error}</span>}
    </div>
  );
}
```

### Generic Components

Use const type parameters for better type inference:

```typescript
interface SelectProps<T extends string> {
  options: readonly T[];
  value: T;
  onChange: (value: T) => void;
}

function Select<const T extends string>({ options, value, onChange }: SelectProps<T>) {
  return (
    <select value={value} onChange={(e) => onChange(e.target.value as T)}>
      {options.map((option) => (
        <option key={option} value={option}>
          {option}
        </option>
      ))}
    </select>
  );
}

// Usage - type is inferred as "red" | "blue" | "green"
<Select options={["red", "blue", "green"] as const} value="red" onChange={handleChange} />
```

### Typing Children

```typescript
// For any valid React children
interface ContainerProps {
  children: React.ReactNode;
}

// For a single element child
interface SingleChildProps {
  children: React.ReactElement;
}

// For function as children (render props)
interface RenderProps<T> {
  children: (data: T) => React.ReactNode;
}
```

---

## State Management

### State Management Hierarchy

Use state management in this order of preference:

1. **Local state** (`useState`/`useReducer`) - Component-specific UI state
2. **Zustand** - Shared client state across components
3. **TanStack Query** - Server state and data synchronization
4. **URL state** - Shareable application state (with TanStack Router)

### Local State (useState/useReducer)

```typescript
// Simple state
function Counter() {
  const [count, setCount] = useState(0);
  return <button onClick={() => setCount((c) => c + 1)}>{count}</button>;
}

// Complex state with useReducer
type State = { items: Item[]; filter: string; sortBy: SortKey };
type Action =
  | { type: "ADD_ITEM"; item: Item }
  | { type: "SET_FILTER"; filter: string }
  | { type: "SET_SORT"; sortBy: SortKey };

function reducer(state: State, action: Action): State {
  switch (action.type) {
    case "ADD_ITEM":
      return { ...state, items: [...state.items, action.item] };
    case "SET_FILTER":
      return { ...state, filter: action.filter };
    case "SET_SORT":
      return { ...state, sortBy: action.sortBy };
  }
}

function ItemList() {
  const [state, dispatch] = useReducer(reducer, initialState);
  // ...
}
```

### Zustand for Client State

```typescript
import { create } from "zustand";

interface UIStore {
  sidebarOpen: boolean;
  theme: "light" | "dark";
  toggleSidebar: () => void;
  setTheme: (theme: "light" | "dark") => void;
}

const useUIStore = create<UIStore>((set) => ({
  sidebarOpen: true,
  theme: "light",
  toggleSidebar: () => set((state) => ({ sidebarOpen: !state.sidebarOpen })),
  setTheme: (theme) => set({ theme }),
}));

// Usage in component
function Sidebar() {
  const { sidebarOpen, toggleSidebar } = useUIStore();
  // ...
}
```

### TanStack Query for Server State

```typescript
function useUser(id: string) {
  return useQuery({
    queryKey: ["user", id],
    queryFn: () => fetchUser(id),
    staleTime: 5 * 60 * 1000, // 5 minutes
  });
}

function useMutation() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: updateUser,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["user"] });
    },
  });
}
```

---

## React 19+ Features

### The use() Hook

Use the `use()` hook for unwrapping promises in components:

```typescript
import { use } from "react";

function UserProfile({ userPromise }: { userPromise: Promise<User> }) {
  const user = use(userPromise);
  return <div>{user.name}</div>;
}

// Usage with Suspense
<Suspense fallback={<Loading />}>
  <UserProfile userPromise={fetchUser(id)} />
</Suspense>
```

### Actions for Form Submissions

```typescript
import { useActionState } from "react";

async function submitForm(prevState: State, formData: FormData) {
  "use server";
  const email = formData.get("email") as string;
  // Validate and process...
  return { success: true, message: "Subscribed!" };
}

function NewsletterForm() {
  const [state, formAction, isPending] = useActionState(submitForm, { success: false });

  return (
    <form action={formAction}>
      <input name="email" type="email" />
      <button type="submit" disabled={isPending}>
        {isPending ? "Subscribing..." : "Subscribe"}
      </button>
      {state.success && <p>{state.message}</p>}
    </form>
  );
}
```

### useFormStatus for Form State

```typescript
import { useFormStatus } from "react-dom";

function SubmitButton() {
  const { pending } = useFormStatus();
  return (
    <button type="submit" disabled={pending}>
      {pending ? "Submitting..." : "Submit"}
    </button>
  );
}
```

### useOptimistic for Instant UI Updates

```typescript
import { useOptimistic } from "react";

function TodoList({ todos, addTodo }: Props) {
  const [optimisticTodos, addOptimisticTodo] = useOptimistic(
    todos,
    (state, newTodo: Todo) => [...state, { ...newTodo, pending: true }]
  );

  async function handleAdd(formData: FormData) {
    const newTodo = { id: crypto.randomUUID(), text: formData.get("text") as string };
    addOptimisticTodo(newTodo);
    await addTodo(newTodo);
  }

  return (
    <form action={handleAdd}>
      <input name="text" />
      <ul>
        {optimisticTodos.map((todo) => (
          <li key={todo.id} className={todo.pending ? "opacity-50" : ""}>
            {todo.text}
          </li>
        ))}
      </ul>
    </form>
  );
}
```

---

## Custom Hooks

### Best Practices

- Extract non-visual logic into custom hooks
- Keep hooks focused on single purpose
- Use clear naming: `useXxx` pattern
- Return arrays for state-like hooks, objects for complex returns
- Document hook dependencies and return values

### State-Like Hooks (Return Array)

```typescript
function useToggle(initial = false) {
  const [value, setValue] = useState(initial);
  const toggle = useCallback(() => setValue((v) => !v), []);
  return [value, toggle] as const;
}

function useLocalStorage<T>(key: string, initialValue: T) {
  const [value, setValue] = useState<T>(() => {
    const stored = localStorage.getItem(key);
    return stored ? JSON.parse(stored) : initialValue;
  });

  useEffect(() => {
    localStorage.setItem(key, JSON.stringify(value));
  }, [key, value]);

  return [value, setValue] as const;
}
```

### Complex Hooks (Return Object)

```typescript
function useUser(id: string) {
  const query = useQuery({
    queryKey: ["user", id],
    queryFn: () => fetchUser(id),
  });

  const updateMutation = useMutation({
    mutationFn: (data: UpdateUserData) => updateUser(id, data),
  });

  return {
    user: query.data,
    isLoading: query.isLoading,
    error: query.error,
    refetch: query.refetch,
    update: updateMutation.mutate,
    isUpdating: updateMutation.isPending,
  };
}

function useDebounce<T>(value: T, delay: number): T {
  const [debouncedValue, setDebouncedValue] = useState(value);

  useEffect(() => {
    const timer = setTimeout(() => setDebouncedValue(value), delay);
    return () => clearTimeout(timer);
  }, [value, delay]);

  return debouncedValue;
}
```

### Hook Composition

```typescript
function useSearchWithFilters<T>(
  searchFn: (query: string, filters: Filters) => Promise<T[]>,
  initialFilters: Filters
) {
  const [query, setQuery] = useState("");
  const [filters, setFilters] = useState(initialFilters);
  const debouncedQuery = useDebounce(query, 300);

  const { data, isLoading, error } = useQuery({
    queryKey: ["search", debouncedQuery, filters],
    queryFn: () => searchFn(debouncedQuery, filters),
    enabled: debouncedQuery.length > 0,
  });

  return {
    query,
    setQuery,
    filters,
    setFilters,
    results: data ?? [],
    isLoading,
    error,
  };
}
```

---

## Data Fetching

### TanStack Query Patterns

```typescript
// Query with proper typing
function useProducts(category?: string) {
  return useQuery({
    queryKey: ["products", category] as const,
    queryFn: () => fetchProducts(category),
    staleTime: 5 * 60 * 1000,
    gcTime: 30 * 60 * 1000,
  });
}

// Prefetching on route/interaction predictions
function ProductLink({ productId }: { productId: string }) {
  const queryClient = useQueryClient();

  const prefetch = () => {
    queryClient.prefetchQuery({
      queryKey: ["product", productId],
      queryFn: () => fetchProduct(productId),
    });
  };

  return (
    <Link to={`/products/${productId}`} onMouseEnter={prefetch}>
      View Product
    </Link>
  );
}

// Optimistic updates
function useUpdateTodo() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: updateTodo,
    onMutate: async (newTodo) => {
      await queryClient.cancelQueries({ queryKey: ["todos"] });
      const previous = queryClient.getQueryData(["todos"]);
      queryClient.setQueryData(["todos"], (old: Todo[]) =>
        old.map((t) => (t.id === newTodo.id ? newTodo : t))
      );
      return { previous };
    },
    onError: (err, newTodo, context) => {
      queryClient.setQueryData(["todos"], context?.previous);
    },
    onSettled: () => {
      queryClient.invalidateQueries({ queryKey: ["todos"] });
    },
  });
}
```

### Loading States with Suspense

```typescript
function ProductPage({ productId }: { productId: string }) {
  return (
    <Suspense fallback={<ProductSkeleton />}>
      <ProductDetails productId={productId} />
    </Suspense>
  );
}
```

---

## Error Handling

### Error Boundaries

```typescript
import { ErrorBoundary } from "react-error-boundary";

function ErrorFallback({ error, resetErrorBoundary }: FallbackProps) {
  return (
    <div role="alert" className="error-container">
      <h2>Something went wrong</h2>
      <pre>{error.message}</pre>
      <button onClick={resetErrorBoundary}>Try again</button>
    </div>
  );
}

function App() {
  return (
    <ErrorBoundary
      FallbackComponent={ErrorFallback}
      onError={(error) => console.error("Error:", error)}
      onReset={() => window.location.reload()}
    >
      <MainContent />
    </ErrorBoundary>
  );
}
```

### Typed Error Handling

```typescript
// Never assume Error type
function handleError(error: unknown): string {
  if (error instanceof Error) {
    return error.message;
  }
  if (typeof error === "string") {
    return error;
  }
  return "An unknown error occurred";
}
```

---

## Performance Optimization

### Memoization

```typescript
// useMemo for expensive calculations
function ProductList({ products, filter }: Props) {
  const filteredProducts = useMemo(
    () => products.filter((p) => p.category === filter),
    [products, filter]
  );
  // ...
}

// useCallback for stable function references
function ParentComponent() {
  const handleClick = useCallback((id: string) => {
    console.log("Clicked:", id);
  }, []);

  return <ChildComponent onClick={handleClick} />;
}

// React.memo for preventing re-renders
const ExpensiveComponent = React.memo(function ExpensiveComponent({ data }: Props) {
  // Expensive rendering...
});
```

### Code Splitting

```typescript
import { lazy, Suspense } from "react";

const HeavyComponent = lazy(() => import("./heavy-component"));

function App() {
  return (
    <Suspense fallback={<Loading />}>
      <HeavyComponent />
    </Suspense>
  );
}
```

### Transitions for Non-Urgent Updates

```typescript
import { useTransition } from "react";

function SearchResults() {
  const [query, setQuery] = useState("");
  const [isPending, startTransition] = useTransition();

  function handleChange(e: React.ChangeEvent<HTMLInputElement>) {
    startTransition(() => {
      setQuery(e.target.value);
    });
  }

  return (
    <div>
      <input onChange={handleChange} />
      {isPending && <Spinner />}
      <Results query={query} />
    </div>
  );
}
```

---

## Accessibility

### Semantic HTML First

```typescript
// CORRECT: Semantic HTML
<nav>
  <ul>
    <li><a href="/home">Home</a></li>
    <li><a href="/about">About</a></li>
  </ul>
</nav>

// INCORRECT: Div soup
<div className="nav">
  <div className="nav-item" onClick={goHome}>Home</div>
</div>
```

### Keyboard Navigation

```typescript
function MenuItem({ label, onClick }: Props) {
  return (
    <button
      onClick={onClick}
      onKeyDown={(e) => {
        if (e.key === "Enter" || e.key === " ") {
          onClick();
        }
      }}
      role="menuitem"
      tabIndex={0}
    >
      {label}
    </button>
  );
}
```

### ARIA Attributes

```typescript
function Modal({ isOpen, onClose, title, children }: ModalProps) {
  return (
    <dialog
      open={isOpen}
      aria-labelledby="modal-title"
      aria-modal="true"
      onClose={onClose}
    >
      <h2 id="modal-title">{title}</h2>
      {children}
      <button onClick={onClose} aria-label="Close modal">
        X
      </button>
    </dialog>
  );
}
```

---

## Testing with Vitest

### Test Framework Setup

- Use **Vitest** as the test runner
- Import test utilities from `vitest`: `describe`, `it`, `expect`, `vi`, `beforeEach`, `afterEach`
- Use `@testing-library/react` for component testing
- Use `@testing-library/jest-dom` matchers for DOM assertions
- Test files: `*.test.tsx` or `*.test.ts`

### Component Testing

```typescript
import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Button } from "./button";

describe("Button", () => {
  it("renders correctly", () => {
    render(<Button>Click me</Button>);
    expect(screen.getByRole("button", { name: "Click me" })).toBeInTheDocument();
  });

  it("calls onClick when clicked", async () => {
    const user = userEvent.setup();
    const onClick = vi.fn();
    render(<Button onClick={onClick}>Click me</Button>);

    await user.click(screen.getByRole("button"));
    expect(onClick).toHaveBeenCalledTimes(1);
  });

  it("is disabled when disabled prop is true", () => {
    render(<Button disabled>Click me</Button>);
    expect(screen.getByRole("button")).toBeDisabled();
  });
});
```

### Hook Testing

```typescript
import { describe, it, expect } from "vitest";
import { renderHook, act } from "@testing-library/react";
import { useCounter } from "./use-counter";

describe("useCounter", () => {
  it("initializes with default value", () => {
    const { result } = renderHook(() => useCounter());
    expect(result.current.count).toBe(0);
  });

  it("increments counter", () => {
    const { result } = renderHook(() => useCounter());
    act(() => {
      result.current.increment();
    });
    expect(result.current.count).toBe(1);
  });

  it("accepts initial value", () => {
    const { result } = renderHook(() => useCounter(10));
    expect(result.current.count).toBe(10);
  });
});
```

### Testing with Queries (TanStack Query)

```typescript
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { renderHook, waitFor } from "@testing-library/react";

function createWrapper() {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
    },
  });
  return ({ children }: { children: React.ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );
}

describe("useUser", () => {
  it("fetches user data", async () => {
    const { result } = renderHook(() => useUser("1"), {
      wrapper: createWrapper(),
    });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(result.current.data).toEqual({ id: "1", name: "John" });
  });
});
```

---

## Code Organization

### File Naming Conventions

| Type | Pattern | Example |
|------|---------|---------|
| Components | kebab-case.tsx | `user-avatar.tsx` |
| Hooks | use-kebab-case.ts | `use-user-data.ts` |
| Utilities | camelCase.ts | `formatDate.ts` |
| Types | types.ts | `types.ts` |
| Tests | *.test.tsx | `user-avatar.test.tsx` |
| Stories | *.stories.tsx | `user-avatar.stories.tsx` |

### Export Patterns

Prefer named exports for components and utilities:

```typescript
// user-avatar.tsx
export function UserAvatar({ src, alt }: UserAvatarProps) {
  return <img src={src} alt={alt} />;
}

// index.ts (feature barrel export)
export { UserAvatar } from "./user-avatar";
export { useUser } from "./use-user";
export type { User, UserAvatarProps } from "./types";
```

---

## Anti-Patterns to Avoid

### 1. Mixing Logic and Rendering

```typescript
// BAD: Logic mixed with rendering
function IssueList() {
  const [query, setQuery] = useState("");
  const [filters, setFilters] = useState({});
  const [issues, setIssues] = useState([]);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    setLoading(true);
    fetchIssues(query, filters).then((data) => {
      setIssues(data);
      setLoading(false);
    });
  }, [query, filters]);

  // Lots of JSX...
}

// GOOD: Extract to hook
function useIssueSearch() {
  // All logic here
}

function IssueList() {
  const { issues, loading } = useIssueSearch();
  // Clean JSX only
}
```

### 2. Prop Drilling

```typescript
// BAD: Passing props through many levels
<App user={user}>
  <Layout user={user}>
    <Sidebar user={user}>
      <UserInfo user={user} />
    </Sidebar>
  </Layout>
</App>

// GOOD: Use context or state management
const UserContext = createContext<User | null>(null);

function App() {
  return (
    <UserContext.Provider value={user}>
      <Layout />
    </UserContext.Provider>
  );
}

function UserInfo() {
  const user = useContext(UserContext);
  // ...
}
```

### 3. Giant Components

Split large components into smaller, focused ones. If a component is over 200 lines, it probably needs splitting.

### 4. Inline Function Props Without useCallback

```typescript
// BAD: Creates new function on every render
<List onItemClick={(id) => handleClick(id)} />

// GOOD: Stable reference
const handleItemClick = useCallback((id: string) => {
  handleClick(id);
}, [handleClick]);

<List onItemClick={handleItemClick} />
```

### 5. Not Handling Loading/Error States

```typescript
// BAD: No loading or error handling
function UserProfile({ userId }: Props) {
  const { data } = useQuery({ queryKey: ["user", userId], queryFn: fetchUser });
  return <div>{data.name}</div>; // Crashes if data is undefined
}

// GOOD: Handle all states
function UserProfile({ userId }: Props) {
  const { data, isLoading, error } = useQuery({ ... });

  if (isLoading) return <Skeleton />;
  if (error) return <ErrorMessage error={error} />;
  if (!data) return <NotFound />;

  return <div>{data.name}</div>;
}
```

---

## Validation Checklist

Before finishing a task involving React:

- [ ] Components are functional and follow single responsibility principle
- [ ] Behavior logic is extracted into custom hooks
- [ ] TypeScript props are typed directly (not using `React.FC`)
- [ ] State management follows the hierarchy (local -> Zustand -> TanStack Query -> URL)
- [ ] Error boundaries are in place for error handling
- [ ] Accessibility requirements are met (semantic HTML, keyboard navigation)
- [ ] Loading and error states are handled
- [ ] Tests are written for components and hooks
- [ ] Run `pnpm run lint`, `pnpm run typecheck`, and `pnpm run test`
