---
title: Keep subagent completion wakes flowing after deletion
type: fix
---

Deleting the only result in a queued subagent completion now cancels the empty wake completely, so later subagents can still wake their parent.
