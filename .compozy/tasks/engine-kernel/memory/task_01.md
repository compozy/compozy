# Task Memory: task_01.md

Keep only task-local execution context here. Do not duplicate facts that are obvious from the repository, task file, PRD documents, or git history.

## Objective Snapshot
- Implemented the public `pkg/compozy/events` package, the `kinds` payload subpackage, bounded generic bus, required unit/integration tests, and re-verified the repository with focused package tests, coverage checks, targeted race tests, and `make verify`.

## Important Decisions
- Kept all event payload contracts public and self-contained instead of exposing `internal/core/model` types through the new `pkg/` API.
- Matched the explicit ADR-003 taxonomy table with 33 public event kinds across 9 domains and shaped payload structs to cover the executor and TUI fields later tasks will need.
- Used per-subscription synchronization inside the bus so snapshot publish stays non-blocking while unsubscribe and close cannot race a send into a closed channel.
- Fixed the unrelated `internal/core/run` verification blocker by serializing `captureExecuteStreams` around temporary `os.Stdout`/`os.Stderr` replacement instead of weakening parallel test coverage.

## Learnings
- `go test -cover ./pkg/compozy/events/...` finished at `84.6%` for `pkg/compozy/events` and `86.8%` for `pkg/compozy/events/kinds`.
- `DroppedFor(id)` is meaningful only while the subscription is still registered; integration assertions need to read drop counts before `Close` or `unsub`.
- `make verify` initially failed on a race in `internal/core/run` because multiple parallel tests mutated global process stdio through `captureExecuteStreams`; a package-level mutex resolved the root cause and restored clean race-detector coverage.

## Files / Surfaces
- `internal/core/run/execution_acp_integration_test.go`
- `pkg/compozy/events/event.go`
- `pkg/compozy/events/bus.go`
- `pkg/compozy/events/event_test.go`
- `pkg/compozy/events/bus_test.go`
- `pkg/compozy/events/bus_integration_test.go`
- `pkg/compozy/events/kinds/run.go`
- `pkg/compozy/events/kinds/job.go`
- `pkg/compozy/events/kinds/session.go`
- `pkg/compozy/events/kinds/tool_call.go`
- `pkg/compozy/events/kinds/usage.go`
- `pkg/compozy/events/kinds/task.go`
- `pkg/compozy/events/kinds/review.go`
- `pkg/compozy/events/kinds/provider.go`
- `pkg/compozy/events/kinds/shutdown.go`
- `pkg/compozy/events/kinds/session_test.go`

## Errors / Corrections
- Initial `make verify` failed on `unparam` because the content-block validator returned an unused value; corrected it by replacing `decodeContentBlock` with `validateContentBlock` for validation-only use.
- A fresh repository verification in this run failed on race-detector warnings in `internal/core/run`; corrected the helper to serialize global stdio capture before rerunning the full pipeline successfully.

## Ready for Next Run
- Task tracking should mark `task_01` complete, but tracking-only updates should remain out of the automatic commit unless explicitly required by the repo.
