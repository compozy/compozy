# Zustand

Use this reference for Zustand stores and selector-driven component access.

## Strong defaults

- Keep the store shape minimal and explicit.
- Read through selectors.
- Write through named actions.
- Use a finite `status` field for mutually exclusive modes.
- Derive values outside the store or through selector helpers unless there is a strong reason to store them.

## Store design

A good Zustand store usually has:

- authoritative source state
- named actions for domain events or transitions
- selectors in components or exported selector helpers

Be careful with giant stores that become a bag of unrelated setters.

## Action guidance

Prefer:

- `startCheckout()`
- `submitOrder()`
- `retrySubmission()`
- `selectItem(id)`

Over:

- `setIsLoading(value)`
- `setError(error)`
- `setSelectedItem(item)`

The latter tends to make invalid states easier to represent.

## Derived state

Do not store values that can be computed from:

- other store fields
- props
- fetched entities

Prefer selectors for:

- filtered collections
- selected entity lookup
- permission booleans
- totals, counts, and summaries

If a selector returns an object or array assembled from store pieces, consider shallow equality or narrower selection when rerender pressure matters.

## Common Zustand smells

- many unrelated setters exported directly from the store
- storing both entities and separately synchronized selected objects
- request lifecycle represented by several booleans
- globalizing component-local concerns because Zustand is available
- mutable updates that blur ownership or transition rules
