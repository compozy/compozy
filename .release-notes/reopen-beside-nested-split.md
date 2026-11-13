---
title: Reopening a window closed beside a nested split restores the whole branch
type: fix
---

Reopen closed tab (⌘⇧T) now brings back a window that was closed beside a nested split in its
original slot and share: with A on the left and B over C on the right, closing and reopening A
returns it as the left column beside the B/C branch instead of squeezing it next to B. When the
branch's windows no longer sit together, the window returns beside its former neighbor as before.

Migration notes: no action needed. Windows closed before this release reopen with the previous
single-neighbor behavior.
