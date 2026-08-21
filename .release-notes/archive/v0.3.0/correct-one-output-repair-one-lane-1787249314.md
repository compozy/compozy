---
title: Correct one output, repair one lane
type: feature
---

Operators can fix a settled node output without rewriting what actually happened, and can act on a single fan-out cell without disturbing its siblings. (#427)

- **Amend output** applies to a settled output while its run, node, or cell is parked, and appears only when the node declares an output shape to validate against. It shows the recorded original read-only beside the corrected value and takes a reason.
- Amendments are append-only: the recorded output is never rewritten, the corrected value becomes what resume and downstream reads see, and both stay visible in history and in a diff. Amending does not re-run consumers — pair it with **Rerun from here**.
- Run detail returns `amendments[]` with bounded, redacted values, or a byte-size and content-hash summary for large data. No API reads an amendment's private output reference directly.
- The control is available as `compozy loop node amend`, `POST /loop-runs/:id/nodes/:node/amend`, and `compozy__loop_node_amend`.
- `--item` (or `item_index`) pauses, resumes, cancels, or kills one fan-out cell without touching the rest of the window.

```bash
compozy loop node amend --run-id <run-id> --node build --item 3 \
  --payload '{"artifact":"dist/app-1.4.2.tgz"}' --reason "wrong tag captured"
```
