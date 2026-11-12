---
title: Long transcripts stay steady when a turn starts
type: fix
---

In a long session, the transcript no longer flickers or stutters when the first tool call of a turn appears. Before, once a session held more than a page of history, each new entry made the app treat the whole loaded transcript as changed and redraw every message. Messages that did not change now keep their place and are not redrawn.

Migration notes: no action needed.
