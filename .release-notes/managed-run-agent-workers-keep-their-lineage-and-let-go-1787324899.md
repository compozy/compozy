---
title: Managed run-agent workers keep their lineage and let go
type: fix
---

Two lifecycle bugs in Loop `run-agent` actions, both reproduced against `v0.3.0-beta.18`: a managed worker lost the trail back to the session that started it, and a worker could outlive the Loop cell it belonged to. (#446, fixes #444 and #445)

- A managed worker now records the nearest originating session as informational parent lineage — parent and root are readable from the session — without borrowing or hijacking that origin session.
- When a Loop cell settles successfully, the run-owned worker binding closes and durable terminal cleanup is enqueued in the same atomic step. Cancellation and terminal failure follow the same path, and cleanup cannot run twice.
- A retryable output failure keeps the same worker session active instead of orphaning it, so a retry reuses the worker and only terminal settlement ends it.
- No public API, schema, migration, or config key changed; existing Loop and session reads simply expose corrected stored state.
