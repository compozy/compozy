# Workflow Memory Guidelines

Use these rules to keep Compozy workflow memory useful across repeated PRD task runs.

## File Roles

### Shared workflow memory: `MEMORY.md`

Use the shared workflow memory for context that should survive across multiple tasks and multiple runs.

Keep:
- current workflow state that affects more than one task
- durable technical or product decisions
- reusable learnings that will matter again
- open risks or handoff notes that change future execution

Avoid:
- step-by-step scratch notes
- large code excerpts
- facts that are already explicit in `_prd.md`, `_techspec.md`, `_tasks.md`, or the repo itself

### Current task memory: `memory/<task filename>`

Use the task memory for context that is specific to the current task.

Keep:
- the current objective snapshot
- important task-local decisions
- local learnings and corrections
- touched files or surfaces worth remembering next run
- ready-for-next-run notes

Avoid:
- cross-task summaries that belong in `MEMORY.md`
- repeated restatements of the task spec
- low-signal command transcripts

## Promotion Rules

Promote an item from task memory into shared workflow memory only when it is:
- durable across runs
- useful to another task
- likely to prevent repeated mistakes or rediscovery

Leave information in task memory when it is:
- operational only for the current task
- temporary
- too detailed for workflow-wide reuse

## Compaction Rules

When compaction is required:
- preserve current state, durable decisions, reusable learnings, open risks, and handoffs
- remove repetition, stale notes, long transcripts, and derivable facts
- rewrite for clarity, not for completeness
- prefer short factual bullets over narrative logs

## Default Section Boundaries

### `MEMORY.md`

- `## Current State`
- `## Shared Decisions`
- `## Shared Learnings`
- `## Open Risks`
- `## Handoffs`

### `memory/<task filename>`

- `## Objective Snapshot`
- `## Important Decisions`
- `## Learnings`
- `## Files / Surfaces`
- `## Errors / Corrections`
- `## Ready for Next Run`
