---
status: pending
title: Executor integration and post-execution event emission
type: refactor
complexity: critical
dependencies:
  - task_01
  - task_02
  - task_03
  - task_04
---

# Task 05: Executor integration and post-execution event emission

## Overview
Thread the journal and event bus through `run.Execute` and `plan.Prepare`, replace all seven `uiCh <- jobXxxMsg{}` send sites in `execution.go` with `journal.Submit`, rewrite `HandleUpdate` in `logging.go` to emit `session.update` events via the journal, and instrument the post-execution helpers (`afterTaskJobSuccess`, `afterReviewJobSuccess`, `resolveProviderBackedIssues`) to emit task/review/provider events per ADR-003's emit-after-success and warn-and-continue policies. This is the critical refactor that makes `events.jsonl` the canonical source of truth.

<critical>
- ALWAYS READ the PRD and TechSpec before starting
- REFERENCE TECHSPEC for implementation details — do not duplicate here
- FOCUS ON "WHAT" — describe what needs to be accomplished, not how
- MINIMIZE CODE — show code only to illustrate current structure or problem areas
- TESTS REQUIRED — every task MUST include tests in deliverables
</critical>

<requirements>
- MUST extend `plan.Prepare` to construct the per-run `*journal.Journal` instance using `RunArtifacts.EventsPath` and the event bus reference
- MUST thread `*journal.Journal` + `*events.Bus[events.Event]` through `run.Execute` into worker context
- MUST replace all seven `uiCh <- ...` send sites in `execution.go` at the original lines (300, 312, 333, 335, 359, 388, 610) with `journal.Submit(ctx, <typed event>)`
- MUST rewrite `sessionUpdateHandler.HandleUpdate` in `logging.go:83` to call `journal.Submit` with a `session.update` event; remove both existing drop sites at lines 105 and 117
- MUST NOT send to `uiCh` directly from executor code after this task (keep `uiCh` field declared for now — it is removed in task_06)
- MUST instrument `afterTaskJobSuccess` to emit `task.file_updated` AFTER `tasks.MarkTaskCompleted` succeeds, and `task.metadata_refreshed` AFTER `tasks.RefreshTaskMeta` succeeds
- MUST instrument `afterReviewJobSuccess` to emit `review.status_finalized` AFTER `reviews.FinalizeIssueStatuses`, `review.round_refreshed` AFTER `reviews.RefreshRoundMeta`
- MUST instrument `resolveProviderBackedIssues` to emit `provider.call_started` BEFORE the API call, then `provider.call_completed` OR `provider.call_failed` with latency/status, plus `review.issue_resolved` (carrying `provider_posted` bool) per ADR-003 Policy 2
- MUST follow warn-and-continue policy for provider call failures per ADR-003 (do not fail the run)
- MUST preserve existing executor shutdown controller behavior (graceful drain, SIGINT, activity timeout)
- MUST preserve existing retry/backoff logic in job workers
</requirements>

## Subtasks
- [ ] 5.1 Extend `plan.Prepare` to construct the `*journal.Journal` and return it in `SolvePreparation`
- [ ] 5.2 Thread journal + bus through `run.Execute` signature and jobExecutionContext
- [ ] 5.3 Replace 7 `uiCh <- ...` send sites in `execution.go` with `journal.Submit` and corresponding typed event payloads
- [ ] 5.4 Rewrite `HandleUpdate` in `logging.go` to emit `session.update` events via journal (no uiCh writes)
- [ ] 5.5 Instrument `afterTaskJobSuccess` with `task.file_updated` + `task.metadata_refreshed` emit-after-success calls
- [ ] 5.6 Instrument `afterReviewJobSuccess` with `review.status_finalized` + `review.round_refreshed` emit-after-success calls
- [ ] 5.7 Instrument `resolveProviderBackedIssues` with provider call lifecycle events and `review.issue_resolved`
- [ ] 5.8 Update executor tests to assert events on the bus rather than on `uiCh`
- [ ] 5.9 Write integration tests for full executor → journal → events.jsonl roundtrip including post-exec events

## Implementation Details
See TechSpec "Core Interfaces" + "Build Order" steps 7 and 10. ADR-004 specifies the journal-upstream-of-fanout semantics that this integration implements. ADR-003 "Mutation/Event Ordering Policy" defines emit-after-success for task/review events and emit-on-attempt+completion for provider events, plus the warn-and-continue failure policy. The 7 `uiCh` send sites and the 2 `HandleUpdate` drop sites are located at the exact line numbers from the explore map. This task does NOT remove the `uiCh` field — task_06 does that once the TUI adapter is in place.

