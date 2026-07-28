# Query Mutations

## Invalidate Related Queries After Mutations

After a mutation, invalidate every query whose data it could have affected — partial invalidation (e.g. hitting `['todos','list']` but not `['todos','count']`) leaves the cache stale. Mutation callbacks receive `(data, variables, context)`, so you can target keys from the mutation input.

```tsx
const updateTodo = useMutation({
  mutationFn: ({ id, data }) => api.updateTodo(id, data),
  onSuccess: (data, { id }) => {
    queryClient.invalidateQueries({ queryKey: ['todos', id] })
    queryClient.invalidateQueries({ queryKey: ['todos', 'list'] })
    queryClient.invalidateQueries({ queryKey: ['todos', 'completed'] })
    queryClient.invalidateQueries({ queryKey: ['todos', 'active'] })
  },
})

const assignTodoToUser = useMutation({
  mutationFn: ({ todoId, userId }) => api.assignTodo(todoId, userId),
  onSuccess: (data, { todoId, userId }) => {
    queryClient.invalidateQueries({ queryKey: ['todos', todoId] })
    queryClient.invalidateQueries({ queryKey: ['users', userId, 'todos'] })
    if (data.previousAssignee) {
      queryClient.invalidateQueries({
        queryKey: ['users', data.previousAssignee, 'todos'],
      })
    }
  },
})
```

When you already hold the new value, update the cache directly with `setQueryData` instead of refetching — or mix both:

```tsx
// Update cache directly (no network request)
onSuccess: (newTodo) => {
  queryClient.setQueryData(['todos'], (old: Todo[]) => [...old, newTodo])
}

// Hybrid — update one query, invalidate others
onSuccess: (newTodo) => {
  queryClient.setQueryData(['todos', 'list'], (old: Todo[]) => [...old, newTodo])
  queryClient.invalidateQueries({ queryKey: ['todos', 'count'] })
}
```

Place invalidation in `onSuccess`; use `onSettled` to invalidate regardless of success or failure.

## Implement Optimistic Updates

Optimistic updates reflect a mutation in the UI before the server confirms it — without one, the user clicks and waits 200–500ms for feedback. Use them for user-initiated mutations with a predictable outcome. Two distinct approaches follow.

### Via cache manipulation (with rollback)

```tsx
const mutation = useMutation({
  mutationFn: toggleTodoComplete,
  onMutate: async (todoId) => {
    await queryClient.cancelQueries({ queryKey: ['todos'] })
    const previousTodos = queryClient.getQueryData(['todos'])
    queryClient.setQueryData(['todos'], (old: Todo[]) =>
      old.map((todo) =>
        todo.id === todoId ? { ...todo, completed: !todo.completed } : todo
      )
    )
    return { previousTodos }
  },
  onError: (err, todoId, context) => {
    queryClient.setQueryData(['todos'], context?.previousTodos)
  },
  onSettled: () => {
    queryClient.invalidateQueries({ queryKey: ['todos'] })
  },
})
```

For an optimistic **create**, insert a temporary-id row in `onMutate`, then swap it for the server row in `onSuccess`:

```tsx
const createTodo = useMutation({
  mutationFn: (newTodo: CreateTodoInput) => api.createTodo(newTodo),
  onMutate: async (newTodo) => {
    await queryClient.cancelQueries({ queryKey: ['todos'] })
    const previousTodos = queryClient.getQueryData(['todos'])
    const optimisticTodo = {
      id: `temp-${Date.now()}`,
      ...newTodo,
      completed: false,
      createdAt: new Date().toISOString(),
    }
    queryClient.setQueryData(['todos'], (old: Todo[]) => [...old, optimisticTodo])
    return { previousTodos, optimisticTodo }
  },
  onError: (err, newTodo, context) => {
    queryClient.setQueryData(['todos'], context?.previousTodos)
  },
  onSuccess: (data, variables, context) => {
    queryClient.setQueryData(['todos'], (old: Todo[]) =>
      old.map((todo) => (todo.id === context?.optimisticTodo.id ? data : todo))
    )
  },
})
```

### Via UI variables (simpler)

Derive the displayed value from `mutation.isPending` instead of touching the cache — no rollback needed.

```tsx
function TodoItem({ todo }: { todo: Todo }) {
  const mutation = useMutation({
    mutationFn: toggleTodoComplete,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['todos'] })
    },
  })

  const displayCompleted = mutation.isPending ? !todo.completed : todo.completed

  return (
    <div>
      <input
        type="checkbox"
        checked={displayCompleted}
        disabled={mutation.isPending}
        onChange={() => mutation.mutate(todo.id)}
      />
      <span style={{ opacity: mutation.isPending ? 0.5 : 1 }}>{todo.title}</span>
    </div>
  )
}
```

| Approach | Use When |
|----------|----------|
| Cache Manipulation | Update appears in multiple places, complex data structures |
| UI Variables | Update only visible in one component, simpler implementation |

For cache manipulation: always `cancelQueries` before writing to avoid a race, roll back from `context` in `onError`, and `invalidateQueries` in `onSettled` to sync with server truth.

## Use useMutationState for Cross-Component Tracking

`useMutationState` reads mutation state from anywhere in the tree — no prop-drilling `mutation.isPending` down to every consumer. It requires a `mutationKey` on the mutations you want to observe and returns an array of `select`ed values from matching mutations.

```tsx
const useCreatePost = () => useMutation({
  mutationKey: ['create-post'],
  mutationFn: createPost,
})

function GlobalLoadingIndicator() {
  const pendingMutations = useMutationState({
    filters: { status: 'pending' },
    select: (mutation) => mutation.state.variables,
  })

  if (pendingMutations.length === 0) return null
  return <div className="global-loading">Saving {pendingMutations.length} item(s)...</div>
}
```

Render pending mutation variables as optimistic list items from a sibling component:

```tsx
function TodoList() {
  const { data: todos } = useQuery({ queryKey: ['todos'], queryFn: fetchTodos })

  const pendingTodos = useMutationState({
    filters: { mutationKey: ['create-todo'], status: 'pending' },
    select: (mutation) => mutation.state.variables as NewTodo,
  })

  return (
    <ul>
      {todos?.map((todo) => <TodoItem key={todo.id} todo={todo} />)}
      {pendingTodos.map((todo, index) => (
        <TodoItem key={`pending-${index}`} todo={{ ...todo, id: `temp-${index}` }} isPending />
      ))}
    </ul>
  )
}
```

Track one specific mutation by keying on an id and selecting a boolean:

```tsx
const isDeletingThisPost = useMutationState({
  filters: { mutationKey: ['delete-post', postId], status: 'pending' },
  select: () => true,
}).length > 0
```

Filters reference:

```tsx
useMutationState({
  filters: {
    mutationKey: ['key'],
    status: 'pending',  // 'idle' | 'pending' | 'success' | 'error'
    predicate: (mutation) => bool,
  },
  select: (mutation) => mutation.state.variables,
})
```
