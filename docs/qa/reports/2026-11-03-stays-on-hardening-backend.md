# Stays-on hardening backend verification

Integration branch: `feat/stays-on-hardening`. Backend completion is in progress.
The current stays-on hardening spec and operator clarifications own the contract.
This report records actual focused checks; it does not claim the Web integration or CI gates.

## Verified runtime increments

- Durable named agent messages resolve the current main under the admission transaction,
  survive a paused runtime and daemon restart, and deliver once after resume.
- Peers use four fields: `name`, `description`, `scope`, `state`. Authorization precedes pagination.
- Trigger targets include `target_instance_scope` on create/read/update. Events preserve the
  rendered trigger instruction, trigger name and event name in their durable run receipt.
- Reports expose optional `source: {kind: "event", trigger_name, event, summary?}`. Expired
  approvals include `action` and `resource`. Lost wakes use `failure` with `reason: "wake_lost"`
  and `detail`; the validator also reads historical `interrupted` reports.
- Provider summaries expose optional `reviewer_mode`. The configured reviewer survives provider
  cloning, agent resolution and process startup. Mode negotiation also uses ACP `configOptions`.
- Rule allows cover every location and never select an allow-always option. Denies veto allows;
  path resolution evaluates symlink targets before matching. Rule events identify the matched rule.
- Workspace approval grants still pass the existing workspace-access policy.
- Metrics expose `agent_instances.pause_counts` and `agent_instances.held_input_count` within the
  caller's profile/workspace. Structured events cover pauses, inputs, repeated denials, skill
  reviews and instance events.

## Focused evidence through 04:58 UTC

- `TestInstancePausedRuntimeIntegration`, `TestInstanceContextExhaustionIntegration`,
  `TestInstanceSkillApprovalIntegration`, `TestInstanceWebhookCheckIntegration`: passed against
  SQLite and real ACP child processes. The webhook test verifies the public report source.
- `TestInstanceUnattendedRuntimeIntegration`: 300 monitor wake turns trip one pause notification;
  a real timer fires while paused, an authenticated human input resets the counter while preserving
  the pause, and HTTP operator resume releases exactly one retained wake. Passed.
- `TestInstancePermissionRuntimeIntegration`: six cases covering declared/undeclared/unadvertised
  reviewer, allow/deny/level rules, operator decision, and authored approve-all. Passed, 8.521s.
- `TestInstancePermissionExpiryIntegration`: one-minute timeout, one pending request, one
  notification/report and immediate repeat denial. Passed within the combined run; final isolated
  receipt remains to be collected.
- `TestOpenGlobalDBReopenPreservesRowsAndStatus`,
  `TestOpenGlobalDBAppliesGlobalMigrationsAndEnablesWAL`, `TestGlobalDBInstanceTriggerRebuild`:
  passed, 26.422s. Includes schema 163 from v0.4.8, retained heartbeat wake/permission/trigger data,
  fresh creation, reopening, integrity and foreign keys.
- Core/session/globaldb/ACP focused policy and scoped-metrics cases passed. Provider regression
  packages passed: config 0.543s, session 0.813s, ACP 0.967s.
- Focused lint across changed backend packages passed with zero issues before the final reviewer
  propagation change; that change is being rechecked.
- `make codegen` and Turborepo's `codegen-check` passed. Both SDK typechecks passed. The Web
  typecheck awaits the F1–F3 integration, whose owners supply required fixture fields and levels;
  this worktree contains no Web edits.

## Impact handoff

Update the owning `docs/_memory/change-impact.md` entry with the new native message address,
peer projection, held delivery/input marker, event trigger scope, report origin/expiry fields,
provider reviewer summary, scoped metrics and structured events. Workspace/profile authority is
preserved before lookup and pagination. Generated OpenAPI, TypeScript and Go SDKs co-ship.
Official skill references, permissions/site docs and QA scenarios accompany the backend changes.
The permissions page's Web-only section remains owned by F2.
