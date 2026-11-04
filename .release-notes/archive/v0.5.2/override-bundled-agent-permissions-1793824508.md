---
title: Change a bundled agent's permissions without copying it
type: feature
---

You no longer need to copy a bundled or extension agent's whole `AGENT.md` just to run it with different permissions or a different model. Override only the fields you need in `config.toml`:

```toml
[agents.overrides.orchestrator]
permissions = "approve-all"
# provider, model, reasoning_effort and speed are optional
```

The override applies to whichever definition of that agent is in effect: workspace, global, extension or built-in. Everything you don't set, including the prompt, appearance, tools, skills and MCP servers, keeps following the original, so package updates still reach you. Agents can't change this setting themselves.
