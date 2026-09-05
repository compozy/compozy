---
name: cy-workflow-memory
description: Maintains workflow-scoped task memory for CompozyOS runs using .compozy/tasks/<name>/memory/ files. Use when a task prompt provides workflow memory paths and requires the agent to read, update, compact, and promote durable context across spec task executions. Do not use for PR review remediation, global user preferences, or programmatic event-log summarization.
---

# Workflow Memory

Use the caller-provided shared/current-task memory paths for a continuing workflow. Read them on entry or after lost context; reuse them while current and do not scan unrelated task memories.

Update only when the goal, meaningful decision, blocker, or handoff evidence changes. A new message, status report, or commit alone does not require a rewrite. Keep task-local details in the task file and promote only durable facts another task needs that are not already clear from the spec or repository.

For compaction, preserve current state, decisions, risks, and handoff pointers; remove stale/repeated notes and transcripts. Read `references/memory-guidelines.md` only when its format or promotion rules are needed. Existing schema headings remain when the caller depends on them.

If a supplied path is missing, inspect the caller's documented initialization contract before reporting a concrete blocker; never guess another workflow's path. Repository contracts and current user decisions override stale notes. A short task without a supplied continuity requirement does not need a new memory subsystem.
