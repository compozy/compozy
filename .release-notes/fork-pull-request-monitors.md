---
title: Pull request monitors can watch fork pull requests
type: fix
---

A `pull_request` monitor can now watch a pull request opened from a fork. Pass the branch as `owner:branch`, for example `alice:fix-login`. Before, the monitor registered but never saw the pull request, because only branches in the base repository were matched. A plain branch name still selects a branch in the base repository, and a malformed `owner:branch` value is rejected when you create the monitor.

A canceled monitor also no longer reports its cancellation time as `fired_at`.

Migration notes: no action needed.
