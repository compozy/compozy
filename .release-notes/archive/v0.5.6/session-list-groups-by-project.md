---
title: The session list groups by project by default
type: feat
---

Under **All projects**, the session list and the board now group sessions by project out of the box. Before, a fresh install showed one flat list that mixed every project's sessions, and most people switched **View options → Group by** to **Project** by hand. A list that shows one project stays flat, as before.

Migration notes: no action needed, and nothing in `config.toml` is rewritten. Only an unset `shell.sessions.group_by` changes, from `none` to `project`; this includes configs that still carry the deprecated `shell.sessions.scope` without `group_by`. A stored `group_by = "none"` keeps the flat list, and a window where you already picked a grouping keeps its choice. To go back to the flat list everywhere, run `compozy config set shell.sessions.group_by none`.
