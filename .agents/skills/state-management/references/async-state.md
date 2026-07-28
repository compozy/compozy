# Async State

Use this reference when the problem involves fetching, submitting, retries, cancellation, polling, optimistic updates, or background refresh.

## Strong defaults

- Model async lifecycle explicitly.
- Distinguish remote cache from local draft or UI state.
- Use one finite status for mutually exclusive request modes.
- Add extra modeling only when the workflow really needs it.

## Common statuses

Simple request lifecycle:

- `idle`
- `loading`
- `success`
- `failure`

Richer workflows may need:

- `refreshing`
- `retrying`
- `cancelling`
- `optimistic`

Do not add extra states unless behavior truly differs.

## Server state vs client state

Separate these concerns:

- remote data and cache lifecycle
- local draft edits
- UI mode such as which panel is open

One blob that mixes all three is hard to reason about and easy to break.

## Questions to ask

- Is this a one-shot request, or ongoing synchronization?
- Do retries or cancellation matter?
- Does stale data need to remain visible during refresh?
- Is there an optimistic update that may need rollback?
- Should the request lifecycle live in a query tool instead of a client store?

## Common smells

- `isLoading`, `isRefreshing`, `isSuccess`, and `error` all floating independently
- clearing good data too early during refresh
- request state stored globally when only one component cares
- local drafts overwritten by fresh server responses without explicit rules
