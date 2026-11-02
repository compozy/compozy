# B2 instance test evidence

Status: B2 coverage complete within the requested boundary. Wake state, streaks and the absent B3 memory/check/occurrence surfaces are excluded.

Verified on 2026-11-01, including the native create journey and workspace identity repair:

```sh
GOCACHE=/private/tmp/compozy-b2-go-cache CGO_ENABLED=1 go test -race -tags=integration ./internal/agentinstance ./internal/store/globaldb ./internal/daemon ./internal/api/core ./internal/session ./internal/hooks -run 'Test(Instance|AgentInstance|NativeInstance)' -count=1
GOCACHE=/private/tmp/compozy-b2-go-cache go build ./...
```

All six focused package selections and the build passed. The native journey uses real isolated daemon/SQLite/ACP mock processes. This is not a full repository gate or a live external-provider run. The final strengthened prompt-error assertions were rerun in `TestInstanceNativeCreateJourney`. The worker brief excludes global gates.

| Case | Test | Coverage boundary |
| --- | --- | --- |
| UT-016 | TestInstanceServiceRecovery; TestInstanceServiceConcurrentCreate | Create returns a new instance |
| UT-017 | TestInstanceServiceRecovery; TestInstanceAdmission | Same identity is idempotent |
| UT-018 | TestInstanceCreateAdmission | On-demand rejected |
| UT-019 | TestInstanceConcurrentLimit | Ten distinct identities admitted |
| UT-020 | TestInstanceCreateAdmission | Different agent rejected |
| UT-021 | TestInstanceCreateAdmission; TestInstanceWorkspaceReach | Workspace denial retained |
| UT-022 | TestInstanceCreateAdmission; TestInstanceAuthority | Child ceiling intersected with definition |
| UT-023 | TestInstanceCreateAdmission; TestInstanceCallerLineage | Depth one and descendants rejected |
| UT-024 | TestInstanceLifecycle | Repeated pause retains manual reason |
| UT-025 | Excluded | B3 streak reset |
| UT-026 | TestInstanceServiceRecovery | Missing definition leaves paused state intact |
| UT-027 | TestInstanceServiceRecreation | New instance and main session after archive |
| UT-028 | TestInstanceAdoptAdmission | On-demand adoption rejected |
| UT-029 | TestInstanceAdoptAdmission | Child adoption rejected |
| UT-030 | TestInstanceAdoption | Existing instance rejected |
| UT-031 | TestInstanceAdoptAdmission; TestInstanceAdoption | Agent, profile and workspace mismatch |
| UT-084 | TestAgentInstanceDefinitionDeletion; TestAgentInstanceDeletionPagination | Definition deletion blocked, including later pages |
| UT-086 | TestAgentInstanceErrorResponses | Twelve typed instance/adoption errors; statuses follow existing HTTP conventions; external _dx.md catalog not read |
| UT-097 | TestInstanceLifecycle | Resume at cap preserves paused instance |
| UT-098 | TestInstanceAdoptAdmission | Wider policy or workspace grant rejected |
| IT-007 | TestInstanceWorkspaceIsolation | Instance isolation only; wake state excluded as B3 |
| IT-008 | TestInstanceWorkspaceArchive; TestInstanceWorkspaceDeletion | Workspace blockers and deletion |
| IT-009 | TestInstanceNativeCreateJourney; TestInstancePermissionMode | Native child creation, persisted child ceiling, ACP prompts before/after context replacement and approval mode |
| IT-010 | TestInstanceAdoptionHistory | History persists through adoption and reopen |
| IT-036 | TestInstanceLifecycleHooks; TestInstanceAdoptionHooks; TestInstanceHookDispatch | Lifecycle/adoption payloads, real asynchronous native executor, ignored patches, observer failure and failed-adoption suppression |
| IT-037 | TestInstanceServiceConcurrentCreate; TestInstanceConcurrentLimit; TestInstanceAdmission; TestInstanceServiceRecovery; TestInstanceServiceRetainedFailure; TestInstanceAdoption | Concurrent creation, cap, crash binding, cleanup/retry, failed resume/adopt preservation |
| IT-038 | TestInstanceMainRuntimeGuard; TestInstanceLifecycle; TestInstanceMainPromptResume | Main archive/delete refusal and stopped-main prompt resume |
| IT-042 | TestInstanceAuthority; TestInstanceAuthorityIsolation; TestNativeInstanceAuthority; TestNativeInstanceWorkspaceAuthority | Current B2 verbs and identity boundaries; memory/occurrence/check execution excluded with B3 |

Additional reconciliation coverage: `TestInstanceDefinitionReconciliation` uses real SQLite to verify definition removal and kind changes pause existing instances; `TestInstanceCatalogReconciliation` verifies catalog publication/update/removal triggers and tool projection invalidation, including reconciliation failure.

The pre-existing `extensions/open-design/lint.gen.mjs` diff renames minified local `css2` to `css` at its declaration and use. It has no apparent behavior change and is deliberately excluded from B2 commits. Its original generating invocation is unknown.

Permission mode regression: `TestInstancePermissionMode` reproduced an instance ceiling allowing approve-all. The session start boundary now narrows approve-all to approve-reads when an instance ceiling exists; stricter modes are retained.

Follow-up verification: `TestInstancePermissionMode` passes with race detection for approve-all, approve-reads and deny-all through create, resume, context replacement and prompt. `TestInstanceAuthority` now also covers allowed own-instance verbs and rejects unknown operations. These session tests use the existing provider I/O test driver; they do not establish a live-provider or complete native-tool journey.

Instance hook interpretation under the restricted spec read: `agent.instance.created`, `paused`, `resumed`, `archived`, and `adopted` are asynchronous observation events emitted after successful state changes. Payloads contain event/timestamp, instance ID, profile/workspace/agent identity, main session ID, state, pause reason, and creation/update timestamps. They do not expose permission ceilings. This interpretation is inferred from lifecycle operations and existing hook conventions, not verified against `_dx.md`.

Hook verification on 2026-11-01: `CGO_ENABLED=1 go test -race ./internal/hooks ./internal/store/globaldb -run 'TestInstance(HookDispatch|LifecycleHooks|AdoptionHooks)' -count=1` passed using the temporary Go cache. The hooks test exercises the real dispatcher and native executor boundary; repository tests use real SQLite. The conventions checker passes the globaldb suite and flags only pre-existing `TestDispatchEventEmitterContextRoundTrip` in the hooks file, which this change does not modify.

Error boundary repair: wrapped `workspace_access_denied` maps to 403; `session_not_root` and `session_scope_mismatch` map to 422; `agent_instance_paused` maps to 409. The four HTTP regressions failed with 500 before production repair. Admission and repository paths now share error identities, and a denied workspace policy preserves the required workspace-specific code. The HTTP test-shape checker reports two false positives in unchanged report tests: their discarded second return is an `AgentInstanceReportsResponse`, not an error.

Native authority repair: ID-targeted pause/resume/archive/ensure-main/reports now authorize the target instance workspace through the same workspace policy seam as HTTP. The owning native suite verifies unreachable owned instances are rejected before invoking the service and that agents cannot resume a manually paused instance. Memory, occurrence and checks execution belong to the absent B3 surfaces; this does not claim those routes have been tested.

Native creation journey: the tool now passes the workspace registry ID, rather than its filesystem identity, to instance admission. The integration fixture explicitly grants the root workspace path, narrows the child tool set, and asserts the child ceiling remains equal in the main session and SQLite through context replacement. Empty path ceilings remain restrictive. The journey uses an isolated daemon and real ACP mock subprocesses, not a live external model.
