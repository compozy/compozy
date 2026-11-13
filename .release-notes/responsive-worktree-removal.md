---
title: Responsive worktree removal with batch previews
type: fix
---

Registered worktrees now leave the active checkout path before a paced background worker deletes their files. Large dependency trees no longer hold the repository lock throughout file deletion. Cleanup pauses during CPU pressure, resumes leftovers on startup and falls back to synchronous removal when staging cannot rename the checkout.

The worktree removal CLI, native tool and HTTP/UDS API accept batches with per-target outcomes and a `dry_run` risk preview. Existing single-target request and response shapes are preserved. Failed fenced reads restore the original record, and repeating removal reconciles abandoned `removing` records without restarting the daemon.

Migration notes: no action needed. Background cleanup may retain staged bytes under the managed worktrees root until CPU pressure subsides.

Collection rediscovers profile-specific roots after restart and only empties CompozyOS-owned trash. Existing trash folders stay untouched, submodule checkouts retain Git's synchronous safety checks, and CLI batch timeouts scale with the number of targets.
