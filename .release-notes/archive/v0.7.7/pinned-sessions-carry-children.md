---
title: Pinned sessions keep their children and your order
type: feature
---

Pinning a session now carries its child sessions with it. In **Pinned**, the children sit nested
under the pinned session at every depth, and they leave their project group and folder while the
pin lasts, so nothing shows twice and group and folder counts leave them out. **Unpin** returns the
session and its children to the places they held. Forks, and continuations of the pinned session
itself, are new top-level work and stay where they are.

You can now order Pinned yourself. A new pin goes on top; drag a pin up or down, or use **Move up**
and **Move down** in its **Move** menu. The order is the same after a reload and in every window.
Reordering works where the list shows one project and profile, and is off while you search. From
the command line, `compozy session reorder <id>... --pinned` sets the order and
`compozy__session_reorder` takes `pinned: true`.

`compozy session list --pinned only|exclude`, the sessions API `pinned` filter, and
`compozy__session_list` now include or leave out the sessions a pin carries along with the pin.
`sort=pinned` follows your Pinned order and pages by pin: each page holds whole subtrees, and its
total counts pins.

## Migration notes

Existing pins keep their order on upgrade: the oldest pin stays first, and new pins go above them.
A `sort=pinned` cursor from a previous version is refused once; read the first page again.
