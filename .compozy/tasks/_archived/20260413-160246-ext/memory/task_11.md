# Task Memory: task_11.md

Keep only task-local execution context here. Do not duplicate facts that are obvious from the repository, task file, PRD documents, or git history.

## Objective Snapshot

- Add the remaining job/run/review/artifact hook dispatches required by task 11 and verify them with unit plus spawned-extension integration coverage.

## Important Decisions

- Reuse the existing generic runtime-manager hook interface from task 10 instead of introducing executor-specific extension code paths.
- Keep nil-manager behavior as an immediate no-op at each insertion point so disabled-extension runs stay behaviorally unchanged.
- Model `review.pre_fetch`, `review.post_fetch`, and `review.pre_batch` in `plan.Prepare`, because that is the real fix-reviews ingestion/batching seam before jobs are materialized.
- Route `artifact.pre_write` / `artifact.post_write` through one shared `writeArtifactFile` helper so both `host.artifacts.write` and `host.tasks.create` pick up the same hook behavior.

## Learnings

- Pre-change, the task-11 hook names only exist in `internal/core/extension/manifest.go`; there are no runtime dispatch call sites yet.
- The protocol rows for `review.pre_fetch`, `review.post_fetch`, `review.pre_batch`, `review.post_fix`, and `review.pre_resolve` need concrete payload helpers because `FetchConfig`, `FixOutcome`, and `RunSummary` are not defined anywhere in the current codebase.
- Group-key mutations from `plan.post_group` / `review.pre_batch` must be normalized back onto `IssueEntry.CodeFile` before batching, otherwise `prepareJobs` silently loses renamed groups when it re-groups by code file.
- `review.pre_resolve.resolve = false` is enough to skip provider-backed issue resolution cleanly, which makes it the correct no-network integration seam for spawned-extension tests.
- Observer-hook completion order is asynchronous; spawned-extension recorders should assert presence or partial ordering for observer hooks instead of strict completion order against later mutable hooks.

## Files / Surfaces

- `internal/core/model/hook_types.go`
- `internal/core/run/executor/hooks.go`
- `internal/core/run/executor/execution.go`
- `internal/core/run/executor/runner.go`
- `internal/core/run/executor/shutdown.go`
- `internal/core/run/executor/review_hooks.go`
- `internal/core/plan/prepare.go`
- `internal/core/extension/host_writes.go`
- `internal/core/extension/host_api_errors.go`
- `internal/core/extension/runtime.go`
- `internal/core/extension/host_helpers.go`
- `internal/core/run/executor/execution_test.go`
- `internal/core/plan/prepare_test.go`
- `internal/core/extension/host_writes_test.go`
- `internal/core/extension/hooks_integration_test.go`
- `internal/core/extension/testdata/mock_extension/main.go`

## Errors / Corrections

- Fixed a semantic regression where hook-mutated review group keys were dropped during batching; `prepareJobs` now flattens normalized group entries so hook-renamed code files survive into the prepared jobs.
- Refactored `prepareWorkflowRun`, `resolvePreparedEntries`, `Execute`, and `resolveProviderBackedIssues` after `make verify` surfaced `funlen` / `gocyclo` lint failures.
- Relaxed spawned-extension ordering assertions for observer hooks after confirming the runtime contract dispatches them before later mutable hooks, but their recorded completion order can lag because observer delivery is async.

## Ready for Next Run

- Task 11 implementation is complete. `make verify` passed cleanly, targeted coverage is `internal/core/run/executor 80.6%` and `internal/core/extension 80.4%`, and the remaining wrap-up is staging only the task-11 code/memory/tracking files for the local commit.
