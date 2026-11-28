---
title: Loops honor authored iteration caps
type: fix
---

New Loop runs use a positive `contract.iteration_cap` from their definition ahead of the
delivery or watch defaults. A run that needs another generation after reaching its cap
ends as `exhausted`. An omitted or zero contract cap uses the configured fallback.

## Migration notes

Review existing definitions with a positive cap if they relied on a higher delivery limit or
on an unbounded watch run. Set a different cap with
`compozy loop configure --name <name> --set iteration_cap=<n>` or set `n` to `0` to make
the Loop unbounded. Previously admitted runs keep the cap pinned at admission.
