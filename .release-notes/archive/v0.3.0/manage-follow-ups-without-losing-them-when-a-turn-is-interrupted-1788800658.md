---
title: Manage follow-ups without losing them when a turn is interrupted
type: feature
---

Queued follow-ups survive interruption of the active turn, including agent- and Goal-owned entries. Edit, remove, steer, or explicitly clear the queue from the session controls. Clearing writes durable actor-attributed records and is also available through `compozy session input clear` and the corresponding HTTP/UDS and native-tool surfaces.

Send identities make retries idempotent: retrying the same message returns its recorded outcome, while reusing the identity with different content is rejected. When an acknowledgment is lost, the client shows Not confirmed and retries the original identity. Daemon-generated follow-ups now use the same durable queue.

PR: [#557](https://github.com/compozy/compozy/pull/557).
