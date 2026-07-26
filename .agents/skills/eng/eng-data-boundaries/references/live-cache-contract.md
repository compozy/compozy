# Live Cache Contract

## Canonical cache

- Keep one canonical TanStack Query entry for each server read model.
- Reuse its option/key factory in loaders, hooks, optimistic writes, polling, and stream writers.
- Store paginated state as `InfiniteData<PageEnvelope>`; keep the live tail and older pages in the same authoritative page set.

## Merge semantics

- Preserve server order and deduplicate by a stable domain identity.
- Update or replace duplicates in place; do not reconstruct causality from display timestamps or random IDs.
- Keep historical pages when the head refreshes or receives a live delta.
- Carry overflow from a bounded head into history instead of dropping it.
- Let an empty visible delta advance its safe cursor when the source projection advanced.

## Fences and reconnect

- Carry the owner's epoch, generation, or equivalent fence through REST, stream frames, and cache state.
- Replace the page set only for an explicit reset whose fence matches the owner contract.
- Treat a raw event channel as a wake signal when it cannot provide contiguous logical changes.
- Reconnect from a cache-owned applied cursor, not a component-local cursor that resets on remount.
- Separate historical `before` paging from incremental `after` tail reads.

## Optimistic writes

- Write through the canonical key factory.
- Snapshot and restore the full page set on failure.
- Reconcile client identities with canonical server identities without erasing page metadata.
- Invalidate only when the owner contract requires a reread; avoid refetch bursts that replace live state with a smaller bounded response.
