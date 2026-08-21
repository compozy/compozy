---
title: A burst of tool updates no longer drops the provider
type: fix
---

An ACP provider can emit hundreds of state-equivalent updates for a single tool call. Those duplicates filled the active prompt's bounded event channel, stalled delivery, and disconnected an otherwise healthy provider. CompozyOS now keeps one canonical projection per tool call for the duration of the prompt. (#442, fixes #439)

- Only redundant nonterminal updates are suppressed. A new title, name, kind, input, or prechecked state still comes through, and terminal results and prompt completion keep their order.
- The projection is prompt-scoped and keyed by the current `tool_call_id`; it is discarded when the prompt ends and never enters a session, workspace, or global store.
- Public event shapes are unchanged — nothing about the session transcript contract moved.
- Verified against 1,100 identical in-progress updates followed by a terminal one: both the original prompt and a follow-up completed with a single call/result pair and no disconnect.
