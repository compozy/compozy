---
provider: manual
pr:
round: 5
round_created_at: 2026-07-22T21:45:58Z
status: resolved
file: web/src/systems/app-shell/hooks/use-workspace-events.ts
line: 172
severity: medium
author: claude-code
provider_ref:
---

# Issue 003: Task Group plan changes leave the spec view stale

## Review Comment

`shouldInvalidateSpec` recognizes `_prd.md`, `_techspec.md`, and ADR paths but omits `_task_groups.md`. The daemon watcher does treat `_task_groups.md` as relevant and emits an artifact-change event after syncing it, while `WorkflowSpec` renders the selected Task Group's `plan_excerpt` from that plan. Because the artifact event does not invalidate the spec query, an open Task Group spec keeps stale title, outcome, dependencies, and completion state until an unrelated full refresh occurs.

Include `_task_groups.md` in spec invalidation and add an event-hook test using a composite `<initiative>/TG-NNN` query key. Verify the selected child spec refetches while unrelated workflows remain scoped correctly.

## Triage

- Decision: `VALID`
- Notes: `artifact.changed` delegates spec invalidation to `shouldInvalidateSpec`, which
  omits `_task_groups.md` even though the daemon watcher emits that path and the selected
  Task Group spec reads its `plan_excerpt` from the initiative plan. Add the missing path
  classification and cover the event-hook path with a composite `demo/TG-NNN` reference,
  asserting that the selected child spec invalidates without invalidating initiative or
  sibling Task Group specs.
