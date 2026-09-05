---
title: A fan-out roster shows only the workers that exist
type: fix
---

Fan-out roster projection treated the highest stored item index as a contiguous range, so a run holding only item `2`, or items `2` and `5`, invented rows for the missing indexes and left them pending forever. Roster rows, rollups, and run progress now project the exact stored item indexes. (#518)

- Response shapes and routes are unchanged across CLI, HTTP, UDS, `compozy__loop_runs`, and Web; the same reads now return correct results.
- The Loop running guide and the official CompozyOS skill document sparse-index behavior, and clarify that `not_taken` requires durable route evidence.
