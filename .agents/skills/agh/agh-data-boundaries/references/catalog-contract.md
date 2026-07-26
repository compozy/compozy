# Catalog Contract

Apply only the sections matching the changed read model.

## Source ownership

- Resolve authorization and workspace scope before reading candidates.
- Apply filters and stable ordering before the page cut.
- Compute exact totals and facets from the complete filtered population.
- Hydrate rich data only for returned IDs and in a bounded batch.
- Use an exact aggregate for exact metrics; keep bounded prompt, preview, or tail reads semantically separate.
- Represent causal order with a persisted sequence or stable domain tuple. A deterministic tie-breaker is insufficient when it does not represent causality.

## Cursor identity

- Bind opaque cursors to normalized scope, stable filters, and ordering.
- Keep stable filters in the base query key.
- Keep `cursor`, `after`, `before`, and `before_sequence` in the page parameter.
- Preserve cursor mutation stability and reject cross-workspace or cross-query reuse.
- Avoid exposing a global sequence when its gaps reveal activity from another workspace.

## Cache and presentation

- Store the complete page envelope, including totals, facets, limits, and continuation metadata.
- Flatten and deduplicate only at the view-model read boundary while preserving server order.
- Label loaded or recent counts as such; display a total only when the owner returns an exact total.
- Separate initial failure, continuation failure, and background refresh failure so loaded data remains usable.
- Remove or defer controls and badges whose truth requires an unavailable aggregate.

## Public parity

- Keep HTTP, UDS, CLI, native tools, Extension Host, generated SDKs, docs, and `skills/agh/` on the same hard-cut contract.
- Preserve a lean list shape; keep expensive, sensitive, and execution-only fields in detail reads.
