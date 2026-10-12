---
title: Attribute messages between sessions
type: feat
---

Messages sent through `compozy__session_prompt` now retain their sender across queued delivery, steering, interrupts, and daemon restarts. Agents receive a sender header; stored message text stays unchanged. Queued messages from another session can be canceled but cannot be edited or promoted by the operator.

Session message chains stop at eight hops. `session_prompt` now rejects calls targeting the calling session with `invalid_request`; continue in the current turn instead. This safety correction applies immediately.

The database upgrade adds nullable origin fields without rewriting existing messages. Pre-upgrade prompt receipts replay without an origin or reply watch through the `session-prompt/v4` compatibility shim, scheduled for removal in v0.5.0.
