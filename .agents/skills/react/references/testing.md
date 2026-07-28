# Testing

Vitest as the runner, `@testing-library/react` ≥ 16.1.0 for components and hooks.

## React 19 setup changes

```diff
- import { act } from "react-dom/test-utils";
+ import { act } from "react";
```

Codemod: `npx codemod@latest react/19/replace-act-import`.

| Removed or deprecated | Replacement |
| --- | --- |
| `react-dom/test-utils` (everything except `act`) | RTL; `renderIntoDocument` → `render`, `Simulate` → `fireEvent` |
| `react-test-renderer/shallow` | `react-shallow-renderer` (shallow rendering is discouraged) |
| `react-test-renderer` | RTL — it implements its own renderer environment that does not match production |
| `@testing-library/react-hooks` | `renderHook` from `@testing-library/react` |

Testing Library v16 notes:

- v16.0.0 made `@testing-library/dom` and `@types/react-dom` **peer** dependencies — install `@testing-library/dom@^10` yourself. An `ERESOLVE` failure usually means the root project pins `@testing-library/dom@8.x`.
- React 19 support landed in **v16.1.0**. Fix React 19 errors by upgrading RTL, not by downgrading it.
- v16.2.0 forwards `onCaughtError` and `onRecoverableError` to React 19 root options; `onUncaughtError` is rejected at runtime.

Vitest specifics:

- `globals: true` is required for RTL's automatic `cleanup()`, which hooks `afterEach`. Otherwise call `cleanup()` yourself.
- If "The current testing environment is not configured to support act(...)" appears, set `IS_REACT_ACT_ENVIRONMENT` on **`globalThis`** in the setup file — under Vitest with jsdom, `self` and `globalThis` are not the same object. RTL normally sets it for you.

## Component tests

Query by role and accessible name, so the test exercises what a user perceives.

```tsx
import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Button } from "./button";

describe("Button", () => {
  it("renders its label", () => {
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

  it("is disabled when disabled is set", () => {
    render(<Button disabled>Click me</Button>);
    expect(screen.getByRole("button")).toBeDisabled();
  });
});
```

## Hook tests

```tsx
import { renderHook, act } from "@testing-library/react";

describe("useCounter", () => {
  it("increments", () => {
    const { result } = renderHook(() => useCounter());
    act(() => {
      result.current.increment();
    });
    expect(result.current.count).toBe(1);
  });
});
```

With a query client, build a fresh one per test so cases cannot share cache:

```tsx
function createWrapper() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return ({ children }: { children: React.ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );
}

const { result } = renderHook(() => useUser("1"), { wrapper: createWrapper() });
await waitFor(() => expect(result.current.isSuccess).toBe(true));
```

## Compiler-era caution

Assert on rendered output and observable behaviour, not on reference identity. A test that asserts a callback prop keeps the same reference between renders, or counts renders to prove a memo works, is asserting the compiler's output rather than your component's contract — those break first when compilation changes.

When a test fails only with the compiler enabled, run the bisection loop in `compiler.md`: add `"use no memo"`, then remove the local memos too. A failure that survives with no memoization at all is a Rules of React violation in the component, and the fix belongs in production code.
