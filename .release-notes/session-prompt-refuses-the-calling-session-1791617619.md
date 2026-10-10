---
title: An agent can no longer send a session message to its own session
type: breaking
---

`compozy__session_prompt` now refuses a call whose `session_id` is the calling session. The call fails with `invalid_request` ("session_prompt cannot target the calling session.") and nothing is admitted. A self-prompt could only queue into, steer into, or interrupt the caller's own turn, so it never did useful work. Now that every session message carries its sender and can request a reply, it would also make a session reply to itself.

- Only calls made from an agent session to that same session are refused. Messages to other sessions, operator prompts from the CLI and HTTP, and `compozy session prompt` behave as before.
- A send admitted before the upgrade and retried with the same `message_id` and `idempotency_key` still returns its original result.

Migration notes: this ships without the usual one-release deprecation window. The exception is recorded in the agent-collaboration spec (Business Rule 18, ADR-002). To keep working in the same session, end the turn. To schedule work, use a subagent (`compozy__subagent_delegate`) or `compozy session prompt` from the operator.
