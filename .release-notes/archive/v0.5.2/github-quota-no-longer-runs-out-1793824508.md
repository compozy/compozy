---
title: PR badges no longer use up your GitHub quota
type: highlight
---

CompozyOS shows a pull request badge on every session and worktree. To keep those badges current, the daemon polled GitHub with your own `gh` login, and that polling alone could exhaust the 5,000 requests/hour that you, `gh` and your agents share. Once the quota ran out, `gh` commands and agents started failing with rate-limit errors.

- **Only live worktrees refresh.** Removed, dismissed and missing worktrees keep showing their last known PR without calling GitHub.
- **Fewer checks for quiet branches.** A branch with no PR is rechecked after 10 minutes, then less often (up to an hour) while it still has none. A merged or closed PR is rechecked every 30 minutes. Open PRs still refresh every minute.
- **No more refresh loops.** A refresh that changes nothing no longer triggers another refresh, and repository capability lookups are cached.
