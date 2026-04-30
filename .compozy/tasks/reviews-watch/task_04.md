---
status: completed
title: Review Watch Extension Hooks, Docs, and QA Coverage
type: chore
complexity: high
dependencies:
  - task_02
  - task_03
---

# Review Watch Extension Hooks, Docs, and QA Coverage

## Overview

This task completes the extension and operator-facing surface for review watch. It adds Go and TypeScript SDK hooks, updates documentation/reference material, and validates the complete workflow with focused QA coverage.

<critical>
- ALWAYS READ the TechSpec before starting; there is no PRD for this feature
- REFERENCE the TechSpec sections "Extensions", "Monitoring and Observability", and "End-to-End QA"
- FOCUS ON "WHAT" — expose lifecycle extension points without allowing hooks to bypass provider-current clean detection
- MINIMIZE CODE — keep SDK additions aligned with existing hook naming, payload, and capability patterns
- TESTS REQUIRED — SDK parity, hook mutability rules, docs examples, and end-to-end watch flow must be verified
</critical>

<requirements>
1. MUST add `review.watch_pre_round`, `review.watch_post_round`, `review.watch_pre_push`, and `review.watch_finished` hooks to Go and TypeScript SDKs.
2. MUST allow hooks to veto or adjust allowed round/push options only where the TechSpec permits it.
3. MUST prevent extension hooks from skipping provider-current clean detection.
4. MUST document `[watch_reviews]`, CLI usage, event payloads, hook payloads, and auto-push safety behavior.
5. MUST include an end-to-end QA scenario using fake provider responses and a temporary git repository.
6. MUST run `compozy tasks validate --name reviews-watch` and the repository verification gate after implementation.
</requirements>

## Subtasks

- [x] 4.1 Add Go SDK hook names, payload types, patches, and dispatch tests.
- [x] 4.2 Add TypeScript SDK hook constants, payload types, handler matrix entries, fluent helpers, and tests.
- [x] 4.3 Update configuration, command, event, and extension documentation for review watch.
- [x] 4.4 Add end-to-end QA coverage for a two-round watch loop with fake provider and git boundaries.
- [x] 4.5 Run task validation and full project verification, then fix any issues at root cause.

## Implementation Details

Implement extension hooks after the coordinator and CLI are available so payloads match real state transitions. Hook behavior should support controlled customization and vetoes while preserving the daemon state machine and provider clean-detection invariants.

### Relevant Files

- `sdk/extension/hooks.go` — add Go hook names and mutability classification.
- `sdk/extension/types.go` — add Go payload and patch types for watch hooks.
- `sdk/extension/handlers.go` — wire typed hook handling if required by the existing SDK pattern.
- `sdk/extension-sdk-ts/src/types.ts` — add TypeScript hook constants and payload/patch interfaces.
- `sdk/extension-sdk-ts/src/handlers.ts` — add handler matrix entries and mutability classification.
- `sdk/extension-sdk-ts/src/extension.ts` — add fluent registration helpers for watch hooks.
- `skills/compozy/references/config-reference.md` — document `[watch_reviews]` and CLI usage.

### Dependent Files

- `sdk/extension/smoke_test.go` — cover Go SDK hook registration and payload compatibility.
- `sdk/extension-sdk-ts/test/handlers.test.ts` — cover TypeScript hook mutability and registration.
- `sdk/extension-sdk-ts/test/extension.test.ts` — cover fluent helper registration.
- `pkg/compozy/events/docs_test.go` — ensure watch events remain documented.
- `.compozy/tasks/reviews-watch/_techspec.md` — source for hook constraints and QA scenario.

### Related ADRs

- [ADR-001: Use a Daemon-Owned Parent Run for Review Watching](adrs/adr-001.md) — hooks observe and influence daemon parent-run lifecycle.
- [ADR-002: Require Provider Watch Status Before Declaring Reviews Clean](adrs/adr-002.md) — hooks must not skip current-head clean detection.
- [ADR-003: Force Auto-Commit and Allow Dirty Worktrees for Auto-Push Watch Runs](adrs/adr-003.md) — hook and docs behavior must preserve auto-push safety boundaries.

## Deliverables

- Go extension SDK watch hooks and payload types.
- TypeScript extension SDK watch hooks, types, handler matrix entries, and fluent helpers.
- Documentation for `[watch_reviews]`, `reviews watch`, watch events, hooks, and auto-push safety behavior.
- End-to-end QA test or reproducible QA fixture for a two-round watch loop.
- Unit tests with 80%+ coverage for new SDK hook behavior **(REQUIRED)**
- Integration tests for extension-hook interaction with the daemon watch flow **(REQUIRED)**

## Tests

- Unit tests:
  - [x] Go SDK recognizes all four watch hook names and classifies pre-round/pre-push mutability correctly.
  - [x] Go SDK watch hook payloads serialize with provider, PR, workflow, round, run IDs, head SHA, and terminal reason fields.
  - [x] TypeScript SDK exposes all four watch hook constants and typed handler entries.
  - [x] TypeScript fluent helpers register the expected hook names without changing existing review hook behavior.
  - [x] Hook veto for pre-push stops push and records an explicit terminal or stopped reason.
  - [x] Hook attempts to bypass provider-current clean detection are ignored or rejected with a clear error.
- Integration tests:
  - [x] Two-round fake provider/git flow runs through watch, child fix, push, second provider review, and clean completion.
  - [x] Watch events are documented, persisted, streamed, and visible through the existing run event APIs.
  - [x] `compozy tasks validate --name reviews-watch` exits 0 after task files are generated.
- Test coverage target: >=80%
- All tests must pass

## Success Criteria

- All tests passing
- Test coverage >=80%
- Go and TypeScript extension SDKs expose equivalent review watch hook support
- Documentation describes config, CLI usage, lifecycle events, hooks, and auto-push safety accurately
- End-to-end QA proves the automated review loop reaches clean completion without manual waiting or pushing
