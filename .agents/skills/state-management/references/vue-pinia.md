# Vue and Pinia

Use this reference for Vue state management with local reactive state, composables, or Pinia stores.

## Strong defaults

- Keep authoritative state in reactive state or store state.
- Put derived values in `computed(...)` or Pinia getters.
- Centralize writes in methods or actions when transitions matter.
- Use one explicit `status` for mutually exclusive modes.

## What belongs in state vs computed

Prefer state for:

- user input
- current IDs and durable facts
- explicit workflow status
- authoritative local settings

Prefer `computed` or getters for:

- filtered and sorted collections
- `isEmpty`, `canSubmit`, `hasErrors`, and similar booleans
- totals and summaries
- selected entity lookup from IDs

Do not keep writing computed values back into state.

## Pinia guidance

Pinia getters are the natural home for derived state. Pinia actions are the place for transitions and side effects that belong to the store boundary.

Be careful with stores that:

- act like global dumping grounds
- expose raw state mutations from many places
- duplicate state that could be recomputed

## Common Vue and Pinia smells

- watchers that synchronize derivable state
- multiple request booleans instead of one status
- storing whole selected objects instead of IDs
- mixing server cache concerns and UI-local concerns in one store
- using a store for state that belongs inside a component or composable
