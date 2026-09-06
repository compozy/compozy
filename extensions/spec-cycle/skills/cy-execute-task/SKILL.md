---
name: cy-execute-task
description: "Implement and verify an existing Compozy spec task, then update its tracking. Excludes review remediation."
---

# Execute Spec Task

Complete the task's accepted outcome, applicable verification, and tracking. Continue through running the changed behavior and repairing failures within the authorized scope; the first implementation is not a review handoff unless the user requested one.

## Ground Once

Read the task, applicable repository instructions, and the relevant `_spec.md`/graph context. Use the task's File References and any current contract inventory to find assigned tests, canonical examples, surface contracts, and relevant ADRs. Read those sections and follow their necessary dependencies; expand the survey only when the index is incomplete, stale, or ambiguous. A fresh native explorer and a full read of every sibling file are not mandatory.

Canonical examples, typed constraints, assigned test cases, and approved visual artifacts outweigh a task-file paraphrase. Current user decisions and repository compatibility policy remain authoritative. Resolve ordinary discrepancies at the owning contract and record the decision briefly. A missing contract or unresolved product/authorization decision is a real blocker; do not invent a partial substitute and mark it complete.

## Implement and Verify

1. Track deliverables, assigned test IDs, and applicable acceptance criteria in the existing task/checklist. A separate printed item-by-item checklist is unnecessary.
2. Capture or reuse the pre-change evidence needed for a bug/behavior claim. Implement only the task-owned changes, following nearby patterns and real dependency APIs.
3. For a named visual contract, inspect the reference and one representative implementation state early. Complete the required states/viewports using `eng-ui-screenshot` at their assigned verification boundary: this task for standalone delivery, or the named integration/QA task in a loop. Production content, primitives, and host chrome retain their owners.
4. Review the changed diff before final validation. Run the checks that prove this task's outcome, plus required project gates, reusing current evidence for unchanged inputs. Apply `cy-final-verify` as the evidence standard within this step, not a second review/test/report cycle. A task checkpoint still follows repository commit policy.
5. Verify affected canonical facts and assigned test IDs; fix mismatches and failures at their owner. In a loop, record the focused result and flag affected QA scenarios with the remaining walks/visual rows and their workflow owner. Full QA, a new lab, both E2E suites, and a complete screenshot bundle are not automatic per-task requirements. Preserve an explicit task acceptance requirement for a live probe or visual proof; do not mark an unverified requirement complete.

## Tracking and Commits

Use caller-provided memory/tracking paths. For long-running or handed-off work, update relevant decisions, changed surfaces, and evidence once; `cy-workflow-memory` owns those files. Mark task checkboxes/status complete only after the requested outcome is verified. `_tasks.md` owns graph topology and is not rewritten during normal completion.

Use `references/tracking-checklist.md` for tracking ambiguity or drift. Commit/push follow the caller's requested delivery scope: with auto-commit disabled, leave the owned diff for the controller; with authorized commit/push delivery, continue through its required checks. This skill does not grant new publishing authority. Preserve unrelated work and stage only owned paths.

On failure, keep task status truthful, repair the relevant cause, and recheck affected evidence. Report a concrete external/missing-contract blocker when progress depends on it; routine uncertainty is resolved from the available contract rather than converted into a permission question.
