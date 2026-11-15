---
title: Automation backoff retries unaccepted provider startup failures
type: fix
---

`retry.strategy = "backoff"` again retries ACP startup, process-exit, handshake, and protocol
failures before session acceptance, including a crash at spawn, package preparation failure, or
`session/new` error. The existing retry count and delay apply, including to repeated typed startup
configuration failures. A provider that already accepted the session is never started again by a
startup retry. Cancellation, authentication failures, errors without an ACP startup classification
(such as agent-definition validation), and cleanup that retains ownership stay terminal. Exhausted
retries preserve the final failed session and diagnostics.
