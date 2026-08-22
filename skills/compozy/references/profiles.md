# Profiles

## Operating Rule

A profile partitions operator work on one CompozyOS installation. `default` is permanent. Profiles are
global identities; workspace and Global lenses only remember a selection. A session binds the resolved
profile at creation, and later selection changes never move that session.

## Resolve And Inspect

Resolution order is root `--profile`, `COMPOZY_PROFILE`, the resolved workspace's remembered choice
(or the Global lens), then `default`. An archived remembered choice falls back to `default` with the
`archived_remembered_fallback` note. `daemon`, `doctor`, and `update` ignore profile selection.

Use structured reads:

```bash
compozy profile list -o json
compozy profile current -o json
compozy profile use <name> -o json
```

Inside a managed session, prefer `compozy__profile_list` and `compozy__profile_current`. Their live
descriptors define the exact schema. They expose the session-bound profile but do not mutate selection.

## Scoped And Aggregate Reads

Work reads use the resolved profile by default. Select another owner with root `--profile`, or request an
explicit cross-profile view with `--all-profiles`; API and UDS callers use `all_profiles=true`. These two
choices conflict. Aggregate rows include `profile_name`, and JSONL output starts with a
`profile_resolution` frame even when no rows match.

A scoped detail read of work owned by another profile returns not found. An aggregate detail read may
return it with its owner label. Session catalog initial state, replay, and live events follow the same
scope. Agent-native calls derive their immutable session profile and cannot accept a caller-supplied
replacement.

Worktrees are intentionally visible across profiles because they represent workspace filesystem state;
their rows still identify the owner. Network delivery is profile-blind, while network channels,
conversations, subscriptions, and work remain owner-scoped.

## Lifecycle

Create and identity update are direct local mutations:

```bash
compozy profile create <name> [--color <hex>] [--icon <name> | --emoji <char>]
compozy profile update <name> [--color <hex>] [--icon <name> | --emoji <char>]
```

Rename, archive, and delete are plan-based. The CLI fetches the current plan and submits its
`plan_revision`; a stale revision must be replanned, never replayed. Rename can include repository
folders with `--repos all|none|<workspace-ids>`. Archive preserves work and freezes guarded queued work.
Unarchive restores availability but does not re-enable paused automations. Delete succeeds only when the
profile owns no work, and structured or non-interactive use requires `--yes`.

```bash
compozy profile rename <old> <new> --repos none
compozy profile archive <name>
compozy profile unarchive <name>
compozy profile delete <name> --yes
```

Inspect durable lifecycle recovery with `compozy profile ops -o json`; retry a failed operation with
`compozy profile ops retry <op-id> -o json` after correcting its reported cause.

## Surfaces And Authority

HTTP and UDS share `/api/profiles`, including selection, plan, and operation routes. Remote reads are
allowed on an enabled tier, but every remote profile-state write returns
`profile_remote_management_forbidden`. Move the mutation to a local CLI, desktop, HTTP, or UDS surface;
do not weaken the Gateway tier.

Stable palette actions are `profile.use`, `profile.create`, `profile.update`, `profile.rename`,
`profile.archive`, `profile.unarchive`, and `profile.delete`. They delegate to the same selection and
lifecycle surfaces and never replace the plan protocol.

## Errors And Events

Profile failures carry `{error:{code,message,action}}`. Preserve all three fields. Follow `action`; for
`profile_plan_stale`, fetch and review a new plan. Normal events are `profile.created`,
`profile.identity_updated`, `profile.renamed`, `profile.archived`, `profile.unarchived`,
`profile.deleted`, and `profile.selection_changed`. Recovery paths use `profile.plan_stale`,
`profile.lifecycle_op_recovered`, and `profile.lifecycle_op_failed`. Event payloads never carry secret
references.

Profile-scoped Vault refs use `vault:profiles/<profile>/<name>`. Rename rewrites only the Manager's
explicit rewrite list. Never reconstruct or bulk-edit refs outside the lifecycle surface.
