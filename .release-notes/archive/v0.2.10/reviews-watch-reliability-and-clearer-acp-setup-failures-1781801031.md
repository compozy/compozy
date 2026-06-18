---
title: Reviews watch reliability and clearer ACP setup failures
type: fix
---

`compozy reviews watch` could fail a remediation round on a transient agent startup stall, and the resulting error gave little to go on. This release fixes the reliability gap and makes ACP session-setup failures diagnosable across every command, not just reviews watch.

### What was wrong

- A slow ACP session setup (creating, loading, or setting the mode of a session) that hit the inactivity timeout was treated as a hard, non-retryable failure — even though an inactivity stall is a transport/runtime hiccup, not a protocol rejection.
- Setup failures surfaced as opaque errors: no launch command, no agent stderr, and no indication when the real cause was a context cancellation (e.g. `Ctrl+C` or a parent timeout).
- Reviews watch child runs did not consistently honor the project's configured retry policy.

### What changed

- **Setup-stage inactivity timeouts are now retryable.** A stalled session create/load/set-mode is handled as a timeout and retried, instead of failing the job outright.
- **Richer setup diagnostics.** When ACP session setup fails, the error now includes the launch command, the agent's stderr, and — when the context was cancelled — the underlying cancellation cause joined into the error chain. This applies to all ACP runtimes.
- **Child retries honor project config.** Reviews-watch child runs now pass through whether the project configured `max_retries`, so the watch loop respects the workspace retry policy rather than guessing.
