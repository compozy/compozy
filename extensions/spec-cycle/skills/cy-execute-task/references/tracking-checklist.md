# Tracking Checklist

Apply this checklist when updating spec task tracking files.

1. Update the current task file checkboxes that correspond to completed subtasks.
2. Change the task status to `completed` only after implementation, validation, and self-review are complete.
3. Do not update `_tasks.md` for normal completion tracking. `_tasks.md` owns task graph topology only; change it only when the caller explicitly asks to modify the DAG.
4. Compare completion against the grounded task contract and current evidence. Reopen supporting spec sections only for changed inputs or unresolved requirements. Pending workflow-owned integration checks stay explicit; they are not reported as executed by this task.
5. Follow the caller's commit mode and repository staging rules when deciding whether tracking files belong in a commit.
