# Zustand State Management Patterns

This document contains comprehensive patterns, best practices, and guidelines for working with Zustand in this project.

## Table of Contents

1. [Core Principles](#core-principles)
2. [Store Architecture](#store-architecture)
3. [Middleware Configuration](#middleware-configuration)
4. [Type Safety Patterns](#type-safety-patterns)
5. [Selector Patterns](#selector-patterns)
6. [Async Operations](#async-operations)
7. [Slices Pattern](#slices-pattern)
8. [Persistence Strategies](#persistence-strategies)
9. [Testing Stores](#testing-stores)
10. [Common Patterns](#common-patterns)
11. [Anti-Patterns to Avoid](#anti-patterns-to-avoid)

---

## Core Principles

### When to Use Zustand

**Use Zustand for:**
- Shared client-side UI state (modals, sidebars, panels)
- User preferences and settings
- Application-wide flags and toggles
- Complex form state that spans multiple components
- Cross-cutting concerns that don't belong to a single component

**Do NOT use Zustand for:**
- Server state (use TanStack Query instead)
- Component-local state (use React useState/useReducer)
- Derived/computed data that can be calculated from server state

### Design Philosophy

1. **Domain Isolation**: Each store should handle one domain/feature
2. **Minimal State**: Only store what you need; derive the rest
3. **Actions Grouping**: Group all actions under an `actions` object
4. **Type Safety First**: Full TypeScript coverage for all stores
5. **Performance by Default**: Use selectors to prevent unnecessary re-renders

---

## Store Architecture

### Basic Store Template

```typescript
import { create } from "zustand";
import { devtools, persist } from "zustand/middleware";
import { immer } from "zustand/middleware/immer";

// 1. Define the state interface
interface ExampleState {
  // State properties
  items: Item[];
  selectedId: string | null;
  isLoading: boolean;
  error: string | null;

  // Group all actions together
  actions: {
    setItems: (items: Item[]) => void;
    selectItem: (id: string | null) => void;
    addItem: (item: Item) => void;
    updateItem: (id: string, updates: Partial<Item>) => void;
    removeItem: (id: string) => void;
    reset: () => void;
  };
}

// 2. Define initial state
const initialState = {
  items: [],
  selectedId: null,
  isLoading: false,
  error: null,
};

// 3. Create the store with middleware
export const useExampleStore = create<ExampleState>()(
  devtools(
    persist(
      immer((set, get) => ({
        ...initialState,

        actions: {
          setItems: (items) => set({ items }),

          selectItem: (id) => set({ selectedId: id }),

          addItem: (item) => set((state) => {
            state.items.push(item);
          }),

          updateItem: (id, updates) => set((state) => {
            const item = state.items.find((i) => i.id === id);
            if (item) {
              Object.assign(item, updates);
            }
          }),

          removeItem: (id) => set((state) => {
            state.items = state.items.filter((i) => i.id !== id);
          }),

          reset: () => set(initialState),
        },
      })),
      {
        name: "example-storage",
        partialize: (state) => ({
          items: state.items,
          selectedId: state.selectedId,
          // Exclude isLoading, error, and actions from persistence
        }),
      }
    ),
    { name: "ExampleStore" }
  )
);

// 4. Export selector hooks for performance
export const useItems = () => useExampleStore((s) => s.items);
export const useSelectedId = () => useExampleStore((s) => s.selectedId);
export const useSelectedItem = () => {
  const items = useExampleStore((s) => s.items);
  const selectedId = useExampleStore((s) => s.selectedId);
  return items.find((i) => i.id === selectedId) ?? null;
};
export const useExampleActions = () => useExampleStore((s) => s.actions);
```

### File Organization

```
src/
├── stores/
│   ├── index.ts              # Barrel export
│   ├── ui-store.ts           # Global UI state (modals, panels)
│   ├── preferences-store.ts  # User preferences
│   └── ...
├── features/
│   └── issues/
│       ├── stores/
│       │   ├── index.ts
│       │   └── issue-filter-store.ts  # Feature-specific store
│       └── ...
```

---

## Middleware Configuration

### Recommended Middleware Order

Apply middleware from outermost to innermost:
1. `devtools` (outermost) - for Redux DevTools integration
2. `persist` - for localStorage persistence
3. `immer` (innermost) - for immutable updates with mutable syntax

```typescript
create<State>()(
  devtools(           // 1. Outermost
    persist(          // 2. Middle
      immer(          // 3. Innermost
        (set, get) => ({
          // store implementation
        })
      ),
      { name: "storage-key" }
    ),
    { name: "StoreName" }
  )
);
```

### Devtools Configuration

```typescript
devtools(
  // store...
  {
    name: "StoreName",      // Shown in Redux DevTools
    enabled: process.env.NODE_ENV === "development",
    anonymousActionType: "Unknown Action",
  }
)
```

### Persistence Configuration

```typescript
persist(
  // store...
  {
    name: "storage-key",

    // Only persist specific fields
    partialize: (state) => ({
      user: state.user,
      theme: state.theme,
      // Exclude: isLoading, error, actions, etc.
    }),

    // Use sessionStorage instead of localStorage
    storage: createJSONStorage(() => sessionStorage),

    // Version for migrations
    version: 1,

    // Migration function
    migrate: (persisted, version) => {
      if (version === 0) {
        // Migration logic
      }
      return persisted as State;
    },

    // Skip hydration (useful for SSR)
    skipHydration: true,

    // Handle rehydration
    onRehydrateStorage: () => {
      return (state, error) => {
        if (error) {
          console.error("Hydration failed:", error);
        }
      };
    },
  }
)
```

### Immer for Nested Updates

```typescript
immer((set) => ({
  updateDeeplyNested: (id, value) => set((state) => {
    // Safe mutable-style updates
    const category = state.categories.find((c) => c.id === id);
    if (category) {
      category.items[0].nested.value = value;
    }
  }),
}))
```

---

## Type Safety Patterns

### Strongly Typed Store

```typescript
// Types file (types.ts)
interface User {
  id: string;
  name: string;
  email: string;
}

interface AuthState {
  user: User | null;
  isAuthenticated: boolean;
  token: string | null;

  actions: {
    login: (user: User, token: string) => void;
    logout: () => void;
    updateUser: (updates: Partial<User>) => void;
  };
}

// Derive types from store
type AuthActions = AuthState["actions"];
```

### Using `StateCreator` for Slices

```typescript
import { StateCreator } from "zustand";

interface SliceA {
  valueA: string;
  setValueA: (v: string) => void;
}

interface SliceB {
  valueB: number;
  setValueB: (v: number) => void;
}

type CombinedState = SliceA & SliceB;

const createSliceA: StateCreator<CombinedState, [], [], SliceA> = (set) => ({
  valueA: "",
  setValueA: (v) => set({ valueA: v }),
});

const createSliceB: StateCreator<CombinedState, [], [], SliceB> = (set) => ({
  valueB: 0,
  setValueB: (v) => set({ valueB: v }),
});

export const useCombinedStore = create<CombinedState>()(
  (...args) => ({
    ...createSliceA(...args),
    ...createSliceB(...args),
  })
);
```

---

## Selector Patterns

### Basic Selectors

```typescript
// Bad - subscribes to entire store, re-renders on any change
const Component = () => {
  const { user, theme } = useSettingsStore();
  // ...
};

// Good - subscribes only to needed state
const Component = () => {
  const user = useSettingsStore((s) => s.user);
  const theme = useSettingsStore((s) => s.theme);
  // ...
};
```

### Pre-defined Selector Hooks

```typescript
// Export selector hooks from store file
export const useUser = () => useAuthStore((s) => s.user);
export const useIsAuthenticated = () => useAuthStore((s) => s.isAuthenticated);
export const useAuthActions = () => useAuthStore((s) => s.actions);

// Usage in component
const Component = () => {
  const user = useUser();
  const { login, logout } = useAuthActions();
  // ...
};
```

### Computed Selectors with Shallow Comparison

```typescript
import { useShallow } from "zustand/react/shallow";

// For selecting multiple values
const { name, email } = useUserStore(
  useShallow((s) => ({
    name: s.user.name,
    email: s.user.email,
  }))
);

// For selecting arrays
const itemIds = useStore(
  useShallow((s) => s.items.map((i) => i.id))
);
```

### Memoized Derived State

```typescript
import { useMemo } from "react";

const useFilteredItems = (filter: string) => {
  const items = useStore((s) => s.items);

  return useMemo(
    () => items.filter((item) => item.name.includes(filter)),
    [items, filter]
  );
};
```

---

## Async Operations

### Pattern with Loading and Error States

```typescript
interface AsyncState<T> {
  data: T | null;
  isLoading: boolean;
  error: Error | null;

  actions: {
    fetchData: () => Promise<void>;
    reset: () => void;
  };
}

const useAsyncStore = create<AsyncState<User[]>>()(
  immer((set) => ({
    data: null,
    isLoading: false,
    error: null,

    actions: {
      fetchData: async () => {
        set({ isLoading: true, error: null });
        try {
          const response = await fetch("/api/users");
          const data = await response.json();
          set({ data, isLoading: false });
        } catch (error) {
          set({
            error: error instanceof Error ? error : new Error("Unknown error"),
            isLoading: false,
          });
        }
      },

      reset: () => set({ data: null, isLoading: false, error: null }),
    },
  }))
);
```

### Optimistic Updates

```typescript
interface TodoState {
  todos: Todo[];

  actions: {
    toggleTodo: (id: string) => Promise<void>;
  };
}

const useTodoStore = create<TodoState>()(
  immer((set, get) => ({
    todos: [],

    actions: {
      toggleTodo: async (id) => {
        // Store original state for rollback
        const originalTodos = get().todos;

        // Optimistic update
        set((state) => {
          const todo = state.todos.find((t) => t.id === id);
          if (todo) {
            todo.completed = !todo.completed;
          }
        });

        try {
          await api.toggleTodo(id);
        } catch (error) {
          // Rollback on error
          set({ todos: originalTodos });
          throw error;
        }
      },
    },
  }))
);
```

---

## Slices Pattern

### For Large Stores

```typescript
import { StateCreator } from "zustand";

// Define slice interfaces
interface UserSlice {
  user: User | null;
  setUser: (user: User | null) => void;
}

interface UISlice {
  sidebarOpen: boolean;
  toggleSidebar: () => void;
}

interface NotificationSlice {
  notifications: Notification[];
  addNotification: (n: Notification) => void;
  removeNotification: (id: string) => void;
}

// Combined type
type AppState = UserSlice & UISlice & NotificationSlice;

// Create slice creators
const createUserSlice: StateCreator<AppState, [["zustand/immer", never]], [], UserSlice> =
  (set) => ({
    user: null,
    setUser: (user) => set({ user }),
  });

const createUISlice: StateCreator<AppState, [["zustand/immer", never]], [], UISlice> =
  (set) => ({
    sidebarOpen: false,
    toggleSidebar: () => set((state) => {
      state.sidebarOpen = !state.sidebarOpen;
    }),
  });

const createNotificationSlice: StateCreator<AppState, [["zustand/immer", never]], [], NotificationSlice> =
  (set) => ({
    notifications: [],
    addNotification: (n) => set((state) => {
      state.notifications.push(n);
    }),
    removeNotification: (id) => set((state) => {
      state.notifications = state.notifications.filter((n) => n.id !== id);
    }),
  });

// Combine slices
export const useAppStore = create<AppState>()(
  devtools(
    immer((...args) => ({
      ...createUserSlice(...args),
      ...createUISlice(...args),
      ...createNotificationSlice(...args),
    })),
    { name: "AppStore" }
  )
);
```

---

## Persistence Strategies

### Partial Persistence

```typescript
persist(
  // store...
  {
    name: "settings",
    partialize: (state) => ({
      // Only persist user preferences
      theme: state.theme,
      locale: state.locale,
      // Exclude runtime state
      // isLoading, error, etc. are NOT persisted
    }),
  }
)
```

### Migration Between Versions

```typescript
persist(
  // store...
  {
    name: "app-state",
    version: 2,
    migrate: (persisted: any, version) => {
      if (version === 0) {
        // v0 -> v1: Rename field
        persisted.username = persisted.name;
        delete persisted.name;
      }
      if (version === 1) {
        // v1 -> v2: Add new field
        persisted.preferences = { notifications: true };
      }
      return persisted;
    },
  }
)
```

### Manual Hydration Control

```typescript
const useStore = create(
  persist(
    // store...
    {
      name: "store",
      skipHydration: true,
    }
  )
);

// Manually hydrate when ready
const App = () => {
  useEffect(() => {
    useStore.persist.rehydrate();
  }, []);

  return <Component />;
};
```

---

## Testing Stores

### Reset Store Between Tests

```typescript
// store.ts
const initialState = {
  count: 0,
  name: "",
};

export const useTestStore = create<TestState>()((set) => ({
  ...initialState,
  actions: {
    increment: () => set((s) => ({ count: s.count + 1 })),
    reset: () => set(initialState),
  },
}));

// test.ts
import { useTestStore } from "./store";

beforeEach(() => {
  useTestStore.getState().actions.reset();
});

test("increment increases count", () => {
  const { actions } = useTestStore.getState();
  actions.increment();
  expect(useTestStore.getState().count).toBe(1);
});
```

### Mock Store in Tests

```typescript
import { create } from "zustand";

// Create a mock store creator
const createMockStore = (overrides = {}) => {
  return create<TestState>()(() => ({
    count: 0,
    name: "test",
    actions: {
      increment: vi.fn(),
      reset: vi.fn(),
    },
    ...overrides,
  }));
};

// Use in tests
test("component uses store", () => {
  const mockStore = createMockStore({ count: 5 });
  // ... render component with mock store
});
```

---

## Common Patterns

### Modal State Management

```typescript
interface ModalState {
  modals: {
    confirmDelete: { open: boolean; itemId: string | null };
    createItem: { open: boolean };
    editItem: { open: boolean; item: Item | null };
  };

  actions: {
    openConfirmDelete: (itemId: string) => void;
    openCreateItem: () => void;
    openEditItem: (item: Item) => void;
    closeModal: (name: keyof ModalState["modals"]) => void;
    closeAllModals: () => void;
  };
}
```

### Feature Flags

```typescript
interface FeatureFlagsState {
  flags: Record<string, boolean>;

  actions: {
    setFlag: (name: string, enabled: boolean) => void;
    isEnabled: (name: string) => boolean;
  };
}

export const useFeatureFlags = create<FeatureFlagsState>()(
  persist(
    (set, get) => ({
      flags: {},

      actions: {
        setFlag: (name, enabled) => set((state) => ({
          flags: { ...state.flags, [name]: enabled },
        })),

        isEnabled: (name) => get().flags[name] ?? false,
      },
    }),
    { name: "feature-flags" }
  )
);

// Selector hook
export const useIsFeatureEnabled = (name: string) =>
  useFeatureFlags((s) => s.flags[name] ?? false);
```

### Undo/Redo Pattern

```typescript
interface UndoableState<T> {
  past: T[];
  present: T;
  future: T[];

  actions: {
    set: (value: T) => void;
    undo: () => void;
    redo: () => void;
    canUndo: () => boolean;
    canRedo: () => boolean;
  };
}

const createUndoableStore = <T>(initialValue: T) =>
  create<UndoableState<T>>()((set, get) => ({
    past: [],
    present: initialValue,
    future: [],

    actions: {
      set: (value) => set((state) => ({
        past: [...state.past, state.present],
        present: value,
        future: [],
      })),

      undo: () => set((state) => {
        if (state.past.length === 0) return state;
        const previous = state.past[state.past.length - 1];
        return {
          past: state.past.slice(0, -1),
          present: previous,
          future: [state.present, ...state.future],
        };
      }),

      redo: () => set((state) => {
        if (state.future.length === 0) return state;
        const next = state.future[0];
        return {
          past: [...state.past, state.present],
          present: next,
          future: state.future.slice(1),
        };
      }),

      canUndo: () => get().past.length > 0,
      canRedo: () => get().future.length > 0,
    },
  }));
```

---

## Anti-Patterns to Avoid

### 1. Storing Server State

```typescript
// Bad - duplicates server state management
const useBadStore = create(() => ({
  users: [],
  fetchUsers: async () => {
    const users = await api.getUsers();
    set({ users });
  },
}));

// Good - use TanStack Query for server state
const useUsers = () => useQuery({
  queryKey: ["users"],
  queryFn: api.getUsers
});
```

### 2. Subscribing to Entire Store

```typescript
// Bad - re-renders on ANY state change
const { name } = useStore();

// Good - only re-renders when name changes
const name = useStore((s) => s.name);
```

### 3. Not Grouping Actions

```typescript
// Bad - actions mixed with state
interface BadState {
  count: number;
  increment: () => void;
  decrement: () => void;
  name: string;
  setName: (n: string) => void;
}

// Good - actions grouped
interface GoodState {
  count: number;
  name: string;
  actions: {
    increment: () => void;
    decrement: () => void;
    setName: (n: string) => void;
  };
}
```

### 4. Persisting Too Much

```typescript
// Bad - persists everything including transient state
persist(store, { name: "app" })

// Good - explicitly choose what to persist
persist(store, {
  name: "app",
  partialize: (state) => ({
    user: state.user,
    preferences: state.preferences,
  }),
})
```

### 5. Large Monolithic Stores

```typescript
// Bad - one store for everything
const useEverythingStore = create(() => ({
  user: null,
  todos: [],
  settings: {},
  notifications: [],
  cart: [],
  // ... 50 more properties
}));

// Good - domain-specific stores
const useUserStore = create(() => ({ user: null }));
const useTodoStore = create(() => ({ todos: [] }));
const useCartStore = create(() => ({ items: [] }));
```

### 6. Mutating State Directly Without Immer

```typescript
// Bad - mutates state directly (won't trigger updates)
set((state) => {
  state.items.push(item); // Mutation without immer
  return state;
});

// Good with immer middleware
set((state) => {
  state.items.push(item); // Safe with immer
});

// Good without immer
set((state) => ({
  items: [...state.items, item],
}));
```

---

## Checklist for Store Implementation

Before considering a store implementation complete:

- [ ] Store handles a single, focused domain
- [ ] TypeScript interfaces are fully defined
- [ ] Actions are grouped in an `actions` object
- [ ] Middleware is applied in correct order (devtools > persist > immer)
- [ ] Selector hooks are exported for all state slices
- [ ] Only necessary state is persisted (using `partialize`)
- [ ] Initial state is defined separately for reset functionality
- [ ] Loading and error states are handled for async operations
- [ ] Store is tested with proper isolation
- [ ] No server state is stored (use TanStack Query instead)
