---
title: Loops take one path and tell you why
type: feature
---

A Loop graph can now pick exactly one forward path with a `route` control node, and a gate verdict can route the same way, instead of forcing authors to encode every choice as nested branches. Each decision is recorded durably, so an operator or an agent reads why a run went the way it did rather than inferring it. (#427)

- A `route` node checks its CEL conditions in declaration order and takes the first match, falling back to a mandatory `default`. Every destination must be a unique direct forward edge.
- A broken condition fails closed with `predicate_evaluation_failed`; it never falls through to the default.
- Gate verdicts (`pass`, `fail`, `error`, `timeout`, `invalid_output`) route to `continue`, `revise`, `next_generation`, `escalate`, `halt`, or an in-body forward target written as `{ route: node_id }`. `approval` accepts only `escalate` or `halt`, so an object route cannot slip past a pending approval.
- Run status carries `generations[].route_causes` — the route node or gate, the selected forward node, the lane index, the cause, and the time. It is read from the durable `route_taken` event, so HTTP, CLI, native-tool status, and SSE replay agree.

Migration notes: the `branch` gate action is removed and is now rejected at authoring time.

```yaml
- id: classify
  class: control
  kind: route
  routes:
    - { when: nodes.score.output.value >= 0.8, to: publish }
    - { when: nodes.score.output.value >= 0.5, to: revise }
  default: reject
```
