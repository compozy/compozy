---
title: The idle-session setting now does what it says
type: fix
---

Settings → General had an "End idle sessions after" control that did nothing: it wrote `session.limits.timeout`, which no session enforced, so no session ever ended. The row is now **Pause idle agents after**. It controls the idle runtime release added for #782: when a session has been idle that long, CompozyOS frees its agent process. The session stays open, and your next message picks up where it left off. Agents that can't resume a conversation keep running.

- The choices are Never, 15 minutes, 30 minutes (the default), 1 hour, 4 hours, and 1 day. A different value set in `config.toml` is shown as it is. Changing it requires a daemon restart, like other `session.*` keys.
- The general settings HTTP/UDS DTO gains `config.idle_release_after`, which maps to `session.supervision.idle_release_after`. Leave it out of an update to keep the current value.
- `session.limits.timeout` and the DTO field `config.session_timeout` are deprecated and still enforce nothing. Both stay readable and writable in this release, with the same meaning as before. A non-zero `session.limits.timeout` logs a deprecation warning when config loads, and a PATCH that sends a non-zero `session_timeout` returns a `Warning` header naming `idle_release_after`. Both are removed in v0.5.0.

Migration notes: if you set `session.limits.timeout` to limit how long sessions run, it never had an effect. Remove it, and use `session.supervision.idle_release_after` to free idle agents. Scripts that PATCH `/api/settings/general` should send `idle_release_after` before v0.5.0 removes `session_timeout`.
