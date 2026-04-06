# Task Memory: task_01.md

Keep only task-local execution context here. Do not duplicate facts that are obvious from the repository, task file, PRD documents, or git history.

## Objective Snapshot
- Implemented the public `pkg/compozy/events` package, the `kinds` payload subpackage, bounded generic bus, required unit/integration tests, and verified the work with `go vet ./...`, focused package tests, coverage checks, and `make verify`.

## Important Decisions
- Kept all event payload contracts public and self-contained instead of exposing `internal/core/model` types through the new `pkg/` API.
- Matched the ADR-003 taxonomy with 36 event kinds and shaped payload structs to cover the executor and TUI fields later tasks will need.
- Used per-subscription synchronization inside the bus so snapshot publish stays non-blocking while unsubscribe and close cannot race a send into a closed channel.

## Learnings
- `go test -cover ./pkg/compozy/events/...` finished at `82.4%` for `pkg/compozy/events` and `86.2%` for `pkg/compozy/events/kinds`.
- `DroppedFor(id)` is meaningful only while the subscription is still registered; integration assertions need to read drop counts before `Close` or `unsub`.

## Files / Surfaces
- `pkg/compozy/events/event.go`
- `pkg/compozy/events/bus.go`
- `pkg/compozy/events/event_test.go`
- `pkg/compozy/events/bus_test.go`
- `pkg/compozy/events/bus_integration_test.go`
- `pkg/compozy/events/kinds/run.go`
- `pkg/compozy/events/kinds/job.go`
- `pkg/compozy/events/kinds/session.go`
- `pkg/compozy/events/kinds/prompt.go`
- `pkg/compozy/events/kinds/tool_call.go`
- `pkg/compozy/events/kinds/usage.go`
- `pkg/compozy/events/kinds/task.go`
- `pkg/compozy/events/kinds/review.go`
- `pkg/compozy/events/kinds/provider.go`
- `pkg/compozy/events/kinds/shutdown.go`
- `pkg/compozy/events/kinds/session_test.go`

## Errors / Corrections
- Initial `make verify` failed on `unparam` because the content-block validator returned an unused value; corrected it by replacing `decodeContentBlock` with `validateContentBlock` for validation-only use.

## Ready for Next Run
- Tracking updates and the local commit still need to respect the repo rule that tracking-only files should stay out of the automatic commit unless explicitly required.
