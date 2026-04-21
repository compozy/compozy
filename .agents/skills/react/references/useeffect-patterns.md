# useEffect Patterns and Best Practices

Effects are an **escape hatch** from React. They let you synchronize with external systems. If there is no external system involved, you shouldn't need an Effect.

## Table of Contents

- [Quick Reference](#quick-reference)
- [When You DO Need Effects](#when-you-do-need-effects)
- [When You DON'T Need Effects](#when-you-dont-need-effects)
- [Decision Tree](#decision-tree)
- [Anti-Patterns](#anti-patterns)
- [Better Alternatives](#better-alternatives)

---

## Quick Reference

| Situation | DON'T | DO |
|-----------|-------|-----|
| Derived state from props/state | `useState` + `useEffect` | Calculate during render |
| Expensive calculations | `useEffect` to cache | `useMemo` |
| Reset state on prop change | `useEffect` with `setState` | `key` prop |
| User event responses | `useEffect` watching state | Event handler directly |
| Notify parent of changes | `useEffect` calling `onChange` | Call in event handler |
| Fetch data | `useEffect` without cleanup | `useEffect` with cleanup OR framework |

---

## When You DO Need Effects

- Synchronizing with **external systems** (non-React widgets, browser APIs)
- **Subscriptions** to external stores (use `useSyncExternalStore` when possible)
- **Analytics/logging** that runs because component displayed
- **Data fetching** with proper cleanup (or use framework's built-in mechanism)

---

## When You DON'T Need Effects

1. **Transforming data for rendering** - Calculate at top level, re-runs automatically
2. **Handling user events** - Use event handlers, you know exactly what happened
3. **Deriving state** - Just compute it: `const fullName = firstName + ' ' + lastName`
4. **Chaining state updates** - Calculate all next state in the event handler

---

## Decision Tree

```
Need to respond to something?
├── User interaction (click, submit, drag)?
│   └── Use EVENT HANDLER
├── Component appeared on screen?
│   └── Use EFFECT (external sync, analytics)
├── Props/state changed and need derived value?
│   └── CALCULATE DURING RENDER
│       └── Expensive? Use useMemo
└── Need to reset state when prop changes?
    └── Use KEY PROP on component
```

---

## Anti-Patterns

### 1. Redundant State for Derived Values

```tsx
// BAD: Extra state + Effect for derived value
function Form() {
  const [firstName, setFirstName] = useState("Taylor");
  const [lastName, setLastName] = useState("Swift");
  const [fullName, setFullName] = useState("");

  useEffect(() => {
    setFullName(firstName + " " + lastName);
  }, [firstName, lastName]);
}

// GOOD: Calculate during rendering
function Form() {
  const [firstName, setFirstName] = useState("Taylor");
  const [lastName, setLastName] = useState("Swift");
  const fullName = firstName + " " + lastName; // Just compute it
}
```

**Why it's bad**: Causes extra render pass with stale value, then re-renders with updated value.

---

### 2. Filtering/Transforming Data in Effect

```tsx
// BAD: Effect to filter list
function TodoList({ todos, filter }) {
  const [visibleTodos, setVisibleTodos] = useState([]);

  useEffect(() => {
    setVisibleTodos(getFilteredTodos(todos, filter));
  }, [todos, filter]);
}

// GOOD: Filter during render (memoize if expensive)
function TodoList({ todos, filter }) {
  const visibleTodos = useMemo(() => getFilteredTodos(todos, filter), [todos, filter]);
}
```

---

### 3. Resetting State on Prop Change

```tsx
// BAD: Effect to reset state
function ProfilePage({ userId }) {
  const [comment, setComment] = useState("");

  useEffect(() => {
    setComment("");
  }, [userId]);
}

// GOOD: Use key prop
function ProfilePage({ userId }) {
  return <Profile userId={userId} key={userId} />;
}

function Profile({ userId }) {
  const [comment, setComment] = useState(""); // Resets automatically
}
```

**Why key works**: React treats components with different keys as different components, recreating state.

---

### 4. Event-Specific Logic in Effect

```tsx
// BAD: Effect for button click result
function ProductPage({ product, addToCart }) {
  useEffect(() => {
    if (product.isInCart) {
      showNotification(`Added ${product.name}!`);
    }
  }, [product]);

  function handleBuyClick() {
    addToCart(product);
  }
}

// GOOD: Handle in event handler
function ProductPage({ product, addToCart }) {
  function handleBuyClick() {
    addToCart(product);
    showNotification(`Added ${product.name}!`);
  }
}
```

**Why it's bad**: Effect fires on page refresh (isInCart is true), showing notification unexpectedly.

---

### 5. Chains of Effects

```tsx
// BAD: Effects triggering each other
function Game() {
  const [card, setCard] = useState(null);
  const [goldCardCount, setGoldCardCount] = useState(0);
  const [round, setRound] = useState(1);
  const [isGameOver, setIsGameOver] = useState(false);

  useEffect(() => {
    if (card?.gold) setGoldCardCount(c => c + 1);
  }, [card]);

  useEffect(() => {
    if (goldCardCount > 3) {
      setRound(r => r + 1);
      setGoldCardCount(0);
    }
  }, [goldCardCount]);

  useEffect(() => {
    if (round > 5) setIsGameOver(true);
  }, [round]);
}

// GOOD: Calculate in event handler
function Game() {
  const [card, setCard] = useState(null);
  const [goldCardCount, setGoldCardCount] = useState(0);
  const [round, setRound] = useState(1);
  const isGameOver = round > 5; // Derived!

  function handlePlaceCard(nextCard) {
    if (isGameOver) throw Error("Game ended");

    setCard(nextCard);
    if (nextCard.gold) {
      if (goldCardCount < 3) {
        setGoldCardCount(goldCardCount + 1);
      } else {
        setGoldCardCount(0);
        setRound(round + 1);
        if (round === 5) alert("Good game!");
      }
    }
  }
}
```

**Why it's bad**: Multiple re-renders (setCard -> setGoldCardCount -> setRound -> setIsGameOver). Also fragile for features like history replay.

---

### 6. Notifying Parent via Effect

```tsx
// BAD: Effect to notify parent
function Toggle({ onChange }) {
  const [isOn, setIsOn] = useState(false);

  useEffect(() => {
    onChange(isOn);
  }, [isOn, onChange]);

  function handleClick() {
    setIsOn(!isOn);
  }
}

// GOOD: Notify in same event
function Toggle({ onChange }) {
  const [isOn, setIsOn] = useState(false);

  function updateToggle(nextIsOn) {
    setIsOn(nextIsOn);
    onChange(nextIsOn); // Same event, batched render
  }

  function handleClick() {
    updateToggle(!isOn);
  }
}

// BEST: Fully controlled component
function Toggle({ isOn, onChange }) {
  function handleClick() {
    onChange(!isOn);
  }
}
```

---

### 7. Passing Data Up to Parent

```tsx
// BAD: Child fetches, passes up via Effect
function Parent() {
  const [data, setData] = useState(null);
  return <Child onFetched={setData} />;
}

function Child({ onFetched }) {
  const data = useSomeAPI();

  useEffect(() => {
    if (data) onFetched(data);
  }, [onFetched, data]);
}

// GOOD: Parent fetches, passes down
function Parent() {
  const data = useSomeAPI();
  return <Child data={data} />;
}
```

**Why**: Data should flow down. Upward flow via Effects makes debugging hard.

---

### 8. Fetching Without Cleanup (Race Condition)

```tsx
// BAD: No cleanup - race condition
function SearchResults({ query }) {
  const [results, setResults] = useState([]);

  useEffect(() => {
    fetchResults(query).then(json => {
      setResults(json); // "hello" response may arrive after "hell"
    });
  }, [query]);
}

// GOOD: Cleanup ignores stale responses
function SearchResults({ query }) {
  const [results, setResults] = useState([]);

  useEffect(() => {
    let ignore = false;

    fetchResults(query).then(json => {
      if (!ignore) setResults(json);
    });

    return () => {
      ignore = true;
    };
  }, [query]);
}
```

---

### 9. App Initialization in Effect

```tsx
// BAD: Runs twice in dev, may break auth
function App() {
  useEffect(() => {
    loadDataFromLocalStorage();
    checkAuthToken(); // May invalidate token on second call!
  }, []);
}

// GOOD: Module-level guard
let didInit = false;

function App() {
  useEffect(() => {
    if (!didInit) {
      didInit = true;
      loadDataFromLocalStorage();
      checkAuthToken();
    }
  }, []);
}

// ALSO GOOD: Module-level execution
if (typeof window !== "undefined") {
  checkAuthToken();
  loadDataFromLocalStorage();
}
```

---

## Better Alternatives

### 1. Calculate During Render (Derived State)

For values derived from props or state, just compute them:

```tsx
function Form() {
  const [firstName, setFirstName] = useState("Taylor");
  const [lastName, setLastName] = useState("Swift");

  // Runs every render - that's fine and intentional
  const fullName = firstName + " " + lastName;
  const isValid = firstName.length > 0 && lastName.length > 0;
}
```

**When to use**: The value can be computed from existing props/state.

---

### 2. useMemo for Expensive Calculations

When computation is expensive, memoize it:

```tsx
import { useMemo } from "react";

function TodoList({ todos, filter }) {
  const visibleTodos = useMemo(() => getFilteredTodos(todos, filter), [todos, filter]);
}
```

**How to know if it's expensive**:

```tsx
console.time("filter");
const visibleTodos = getFilteredTodos(todos, filter);
console.timeEnd("filter");
// If > 1ms, consider memoizing
```

**Note**: React Compiler can auto-memoize, reducing manual useMemo needs.

---

### 3. Key Prop to Reset State

To reset ALL state when a prop changes, use key:

```tsx
// Parent passes userId as key
function ProfilePage({ userId }) {
  return (
    <Profile
      userId={userId}
      key={userId} // Different userId = different component instance
    />
  );
}

function Profile({ userId }) {
  // All state here resets when userId changes
  const [comment, setComment] = useState("");
  const [likes, setLikes] = useState([]);
}
```

**When to use**: You want a "fresh start" when an identity prop changes.

---

### 4. Store ID Instead of Object

To preserve selection when list changes:

```tsx
// BAD: Storing object that needs Effect to "adjust"
function List({ items }) {
  const [selection, setSelection] = useState(null);

  useEffect(() => {
    setSelection(null); // Reset when items change
  }, [items]);
}

// GOOD: Store ID, derive object
function List({ items }) {
  const [selectedId, setSelectedId] = useState(null);

  // Derived - no Effect needed
  const selection = items.find(item => item.id === selectedId) ?? null;
}
```

**Benefit**: If item with selectedId exists in new list, selection preserved.

---

### 5. Event Handlers for User Actions

User clicks/submits/drags should be handled in event handlers, not Effects:

```tsx
// Event handler knows exactly what happened
function ProductPage({ product, addToCart }) {
  function handleBuyClick() {
    addToCart(product);
    showNotification(`Added ${product.name}!`);
    analytics.track("product_added", { id: product.id });
  }

  function handleCheckoutClick() {
    addToCart(product);
    showNotification(`Added ${product.name}!`);
    navigateTo("/checkout");
  }
}
```

**Shared logic**: Extract a function, call from both handlers:

```tsx
function buyProduct() {
  addToCart(product);
  showNotification(`Added ${product.name}!`);
}

function handleBuyClick() {
  buyProduct();
}
function handleCheckoutClick() {
  buyProduct();
  navigateTo("/checkout");
}
```

---

### 6. useSyncExternalStore for External Stores

For subscribing to external data (browser APIs, third-party stores):

```tsx
// Instead of manual Effect subscription
function useOnlineStatus() {
  const [isOnline, setIsOnline] = useState(true);

  useEffect(() => {
    function update() {
      setIsOnline(navigator.onLine);
    }
    window.addEventListener("online", update);
    window.addEventListener("offline", update);
    return () => {
      window.removeEventListener("online", update);
      window.removeEventListener("offline", update);
    };
  }, []);

  return isOnline;
}

// Use purpose-built hook
import { useSyncExternalStore } from "react";

function subscribe(callback) {
  window.addEventListener("online", callback);
  window.addEventListener("offline", callback);
  return () => {
    window.removeEventListener("online", callback);
    window.removeEventListener("offline", callback);
  };
}

function useOnlineStatus() {
  return useSyncExternalStore(
    subscribe,
    () => navigator.onLine, // Client value
    () => true // Server value (SSR)
  );
}
```

---

### 7. Lifting State Up

When two components need synchronized state, lift it to common ancestor:

```tsx
// Instead of syncing via Effects between siblings
function Parent() {
  const [value, setValue] = useState("");

  return (
    <>
      <Input value={value} onChange={setValue} />
      <Preview value={value} />
    </>
  );
}
```

---

### 8. Custom Hooks for Data Fetching

Extract fetch logic with proper cleanup:

```tsx
function useData(url) {
  const [data, setData] = useState(null);
  const [error, setError] = useState(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    let ignore = false;
    setLoading(true);

    fetch(url)
      .then(res => res.json())
      .then(json => {
        if (!ignore) {
          setData(json);
          setError(null);
        }
      })
      .catch(err => {
        if (!ignore) setError(err);
      })
      .finally(() => {
        if (!ignore) setLoading(false);
      });

    return () => {
      ignore = true;
    };
  }, [url]);

  return { data, error, loading };
}

// Usage
function SearchResults({ query }) {
  const { data, error, loading } = useData(`/api/search?q=${query}`);
}
```

**Better**: Use framework's data fetching (TanStack Query, SWR, Next.js, etc.)

---

## Summary: When to Use What

| Need | Solution |
|------|----------|
| Value from props/state | Calculate during render |
| Expensive calculation | `useMemo` |
| Reset all state on prop change | `key` prop |
| Respond to user action | Event handler |
| Sync with external system | `useEffect` with cleanup |
| Subscribe to external store | `useSyncExternalStore` |
| Share state between components | Lift state up |
| Fetch data | Custom hook with cleanup / TanStack Query |
