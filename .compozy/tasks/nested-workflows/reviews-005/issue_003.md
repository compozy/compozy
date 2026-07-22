---
provider: manual
pr:
round: 5
round_created_at: 2026-07-22T21:45:58Z
status: pending
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

- Decision: `UNREVIEWED`
- Notes:
