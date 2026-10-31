# Worktrees

## Ownership And Selection

A worktree belongs to one workspace and remains workspace-isolated across registry, status, event,
HTTP, UDS, and CLI reads. Resolve it by id, name, or contained path. A ready worktree can host a
session through `compozy session new --worktree <ref>`; create and bind one atomically with
`--new-worktree [name]`. These selectors are mutually exclusive with `--cwd`.

A governed child gets the same binding: `compozy spawn --worktree <ref>` or
`--new-worktree[=name] [--base <ref>] [--branch <name>]`, native `compozy__session_spawn`
`worktree`/`new_worktree`, and HTTP/UDS `POST /api/agent/spawn` with the same fields. The child
starts with the worktree as its working directory and provider sandbox root. With a `workspace`
that names another registered workspace, the cross-workspace spawn is authorized first and the
worktree is resolved or created in that workspace. A worktree created for
a refused spawn is removed again. The spawn result reports the worktree's `id`, `branch`, `path`,
and `base_sha`; read live Git facts (commits ahead, dirty files, PR) with
`compozy worktree status <ref> --refresh --forge -o json` or `compozy__worktree_inspect`.

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
`pending`; success reaches `ready`. A failed accepted creation rolls back in order — the linked
checkout and its Git admin entry first (a locked, half-written checkout included), then the branch,
only when that creation minted it and the checkout is gone — and the record stays `failed` with
`pending_phase` naming the failed phase and `create_error` its redacted cause (`inspect`/`list`).
It emits `worktree.creation_failed` plus a WARN log, or an ERROR log when rollback is incomplete
(the cause then says `rollback incomplete` and leftover Git state is kept for inspection). A failed
record keeps its name until `compozy worktree dismiss <ref>`; binding a session to it returns
`worktree_not_ready` with the cause. Only `cancel` with a complete rollback deletes the pending row.
Boot reconciliation settles an interrupted record the same way. `git worktree add` runs under
`worktrees.checkout_timeout` (default `15m`); raise it for very large repositories. A setup-command
failure instead leaves a usable `ready` record with `setup_state=failed`.
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
`POST .../exit/cancel`. `POST .../exit/prepare` resolves forge capabilities with an eight-second
request deadline and returns the same exit-plan payload without mutating Git. The Web PR action
uses this explicit preparation before choosing native creation or browser fallback, including after
a daemon restart. Ordinary `GET .../exit` reads remain provider-free. Repeat `include` query parameters on `GET .../exit` for a scoped plan. Action input is
`{action, message?, title?, body?, draft?, base?, include_paths?, expected_scope?, delivery_id?, expected_head?}`
and accepted execution returns `{op_id}`. For `action: "deliver"`, `delivery_id`, `expected_head`,
`message`, `base`, a nonempty `include_paths`, and `expected_scope` are required; the validated
native session owns caller identity.

## Cleanup

Inspect the exit plan's cleanup evidence before removal. Local evidence proves whether commits exist
elsewhere; fresh forge state can prove a PR merged. Removal preserves operator-owned branches and Git
history. The narrow exception is an unchanged runtime-owned per-run branch: CompozyOS compare-deletes
it only while it still points at the recorded creation commit and emits `worktree.branch_reclaimed`.
Removal excludes commits whose patches are already present in the comparison base, local default branch, or a local
remote-tracking ref, including cherry-picked or rebased landings with different commit IDs.
No upstream or PR is required for that evidence. Unique commits still require explicit force;
remote evidence reflects locally available refs, so fetch first if a recent landing is not visible.
When dirty or unpushed work remains, the first remove call returns a machine-readable refusal; use
`compozy worktree remove <ref> --force` only after reviewing it. Use `compozy worktree dismiss <ref>`
to clear a retained tombstone after external deletion or an unrecoverable missing path. `worktree remove` also accepts a
`missing` record when its saved path is still absent and every bound session is archived. This
metadata-only path runs no Git commands, hooks, or filesystem deletion; `--force` cannot override
a reappeared path or a non-archived session. Historical attribution is retained as a dismissed record. Every
worktree verb that takes `<ref>` accepts an ID or name. Dismissal keeps history addressable by the old
ID while releasing the name for a new worktree; every non-dismissed row continues to reserve its name.
Structured mutation receipts return the canonical ID after resolving a name. Because removal keeps
the branch, recreate a released name with `--existing-branch <branch>` to retain that history, or
choose `--branch <new-branch>` for a distinct branch.

Local branch deletion is an explicit opt-in: `compozy worktree remove <ref> --delete-branch` or, for a
missing record, `compozy worktree dismiss <ref> --delete-branch` (HTTP/UDS `delete_branch=true`;
native `compozy__worktree_remove` input `delete_branch`). It runs only after the checkout removal or
dismissal succeeds and uses Git's merged-only delete; `--force` also deletes unmerged commits. The
root checkout's HEAD branch and the default branch (`origin/HEAD`, or `main` without a remote) are always kept.
If a remote exists but its default branch is unknown, deletion is refused with `delete_failed`
and diagnostic detail until `origin/HEAD` is available. The
receipt's `branch` object reports `deleted: true` or a kept `reason` (`root_checkout_branch`,
`default_branch`, `not_fully_merged`, `checked_out_elsewhere`, `checkout_present`,
`registration_present`, `branch_absent`, `no_branch`, `delete_failed`) — a kept branch is not a
failed cleanup. Re-running the same command on the removed or dismissed record retries only the
branch step. Dismissal with the opt-in releases Git's stale registration only when the path,
branch and saved Git administrative identity match the record;
it never prunes other worktrees. Without the opt-in, callers keep the established `204` behavior.

A configured but deleted upstream is readable: `has_upstream=true` with unknown `ahead`/`behind`.
Refresh with `compozy worktree status <ref> --refresh --forge -o json` before cleanup. Fresh merged
forge evidence matching the current HEAD permits removal after squash merge and remote branch
deletion without `--force`. Missing, expired, or mismatched evidence does not excuse unpushed work;
dirty files and active sessions still block removal. The exit plan describes an unavailable upstream
instead of claiming that no push is pending. Removal checks evidence again after pre-remove hooks.

A discovered (not adopted) linked worktree can be removed without adopting it. Pass the path exactly
as `compozy worktree list -o json` reports it in `discovered[].path`, because Git matches stale
entries by that spelling (on macOS, `/private/tmp/...` is not `/tmp/...`):

    compozy worktree remove --path /private/tmp/detached-a217e52e -o json

HTTP/UDS use `POST /api/workspaces/{workspace_id}/worktrees/discovered/remove` with
`{path, force?, expect_stale?}`; native `compozy__worktree_remove` takes `path` instead of `ref`. A stale (prunable) entry
only loses its Git metadata. `expect_stale: true` freezes a stale-only confirmation: if the checkout is present
again, the request is refused with `worktree_discovered_changed` and nothing is removed. A live checkout gets the same identity checks
as adoption and the same `worktree_dirty_requires_force` / `worktree_unpushed_requires_force`
refusal with a risk inventory. A detached HEAD counts commits that no branch, tag or remote keeps.
Retry with `--force` after review. Discovered removal never deletes a branch, so `--path` cannot be
combined with a ref or `--delete-branch`. A path that a live record owns, in this or any other
workspace on the same repository, is refused with `worktree_registered`: remove that record by its ref. The `worktree.pre_remove` gate runs with
`discovered: true`, an empty `worktree_id` and no `origin`.

An orphaned checkout is a folder that still exists while its `.git` link is gone and Git still
registers it (`prunable`), usually left by an interrupted removal. Git refuses to remove it even with
force, so CompozyOS answers `409 worktree_checkout_orphaned` on both the record and the `--path` route,
with an `error` sentence and a risk inventory: `risk.leftover_files` counts every leftover file (Git
cannot tell which changed) and `risk.unpushed_commits` counts commits only a detached HEAD still
holds. Nothing is removed. Retry with `--force` after review: CompozyOS deletes the leftover folder,
then clears only that entry from Git's list; the branch is kept unless `--delete-branch` applies its
usual rules. A daemon restart during a removal leaves such a record `ready` for this decision instead
of failing startup.

In Web bulk removal, **Retry failed** remains non-force. A dirty/unpushed/orphaned refusal with a risk
inventory offers **Force remove…**, which opens a second confirmation listing only eligible
worktrees and their changed files and unique commits. Confirming retries only those targets with
force. With **Also delete local branches** selected, this also authorizes deletion of their unmerged
branches; the root and default branches remain protected. Other refusals cannot be forced. Discovered rows offer **Delete worktree…** (stale rows: **Remove
stale entry…**) and join **Select worktrees…**. They are removed by Git path and keep their branch.

## Configuration And Errors

`[worktrees]` controls new managed placement, per-run branch namespace, bootstrap copy/setup, and
discovery freshness. Defaults are empty `root` (resolved under `$COMPOZY_HOME/worktrees`), `run/`,
empty `copy_list`, `setup_command` and `teardown_command`, `10m` setup and teardown timeouts,
and `30s` discovery TTL. All apply live to
later operations; existing paths and accepted decisions do not move. Read
`references/configuration.md` in full before changing them. Task and Loop policies remain in
`references/tasks-and-orchestration.md` and `references/loops.md`.

Branch on the deterministic API/CLI code before free-form text. The worktree vocabulary is:
`worktree_git_unavailable`, `worktree_git_version_unsupported`, `workspace_not_git_backed`,
`worktree_name_taken`, `worktree_path_exists`, `branch_held_by_worktree`,
`branch_checked_out_at_root`, `base_ref_not_found`, `repo_has_no_commits`, `worktree_not_found`,
`worktree_not_ready`, `worktree_pending`, `worktree_missing`, `worktree_ref_invalid`,
`adoption_main_checkout`, `adoption_foreign_repository`, `adoption_unreadable`,
`worktree_registered`, `worktree_discovered_changed`, `worktree_operation_in_progress`, `worktree_session_active`, `worktree_status_unreadable`,
`worktree_dirty_requires_force`, `worktree_unpushed_requires_force`,
`worktree_safety_check_failed`, `worktree_removal_failed`, `worktree_checkout_orphaned`,
`per_run_materialization_failed`,
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

