---
title: Restart a stopped session and keep its runtime
type: fix
---

A stopped session used to be a dead end: the UI went read-only and the only way forward was creating a new one. Sending a normal prompt to a stopped session now restarts its agent process, reloads the retained provider history, and continues under the same session ID and transcript. The provider, model, reasoning effort, and speed you picked are stored on the session itself, so they survive a stop and a daemon restart instead of silently reverting to the default. (#307)

- The lifecycle gained a `starting` state, and a normal prompt is the only operation that moves a stopped session back toward execution. `session resume` stays attach-only, and queue, steer, interrupt, and attach do not restart a session.
- `compozy session runtime set <id>` takes `--provider`, `--model`, `--reasoning-effort`, and `--speed`, and `compozy session runtime clear <id>` drops the choice. Both fence on `--expected-revision` and report a conflict on a stale one. Agents get `compozy__session_runtime_set` and `compozy__session_runtime_clear`; extensions get `sessions/runtime/set` and `sessions/runtime/clear` under `session.write`.
- Session reads expose `runtime.selected`, `runtime.effective`, and `runtime.selection_revision`. A prompt resolves its runtime from an explicit snapshot first, then the stored selection, then the current effective values, and an already-queued prompt keeps the snapshot it was accepted with.
- The composer stays enabled for a stopped session, and closing a session window during a live turn no longer breaks the transcript view.

Migration notes: the `Use as Goal` action on settled assistant messages is removed. `/goal` is the single entry point for Goals.
