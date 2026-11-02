---
title: Goals wait for external work instead of spending their turns
type: fix
---

A Goal could use up its turn limit while a monitor or subagent it started was still running. Now, while the session has a live monitor or subagent, the Goal shows **Waiting** and starts no new work, compaction or judge check. The turn that started the wait counts once, the wake consumes no Goal turn, and the Goal carries on when the wait ends. CompozyOS checks this itself, so an agent saying it is waiting isn't enough.

- A Goal waiting on a subagent survives a daemon restart, and picks up the result when the subagent finishes during the restart.
- A subagent run in wait mode hands its result straight to the caller, so it no longer holds a Goal in **Waiting**.
- **Pause** keeps working during a wait, and so do Stop and your own messages. Wall-clock budgets keep counting.
- A wait that lasts 25 hours pauses the Goal for your attention with `goal_wait_timeout`. If the wait can't be read, it pauses with `goal_wait_unavailable`. Check the monitor or subagent before you resume.
- A deliberate daemon stop pauses the Goal and stops its session. Resume the session first, then the Goal.

Migration notes: no action is needed, and existing Goals keep their progress. `waiting` is a new Goal status in HTTP, UDS, CLI output and the Goal tools, so update any client that switches on status.
