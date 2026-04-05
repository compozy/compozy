---
status: resolved
file: internal/core/run/result.go
line: 98
severity: medium
author: claude-code
provider_ref:
---

# Issue 001: jobStatusOrDefault masks empty job status as succeeded

## Review Comment

`jobStatusOrDefault` treats any job with an empty `status` field as
`"succeeded"`, which can misrepresent failure scenarios in the JSON result
payload consumed by automation.

```go
func jobStatusOrDefault(status string) string {
    if strings.TrimSpace(status) == "" {
        return runStatusSucceeded
    }
    return status
}
```

`job.status` is only set by the lifecycle helpers (`markSuccess`,
`markGiveUp`, `markCanceled`, `markRetry`). If `executeJobsWithGracefulShutdown`
fails before any worker runs — for example, `newJobExecutionContext` returns
an error from `os.Getwd()` (execution.go:549-553) — the returned `failures`
slice has one entry but every job in `internalJobs` still carries an empty
`status`. `buildExecutionResult` then reports overall
`status: "failed"` with every per-job `status: "succeeded"`, which is
contradictory and hard to interpret from a JSON consumer.

The same mismatch can surface if a future code path forgets to call a
lifecycle finalizer; defaulting to `"succeeded"` hides the drift instead of
surfacing it.

**Suggested fix:** do not invent a terminal status. Default empty to a
neutral marker that clearly isn't a positive outcome (e.g. `"unknown"` or
`"pending"`), or propagate the overall run status when a job never reached
a terminal lifecycle phase. Whatever sentinel you pick, make sure it is
consistent with `deriveRunStatus` so the top-level and per-job statuses
tell the same story.

## Triage

- Decision: `valid`
- Notes:
  Root cause confirmed in `internal/core/run/result.go`: `buildExecutionResult` derives the top-level run status first, but `jobStatusOrDefault` still rewrites any blank per-job status to `"succeeded"`.
  This creates a contradictory JSON payload when execution fails before any lifecycle finalizer runs, such as the `newJobExecutionContext` early-return path in `internal/core/run/execution.go`.
  Fix approach: stop inventing success for blank job statuses. Keep the top-level status derivation unchanged, and make per-job status fall back to a neutral non-success value with regression coverage in `internal/core/run/result_test.go`.
  Verified with focused package tests and a clean `make verify`; `internal/core/run/result_test.go` now covers the blank-status failure path explicitly.
