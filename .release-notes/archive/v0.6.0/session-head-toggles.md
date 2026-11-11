---
title: The session list toggle leads the session header
type: fix
---

The button that shows or hides the session list now sits first in the session header, before the project name, in every session window. Before, it sat among the actions on the right, and in an always-on session it disappeared once the list was hidden, leaving no way to bring the list back from the header.

The context panel now opens and closes from the context button in the message box, which works as a toggle. A regular session's header actions are **Stop** and **Changes**. A read-only superseded session keeps the header's context toggle because it has no message box. An always-on agent's main session keeps **Always-on**, the **…** menu, and toggles for the **Changes** and context panels while each panel is open.

Migration notes: no action needed; keyboard shortcuts and command palette entries for these panels are unchanged.
