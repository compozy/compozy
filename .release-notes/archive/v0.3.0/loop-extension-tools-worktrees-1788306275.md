---
title: Loop extension tools run in the worktree the Loop selected
type: fix
---

Extension tools invoked from a Loop now resolve against the selected ready worktree instead of the workspace root, so work stays where the run was pointed. Direct and root-scoped calls are unchanged. (#519)

- Web Loop environment authoring is limited to inherit, workspace root, and a named worktree; directory and per-run values set through the API or CLI remain visible and read-only.
- A worktree is resolved by workspace ID and worktree ref and must be ready before it is used.
- Removing a worktree now takes an exclusive usage lease, so `compozy worktree remove` and the matching delete route report that an operation is in progress while a Loop action is holding that worktree, instead of pulling it out from under the run.
