# U5 unrestricted agent grant verification

Scope: User Scope Work BR-9–13; T-22–27/33–35/37. This is slice evidence, not an integrated browser verdict.

## Implemented boundaries

Strict operator-authored AGENT.md grant, untrusted extension stripping, immutable session grant,
authenticated immediate origin, persisted deferred ceilings, atomic narrowing on executable edits,
policy/audit branch, profile selection, aggregate guards and desktop restrictions. Generated API,
SDK and native descriptors co-ship. Web badge/editor and pseudo-workspace migration are separate slices.

## Verification

- `make codegen`: passed, including SQL, OpenAPI, SDK and native catalog generation.
- `go build ./...`: passed.
- Scoped `go vet` on workspaceaccess/config/session/automation/task/loop/api/core/daemon/store/globaldb: passed.
- Config `TestAgentWorkspaceAccess`, extension `TestLoadAgentResources`, policy `TestUnrestrictedGrant` and `TestUserScopeAccessPolicy`: passed.
- Session `TestSessionWorkspaceGrant`: passed, including resume after definition change.
- Automation `TestAutomationGrantCeiling`: passed against SQLite, including executable edit narrowing and operator re-save.
- Globaldb `TestTaskExecutionProfileGrantCeiling`: passed; granted agent cannot restore a previously narrowed ceiling.
- Integration-tagged `TestGlobalDBTaskPersistenceSurvivesReopenWithGlobalAndWorkspaceTasks`: passed against SQLite after reopen.
- Native `TestNativeGrantSelection`, `TestNativeUserSession`, `TestNativeUserWorkspaceReference`, `TestNativeWindowGlobalPartition`, `TestNativeAgentCreate`: passed through the registry or owning boundary.
- Existing scoped agent HTTP mutations, native workspace, task profile, Loop profile and migration stream/schema equivalence checks: passed.

## Existing failure and remaining integration evidence

The broader targeted `TestLoop.*` daemon selection found
`TestLoopWatchEventsObserverShouldApplyDoorbellIsolation/Should_reject_known_foreign_workspace_and_allow_unknown_workspace_payloads`.
The base implementation already compares workspace IDs exactly; the base test expects an empty workspace to match a project.
U3 requires exact matching for User isolation. Neither that test nor the implementation was changed by U5.
The integration owner must reconcile that existing expectation with U3.

The integrated Web/CLI scenario, full dispatcher authority matrix and CI heavy gates remain integration-owned.
No heavy gate, E2E suite or full Go test suite was run under the worker contract.


## A4 integration

Merged `feat/all-projects` at `2ad164266`. Preserved A4 migrations 00148/00149 and regenerated
U5 as `00150_schema.sql`. `requireAggregateSurface` now delegates to
`requireOperatorOrGrantedAgent`, covering the newly integrated automation, suggestions,
worktree, terminal and agent aggregates with the same aggregate audit target.

Passed after integration: codegen; `TestGrantedAggregateProfileScope`; the existing agent,
Loop, task, overview and terminal aggregate handler suites; automation list/attribution cursor
suites; and integration-tagged `TestGlobalDBAutomationAttributionMigration` plus task reopen.
The new HTTP guard test checks granted `all_profiles` admission and the aggregate policy target,
and rejects an ordinary session. The native registry test checks cross-profile User creation
and propagation of the authenticated creator ceiling.
