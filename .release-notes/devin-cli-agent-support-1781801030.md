---
title: Devin CLI agent support
type: feature
---

Compozy now supports [Devin CLI](https://devin.ai/cli) as a first-class ACP execution runtime, alongside Claude Code, Codex, Copilot, Cursor, Droid, OpenCode, and the others.

### Usage

Install Devin CLI and expose `devin` on your `PATH`, then select it like any other runtime:

```bash
compozy tasks run my-feature --ide devin
```

Compozy launches it via `devin acp`. Skill installation (`compozy setup`) and the runtime registry both recognize `devin`, so it shows up in agent detection and the setup catalog.

### Notes

- Devin CLI resolves its own model, reasoning, and access defaults, so Compozy does not pass model/reasoning/access bootstrap flags to it.
- The default model recorded for the runtime is `anthropic/claude-opus-4-6`.
