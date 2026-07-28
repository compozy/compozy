# Components, composition, and TypeScript

## Component shape

Functional components only — class components are legacy and the compiler never optimizes them.

Split for **single responsibility and readability**, never to create a memo boundary; the compiler already gives every component one. Declare every component at module scope (see `static-components` in `rules-of-react.md`). A component past ~200 lines usually has more than one job.

Keep behaviour out of rendering. A custom hook owns state, derivation, and side effects; the component owns markup:

```tsx
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

Custom hooks: one purpose each, `useXxx` naming, arrays for state-like returns and objects for compound returns.

```tsx
// State-like: array, so callers can rename freely
function useToggle(initial = false) {
  const [value, setValue] = useState(initial);
  const toggle = () => setValue((v) => !v); // the compiler stabilizes this
  return [value, toggle] as const;
}

// Compound: object, so callers destructure what they need
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

## Composition over boolean props

```tsx
// Compound components
function Card({ children, className }: React.ComponentProps<"div">) {
  return <div className={cn("card", className)}>{children}</div>;
}
function CardHeader({ children }: { children: React.ReactNode }) {
  return <div className="card-header">{children}</div>;
}

<Card>
  <CardHeader>Title</CardHeader>
  <CardContent>Content</CardContent>
</Card>;

// Render props for generic lists
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

Inline callbacks in JSX are fine — the compiler memoizes them. Do not extract a `useCallback` to "stabilize" a prop.

## Extend native element props

**Default rule for wrappers:** when a component's root output is a **single native element**, its props MUST extend that element's intrinsic props — the same contract shadcn/ui primitives follow. Callers keep `aria-*`, `data-*`, `onClick`, `disabled`, and `ref` without a bespoke passthrough list.

| Requirement | Detail |
| --- | --- |
| Base type | `interface XProps extends React.ComponentProps<"button">` (or `"input"`, `"a"`, `"div"`, …) |
| Spreading | Destructure your own fields, then spread `{...props}` and a merged `className` onto the DOM node |
| Ref | `React.ComponentProps<"…">` already includes `ref` in React 19 — no `forwardRef` |

```tsx
interface TextFieldProps extends React.ComponentProps<"input"> {
  label: string;
  error?: string;
}

function TextField({ label, error, className, ...props }: TextFieldProps) {
  return (
    <label className="flex flex-col gap-1">
      <span>{label}</span>
      <input
        className={cn("rounded border px-2 py-1", error && "border-destructive", className)}
        {...props}
      />
      {error ? <span className="text-destructive text-sm">{error}</span> : null}
    </label>
  );
}
```

With `class-variance-authority`, combine intrinsic props with `VariantProps<typeof variants>` and follow the `shadcn` skill.

## Props typing

Type props directly on the function. `React.FC` was not removed in React 19 and its historical objections were fixed, so it is a stylistic choice rather than an error — but it still breaks inference for generic components, and since `ref` is now an ordinary prop it belongs in the props type anyway. Default to a plain function with an explicit interface.

```tsx
interface CardProps {
  title: string;
  description?: string;
  ref?: React.Ref<HTMLDivElement>;
}

function Card({ title, description, ref }: CardProps) {
  return (
    <div ref={ref}>
      <h3>{title}</h3>
      {description ? <p>{description}</p> : null}
    </div>
  );
}
```

Generic components with `const` type parameters for literal inference:

```tsx
interface SelectProps<T extends string> {
  options: readonly T[];
  value: T;
  onChange: (value: T) => void;
}

function Select<const T extends string>({ options, value, onChange }: SelectProps<T>) {
  return (
    <select value={value} onChange={(e) => onChange(e.target.value as T)}>
      {options.map((option) => (
        <option key={option} value={option}>{option}</option>
      ))}
    </select>
  );
}
```

Children:

```tsx
{ children: React.ReactNode }          // any valid children
{ children: React.ReactElement }       // exactly one element
{ children: (data: T) => React.ReactNode } // render prop
```

## React 19 TypeScript changes

Run the codemod first: `npx types-react-codemod@latest preset-19 ./src` ([upgrade guide](https://react.dev/blog/2024/04/25/react-19-upgrade-guide)).

### ref as a prop; forwardRef deprecated

```tsx
// Legacy
const Input = forwardRef<HTMLInputElement, Props>((props, ref) => <input ref={ref} {...props} />);

