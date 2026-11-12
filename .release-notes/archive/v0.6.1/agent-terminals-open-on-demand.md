---
title: Agent terminals no longer leave stale windows behind
type: fix
---

Terminals that agents open now stay in the **Terminal** list until you open one, instead of opening a desktop window each time. Before, every agent terminal created a window that outlived the process, so desktops filled with windows that restored as "This terminal isn't here" and the Terminal dock item could show a terminal as open when none was running.

When an agent's terminal exits, or any terminal is cleaned up, its windows close on every desktop, and Undo or Reopen cannot bring them back. Terminal windows left over from before a daemon restart are removed at startup. Terminal output history and recordings are kept.

Migration notes: no action needed. Open an agent's terminal from the Terminal list or from the link in the transcript.
