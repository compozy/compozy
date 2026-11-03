---
title: Switching sessions stays responsive while others stream
type: fix
---

With several sessions streaming, switching to another one could leave the previous conversation on screen, and still interactive, for more than five seconds. The new session or its loading state now appears as soon as you select it.

The dashboard, Loop, task and worktree views also make far fewer repeat reads when many events arrive at once, the daemon does less repeated work on session, scheduler and worktree lists, and a closed About dialog no longer polls full status. These results come from controlled tests, so your gain depends on your load.
