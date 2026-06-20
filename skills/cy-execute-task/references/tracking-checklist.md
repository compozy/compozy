# Tracking Checklist

Apply this checklist when updating PRD task tracking files.

1. Update the current task file checkboxes that correspond to completed subtasks.
2. Change the task status to `completed` only after implementation, validation, and self-review are complete.
3. Do not update `_tasks.md` for normal completion tracking. `_tasks.md` owns task graph topology only; change it only when the caller explicitly asks to modify the DAG.
4. Re-check the task specification and supporting PRD docs before marking anything complete.
5. Follow the caller's commit mode and repository staging rules when deciding whether tracking files belong in a commit.
