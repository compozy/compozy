---
title: First-class Git worktrees
type: feature
---

Git worktrees are runtime objects across the daemon, Web desktop, CLI, HTTP and UDS, native tools, extensions, configuration, generated contracts, and documentation: create, adopt, discover, inspect, reconcile, dismiss, and safely remove isolated checkouts. Sessions, Task runs, fan-out workers, and Loop environments bind to an exact worktree without losing the parent workspace's config, skills, agents, or memory. (#388, #410)

- The new worktree domain owns Git capability detection, canonical repository identity, naming and placement, per-repository mutation locking, and durable lifecycle state.
- Creation is a phased `pending → ready` operation with recorded ownership checkpoints, bootstrap copy and setup support, cancel-safe rollback, and boot recovery.
- Adoption verifies the linked checkout, the common Git directory, main-checkout exclusion, and repository identity before registering it. Discovery merges Git-known checkouts with durable records without turning a discovered row into an adopted worktree.
- Removal fences the record as removing, rechecks session activity and Git safety under the repository lock, preserves branches and history, and requires an explicit second step for dirty or unique-unpushed work.
- Finishing work runs through a truthful assisted-exit ladder for commit, push, pull request, merged evidence, and cleanup. Dismissed tombstones release their reserved name, exit actions resolve caller references to canonical record ids, and CLI mutation output keeps worktree identity.
