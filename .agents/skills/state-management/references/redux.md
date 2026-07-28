# Redux

Use this reference for Redux and Redux Toolkit codebases.

## Strong defaults

- Treat reducers as transition logic, not random field setters.
- Prefer domain events for action names.
- Keep state serializable and normalized where relational data is involved.
- Put derived reads in selectors.
- Keep most form state out of Redux unless multiple distant consumers truly need it.

## State shape

Prefer state that looks like:

- entity collections keyed by ID
- explicit `status` fields for request or workflow modes
- small, authoritative source facts

Avoid:

- deeply nested relational data with manual syncing
- storing filtered/sorted arrays that can be derived
- storing both entity objects and parallel copies elsewhere in the tree

## Action guidance

Prefer events like:

- `checkout.started`
- `checkout.submitted`
- `checkout.succeeded`
- `checkout.failed`

Be careful with setter-shaped actions like:

- `setLoading`
- `setItems`
- `setError`

Those often leak implementation details and make invalid combinations easy to create.

## Selectors

Selectors are where derived state belongs:

- filtered and sorted views
- aggregate counts
- permission checks
- `canSubmit` and similar booleans
- joining normalized entities

Do not store selector results back into Redux state.

## Async and server state

Redux can model async workflows, but do not force all remote data concerns into Redux just because Redux exists. If the problem is mainly remote caching, freshness, invalidation, retries, and background refetching, a dedicated server-state tool may be the better fit.

## Common Redux smells

- slices that mix entities, UI flags, and transient form state without boundaries
- duplicated request flags instead of one explicit status
- selectors that are trivial because derived state was stored instead
- reducers that can create combinations like "loading and succeeded"
- large stores where ownership and write paths are unclear
