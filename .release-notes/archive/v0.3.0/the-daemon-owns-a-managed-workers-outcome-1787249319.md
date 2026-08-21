---
title: The daemon owns a managed worker's outcome
type: fix
---

A managed Loop worker session could call complete or fail and race the daemon's own validated action result, so what a generation recorded depended on which side got there first. The daemon is now the single terminal authority for managed workers. (#438)

- A worker session may heartbeat while it holds the lease, but terminal settlement calls are denied by session lineage.
- A generation settles as succeeded only with schema-valid structured output; an invalid capture terminates as `invalid_output` instead of passing.
- The exact validated object round-trips through inline and content-addressed storage, including downstream template and CEL hydration, so a large output no longer loses required fields.
