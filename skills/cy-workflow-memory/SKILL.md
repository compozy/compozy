---
name: cy-workflow-memory
description: Maintains workflow-scoped task memory for Compozy runs using .compozy/tasks/<name>/memory/ files. Use when a task prompt provides workflow memory paths and requires the agent to read, update, compact, and promote durable context across PRD task executions. Do not use for PR review remediation, global user preferences, or programmatic event-log summarization.
---

# Workflow Memory

Maintain the workflow memory files provided by the caller.

## Required Inputs

- Workflow memory directory path.
- Shared workflow memory file path.
- Current task memory file path.
- Optional caller signal indicating whether either file must be compacted before proceeding.

## Workflow

1. Load the memory state before editing code.
   - Read the shared workflow memory file and the current task memory file before making any code change.
   - Treat these files as mandatory context for the run, not optional notes.
   - If the caller marks either file for compaction, read `references/memory-guidelines.md` and compact that file before proceeding with implementation.

2. Keep memory current while the task runs.
   - Update the current task memory whenever the objective changes, a non-obvious decision is made, an important learning appears, or an error changes the plan.
   - Promote only durable cross-task context into the shared workflow memory.
   - Keep task-local execution details in the current task memory file.

3. Close out the run cleanly.
   - Update memory before any completion claim, handoff, or commit.
   - Record only facts that help the next run start faster and with fewer mistakes.
   - Re-read `references/memory-guidelines.md` before compacting if the file has grown noisy or repetitive.

## Critical Rules

- Do not invent history, decisions, or status that did not happen.
- Do not copy large code blocks, stack traces, or task specs into memory files.
- Do not duplicate facts that are obvious from the repository, git diff, task file, or PRD documents.
- Do not read unrelated task memory files unless the shared workflow memory or the caller explicitly points to them.
- Keep shared memory durable and cross-task. Keep task memory local and operational.

## Promotion Decision Test

Before promoting an item from task memory to shared workflow memory, ask:

1. Will another task need this information to avoid a mistake or rediscovery?
2. Is this fact durable across multiple runs, not just the current execution?
3. Is this information NOT already obvious from the PRD, techspec, task files, or the repository itself?

All three must be "yes" to promote. If any is "no," the item stays in task memory.

**Examples that belong in shared workflow memory:**
- A discovered constraint that affects multiple tasks (e.g., "the API rate limits to 100 req/s, batch operations must respect this")
- A cross-cutting architectural decision made during implementation (e.g., "chose channel-based coordination over mutex for the pipeline")
- An open risk that future tasks must account for (e.g., "migration depends on schema v3 which is not yet deployed to staging")

**Examples that stay in task memory:**
- Files touched during this task's implementation
- Debugging steps taken to resolve a task-specific error
- The current task's objective and acceptance criteria snapshot
- A workaround applied only to the current task's scope

## Compaction Rules

When the caller flags a memory file for compaction, apply these rules inline. Read `references/memory-guidelines.md` for full detail, but these rules are sufficient for most compaction passes:

1. If both files need compaction, compact the shared workflow memory first, then compact the task memory. The shared file sets the cross-task context that the task file should not duplicate.
2. **Preserve:** current state, durable decisions, reusable learnings, open risks, and handoff notes.
3. **Remove:** repetition, stale notes, long command transcripts, and facts already derivable from the repository, PRD, or task files.
4. **Rewrite** retained items as short factual bullets. Do not preserve narrative logs or chronological play-by-play.
5. Keep the default section headings from the template intact. Remove empty sections only if they are truly unused.

## When To Read the Reference

Read `references/memory-guidelines.md` when any of these apply:

- the caller requests compaction and the inline rules above do not cover the situation
- it is unclear what belongs in shared memory versus task memory
- the current memory file has drifted into noisy notes or redundant detail

## Error Handling

- If any caller-provided memory path is missing, stop and report the mismatch instead of guessing another path.
- If memory content conflicts with the repository or task specification, trust the repository and task documents, then correct the memory file.
- If compaction would remove active risks, decisions, or handoff context, keep those items and remove lower-value repetition first.