### Display observations and delivery evidence

Worktree and session displays share canonical PR observations keyed by workspace, normalized origin
repository and branch. Changing origin never reuses facts from the old repository. The cache favors
an open PR from that same repository, then the most recently updated terminal PR. Display observations
expire after 60 seconds for open/no PR results and 10 minutes for terminal PRs. Provider errors retain
facts, delay automatic retries and expose lookup state. These cached facts never authorize delivery
or removal: exact delivery queries and current-HEAD cleanup checks obtain action-local evidence.
Completed actions request a separate canonical display refresh.

## Teardown on removal

Set `worktrees.teardown_command` to clean up resources started by setup, for example
`docker compose down`. The default is empty (disabled); `teardown_timeout` defaults to `10m`
and must be positive. Both keys merge independently across user, profile, workspace and
workspace-profile layers. Removal resolves these settings for the worktree owner profile.
An explicit empty command disables an inherited command.

Teardown runs in the managed checkout after removal safety checks and `worktree.pre_remove`
approval, before Git removes the directory. Hook denial skips teardown, including forced removal.
It uses the same shell and allowlisted environment as setup: `PATH`, `HOME`, `LANG`, `TMPDIR`,
`LC_*`, `SHELL`, `COMPOZY_WORKSPACE_ROOT`, `COMPOZY_WORKTREE_PATH`, `COMPOZY_WORKTREE_ID`, and
`COMPOZY_WORKTREE_BRANCH`. Load any project-specific environment from the checkout explicitly.

Each stdout/stderr stream is captured up to 16 KiB and redacted as a whole, including multiline
secrets. When the command finishes, the worktree event stream publishes the captured output as
`worktree.teardown_output` with `stream` and `chunk`. A launch failure, nonzero exit or timeout emits `worktree.teardown_failed`
with a bounded diagnostic and continues removal. A timeout terminates the command's process group.
Removal rechecks checkout identity and unpushed-work risks after teardown; commands should be safe to retry.
These events do not change the recorded setup result.

Dismiss remains metadata-only and never runs teardown, even if a checkout reappears. Missing paths,
already removed records and discovered, unregistered worktrees also skip teardown. Adopted records
use the configured teardown when removed. Unset teardown preserves existing removal behavior.

## Change an existing session's worktree

Use `compozy__session_worktree_bind` with `session_id`, `worktree`, and optional
`expected_worktree_id`, or `compozy__session_worktree_unbind` with the session and optional expected
binding. These sessions-toolset mutations preserve the session ID, transcript, window, and list
row. They do not fork. The target must be ready in the same workspace/profile. Bind and unbind
set the execution directory to the destination root; files remain in their original checkout.

Only idle ordinary project user sessions are eligible. Busy sessions (including self-calls during
a turn), pending input, task/Loop/review/subagent ownership, archived sessions, and User scope
are refused. `session_busy` is immediate; no deferred change is queued. A stale expected binding
returns `session_worktree_conflict`. An explicit empty expected ID means workspace root.

The daemon holds old/target checkout usage against removal and delivery, verifies the old provider
process exit, and persists a recoverable binding transition. The next turn uses the new cwd and
permission root with native resume or bounded transcript replay, plus one daemon directory-change
note. Explicitly unbind to recover from an unavailable checkout; missing worktrees do not silently
fall back. A restart completes an accepted durable transition. No database shape migration is needed.

CLI: `compozy session worktree bind <session> <worktree-ref>` and
`compozy session worktree unbind <session>`, both with optional `--expected-worktree`.
HTTP/UDS: `PUT /api/workspaces/{ws}/sessions/{id}/worktree` accepts
`{"worktree":"feature-auth","expected_worktree_id":""}`; `DELETE` accepts optional
`?expected_worktree_id=`. Both return 200 with the same `SessionResponse`; app-token access is denied.

Web offers Use worktree…, Change worktree…, and Use workspace root in the environment control.
The existing `session_catalog_changed` upsert carries optional `execution_location_changed: true`
after a binding transition. File-picker clients must invalidate their session-scoped results even
if independent detail polling already observed the new binding; ordinary upserts do not imply a cwd change.

New worktree… composes materialization before explicit binding. Branch/PR/board/status/delivery
projections update without navigation. `/worktree`, `/worktree-fork`, and conversation forks retain
their existing new-session behavior. Confirm `worktree_id` through
`GET /api/workspaces/{ws}/sessions/{id}` or `compozy session list --workspace <ws> --worktree <ref> -o json`.
