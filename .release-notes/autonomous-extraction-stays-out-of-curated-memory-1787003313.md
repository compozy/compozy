---
title: Autonomous extraction stays out of curated memory
type: fix
---

The autonomous memory extractor could write operational chatter into curated memory, and a generated slug collision could overwrite an unrelated entry. The deterministic scanner now rejects Memory v2 operational identifiers — `memory_propose`, native `compozy__memory_*` tool names, controller event names, and scanner rule IDs — and extractor, provider, and dreaming candidates no longer update an existing memory solely because their generated slug collides. Explicit filename-collision updates from direct CLI or user writes keep working. (#396)
