# Boundary Map

Complete one row per changed list, metric, detail read, or stream before implementation.

| Field | Required decision |
| --- | --- |
| Read model | Name the user- or agent-visible datum. |
| Owner | Name the store, projection, service, or external authority. |
| Scope | Classify it as global, workspace, session, or agent scoped. |
| Population | Define the complete eligible population before any cut. |
| Order | Name the persisted stable tuple or causal sequence. |
| Completeness | Classify the response as exact, counted page, uncounted page, sample, tail, or projection. |
| Stable identity | List scope and filters that select one cache entry. |
| Page identity | List cursor, after, before, or sequence parameters that select continuation. |
| Live lifecycle | Classify frames as snapshot, delta, wake signal, or raw event. |
| Reset | Name the explicit fence or reason that permits replacement. |
| Surfaces | Mark store/core, HTTP, UDS, CLI, native, Extension Host, Web, SSE, docs, and official skill impact. |

## Vertical trace

For Web reads, record the concrete symbols for:

```text
route -> loader -> option factory -> key factory -> adapter -> generated contract
      -> transport -> core/service -> store/projection
```

For live reads, also record:

```text
durable source -> projection cursor -> frame -> listener -> cache writer
               -> page merge -> view model
```

Mark the first boundary that drops, invents, truncates, reorders, or changes the scope of a field. Repair that boundary before changing downstream presentation.
