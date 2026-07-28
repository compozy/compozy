---
title: Safe workspace deletion, plus compozy session remove and compozy open
type: fix
---

Workspace deletion is now safe: Compozy refuses to delete a workspace while any of its sessions are still active — returning a 409 that names the blocking sessions — and cleans up the workspace's stopped session history transactionally when deletion proceeds. Two agent-manageable CLI commands ship alongside it: `compozy session remove <id>` deletes a single session and its persisted history, and `compozy open` opens the Compozy web UI in your default browser. The CLI reference also gains documentation for `compozy open`, `compozy session remove`, and the existing `compozy onboarding` command group.
