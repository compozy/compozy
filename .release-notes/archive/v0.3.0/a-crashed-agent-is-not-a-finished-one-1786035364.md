---
title: A crashed agent no longer looks like a finished one
type: fix
---

When an agent process disconnected mid-answer, the stream simply ended — and everything downstream read that silence as success. A CLI consumer reached end of file and exited zero, `compozy__session_prompt` returned a result, and the only evidence left behind was stderr with no exit code. Streams are now fail-closed: success requires an explicit completion event, and disconnect, terminal error, and process exit stay three distinct outcomes. (#315, #319)

- Chunks already received stay persisted and visible. CompozyOS never synthesizes a completion for them.
- A stream that ends after partial output without a completion event fails the CLI with a clear non-zero exit, and terminal error frames are forwarded before the error is returned so machine-readable diagnostics survive.
- `compozy__session_prompt` classifies a subprocess exit as `tool_backend_failed` with `backend_dead` instead of reporting success; the partial events remain readable in the session transcript.
- Crash evidence now carries the subprocess exit code and, where the operating system exposes it, the terminating signal.
- Fatal cleanup gives the process a bounded grace period to exit on its own before being stopped, so the real exit result is no longer lost to a race with forced teardown.
- CompozyOS does not replay a prompt automatically, because a prompt may already have caused external side effects. Sending the next prompt restarts the agent process and continues the same session and transcript.

Migration notes: crash bundles move to `compozy.session_crash_bundle.v2` with structured `exit_code` and `signal`, with no v1 branch. Any consumer that treated a closed stream as success will now correctly see a failure unless a completion event was sent.
