---
title: Orchestrated task delivery uses the implementer you picked
type: fix
---

`implement-tasks` running in `mode=orchestrated` replaced your selected `implementer` Agent with `code_implementer`. The typed Agent input now flows through the orchestrated objective, the conductor skill, and `compozy spawn`, so every worker runs as the Agent you chose and keeps its identity, Agent-local Skills, permissions, provider defaults, and category runtime overrides. (#502)

- `code_implementer` is still the default, so omitting the input behaves as before.
- A recovered session whose Agent does not match fails closed instead of being adopted silently.
- Settlement still requires completed Task frontmatter and zero live workers created by the conductor.
