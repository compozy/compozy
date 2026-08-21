---
title: Every domain opens inside the palette
type: feature
---

The palette is not only a launcher — it browses. Sessions, Tasks, Loops, Jobs, Agents, Triggers, Marketplace, Bridges, Knowledge, Vault, Worktrees, Network channels, and Extensions each open as a view without leaving the overlay, and views stack so one selection can push the next. (#441)

- Four view kinds ship: **list**, **detail**, **grid**, and **form**. Lists carry domain chips with truthful counts and single-select semantics; a chip with zero matches names the filter and clears in one keystroke.
- State badges come from the shared status vocabulary and always pair a glyph with a label — never color alone.
- Selecting a row previews its metadata and sanitized text in a detail pane without stealing focus from the list, and the pane clears when the row disappears from another surface instead of showing stale content.
- Form views traverse typed fields in declared order, block an invalid submit on the first failing field, and discard values when you pop the view.
- Vault rows show names and metadata only. A secret value never enters a view, a preview, or a match highlight.
- A cold open shows a loading state, never a blank list dressed up as empty; an oversized list either scrolls virtually or states the exact `showing N of M`.
- Views stream patches, so a list already on screen updates in place as the runtime changes.

```bash
compozy cmd-palette list --source core -o json | grep palette.view.
# palette.view.sessions, .tasks, .loops, .jobs, .agents, .triggers,
# .marketplace, .bridges, .knowledge, .vault, .worktrees,
# .network-channels, .extensions
```
