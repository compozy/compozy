---
status: completed
title: Reviews Watch API, Client, and CLI Surface
type: backend
complexity: high
dependencies:
  - task_02
---

# Reviews Watch API, Client, and CLI Surface

## Overview

This task exposes the daemon watch coordinator through the HTTP/UDS transport, Go API client, and `compozy reviews watch` CLI. It must preserve the existing review command UX while adding watch-specific flags, attach behavior, output formatting, and auto-push validation.

<critical>
- ALWAYS READ the TechSpec before starting; there is no PRD for this feature
- REFERENCE the TechSpec sections "API Endpoints" and "CLI Contract" instead of duplicating request structures here
- FOCUS ON "WHAT" — surface the daemon coordinator without reimplementing watch behavior in the CLI
- MINIMIZE CODE — reuse existing daemon bootstrap, attach mode, JSON output, and review fix flag helpers
- TESTS REQUIRED — CLI parsing, transport status codes, client method behavior, and attach/detach output must be covered
</critical>

<requirements>
1. MUST add `POST /api/reviews/:slug/watch` returning a normal daemon run response.
2. MUST add client support for starting review watch runs with the typed watch request.
3. MUST add `compozy reviews watch [slug]` with watch flags and existing review-fix runtime/batching flags.
4. MUST force `auto_commit=true` when `--auto-push` is set and reject contradictory `--auto-push --auto-commit=false` input before daemon work begins.
5. MUST allow dirty worktrees at CLI startup and leave dirty-state warnings to the daemon event stream.
6. MUST support `--attach`, `--ui`, `--stream`, `--detach`, `--format json`, and `--format raw-json` through existing daemon run observation behavior.
</requirements>

## Subtasks

- [x] 3.1 Add the review watch HTTP/UDS route and map service errors to the TechSpec status codes.
- [x] 3.2 Add the Go API client method for starting a review watch run.
- [x] 3.3 Add `reviews watch` command registration, flags, config application, and workflow-name resolution.
- [x] 3.4 Reuse existing review fix runtime/batching flags while forcing auto-commit for auto-push.
- [x] 3.5 Add command, client, route, and output-format tests.

## Implementation Details

Use the daemon coordinator from task 02 as the only source of watch execution. The CLI should validate and serialize intent, then attach to the returned run using the same paths used by existing daemon-backed review fix runs.

### Relevant Files

- `internal/api/core/routes.go` — add the route definition for review watch.
- `internal/api/core/handlers.go` — decode requests, call the review service, and encode `RunResponse`.
- `internal/api/httpapi/routes.go` — expose the new review watch route through HTTP transport.
- `internal/api/client/reviews_exec.go` — add `StartReviewWatch`.
- `internal/cli/reviews_exec_daemon.go` — add the `watch` subcommand, flags, validation, and run observation.
- `internal/cli/task_runtime_flag.go` — reuse runtime override flag capture and auto-commit handling.
- `internal/cli/reviews_exec_daemon_additional_test.go` — extend CLI behavior coverage.

### Dependent Files

- `internal/api/client/client_contract_test.go` — verify request path, method, and payload.
- `internal/api/core/handlers_contract_test.go` — cover handler status codes and response shape.
- `internal/api/httpapi/openapi_contract_test.go` — update OpenAPI contract expectations if route generation includes the new endpoint.
- `internal/cli/testdata/*` — update help golden files if the command tree changes generated help output.

### Related ADRs

- [ADR-001: Use a Daemon-Owned Parent Run for Review Watching](adrs/adr-001.md) — requires the CLI to start a daemon parent run, not a foreground loop.
- [ADR-003: Force Auto-Commit and Allow Dirty Worktrees for Auto-Push Watch Runs](adrs/adr-003.md) — defines CLI auto-push validation and dirty-worktree tolerance.

## Deliverables

- `POST /api/reviews/:slug/watch` transport endpoint.
- API client method for review watch start.
- `compozy reviews watch [slug]` command with watch flags and inherited fix flags.
- Output behavior for attach, detach, JSON, raw JSON, UI, and stream modes.
- Unit tests with 80%+ coverage for CLI/client/handler additions **(REQUIRED)**
- Integration tests for command-to-daemon route execution with fake services **(REQUIRED)**

## Tests

- Unit tests:
  - [x] `reviews watch tools-registry --provider coderabbit --pr 85 --auto-push --until-clean --max-rounds 6` serializes the expected daemon request.
  - [x] `--auto-push --auto-commit=false` exits before daemon bootstrap with `invalid_watch_request`.
  - [x] `--auto-push` sets runtime override `auto_commit=true` even when the config omits it.
  - [x] `--poll-interval`, `--review-timeout`, `--quiet-period`, `--push-remote`, and `--push-branch` parse into the watch request.
  - [x] Client method uses `POST /api/reviews/{slug}/watch` and propagates daemon errors.
  - [x] Handler returns `409 review_watch_already_active`, `422 invalid_watch_request`, and `503 review_service_unavailable` from service errors.
- Integration tests:
  - [x] CLI starts a fake daemon watch run and attaches in stream mode.
  - [x] CLI starts a fake daemon watch run and prints a detached run summary.
  - [x] JSON and raw JSON output include watch events without dropping parent run metadata.
- Test coverage target: >=80%
- All tests must pass

## Success Criteria

- All tests passing
- Test coverage >=80%
- Operators can start `compozy reviews watch` through the daemon-backed command surface
- Auto-push validation happens before daemon execution
- Existing `reviews fetch`, `reviews fix`, `reviews list`, and `reviews show` behavior remains unchanged
