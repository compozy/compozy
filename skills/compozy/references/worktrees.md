# Worktrees

## Ownership And Selection

A worktree belongs to one workspace and remains workspace-isolated across registry, status, event,
HTTP, UDS, and CLI reads. Resolve it by id, name, or contained path. A ready worktree can host a
session through `compozy session new --worktree <ref>`; create and bind one atomically with
`--new-worktree [name]`. These selectors are mutually exclusive with `--cwd`.

Use structured output for reads and mutations:

    compozy worktree list --refresh -o json
    compozy worktree create feature-auth --base main -o json
    compozy worktree adopt /absolute/path/to/worktree -o json
    compozy worktree inspect feature-auth -o json
    compozy worktree status feature-auth --refresh --forge -o json

Creation is asynchronous. Follow the per-worktree stream or inspect the record until it reaches
`ready` or `failed`; `compozy worktree cancel <ref>` applies only while creation is pending.

## Exit Plan And Actions

Read `compozy worktree exit <ref> -o json` before mutating Git state. The daemon-computed plan is the
source of truth for the primary action, every blocked reason, staged scope, forge vocabulary, browser
fallback, and cleanup evidence. A running bound session or unreadable Git status pauses the ladder.
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
`GET /api/workspaces/{workspace_id}/worktrees/{worktree_id}/stream` for replayable step events. Cancel
only the intended running operation with `compozy worktree exit-cancel <ref> --op <op_id>`; a stale or
finished id is a no-op and cannot cancel a later action.

HTTP and UDS expose the same exit contract at `GET .../exit`, `POST .../exit/actions`, and
`POST .../exit/cancel`. Action input is `{action, message?, title?, body?, draft?, base?}` and accepted
execution returns `{op_id}`.

## Cleanup

Inspect the exit plan's cleanup evidence before removal. Local evidence proves whether commits exist
elsewhere; fresh forge state can prove a PR merged. Removal preserves the branch and Git history.
When dirty or unpushed work remains, the first remove call returns a machine-readable refusal; use
`compozy worktree remove <ref> --force` only after reviewing it. Use `compozy worktree dismiss <ref>`
to clear a retained tombstone after external deletion or an unrecoverable missing path.

Worktree lifecycle has no `config.toml` keys. Task and Loop worktree policies are separate and remain
documented in `references/tasks-and-orchestration.md` and `references/loops.md`.
