---
title: One task-delivery Loop with two execution modes
type: breaking
---

`implement-tasks` now owns both task-delivery paths. Its default `per-task` mode keeps one isolated `code_implementer` session per task; `mode=orchestrated` uses the bundled `orchestrator` agent to spawn, prompt, verify, and stop one bounded worker per task.

- Four optional runtime inputs choose the conductor, backend workers, frontend workers, and every other worker. Task-frontmatter runtime fields still win over these run inputs.
- `compozy spawn` now accepts provider, model, reasoning-effort, and speed overrides, so orchestrated workers preserve the complete runtime choice.
- Goal output contracts now require the runtime's `complete|blocked` vocabulary, and Goal prompts receive the authored output schema.
- The standalone `orchestrate-tasks` Loop and its docs/catalog entry are removed. Operator-side `[loops.inputs.orchestrate-tasks]` config blocks are now inert and should be deleted; move any desired values under `[loops.inputs.implement-tasks]` and set `mode = "orchestrated"`.
