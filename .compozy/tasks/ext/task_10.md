---
status: completed
title: Plan, prompt, and agent phase hook dispatches
type: backend
complexity: high
dependencies:
  - task_09
---

# Task 10: Plan, prompt, and agent phase hook dispatches

## Overview
Insert mutable and observer hook dispatch calls at the plan, prompt, and agent pipeline boundaries so extensions can participate in how runs are built and how agents are invoked. This task covers 14 of the 26 hook events: 6 plan hooks, 3 prompt hooks, and 5 agent hooks. Each insertion is a thin call to `Manager.DispatchMutable` or `Manager.DispatchObserver` at the right place in the existing code; the interesting engineering is picking the correct boundary and passing the right payload.

<critical>
- ALWAYS READ the PRD and TechSpec before starting
- REFERENCE TECHSPEC for implementation details — do not duplicate here
- FOCUS ON "WHAT" — describe what needs to be accomplished, not how
- MINIMIZE CODE — show code only to illustrate current structure or problem areas
- TESTS REQUIRED — every task MUST include tests in deliverables
- NOTE: No `_prd.md` exists. Requirements derive from `_techspec.md` and `_protocol.md` section 6.5.
</critical>

<requirements>
- MUST insert `DispatchMutable` calls for the six `plan.*` hooks around `resolvePreparedEntries`, `groupIssuesByCodeFile`, and `prepareJobs` inside `internal/core/plan/prepare.go`.
- MUST insert `DispatchMutable` calls for the three `prompt.*` hooks around `Build` and `BuildSystemPromptAddendum` inside `internal/core/prompt/common.go`.
- MUST insert `DispatchMutable` calls for `agent.pre_session_create` and `agent.pre_session_resume` inside `internal/core/agent/client.go` (or wherever the client resolves its request before calling into the ACP subprocess).
- MUST insert `DispatchObserver` calls for `agent.post_session_create`, `agent.on_session_update`, and `agent.post_session_end`.
- MUST honor the payload and patch shapes defined in `_protocol.md` section 6.5 for every inserted hook.
- MUST pass a nil-safe manager reference: when the run scope disables executable extensions, the dispatch calls must be no-ops and must not allocate anything.
- MUST NOT change any existing behavior when the manager is nil or empty.
- MUST NOT modify the wire shape of `BatchParams`, `SessionRequest`, or `ResumeSessionRequest` beyond what is strictly needed to pass them through the dispatch boundary.
</requirements>

## Subtasks
- [x] 10.1 Insert the six plan hooks in `plan.Prepare` at the boundaries before and after `resolvePreparedEntries`, `groupIssuesByCodeFile`, and `prepareJobs`.
- [x] 10.2 Insert the three prompt hooks in `prompt.Build` and `prompt.BuildSystemPromptAddendum`: `prompt.pre_build` before template render, `prompt.post_build` after render, `prompt.pre_system` on the system addendum path.
- [x] 10.3 Insert `agent.pre_session_create` and `agent.pre_session_resume` in the agent client before spawning the ACP session.
- [x] 10.4 Insert `agent.post_session_create` as an observer emission after session creation.
- [x] 10.5 Insert `agent.on_session_update` as an observer emission inside the session update channel reader loop.
- [x] 10.6 Insert `agent.post_session_end` as an observer emission when the session finishes.
- [x] 10.7 Write tests covering: nil-manager no-op path, successful mutation of plan entries, prompt text mutation, and session request mutation.

## Implementation Details
See `_protocol.md` section 6.5 for the canonical payload and patch shapes per hook, and TechSpec "System Architecture → Data Flow → Hook dispatch (mutable)" for the dispatch flow.

Modified files:
- `internal/core/plan/prepare.go` — add 6 plan hook dispatches
- `internal/core/prompt/common.go` — add 3 prompt hook dispatches
- `internal/core/agent/client.go` — add agent pre/post session create/resume hooks
- `internal/core/agent/session.go` — add `agent.on_session_update` observer dispatch
- `internal/core/plan/prepare_test.go` — test plan hook insertion paths
- `internal/core/extension/hooks_integration_test.go` — new integration test covering the three phases together

Key invariants:
- Every hook dispatch happens exactly once per real event (no double-dispatch).
- Mutable dispatches receive the most-current pipeline value and their returned patch replaces it.
- Observer dispatches never block the pipeline.
- Nil-manager (extensions disabled) path allocates zero heap memory for the dispatch call.

### Relevant Files
- `internal/core/plan/prepare.go` — Lines for `resolvePreparedEntries`, `groupIssuesByCodeFile`, `prepareJobs`.
- `internal/core/prompt/common.go` — `Build` and `BuildSystemPromptAddendum` function bodies.
- `internal/core/agent/client.go` — Session creation path.
- `internal/core/agent/session.go` — Update channel reader loop.
- `internal/core/extension/manager.go` — From task 08. Provides `DispatchMutable` / `DispatchObserver`.
- `_protocol.md` section 6.5 — Hook event matrix.

### Dependent Files
- Task 14 (Go SDK) depends on the payload shapes being stable after this task lands.
- Task 15 (TS SDK) depends on the same shapes.

### Related ADRs
- [ADR-004: Priority-Ordered Mutation Pipeline for Hooks](adrs/adr-004.md) — Dispatch semantics used here.

## Deliverables
- 14 hook dispatches inserted in plan, prompt, and agent phases.
- Unit tests with 80%+ coverage for the new dispatch paths **(REQUIRED)**
- Integration test running a mock extension through a fixture plan and verifying mutation effects **(REQUIRED)**

## Tests
- Unit tests:
  - [x] With the manager nil, `plan.Prepare` returns exactly the same output as before this task on a fixture workflow.
  - [x] With a mock extension that appends a marker to `plan.post_discover` entries, `plan.Prepare` output contains the marker.
  - [x] With a mock extension that mutates `plan.post_group` groups, the final `prepareJobs` input reflects the mutation.
  - [x] With a mock extension that appends to `prompt.post_build`, the rendered prompt contains the appended text.
  - [x] With a mock extension that mutates `agent.pre_session_create.session_request.prompt`, the ACP client receives the mutated prompt.
  - [x] `agent.on_session_update` is dispatched for every session update without blocking the update channel reader.
  - [x] `agent.post_session_end` is dispatched once per session termination, regardless of success or error.
- Integration tests:
  - [x] End-to-end run with a mock extension chain (priority 100, 500, 900) covering plan, prompt, and agent phases produces expected mutations at each step.
- Test coverage target: >=80%
- All tests must pass

## Success Criteria
- All tests passing
- Test coverage >=80%
- `make verify` exits zero with zero lint issues
- Existing plan/prompt/agent tests still pass without change
- Mutations from mock extensions are observable in run artifacts (prompt files, job definitions)