### Relevant Files
- `internal/core/plan/prepare.go:24` — `Prepare()` extended to construct journal
- `internal/core/plan/prepare.go` — `SolvePreparation` struct gains journal handle field
- `internal/core/run/execution.go:27-112` — `Execute` signature extended with journal + bus
- `internal/core/run/execution.go:300,312,333,335,359,388,610` — seven uiCh send sites to replace
- `internal/core/run/execution.go:733-824` — post-execution helpers to instrument
- `internal/core/run/execution.go:744` — `afterTaskJobSuccess` implementation
- `internal/core/run/execution.go:775` — `afterReviewJobSuccess` implementation
- `internal/core/run/execution.go:827` — `resolveProviderBackedIssues` implementation with provider API calls
- `internal/core/run/logging.go:83-130` — `sessionUpdateHandler.HandleUpdate` with two drop sites at 105 and 117
- `internal/core/run/exec_flow.go` — exec-mode event writer to replace with shared journal.Submit
- `internal/core/tasks/store.go:43` — `MarkTaskCompleted` whose successful return triggers `task.file_updated` event
- `internal/core/reviews/store.go:198` — `FinalizeIssueStatuses` and `RefreshRoundMeta` targets

### Dependent Files
- `internal/core/run/ui_model.go` (task_06) — TUI currently reads from `uiCh`; this task stops WRITING to uiCh but leaves the channel field; task_06 wires the bus subscriber adapter
- `internal/core/run/execution_test.go`, `execution_ui_test.go`, `logging_test.go`, `execution_acp_test.go` — test assertions must migrate from uiCh observation to bus subscription

### Related ADRs
- [ADR-002: Custom Event Bus with Bounded Per-Subscriber Backpressure](adrs/adr-002.md) — bus receives events AFTER journal append
- [ADR-003: Event Taxonomy with Schema Versioning and Complete Side-Effect Coverage](adrs/adr-003.md) — emission policies (emit-after-success, emit-on-attempt+completion, warn-and-continue)
- [ADR-004: Journal Upstream of Fanout with Single-Writer Per-Run Model](adrs/adr-004.md) — defines seq assignment, batch flush, journal ownership

## Deliverables
- `plan.Prepare` constructs per-run journal and includes it in returned preparation
- `run.Execute` accepts journal + bus and threads them through worker context
- Seven `uiCh` send sites rewritten as `journal.Submit` calls emitting typed job lifecycle events
- `HandleUpdate` rewritten to emit `session.update` events via journal (zero drop sites remaining in logging.go)
- Post-execution helpers instrumented with 8-10 journal.Submit call sites emitting task/review/provider events
- Provider call failure path emits `provider.call_failed` and continues run (warn-and-continue)
- Unit + integration tests asserting event sequences on bus and in `events.jsonl` **(REQUIRED)**
- Test coverage >=80% for executor integration paths **(REQUIRED)**

## Tests
- Unit tests:
  - [ ] `plan.Prepare` returns a `SolvePreparation` carrying a non-nil journal handle pointing at `RunArtifacts.EventsPath`
  - [ ] Executor worker emits `job.started` event on job begin, `job.completed` on successful finish, `job.failed` on permanent failure, `job.retry_scheduled` on retry
  - [ ] Executor shutdown controller state transitions emit `shutdown.draining` and `shutdown.terminated` events
  - [ ] `HandleUpdate` emits one `session.update` event per ACP session update with payload wrapping `model.SessionUpdate`
  - [ ] `HandleUpdate` emits `usage.updated` event when ACP session update includes non-zero usage
  - [ ] `afterTaskJobSuccess` emits `task.file_updated` AFTER `MarkTaskCompleted` returns nil
  - [ ] `afterTaskJobSuccess` does NOT emit `task.file_updated` when `MarkTaskCompleted` returns error
  - [ ] `afterReviewJobSuccess` emits `review.status_finalized` AFTER `FinalizeIssueStatuses` returns nil, then `review.round_refreshed` after `RefreshRoundMeta`
  - [ ] `resolveProviderBackedIssues` emits `provider.call_started` BEFORE provider API call, then `provider.call_completed` with status code on 2xx
  - [ ] `resolveProviderBackedIssues` emits `provider.call_failed` on non-2xx, run continues without error propagation
  - [ ] `review.issue_resolved` carries `provider_posted=false` when provider call failed
- Integration tests:
  - [ ] PRD-tasks mode run end-to-end: asserts seq-ordered events.jsonl contains `run.started`, N × `job.started`, N × `session.update`, N × `job.completed`, N × `task.file_updated`, N × `task.metadata_refreshed`, `run.completed`
  - [ ] PR-review mode run with successful provider resolution: events.jsonl contains expected sequence including `provider.call_completed` and `review.issue_resolved{provider_posted=true}`
  - [ ] PR-review mode run with failing provider API: events.jsonl contains `provider.call_failed`, `review.issue_resolved{provider_posted=false}`, run still completes with `run.completed`
  - [ ] Bus subscriber receives same events (with identical seq) that are written to `events.jsonl`
- Test coverage target: >=80%
- All tests must pass

## Success Criteria
- All tests passing
- Test coverage >=80%
- Zero golangci-lint issues
- `grep "uiCh <-" internal/core/run/` finds zero matches in execution.go and logging.go
- events.jsonl contains every side effect (task/review file mutations, provider API calls) in the run
- Provider call failure does not fail the run (warn-and-continue policy enforced)
- `go test -race` passes for executor and logging packages
