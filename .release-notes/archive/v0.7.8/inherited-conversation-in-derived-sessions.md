---
title: Forked and continued sessions show the conversation they came from
type: fix
---

A forked or continued session used to show only a divider and "Nothing said here yet", so it looked
as if its messages were lost. It now shows a collapsed, read-only **Earlier conversation** row above
the divider, with the number of messages it carried. Nothing loads until you expand it. Expanding it
shows the newest 50 messages from the source up to the fork or continue point, and **Show earlier
messages** loads older ones. The row is hidden when nothing was carried over.

These messages belong to the source. You can't edit them or fork from them, and they are not part
of the session's search, outline, message counts or find results. The expanded panel says what the
agent received, that is, how many of the carried messages it gets, or that the agent continues
from its own copy of the conversation. If the source was deleted, archived, moved, or rewound or
cleared past that point, the panel says the source is no longer available and that the agent's
carried context is unaffected.

Nothing is copied into the new session, so forking and continuing cost the same as before.
`compozy session history` and `compozy__session_history` still return only the session's own
messages.
