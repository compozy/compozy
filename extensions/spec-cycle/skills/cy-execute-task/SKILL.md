---
name: cy-execute-task
description: Executes one spec task end-to-end uninterrupted — resolves spec conflicts autonomously, implements, validates, and updates tracking without pausing for questions. Use when a prompt includes a task specification that must be implemented, validated, and reflected in task tracking files. Do not use for PR review batches, generic coding tasks without a spec task file, or standalone verification-only work.
---

# Execute Spec Task

Implement the provided task and its accepted contract through validation and tracking. Reuse existing authorization; do not add approval loops for routine implementation choices.

## Ground Once

Read the task, applicable repository instructions, and the relevant `_spec.md`/graph context. Use the task's File References and any current contract inventory to find assigned tests, canonical examples, surface contracts, and relevant ADRs. Read those sections and follow their necessary dependencies; expand the survey only when the index is incomplete, stale, or ambiguous. A fresh native explorer and a full read of every sibling file are not mandatory.

Canonical examples, typed constraints, assigned test cases, and approved visual artifacts outweigh a task-file paraphrase. Current user decisions and repository compatibility policy remain authoritative. Resolve ordinary discrepancies at the owning contract and record the decision briefly. A missing contract or unresolved product/authorization decision is a real blocker; do not invent a partial substitute and mark it complete.

## Implement and Verify

1. Track deliverables, assigned test IDs, and applicable acceptance criteria in the existing task/checklist. A separate printed item-by-item checklist is unnecessary.
2. Capture or reuse the pre-change evidence needed for a bug/behavior claim. Implement only the task-owned changes, following nearby patterns and real dependency APIs.
3. For a named visual contract, inspect the reference and one representative implementation row early, then complete the required states/viewports using `eng-ui-screenshot`. Production content, primitives, and host chrome retain their owners.
4. Review the changed diff before final validation. Run the task's applicable tests/probes and required project gates, reusing current results for unchanged inputs. `cy-final-verify` owns evidence rules; do not repeat its review/report as another stage.
5. Verify affected canonical facts and assigned test IDs; fix mismatches and failures at their owner. A loop slice reports its outcome/evidence and flags affected QA scenarios; the workflow owns remaining integration QA and final PR delivery.

## Tracking and Commits

Use caller-provided memory/tracking paths. For long-running or handed-off work, update relevant decisions, changed surfaces, and evidence once; `cy-workflow-memory` owns those files. Mark task checkboxes/status complete only after the requested outcome is verified. `_tasks.md` owns graph topology and is not rewritten during normal completion.

Read `references/tracking-checklist.md` when applying tracking or commit changes. If auto-commit is enabled, commit only task-owned paths after validation and tracking. Otherwise leave the reviewed diff. Never push automatically. Preserve unrelated work and do not use blanket staging in a shared worktree.

On failure, keep task status truthful, repair the relevant cause, and recheck affected evidence. Report a concrete external/missing-contract blocker when progress depends on it; routine uncertainty is resolved from the available contract rather than converted into a permission question.
