## TC-FUNC-005: Exec Flow Compatibility Over the Daemon

**Priority:** P1 (High)
**Type:** Functional
**Status:** Not Run
**Estimated Time:** 15 minutes
**Created:** 2026-04-18
**Last Updated:** 2026-04-18
**Automation Target:** E2E
**Automation Status:** Existing
**Automation Command/Spec:**
- `go test ./internal/cli -run 'Test(ExecCommandUsesDaemonLifecycleAcrossFormats|ExecCommandExecuteStdinWorksEndToEnd|ExecCommandExecutePromptFileJSONEmitsJSONLByDefault|ExecCommandExecuteRunIDUsesPersistedRuntimeDefaults)' -count=1`
- Supporting seams:
- `go test ./internal/daemon -run 'Test(RunManagerStartExecRunCancelsAndGetReturnsUpdatedRow|RunManagerExecRunCompletesAndReplaysPersistedStream|RunManagerExecRunFailureMarksRunFailed)' -count=1`
- `go test ./internal/api/client -run 'TestClientExecRequestAndGuardErrors' -count=1`
**Automation Notes:** This lane locks the daemon-backed exec contract for prompt sources, persisted state, and JSON/raw-JSON compatibility.

### Objective

Verify that `compozy exec` now starts a daemon-managed run bound to the current workspace, persists `mode=exec`, and preserves existing prompt/stdin/output behavior.

### Preconditions

- [ ] Exec fixtures are available for prompt text, prompt file, stdin, JSON, and raw-JSON variants.
- [ ] Isolated temp home/runtime directories are available for persisted-run scenarios.

### Test Steps

1. Run the focused exec CLI suite listed above.
   **Expected:** The package exits `0` and each public exec format variant passes.

2. Confirm the suite covers stdin input, prompt-file JSON output, and persisted run ID reuse/defaults.
   **Expected:** Output shape and persisted-run behavior stay compatible with the daemon lifecycle.

3. Run the supporting daemon-manager and API-client suites if deeper lifecycle evidence is needed.
   **Expected:** Exec runs persist daemon-owned state, terminal replay remains available, failures mark the run correctly, and client request encoding stays stable.

### Edge Cases & Variations

| Variation | Input | Expected Result |
|---|---|---|
| Stdin prompt | Prompt from stdin | Exec completes through daemon lifecycle without losing input |
| Prompt file JSON mode | Prompt file plus JSON output | Lean JSONL output stays stable |
| Persisted run ID | Existing run ID resume/defaults | Persisted runtime defaults are reused correctly |
| Exec failure | Daemon-backed failure path | Terminal state is durable and failure is surfaced correctly |

### Related Test Cases

- `TC-FUNC-004`
- `TC-INT-003`

### Traceability

- TechSpec Integration Tests: `compozy exec` binds to the current workspace, auto-registers it, skips workflow sync, and persists `mode=exec`.
- ADR-002: exec runtime state is daemon-owned operational data.
- ADR-004: exec remains a user-facing CLI surface.
- Task reference: `task_15`.

### Notes

- If a bug appears here, the fix should update the narrowest daemon/CLI exec regression rather than broadening unrelated workflow tests.
