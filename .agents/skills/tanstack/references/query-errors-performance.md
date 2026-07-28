# Query Errors and Performance

## Use Error Boundaries with useQueryErrorResetBoundary

With Suspense queries (`useSuspenseQuery`), errors propagate to the nearest error boundary. Wire the boundary's `onReset` to `useQueryErrorResetBoundary().reset` so retrying clears the query error — without it the boundary stays stuck in its error state after a retry.

```tsx
import { useQueryErrorResetBoundary } from '@tanstack/react-query'
import { ErrorBoundary } from 'react-error-boundary'

function QueryErrorBoundary({ children }: { children: React.ReactNode }) {
  const { reset } = useQueryErrorResetBoundary()

  return (
    <ErrorBoundary
      onReset={reset}
      fallbackRender={({ error, resetErrorBoundary }) => (
        <div className="error-container">
          <h2>Something went wrong</h2>
          <pre>{error.message}</pre>
          <button onClick={resetErrorBoundary}>Try again</button>
        </div>
      )}
    >
      {children}
    </ErrorBoundary>
  )
}

function App() {
  return (
    <QueryErrorBoundary>
      <Suspense fallback={<Loading />}>
        <Posts />
      </Suspense>
    </QueryErrorBoundary>
  )
}
```

With TanStack Router, call `reset()` from `useQueryErrorResetBoundary` inside the route's `errorComponent`, alongside the router's own `reset`:

```tsx
import { createFileRoute } from '@tanstack/react-router'
import { useQueryErrorResetBoundary } from '@tanstack/react-query'

export const Route = createFileRoute('/posts')({
  loader: ({ context: { queryClient } }) =>
    queryClient.ensureQueryData(postQueries.list()),

  errorComponent: ({ error, reset }) => {
    const { reset: resetQuery } = useQueryErrorResetBoundary()

    return (
      <div>
        <p>Failed to load posts: {error.message}</p>
        <button
          onClick={() => {
            resetQuery()
            reset()
          }}
        >
          Retry
        </button>
      </div>
    )
  },

  component: PostsPage,
})
```

Place a boundary per failure-isolation unit — wrap each independent Suspense region (activity, stats, notifications) in its own `QueryErrorBoundary` so one failing query doesn't blank the whole view. `reset` clears error state for every query in the boundary.

## Use Select to Transform and Filter Data

The `select` option transforms query data before it reaches your component — use it for filtering, sorting, or deriving values instead of recomputing in the component body on every render. It memoizes (re-runs only when data or the `select` reference changes) and limits re-renders to when the selected slice changes.

```tsx
function CompletedTodos() {
  const { data: completedTodos } = useQuery({
    queryKey: ['todos'],
    queryFn: fetchTodos,
    select: (todos) =>
      todos
        .filter(todo => todo.completed)
        .sort((a, b) =>
          new Date(b.completedAt).getTime() - new Date(a.completedAt).getTime()
        ),
  })

  return <TodoList todos={completedTodos ?? []} />
}
```

When `select` closes over external state (props, filters), wrap it in `useCallback` so the reference stays stable and it doesn't re-run every render:

```tsx
function FilteredTodos({ status }: { status: 'all' | 'active' | 'completed' }) {
  const selectTodos = useCallback(
    (todos: Todo[]) => {
      switch (status) {
        case 'active':
          return todos.filter(t => !t.completed)
        case 'completed':
          return todos.filter(t => t.completed)
        default:
          return todos
      }
    },
    [status]
  )

  const { data: filteredTodos } = useQuery({
    queryKey: ['todos'],
    queryFn: fetchTodos,
    select: selectTodos,
  })

  return <TodoList todos={filteredTodos ?? []} />
}
```

| Scenario | Use Select? |
|----------|-------------|
| Filtering list data | Yes |
| Sorting data | Yes |
| Computing derived values | Yes |
| Picking single item from list | Yes |
| Heavy transformations | Yes (memoized) |
| Simple data pass-through | No |
| Transformation needs external state | Yes, with useCallback |

`select` leverages structural sharing — original query data stays cached and multiple components can apply different `select` to the same query.
