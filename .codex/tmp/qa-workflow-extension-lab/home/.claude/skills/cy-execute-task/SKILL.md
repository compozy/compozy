---
name: cy-execute-task
description: Executes one PRD task end-to-end using a provided task file, PRD directory, tracking file paths, and auto-commit mode. Use when a prompt includes a task specification that must be implemented, validated, and reflected in task tracking files. Do not use for PR review batches, generic coding tasks without a PRD task file, or standalone verification-only work.
---

# Execute PRD Task

Execute one PRD task from exploration through tracking updates.

## Required Inputs

- Task specification markdown.
- PRD directory path.
- Task file path.
- Master tasks file path.
- Auto-commit mode.
- Optional workflow memory directory path.
- Optional shared workflow memory path.
- Optional current task memory path.

## Workflow

1. Ground in repository and PRD context.
   - Read the provided task specification.
   - Read the repository guidance files named by the caller.
   - Read the PRD documents under the provided directory, especially `_techspec.md` and `_tasks.md`.
   - Read ADRs from the `adrs/` subdirectory of the PRD directory to understand the architectural decision context for this task.
   - After reading all sources, check for conflicts between the task specification, techspec, and ADRs. If any requirements contradict each other, stop and report the conflict instead of guessing — do not proceed to step 2.
   - If the caller provides workflow memory paths, use the installed `cy-workflow-memory` skill before editing code.
   - Reconcile the current workspace state before new edits.

2. Build the execution checklist.
   - Extract deliverables, acceptance criteria, and every explicit `Validation`, `Test Plan`, or `Testing` item into a numbered working checklist.
   - Print the full checklist before starting implementation so it is visible and trackable.
   - Capture the concrete pre-change signal that proves the task is not finished yet.
   - Use this checklist as a gate: mark each item done as evidence is produced during implementation, and do not proceed to validation until every checklist item has been addressed.

3. Implement the task.
   - Keep scope tight to the task specification.
   - Follow repository patterns and real dependency APIs.
   - Record meaningful out-of-scope work as follow-up notes instead of silently expanding the task.

4. Validate and self-review.
   - Run every test and validation command listed in the task specification — not just the repository-wide verification.
   - Use the installed `cy-final-verify` skill. This step is mandatory regardless of auto-commit mode — always verify before claiming completion.
   - Perform a self-review after verification and resolve every blocking issue before proceeding.

5. Update task tracking.
   - If workflow memory paths were provided, update the memory files first — record decisions, learnings, and touched surfaces before updating tracking status.
   - Use the caller-provided task file path and master tasks file path.
   - Mark subtasks complete only when the implementation and evidence are actually complete.
   - Change task status to completed only after clean verification and self-review.
   - Read `references/tracking-checklist.md` when applying status, checklist, or commit updates.
   - Sequence: memory update (if applicable) -> task file checkboxes -> task status -> master tasks file -> commit (if applicable).

6. Handle commit behavior.
   - If auto-commit is enabled, create one local commit after clean verification, self-review, and tracking updates.
   - If auto-commit is disabled, leave the diff ready for manual review and commit.
   - Never push automatically.

## Error Handling

- If the pre-change signal cannot be reproduced directly, capture the strongest available baseline signal and state the limitation.
- If validation fails, keep the task status unchanged until the failure is resolved.
- If tracking files are missing, stop and report the missing path before marking completion.
