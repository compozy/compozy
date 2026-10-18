---
title: Idle sessions no longer warn or stop
type: fix
---

A session whose agent had finished and was simply waiting for you used to show a large yellow "This session stops in …" notice after 30 minutes and then turn into a stopped session. Quiet supervision is now a watchdog for turns that hang, the way other agent harnesses handle it: a session waiting for you is never warned about or stopped. (fixes #782)

- `session.supervision.quiet_after` and `stop_grace` now only count silence while a turn is in flight and waiting on the agent. A turn waiting on your answer to a permission or clarify request does not count. A hung turn still warns and then stops with the `inactivity` cause, so task recovery, Loop recovery and reply wakes behave as before.
- New `session.supervision.idle_release_after` (default `30m`, `0` disables) frees the agent process of a session that has been idle that long, but only when the agent supports ACP `session/load`. The session stays active and idle in every list, and your next message reloads the same conversation. If the agent can no longer load it, CompozyOS starts a fresh agent session and replays the recent transcript.
- A released runtime records no stop and fires no `session.post_stop` hook. The process-level `agent.stopped` hook still fires, as it does when a runtime is replaced.
- The quiet-warning notice and its "Stop now" button are gone. A turn that has gone quiet now shows "quiet for 12m" on the working status line, and Stop stays where it was.
