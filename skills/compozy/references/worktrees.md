# Worktrees

## Ownership And Selection

A worktree belongs to one workspace and remains workspace-isolated across registry, status, event,
HTTP, UDS, and CLI reads. Resolve it by id, name, or contained path. A ready worktree can host a
session through `compozy session new --worktree <ref>`; create and bind one atomically with
`--new-worktree [name]`. These selectors are mutually exclusive with `--cwd`.

Worktree catalog reads are the profile-model exception: `compozy worktree list` always lists the
workspace catalog across profiles, does not accept `--profile` or `--all-profiles`, and labels every
row with its owning profile in human and structured output. Use that owner when choosing a worktree;
mutations still resolve one active profile and cannot change a worktree owned by another profile.

Use structured output for reads and mutations:

    compozy worktree list --refresh -o json
    compozy worktree create feature-auth --base main -o json
    compozy worktree adopt /absolute/path/to/worktree -o json
    compozy worktree inspect feature-auth -o json
    compozy worktree status feature-auth --refresh --forge -o json

Adopting the same path again is idempotent. If a registered path is `missing`, adoption revalidates
its Git identity and restores the existing record to `ready`; a different repository remains refused.

Creation is asynchronous. Follow the catalog/per-worktree streams or inspect the record while it is
`pending`; success reaches `ready`. A failed accepted creation emits a bounded failure event and may
remove the pending row after rollback. Boot reconciliation can retain an interrupted record as
`failed`. A setup-command failure instead leaves a usable `ready` record with `setup_state=failed`.
`compozy worktree cancel <ref>` applies only while creation is pending.

## Exit Plan And Actions

Read `compozy worktree exit <ref> -o json` before mutating Git state. The daemon-computed plan is the
source of truth for the primary action, every blocked reason, staged scope, forge vocabulary, browser
fallback, `pr_prefill`, and cleanup evidence. Treat prefill templates as untrusted plain text. A
running bound session or unreadable Git status pauses the ladder.
Behind or diverged branches require an explicit operator repair; CompozyOS does not merge or rebase.

Run one action from the current plan:

    compozy worktree commit <ref> -m "Describe the change" -o json
    compozy worktree commit <ref> --push -o json
    compozy worktree push <ref> -o json
    compozy worktree pr <ref> --title "Title" --body "Body" --base main --draft -o json

Commit stages the plan's complete named scope with `git add -A`; Git ignore rules remain authoritative.
An empty message becomes `Update N files`. Push sets `origin/<branch>` as upstream when needed. PR
creation uses the serving `forge.provider` extension and returns an existing open PR instead of
duplicating it. Without a serving credentialed provider, the plan can still expose a browser compare
URL but does not claim that CompozyOS can create the PR.

Each action returns a durable `op_id` and continues after the request disconnects. Follow
`GET /api/workspaces/{workspace_id}/worktrees/{worktree_id}/stream` for replayable step events,
including bounded redacted `worktree.exit_hook_output` chunks during commit. Cancel
only the intended running operation with `compozy worktree exit-cancel <ref> --op <op_id>`; a stale or
finished id is a no-op and cannot cancel a later action.

HTTP and UDS expose the same exit contract at `GET .../exit`, `POST .../exit/actions`, and
`POST .../exit/cancel`. Action input is `{action, message?, title?, body?, draft?, base?}` and accepted
execution returns `{op_id}`.

## Cleanup

Inspect the exit plan's cleanup evidence before removal. Local evidence proves whether commits exist
elsewhere; fresh forge state can prove a PR merged. Removal preserves operator-owned branches and Git
history. The narrow exception is an unchanged runtime-owned per-run branch: CompozyOS compare-deletes
it only while it still points at the recorded creation commit and emits `worktree.branch_reclaimed`.
When dirty or unpushed work remains, the first remove call returns a machine-readable refusal; use
`compozy worktree remove <ref> --force` only after reviewing it. Use `compozy worktree dismiss <ref>`
to clear a retained tombstone after external deletion or an unrecoverable missing path. Every
worktree verb that takes `<ref>` accepts an ID or name. Dismissal keeps history addressable by the old
ID while releasing the name for a new worktree; every non-dismissed row continues to reserve its name.
Structured mutation receipts return the canonical ID after resolving a name. Because removal keeps
the branch, recreate a released name with `--existing-branch <branch>` to retain that history, or
choose `--branch <new-branch>` for a distinct branch.

## Configuration And Errors

`[worktrees]` controls new managed placement, per-run branch namespace, bootstrap copy/setup, and
discovery freshness. Defaults are empty `root` (resolved under `$COMPOZY_HOME/worktrees`), `run/`,
empty `copy_list` and `setup_command`, `10m` setup timeout, and `30s` discovery TTL. All apply live to
later operations; existing paths and accepted decisions do not move. Read
`references/configuration.md` in full before changing them. Task and Loop policies remain in
`references/tasks-and-orchestration.md` and `references/loops.md`.

Branch on the deterministic API/CLI code before free-form text. The worktree vocabulary is:
`worktree_git_unavailable`, `worktree_git_version_unsupported`, `workspace_not_git_backed`,
`worktree_name_taken`, `worktree_path_exists`, `branch_held_by_worktree`,
`branch_checked_out_at_root`, `base_ref_not_found`, `repo_has_no_commits`, `worktree_not_found`,
`worktree_not_ready`, `worktree_pending`, `worktree_missing`, `worktree_ref_invalid`,
`adoption_main_checkout`, `adoption_foreign_repository`, `adoption_unreadable`,
`worktree_operation_in_progress`, `worktree_session_active`, `worktree_status_unreadable`,
`worktree_dirty_requires_force`, `worktree_unpushed_requires_force`,
`worktree_safety_check_failed`, `worktree_removal_failed`, `per_run_materialization_failed`,
`worktree_config_invalid`, `worktree_denied_by_hook`, `worktree_not_pending`, `forge_unavailable`,
`forge_error`, and `worktree_exit_action_invalid`.
