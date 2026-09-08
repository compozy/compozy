---
title: Reconnect and read long sessions with a truthful live view
type: feature
---

Session streams recover from their last delivered position, including an empty catch-up acknowledgment when already current. Brief interruptions stay quiet; repeated failures show Reconnecting, then Disconnected with Try again. Sending while disconnected keeps the draft. Compaction and retention recovery preserve a consistent active turn and replace expired history windows explicitly.

Completed tool work and settled turns fold into readable summaries, interrupted work stays open, and Thinking is explicit. Reading older history or expanding work no longer competes with live scrolling. Large payloads have Show all and Download controls; stop and steering provenance remain visible on the relevant messages.

Bounded agent event ingestion and durable-history recovery prevent a slow watcher from wedging the producer or silently losing accepted output. Streaming redaction was optimized while preserving its output.

PR: [#557](https://github.com/compozy/compozy/pull/557).
