---
title: Preserve successful SQLite commits during cancellation
type: fix
---

A request or persistence deadline arriving during a successful SQLite commit no longer reports that committed write as failed. This prevents persistence retries from duplicating completed tool events. Cancellation observed before commit still rolls back the write.
