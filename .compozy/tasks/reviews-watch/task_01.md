---
status: completed
title: Review Watch Contracts, Config, and Provider Status
type: backend
complexity: high
dependencies: []
---

# Review Watch Contracts, Config, and Provider Status

## Overview

This task creates the typed foundation for `reviews watch`: transport contracts, workspace config, provider watch-status capability, and CodeRabbit current-head status. It must make clean detection explicit before daemon orchestration or CLI work can rely on it.

<critical>
- ALWAYS READ the TechSpec before starting; there is no PRD for this feature
- REFERENCE the TechSpec sections "Core Interfaces", "Data Models", and "GitHub and CodeRabbit" instead of duplicating them here
- FOCUS ON "WHAT" — define stable contracts and provider state, not the daemon loop itself
- MINIMIZE CODE — extend existing fetch/fix/provider patterns rather than creating a parallel provider stack
- TESTS REQUIRED — every changed contract, config path, and provider status branch needs coverage
</critical>

<requirements>
1. MUST add the review watch request and response aliases without breaking existing `reviews fetch` or `reviews fix` contracts.
2. MUST add `[watch_reviews]` config with the TechSpec precedence and validation rules.
3. MUST add a provider watch-status capability that distinguishes pending, stale, current-reviewed, and unsupported states.
4. MUST implement CodeRabbit status using the existing `gh` command-runner boundary and repository metadata resolution style.
5. MUST keep `Provider.FetchReviews` and `Provider.ResolveIssues` backward-compatible.
6. SHOULD use fixture tests from representative GitHub/CodeRabbit payloads for current, stale, pending, and clean states.
</requirements>

## Subtasks

- [x] 1.1 Add review watch API/core contract types and aliases.
- [x] 1.2 Add `[watch_reviews]` workspace config fields, merge behavior, validation, and docs references.
- [x] 1.3 Add provider watch-status types and explicit unsupported-provider error behavior.
- [x] 1.4 Implement CodeRabbit watch status through `gh` REST/GraphQL calls.
- [x] 1.5 Add fixture-backed tests for config precedence and CodeRabbit status classification.

## Implementation Details

Implement the typed contract and provider capability described in the TechSpec "Core Interfaces" and "Data Models" sections. This task should stop at reporting trustworthy provider state; it should not start daemon runs, write review rounds, or push git state.

### Relevant Files

- `internal/api/contract/types.go` — add request/result structures used by transport and client code.
- `internal/api/core/interfaces.go` — expose contract aliases and the future review service method shape.
- `internal/core/workspace/config_types.go` — add `WatchReviewsConfig` to project config.
- `internal/core/workspace/config_merge.go` — merge global/workspace/default watch config values.
- `internal/core/workspace/config_validate.go` — validate positive durations, max rounds, and push target rules.
- `internal/core/provider/provider.go` — add watch-status request/result types and optional capability.
- `internal/core/provider/coderabbit/coderabbit.go` — implement CodeRabbit current-head status through the existing runner seam.

### Dependent Files

- `internal/core/provider/coderabbit/coderabbit_test.go` — add status fixtures and command-runner assertions.
- `internal/core/workspace/config_test.go` — cover config precedence and invalid duration/max-round behavior.
- `skills/compozy/references/config-reference.md` — document `[watch_reviews]` once the config shape is implemented.
- `.compozy/tasks/reviews-watch/_techspec.md` — source of contract and config requirements.

### Related ADRs

- [ADR-002: Require Provider Watch Status Before Declaring Reviews Clean](adrs/adr-002.md) — defines why clean detection requires provider current-head state.
- [ADR-003: Force Auto-Commit and Allow Dirty Worktrees for Auto-Push Watch Runs](adrs/adr-003.md) — constrains auto-push config validation.

## Deliverables

- Review watch request/config/provider status types.
- Workspace config parsing, merge, validation, and reference documentation for `[watch_reviews]`.
- CodeRabbit watch-status implementation with fixture-backed coverage.
- Unit tests with 80%+ coverage for new config and provider status logic **(REQUIRED)**
- Integration-style provider tests through the fake `gh` command runner **(REQUIRED)**

## Tests

- Unit tests:
  - [x] CLI/config precedence input with `[watch_reviews]`, `[fix_reviews]`, `[fetch_reviews]`, and `[defaults]` resolves the expected effective values.
  - [x] `max_rounds = 0` with `until_clean = true` returns a validation error.
  - [x] Negative or zero `poll_interval`, `review_timeout`, and `quiet_period` values return validation errors.
  - [x] Provider without watch-status support returns an explicit unsupported error for watch mode.
  - [x] CodeRabbit status reports pending when the latest review does not match the current PR head.
  - [x] CodeRabbit status reports stale when the provider review commit differs from the current PR head.
  - [x] CodeRabbit status reports reviewed when the latest provider review matches the current PR head.
- Integration tests:
  - [x] Fake `gh` payloads for PR metadata, reviews, review threads, and review-body comments produce deterministic watch status and unchanged fetch normalization.
- Test coverage target: >=80%
- All tests must pass

## Success Criteria

- All tests passing
- Test coverage >=80%
- Provider current-head status is available without changing existing fetch/fix provider behavior
- Invalid watch config fails before daemon orchestration begins
- CodeRabbit status tests cover pending, stale, current-reviewed, unsupported, and malformed payload paths
