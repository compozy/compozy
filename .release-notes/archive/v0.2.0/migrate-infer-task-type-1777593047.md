---
title: Migrate now infers task type for legacy workflows
type: fix
---

`compozy migrate` no longer emits `type: ""` for legacy `feature` / `feature implementation` tasks, which previously broke `compozy sync` on the migrated workflow. A valid v2 task type is now inferred from the legacy type, with `domain` used as a constrained fallback only when the direct remap is genuinely ambiguous. The unmapped-type follow-up prompt is now emitted only when inference is unsafe.

This release also tightens API error reporting: validation/parse failures from the daemon HTTP API now return `422 Unprocessable Entity` with cleaner messages instead of generic `500`, and the API core preserves original error identity so callers using `errors.Is` / `errors.As` get consistent results.

### Before / after

```bash
# Before — produced workflow with empty `type`, then failed on sync.
compozy migrate
compozy sync   # error: missing/invalid task type

# After — migrated workflow has a valid inferred type; sync succeeds.
compozy migrate
compozy sync   # ok
```
