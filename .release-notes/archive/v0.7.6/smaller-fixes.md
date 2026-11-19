---
title: Composer answers to clarifications, faster Marketplace installs and more fixes
type: fix
---

- **Answer an agent's question from the composer.** When an agent is waiting on a clarification,
  a message typed in the session composer now answers that question and the turn continues. Before,
  the turn stayed waiting until the question timed out. While a question is pending, the composer
  reads "Type here to answer the agent's question". A message with attached files is refused and
  your draft is kept. A question that only accepts one of its listed choices refuses a typed answer
  with a reason instead of dropping it, and you can still answer it by choosing an option.
- **Marketplace installs during refresh.** Opening an install preview no longer waits behind a
  catalog source refresh. Previously a preview could take minutes while sources refreshed.
- **Claude Code plugins in Marketplace.** Marketplace now installs plugins whose
  `.claude-plugin/plugin.json` uses the official Claude Code schema; the installer used to reject
  them. A package with an invalid manifest is now marked as not installable, and its detail page and
  install preview show why.
- **Retryable refusal when MCP tools fail to load.** If a session's MCP tools time out or fail to
  load while a message starts or restarts its runtime, the message is refused as not sent instead of
  failing with a server error. The composer restores your draft and shows "Message not sent", so you
  can send it again.
