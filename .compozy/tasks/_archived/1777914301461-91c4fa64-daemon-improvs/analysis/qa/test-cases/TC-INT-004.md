## TC-INT-004: ACP Liveness, Fault Handling, and Reconcile Honesty

**Priority:** P1 (High)
**Type:** Integration
**Status:** Passed
**Estimated Time:** 20 minutes
**Created:** 2026-04-21
**Last Updated:** 2026-04-21
**Automation Target:** Integration
**Automation Status:** Existing with required daemon-E2E follow-up
**Automation Command/Spec:**
- `go test ./internal/core/run/executor -run 'Test(ExecuteJobWithTimeoutACPFullPipelineRoutesTypedBlocks|ExecuteJobWithTimeoutACPCycleBlockKeepsParentSessionUsable|JobRunnerACPErrorThenSuccessRetries|ExecuteACPJSONModeWritesStructuredFailureResult|ExecuteACPJSONModeWritesStructuredSuccessResult|ExecuteJobWithTimeoutACPSubcommandRuntimeUsesLaunchSpec|JobExecutionContextLaunchWorkersRunsMultipleACPJobs|JobExecutionContextLaunchWorkersReturnsPromptlyWithPendingACPJobs|ExecuteJobWithTimeoutActiveACPUpdatesExtendTimeout|JobRunnerRetriesRetryableACPSetupFailureThenSucceeds|JobRunnerDoesNotRetryNonRetryableACPSetupFailure)' -count=1`
- `go test ./internal/daemon -run 'TestStartRemainsHealthyWhenInterruptedRunDBIsMissingOrCorrupt' -count=1`
**Automation Notes:** Current automation proves executor-scoped ACP behaviors and reconcile honesty around interrupted runs. It does not yet prove that a managed daemon surfaces the same failures through public task/review/exec flows, so that follow-up is mandatory in `task_09`.

### Objective

Verify that ACP-backed runtime hardening still handles retries, structured failures, pending-job exit, parent-session recovery, and reconcile-adjacent failure honesty without silently masking daemon risk.

### Preconditions

- [ ] The ACP helper fixture remains available in `internal/core/run/executor`.
- [ ] The daemon reconcile test can simulate interrupted-run state in the current branch.
- [ ] No planning artifact assumes a non-existent shared `internal/testutil/acpmock` package.

### Test Steps

1. Run the focused executor ACP command listed above.
   **Expected:** The package exits `0` and the helper-backed ACP scenarios pass, including retries, structured success/failure payloads, and pending-job handling.

2. Run the reconcile honesty command listed above.
   **Expected:** The daemon remains healthy and reports interrupted-run issues honestly when run DB state is missing or corrupt.

3. Confirm the current automation remains integration-only for daemon public flows.
   **Expected:** The artifact records the missing daemon-backed E2E proof rather than claiming it implicitly.

4. Promote any public-surface ACP gap found during execution into a dedicated `task_09` follow-up.
   **Expected:** Public daemon task/review/exec fault surfacing becomes an explicit execution objective.

### Edge Cases & Variations

| Variation | Input | Expected Result |
|---|---|---|
| Retryable ACP setup failure | Transient ACP error on first attempt | Retry occurs and later success is accepted |
| Non-retryable setup failure | Hard ACP setup failure | No silent retry loop |
| Heartbeat/active updates | Ongoing ACP activity before timeout | Timeout extension logic prevents false failure |
| Pending ACP jobs | Worker returns while jobs remain pending | Prompt return without hanging the runtime forever |
| Interrupted run DB damage | Corrupt or missing persisted run state | Daemon reports degraded honesty without pretending the run is healthy |

### Related Test Cases

- `TC-INT-003`
- `TC-FUNC-002`

### Traceability

- TechSpec `Integration Points`
- TechSpec `Testing Approach`
- TechSpec `Known Risks`
- ADR-002: incremental runtime supervision hardening
- ADR-003: validation-first daemon hardening
- Task reference: `task_06.md`

### Notes

- This case intentionally distinguishes between strong executor integration coverage and missing daemon-E2E proof. Task `09` must not collapse those into the same confidence claim.
