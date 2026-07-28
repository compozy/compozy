# Effects

An effect exists to synchronize React with something outside React. The governing principle from [You Might Not Need an Effect](https://react.dev/learn/you-might-not-need-an-effect):

> Use Effects only for code that should run *because* the component was displayed to the user.

Under the compiler this stopped being advice. `react-hooks/set-state-in-effect` is **error**-level, and its rationale is concrete: a synchronous `setState` in an effect restarts the render cycle — re-render, DOM commit, effects, then an extra render pass — before the browser can paint. That is visual jank, not merely waste.

## An effect is warranted for

- Synchronizing with **external systems** — non-React widgets, browser APIs, media elements.
- **Subscriptions** to external stores — prefer `useSyncExternalStore` (see `state.md`).
- **Analytics or logging** that fires because the component was displayed.
- **Data fetching** with cleanup — or, better, a query library / framework loader.
- **Reading layout from the DOM** — `useLayoutEffect` plus `setState` is the sanctioned exception to the setState-in-effect rule.

```jsx
// VALID: the value can only come from a committed DOM node
useLayoutEffect(() => {
  const { height } = ref.current.getBoundingClientRect();
  setTooltipHeight(height);
}, []);
```

## Decision table

| Need | Solution |
| --- | --- |
| Value derived from props or state | Compute during render |
| Genuinely slow computation | The compiler memoizes it; measure before adding `useMemo` |
| Reset **all** state when an identity prop changes | `key` prop |
| Adjust **some** state when a prop changes | Compare against previous state during render, or derive |
| Respond to a user action | Event handler |
| Notify a parent of a change | Call the callback in the same handler as the `setState` |
| Read a value that must not re-trigger an effect | `useEffectEvent` |
| Sync with an external system | `useEffect` with cleanup |
| Subscribe to an external store | `useSyncExternalStore` |
| Share state between components | Lift it up |
| Fetch data | Query library, or a custom hook with an `ignore` flag |
| Preserve state and DOM while hidden | `<Activity mode="hidden">` — see `react19-apis.md` |

## Anti-patterns and replacements

### Derived state

```jsx
// INVALID: extra render pass with a stale value
const [fullName, setFullName] = useState("");
useEffect(() => {
  setFullName(firstName + " " + lastName);
}, [firstName, lastName]);

// VALID
const fullName = firstName + " " + lastName;
```

Same shape, same fix, for filtering and mapping:

```jsx
// INVALID
useEffect(() => {
  setVisibleTodos(getFilteredTodos(todos, filter));
}, [todos, filter]);

// VALID — the compiler memoizes this
const visibleTodos = getFilteredTodos(todos, filter);
```

### Resetting all state on a prop change

```jsx
// INVALID
useEffect(() => {
  setComment("");
}, [userId]);

// VALID: a different key is a different component instance, so all state resets
<Profile userId={userId} key={userId} />
```

### Adjusting some state on a prop change

```jsx
// INVALID
useEffect(() => {
  setSelection(null);
}, [items]);

// VALID: compare during render (terminates because it is conditional)
const [prevItems, setPrevItems] = useState(items);
if (items !== prevItems) {
  setPrevItems(items);
  setSelection(null);
}
```

Better still, remove the state. Storing an id instead of an object makes the adjustment unnecessary and preserves the selection when the item survives the update:

```jsx
// INVALID: stores the object, so it needs an effect to "fix" it
const [selection, setSelection] = useState(null);

// VALID: store the id, derive the object
const [selectedId, setSelectedId] = useState(null);
const selection = items.find((item) => item.id === selectedId) ?? null;
```

### Event-specific logic in an effect

```jsx
// INVALID: fires on page refresh too, because isInCart is already true
useEffect(() => {
  if (product.isInCart) showNotification(`Added ${product.name}!`);
}, [product]);

// VALID: the handler knows exactly what happened
function handleBuyClick() {
  addToCart(product);
  showNotification(`Added ${product.name}!`);
  analytics.track("product_added", { id: product.id });
}
```

Share logic between handlers by extracting a function, not by adding an effect:

```jsx
function buyProduct() {
  addToCart(product);
  showNotification(`Added ${product.name}!`);
}
function handleBuyClick() { buyProduct(); }
function handleCheckoutClick() { buyProduct(); navigateTo("/checkout"); }
```

A POST that belongs to a submit belongs in `handleSubmit`, not in an effect watching the state it set.

### Chains of effects

```jsx
// INVALID: each effect triggers the next — four render passes, fragile under replay
useEffect(() => { if (card?.gold) setGoldCardCount((c) => c + 1); }, [card]);
useEffect(() => {
  if (goldCardCount > 3) { setRound((r) => r + 1); setGoldCardCount(0); }
}, [goldCardCount]);
useEffect(() => { if (round > 5) setIsGameOver(true); }, [round]);

// VALID: one handler computes the whole transition, and the rest is derived
const isGameOver = round > 5;

function handlePlaceCard(nextCard) {
  if (isGameOver) throw Error("Game already ended");
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
```

### Notifying a parent

```jsx
// INVALID
useEffect(() => {
  onChange(isOn);
}, [isOn, onChange]);

// VALID: same event, one batched render
function updateToggle(nextIsOn) {
  setIsOn(nextIsOn);
  onChange(nextIsOn);
}

// BEST: fully controlled, no local state to sync
function Toggle({ isOn, onChange }) {
  return <button onClick={() => onChange(!isOn)}>{isOn ? "On" : "Off"}</button>;
}
```

### Passing data child to parent

```jsx
// INVALID: data flowing up through an effect
function Child({ onFetched }) {
  const data = useSomeAPI();
  useEffect(() => {
    if (data) onFetched(data);
  }, [onFetched, data]);
}

// VALID: fetch in the parent, pass down
function Parent() {
  const data = useSomeAPI();
  return <Child data={data} />;
}
```

### Fetching without cleanup

```jsx
// INVALID: the "hell" response can land after "hello"
useEffect(() => {
  fetchResults(query).then(setResults);
}, [query]);

// VALID: cleanup ignores stale responses
useEffect(() => {
  let ignore = false;
  fetchResults(query).then((json) => {
    if (!ignore) setResults(json);
  });
  return () => {
    ignore = true;
  };
}, [query]);
```

A query library (see the `tanstack` skill) removes the whole category — caching, deduplication, and cancellation included.

### App initialization

```jsx
// INVALID: runs twice in development, may invalidate the auth token
useEffect(() => {
  checkAuthToken();
  loadDataFromLocalStorage();
}, []);

// VALID: module-scope guard
let didInit = false;
function App() {
  useEffect(() => {
    if (!didInit) {
      didInit = true;
      checkAuthToken();
      loadDataFromLocalStorage();
    }
  }, []);
}

// ALSO VALID: run at module init
if (typeof window !== "undefined") {
  checkAuthToken();
  loadDataFromLocalStorage();
}
```

## useEffectEvent

Stable in React 19.2 ([reference](https://react.dev/reference/react/useEffectEvent)). It separates the *event* part of an effect from the *synchronization* part, so reading a value no longer forces the effect to re-run.

```jsx
function ChatRoom({ roomId, theme }) {
  const onConnected = useEffectEvent(() => {
    showNotification("Connected!", theme); // always reads the latest theme
  });

  useEffect(() => {
    const connection = createConnection(serverUrl, roomId);
    connection.on("connected", () => onConnected());
    connection.connect();
    return () => connection.disconnect();
  }, [roomId]); // theme is not a dep, and onConnected must not be one either
}
```

Constraints that matter:

- Its identity is **intentionally unstable** and changes every render. Never put it in a dependency array — the lint rules warn.
- Callable only from effects or other Effect Events. Not during render, not from event handlers.
- Cannot be passed to another component.
- It is not a tool for silencing `exhaustive-deps`. Reach for it when a value should be *read* by an effect without *triggering* it.

## useSyncExternalStore over manual subscriptions

```jsx
// INVALID for external state: hand-rolled subscribe + local mirror state
function useOnlineStatus() {
  const [isOnline, setIsOnline] = useState(true);
  useEffect(() => {
    function update() { setIsOnline(navigator.onLine); }
    window.addEventListener("online", update);
    window.addEventListener("offline", update);
    return () => {
      window.removeEventListener("online", update);
      window.removeEventListener("offline", update);
    };
  }, []);
  return isOnline;
}

// VALID
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
    subscribe,             // defined outside the component: a changing identity re-subscribes
    () => navigator.onLine, // client snapshot
    () => true,             // server snapshot for SSR
  );
}
```
