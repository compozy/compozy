---
title: Fan-out settles with an honest count
type: feature
---

A fan-out can declare how it settles, and a partial result stays partial everywhere it is read instead of being rounded up to success or down to failure. (#427)

- `strategy` accepts `wait_all` (the default), `fail_fast`, `race`, and `best_effort`. `best_effort` requires both a threshold — a percentage like `66%` or a count like `{ count: 2 }` — and an explicit `missing: acceptable`.
- A collect result is `succeeded`, `partial`, or `failed`, and its output carries `total`, `succeeded`, `failed`, `canceled`, `coverage_rate`, and `partial`.
- Live counts read through `nodes.<fan-out-id>.progress.*` — `total`, `succeeded`, `failed`, `canceled`, `running`, `pending`, `settled`, `success_rate`, `failure_rate` — with the short `progress.*` form inside the fan-out body. Rates are `0` for an empty collection.
- The run page separates lanes that succeeded, lanes that failed, lanes the strategy canceled, and lanes that never materialized because the window did not open them. Partiality is a run-level fact (`completion_state`), so it reads `partial` in the outcome card, the run lists, and a diff. A wide fan-out reports aggregate counts instead of one row per lane.
- The fan-out window has no daemon-wide ceiling; logical width stays bounded by each node's positive `max_fan_out`. Write-time validation rejects a negative `fan_out_width`.

```yaml
- id: inspect_files
  class: control
  kind: fan-out
  collection: "{{ .nodes.changed.output.files }}"
  bind_as: file
  strategy:
    kind: best_effort
    threshold: 66%
    missing: acceptable
```
