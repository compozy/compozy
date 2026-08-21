---
title: Recover a Loop-owned task run without losing its place
type: fix
---

A Loop worker task run parked in `needs_attention` had no honest way back. Generic subprocess-health escalation swallowed Loop-owned crashes, and `task run recover` re-enqueued a run that no longer belonged to its Loop. (#447, fixes #437)

- A confirmed agent crash inside a Loop-owned task run stays out of the generic escalation path and projects into the Loop's own node control and event model as worker attention.
- `task run recover` now fails the parked source run, creates and links a child run, and rebinds it to the exact same Loop node and item with the next attempt and epoch — all atomically. Workspace, runtime selection, designation, worktree, network, capabilities, and metadata stay attached to that cell, and attention plus death-streak state is cleared.
- Recovery diagnostics point where they should: a run that needs attention names `task run recover`, while an active run names cancellation.
- No schema, migration, or config key changed — recovery reuses the existing `wait_intervention` attention flag and the existing Loop event vocabulary.

```bash
compozy task run recover <run-id> --reason "operator recovery" -o json
```
