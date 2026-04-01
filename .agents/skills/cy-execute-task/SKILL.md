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
   - If the caller provides workflow memory paths, use the installed `cy-workflow-memory` skill before editing code.
   - Reconcile the current workspace state before new edits.

2. Build the execution checklist.
   - Extract deliverables, acceptance criteria, and every explicit `Validation`, `Test Plan`, or `Testing` item into a working checklist.
   - Capture the concrete pre-change signal that proves the task is not finished yet.

3. Implement the task.
   - Keep scope tight to the task specification.
   - Follow repository patterns and real dependency APIs.
   - Record meaningful out-of-scope work as follow-up notes instead of silently expanding the task.

4. Validate and self-review.
   - Run targeted proof for the task outcome.
   - Use the installed `cy-final-verify` skill before any completion claim or commit.
   - Perform a self-review after verification and resolve every blocking issue before proceeding.

5. Update task tracking.
   - Use the caller-provided task file path and master tasks file path.
   - Mark subtasks complete only when the implementation and evidence are actually complete.
   - Change task status to completed only after clean verification and self-review.
   - Read `references/tracking-checklist.md` when applying status, checklist, or commit updates.
   - If workflow memory paths were provided, update the memory files before any completion claim or handoff.

6. Handle commit behavior.
   - If auto-commit is enabled, create one local commit after clean verification, self-review, and tracking updates.
   - If auto-commit is disabled, leave the diff ready for manual review and commit.
   - Never push automatically.

## Error Handling

- If the task specification conflicts with supporting PRD documents, stop and report the conflict instead of guessing.
- If the pre-change signal cannot be reproduced directly, capture the strongest available baseline signal and state the limitation.
- If validation fails, keep the task status unchanged until the failure is resolved.
- If tracking files are missing, stop and report the missing path before marking completion.
