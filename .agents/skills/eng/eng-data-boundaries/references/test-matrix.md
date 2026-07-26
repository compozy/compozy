# Data-Boundary Test Matrix

Select only rows matching the changed contract. Name one primary owning layer and reuse its canonical suite.

| Changed contract | Falsifying case |
| --- | --- |
| Counted page | Create more eligible rows than one page; require exact total, continuation, no gaps, and no duplicates. |
| Filter | Place matching rows beyond the first unfiltered cut; require filtering before limit. |
| Facets | Put facet members off page; require counts from the complete filtered population. |
| Cursor | Reuse after mutation and attempt reuse with different filters or workspace; require stability and isolation. |
| Stable order | Give equal display timestamps reverse-lexical IDs; require persisted domain order. |
| Exact metric | Exceed the limit of a related bounded read; require the metric to count the complete population. |
| Query identity | Change one stable filter and then only the cursor; require a different base key for the former and the same base key for the latter. |
| Route preload | Navigate through the loader and mount the hook; require one canonical request and the same cache entry. |
| Infinite cache | Load more/older pages, then refetch or mutate the head; require all loaded pages to survive. |
| Live reconnect | Exercise duplicate frames, empty deltas, remount, stale fences, and explicit reset; require monotonic applied cursor and lossless history. |
| Workspace scope | Seed overlapping IDs in two workspaces; require distinct store reads, transport results, cache keys, and SSE updates. |
| Hydration cost | Increase catalog size while keeping page size fixed; require bounded statement/call count. |
| Deep link scope | Use missing, stale, and explicitly valid foreign selections; require only missing/stale selection to adopt the route owner. |

## Placement

- Let store/service suites own filter, order, cursor, aggregate, and hydration-cost semantics.
- Let transport suites own parsing, forwarding, status, and envelope shape.
- Let query/cache pure suites own key identity, envelope preservation, merge, and fences.
- Let route integration own loader-to-hook cache reuse.
- Use browser/E2E only for behavior that requires navigation, viewport anchoring, reload, or a real stream/process boundary.
