---
title: Feedback semantics for durable Loops
type: feature
---

A rejected Loop generation no longer restarts blind. The rejection is carried into the next attempt as context, only the producers responsible for it are re-run, and an opt-in ratchet keeps the best-scoring generation instead of losing it to a later regression. Every generation now records its origin, its parent, the gate verdict, the score, and the blocking issues inside claim-fenced transactions, so the CLI, HTTP, UDS, native tools, SSE, and the web UI all read the same durable run truth. (#290)

- Loop templates can read `previous.*` (including `previous.generation` and `previous.route_causes`) and `best.*` to steer the next attempt from what actually failed.
- Metric gates take a direction — `maximize` or `minimize` — plus a `min_delta` improvement threshold, so a regression is rejected deterministically. Invalid thresholds fail authoring with `metric_min_delta_invalid`.
- `compozy__loop_status` and `compozy__loop_runs` project score, best generation, gate verdict, and generation origin and parentage; run detail, catalog, and recent-runs views render the same fields.
- `compozy extension list` and `compozy extension status` accept `--workspace`, so agents can inspect workspace dev overlays without dropping to raw HTTP.

Migration notes: this is a greenfield hard cut that discards existing Loop run history. The migration clears Loop runs, run events, gate decisions, generation outputs, goal turns and checkpoints, session bindings, and output blobs, along with the task and automation runs that referenced them. Export anything you need before upgrading.
