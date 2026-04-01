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

## When To Read the Reference

Read `references/memory-guidelines.md` when any of these apply:

- the caller requests compaction
- it is unclear what belongs in shared memory versus task memory
- the current memory file has drifted into noisy notes or redundant detail

## Error Handling

- If any caller-provided memory path is missing, stop and report the mismatch instead of guessing another path.
- If memory content conflicts with the repository or task specification, trust the repository and task documents, then correct the memory file.
- If compaction would remove active risks, decisions, or handoff context, keep those items and remove lower-value repetition first.
