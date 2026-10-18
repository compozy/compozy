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

The default commit stages the whole worktree with `git add -A`; Git ignore rules remain authoritative.
The untracked preview can be truncated and is informational, never a reviewed allowlist.
For selective delivery, read and review an explicit scope first:

    compozy worktree exit <ref> --include src/change.go --include docs/change.md -o json
    compozy worktree commit <ref> --include src/change.go --include docs/change.md --expected-scope <commit_scope.fingerprint> -m "Reviewed change" --push -o json

A scoped plan returns the complete `commit_scope.include_paths` and fingerprint even when the
whole-worktree untracked preview exceeds 200 names. The fingerprint binds the branch HEAD, exact
selected file bytes and modes, and selected staged state. Reuse it only for the reviewed include set.
The daemon validates it at acceptance and again under the repository lock immediately before staging.
Exact relative file paths use literal Git pathspecs; directories, traversal, Git metadata and symlink
paths are refused. Selective commit preserves unrelated staged versions and untracked local task files.
Accepted and terminal step events record the selected paths. A changed scope requires fresh review.
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
`POST .../exit/cancel`. Repeat `include` query parameters on `GET .../exit` for a scoped plan. Action input is
`{action, message?, title?, body?, draft?, base?, include_paths?, expected_scope?, delivery_id?, expected_head?}`
and accepted execution returns `{op_id}`. For `action: "deliver"`, `delivery_id`, `expected_head`,
`message`, `base`, a nonempty `include_paths`, and `expected_scope` are required; the validated
native session owns caller identity.

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

### Managed delivery handoff

A bound managed session first reviews a scoped exit plan, then submits that exact scope:

    compozy worktree exit <ref> --include src/change.go --include docs/change.md -o json
    compozy worktree deliver <ref> --delivery-id <stable-intent-id> --expected-head <reviewed-sha> --include src/change.go --include docs/change.md --expected-scope <commit_scope.fingerprint> -m <message> --title <title> --body <body> --base <branch>

Both the repeatable `--include` paths and the scoped plan's `--expected-scope` fingerprint are
required for managed delivery. Whole-worktree `commit` remains a separate action. The local UDS
validates `COMPOZY_SESSION_ID` and `COMPOZY_AGENT`; a request body cannot select another caller.
This action always creates or reuses a draft PR through the native forge provider.

The command returns an operation ID after durably admitting the delivery intent. CompozyOS then stops that exact bound caller and completes delivery autonomously. A session admission fence prevents new or resumed sessions on that checkout; another active bound session refuses the handoff. Unrelated sessions remain running. Repository and worktree usage locks serialize the Git effects. Selected delivery preserves unrelated working files and staged entries; the actual commit tree must match the authorized candidate before any push.

Use a stable delivery ID when retrying an identical intent. The daemon retains its journal under the configured worktrees root and reconciles worktree path, branch, base SHA, remote destination, original HEAD, expected commit tree and exact draft PR before repeating effects after restart or an ambiguous provider response. Reusing an ID for another intent is refused. Managed delivery requires a single identical fetch/push destination and publishes one explicit branch refspec. A changed identity or competing session fails explicitly instead of broadening the authorization.

The selected index is checked again before staging. Recovery accepts only the original reviewed
scope or the recorded proof of the daemon's own authorized staging. A safety refusal is terminal
and requires a new reviewed intent; it cannot repeatedly stop a resumed caller after restart.
Unreadable journals remain available for diagnosis while unmatched interrupted operation receipts
settle as explicit failures without session, Git, or forge effects.

## Isolated subagents

Use `compozy__subagent_delegate` with `isolation: "worktree"` when a child should change code on
its own branch. The branch is `<run_branch_namespace><title-slug>-<8 hex>` (the suffix comes from
the subagent ID). `base_ref` accepts a commit-ish; omitted or blank uses the caller checkout's
HEAD at delegation time. Commit intended starting changes first: uncommitted files are not copied.
A shared child of an isolated subagent inherits that checkout; another isolated child starts from
its HEAD. The workspace and permission boundaries remain those of the parent.

The child commits on that branch and can run `compozy worktree deliver` to open a pull request.
`compozy session subagents show <id>` and the status tool expose its branch, immutable base SHA,
commits ahead, dirty-file count and PR facts, captured when it settles. These are snapshots, not
live status; use `compozy worktree status --refresh --forge` for a fresh read.

Completion, cancellation, stop cascades and archive retain admitted worktrees. Remove them through
the existing worktree commands and their safety refusals. Before admission, cleanup stops and joins
any child first; dirty files or commits preserve the checkout with `worktree_retained`. Failed
cleanup remains anchored and is retried at the next daemon boot.

Settlement fields, including `pull_request_status`, are omitted while the child is running.
A settled `unknown` PR status means the forge lookup was unavailable or failed; it does not mean
that no PR exists.