// React 19
function Input({ ref, ...props }: Props & { ref?: React.Ref<HTMLInputElement> }) {
  return <input ref={ref} {...props} />;
}
```

Ref-as-prop does **not** work for class components.

### useRef requires an argument

```ts
useRef();          // error: Expected 1 argument but saw none
useRef(undefined); // ok

const ref = useRef<number>(null);
ref.current = 1;   // all refs are mutable now; previously read-only
```

`MutableRefObject` is deprecated in favour of a single `RefObject<T> { current: T }`, and `RefObject<T>` effectively became `RefObject<T | null>` — annotations must include `null`. The `useref-required-initial` transform is skipped for aliased named imports (`useRef as useReactRef`).

### JSX namespace moved to React.JSX

A bare `JSX.Element` now fails with `TS2503: Cannot find namespace 'JSX'`. Use `import { JSX } from "react"` or `React.JSX.Element`. Module augmentation must be wrapped:

```ts
// global.d.ts
declare module "react" {
  namespace JSX {
    interface IntrinsicElements {
      "my-element": { myElementProps: string };
    }
  }
}
```

The module specifier depends on `compilerOptions.jsx`: `react-jsx` → `react/jsx-runtime`, `react-jsxdev` → `react/jsx-dev-runtime`, `react`/`preserve` → `react`.

### Ref callbacks cannot have implicit returns

A returned function is now read as a cleanup function:

```tsx
// Invalid
<div ref={(current) => (instance = current)} />

// Valid
<div ref={(current) => { instance = current; }} />
```

### ReactElement["props"] defaults to unknown

```ts
type A = ReactElement["props"];               // was any, now unknown
type B = ReactElement<{ id: string }>["props"]; // { id: string }
```

Codemod: `npx types-react-codemod@latest react-element-default-any-props ./src`.

### Removed and relocated types

Removed: `ReactChild` (use `ReactElement | string | number`), `ReactText`, `ReactNodeArray` (use `ReadonlyArray<ReactNode>`), `SFC`, `StatelessComponent`, `SFCFactory`.

Moved to `create-react-class`: `ClassicComponentClass`, `ClassicComponent`, `ClassicElement`, `ComponentSpec`, `Mixin`, `ReactChildren`, `ReactHTML`, `ReactSVG`.

### useReducer inference

```ts
// Legacy
useReducer<React.Reducer<State, Action>>(reducer);

// React 19
useReducer(reducer);
useReducer<State, [Action]>(reducer);            // when an annotation is required
useReducer((state: State, action: Action) => state); // or annotate inline
```

## File naming

| Type | Pattern | Example |
| --- | --- | --- |
| Components | kebab-case.tsx | `user-avatar.tsx` |
| Hooks | use-kebab-case.ts | `use-user-data.ts` |
| Utilities | camelCase.ts | `formatDate.ts` |
| Types | types.ts | `types.ts` |
| Tests | *.test.tsx | `user-avatar.test.tsx` |
| Stories | *.stories.tsx | `user-avatar.stories.tsx` |

Co-locate by feature, not by type, and expose a public API through a barrel:

```
src/features/products/
├── components/product-card.tsx
├── hooks/use-product-search.ts
├── api/product-api.ts
├── types/product.ts
└── index.ts   // public API
```

Prefer named exports.

## Accessibility

Semantic HTML before ARIA. A `<nav>` with a list of `<a>` beats styled `<div>`s with `onClick`, and a real `<button>` already handles Enter, Space, and focus — adding `onKeyDown` for those keys duplicates browser behaviour.

```tsx
function Modal({ isOpen, onClose, title, children }: ModalProps) {
  return (
    <dialog open={isOpen} aria-labelledby="modal-title" aria-modal="true" onClose={onClose}>
      <h2 id="modal-title">{title}</h2>
      {children}
      <button onClick={onClose} aria-label="Close modal">×</button>
    </dialog>
  );
}
```
