# Task Memory: task_09.md

Keep only task-local execution context here. Do not duplicate facts that are obvious from the repository, task file, PRD documents, or git history.

## Objective Snapshot

- Execute the `task_08` daemon-improvements QA matrix end to end for `task_09`, using `.compozy/tasks/daemon-improvs/analysis/qa/` as the artifact root.
- Leave fresh evidence for transport parity, timeout/client compatibility, runtime shutdown/logging, ACP fault handling, observability, and one external-workspace operator flow.
- If QA exposes a regression, fix it at the root, add/update durable automated coverage, rerun the impacted scenarios, and finish with a clean `make verify`.

## Important Decisions

- Treat the `task_08` regression suite and `TC-*.md` files as the execution matrix seed; do not invent a new QA lane.
- Resolve the generic contract-discovery `web_ui.detected` signal as a false positive for this branch because the worktree has no `web/` directory and no browser automation support.
- Keep browser validation blocked/out of scope unless repository evidence changes during execution.
- Use a real daemon-backed temp Node workspace fixture for the required operator-flow proof and store all captured JSON/stdout/stderr artifacts under `.compozy/tasks/daemon-improvs/analysis/qa/logs/`.

## Learnings

- `make verify` is the only repository-wide verification gate in this worktree; there is no `make test-integration` target, so focused `go test` commands remain the canonical targeted lanes.
- Existing task-tracking files are already dirty in this worktree, so tracking updates for `task_09` must avoid disturbing unrelated entries.
- The seeded `task_08` smoke/targeted suites and the live daemon operator-flow script all passed after setting `COMPOZY_DAEMON_HTTP_PORT=0` for the real-binary QA lane.
- The real operator-flow regression was at the CLI watch boundary: `defaultWatchCLIRun` returned on the terminal event before it had waited for the daemon’s durable terminal snapshot, so immediate post-run inspection could race teardown and state mirroring.
- The low-level daemon API client intentionally reconnects run streams on EOF, so operator-flow tests that use raw `OpenRunStream` should stop after the terminal event rather than waiting for the channel itself to close.

## Files / Surfaces

- `.compozy/tasks/daemon-improvs/analysis/qa/test-plans/daemon-improvs-analysis-test-plan.md`
- `.compozy/tasks/daemon-improvs/analysis/qa/test-plans/daemon-improvs-analysis-regression.md`
- `.compozy/tasks/daemon-improvs/analysis/qa/test-cases/TC-FUNC-001.md`
- `.compozy/tasks/daemon-improvs/analysis/qa/test-cases/TC-FUNC-002.md`
- `.compozy/tasks/daemon-improvs/analysis/qa/test-cases/TC-INT-001.md`
- `.compozy/tasks/daemon-improvs/analysis/qa/test-cases/TC-INT-002.md`
- `.compozy/tasks/daemon-improvs/analysis/qa/test-cases/TC-INT-003.md`
- `.compozy/tasks/daemon-improvs/analysis/qa/test-cases/TC-INT-004.md`
- `.compozy/tasks/daemon-improvs/analysis/qa/test-cases/TC-INT-005.md`
- `.compozy/tasks/daemon-improvs/analysis/qa/logs/`
- `internal/cli`
- `internal/cli/operator_transport_integration_test.go`
- `internal/api/httpapi`
- `internal/api/client`
- `internal/api/contract`
- `internal/daemon`
- `internal/daemon/run_manager.go`
- `internal/daemon/run_manager_test.go`
- `internal/core/run/executor`
- `pkg/compozy/runs`

## Errors / Corrections

- Confirmed regression: stream-attached CLI runs could return before the durable terminal snapshot was observable, which broke immediate run inspection and extension-shutdown audit expectations.
- Correction: `defaultWatchCLIRun` now waits for `waitForTerminalDaemonRunSnapshot(...)` after observing a terminal event, covered by `TestDaemonPublicSnapshotAndStreamMatchAcrossHTTPAndUDSForTempWorkspaceRun` and `TestReviewsExecDaemonStreamHelpers`.
- Correction: live daemon health parity in the operator test now compares the stable contract fields and only sanity-checks DB byte diagnostics, since those values are point-in-time measurements rather than transport identity fields.

## Ready for Next Run

- Remaining work is final artifact writing, workflow/task tracking updates, a fresh `make verify`, and the required local commit after clean self-review.
