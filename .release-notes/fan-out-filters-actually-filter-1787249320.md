---
title: Fan-out filters actually filter
type: fix
---

A `filter` on a fan-out node was accepted at authoring time but never applied, so batching and `max_fan_out` still saw the whole candidate list. Each filter is now evaluated against the raw candidate before batching and branch limits. (#438)

- The candidate `item`, its original `index`, the fan-out alias, and outer fan-out aliases are all available during compilation and evaluation.
- Original order and candidate indexes survive filtering.
- Zero matches is a valid zero-branch materialization, not an error.
- A predicate failure routes through the existing `on_eval_error` policy instead of being silently ignored.

```yaml
- id: inspect_files
  class: control
  kind: fan-out
  collection: "{{ .nodes.changed.output.files }}"
  filter: "item.endsWith('.go')" # now decides what gets batched
  batch_size: 1
  max_fan_out: 8
```
