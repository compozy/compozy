---
title: Window manager follow-ups for tabs and splits
type: fix
---

Several rough edges left after tabs and splits are fixed:

- Editing a shortcut at project scope now saves only what differs from the inherited settings, so later user-level changes still reach that project. Existing project files load the same values; the next project save drops copies equal to the inherited value.
- Agent and extension rows in the command palette update without a reload, and a disabled extension shows "disabled" instead of a health word.
- A terminal window that hit the per-project terminal limit opens a terminal as soon as a slot frees up. Dismissing the limit dialog shows the empty state with its open control.
- Removing a project whose window layout can't be read answers `503 window_manager_unavailable` with the project and ID to retry, instead of a generic 500. Retrying `compozy workspace remove` finishes the removal.
- `compozy hooks list` shows a user-level hook once instead of once per registered project. JSON output is unchanged.
- The tab context menu groups its commands the same way as the other CompozyOS menus.

Migration notes: no action needed.
