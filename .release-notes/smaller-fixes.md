---
title: Smaller fixes
type: fix
---

- **Skills in User sessions.** User sessions can read a bundled skill's resource files again. Thanks to @zaptoscompany for the detailed report.
- **Pending model switch is visible.** When you switch a running session to another provider, its sidebar row and board card show `current → next` until the next message applies it.
- **Cleaner always-on sessions.** An always-on agent's main session hides controls that don't apply to it, such as the linked-work chip, Stop session and the sidebar toggles.
- **PR monitors notice merge conflicts.** A session watching a pull request now wakes up once when the PR starts to conflict with its base branch.
- **Moving sessions between profiles.** A session whose sub-sessions are all stopped and archived can now move to another profile.
- **Fewer GitHub calls.** Branch and PR lookups are batched, and branches you're looking at refresh on demand.
