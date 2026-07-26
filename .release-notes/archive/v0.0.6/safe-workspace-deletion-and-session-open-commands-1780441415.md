---
title: Safe workspace deletion, plus agh session remove and agh open
type: fix
---

Workspace deletion is now safe: AGH refuses to delete a workspace while any of its sessions are still active — returning a 409 that names the blocking sessions — and cleans up the workspace's stopped session history transactionally when deletion proceeds. Two agent-manageable CLI commands ship alongside it: `agh session remove <id>` deletes a single session and its persisted history, and `agh open` opens the AGH web UI in your default browser. The CLI reference also gains documentation for `agh open`, `agh session remove`, and the existing `agh onboarding` command group.
