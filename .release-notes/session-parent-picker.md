---
title: Choose a parent for a session from its menu or the command palette
type: feature
---

Nesting a session under another used to need a drag, or a submenu that listed only the sessions already loaded in the sidebar.

- **Move under….** A session's row menu and right-click menu open a parent picker that searches the project's sessions by title or agent. Arrow keys and Enter choose the new parent, and the confirmation offers Undo.
- **From the command palette.** With a session window focused, type `move` or `parent` and choose "Move under another session…" to open the same picker.
- **Back to the top level.** On a nested session, the picker offers Make top-level, as the row menu does.
- **Continued and forked sessions stay put.** Their picker explains that they stay linked to the session they came from instead of listing parents. The daemon's refusal now says the same.
