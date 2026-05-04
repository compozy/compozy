# Technical Specification: Daemon-Owned Review Watch Automation

## Executive Summary

No `_prd.md` exists for this feature. This TechSpec is derived from the accepted planning discussion, direct codebase exploration, and the current daemon-backed review architecture. The feature adds `compozy reviews watch`, a daemon-owned parent run that automates the current manual CodeRabbit loop: wait for provider review, import actionable feedback, run `reviews fix`, push committed fixes when requested, and repeat until the current PR head is clean or a bounded stop condition is reached.

The main architectural decision is to implement watch as a daemon orchestration layer over existing `reviews fetch`, `reviews fix`, provider resolution, run streams, and workspace sync. The primary trade-off is adding a new parent run mode and provider watch capability in exchange for durable state, attach/cancel support, and correct clean detection. The design deliberately avoids shell-script automation, arbitrary sleeps, destructive git commands, and a second remediation pipeline.

## System Architecture

### Component Overview

- `reviews watch` CLI command: parses loop flags, applies config, validates `--auto-push` semantics, bootstraps the daemon, starts the watch run, and attaches or detaches using the same client patterns as existing daemon-backed commands.
- Review watch transport: adds `POST /api/reviews/:slug/watch` and forwards the request to the daemon review service. The route returns a normal daemon run for the parent watch job.
- Review watch coordinator: lives under the daemon boundary and owns the parent `review_watch` run lifecycle. It waits for provider state, imports rounds, starts child review-fix runs, observes terminal states, pushes committed changes when enabled, and emits watch lifecycle events.
- Existing review fetch path: remains the only path that writes `.compozy/tasks/<slug>/reviews-NNN/` issue files. Watch calls the same provider registry and review store behavior rather than writing issue files itself.
- Existing review fix path: remains the only path that performs agent remediation. Watch starts child `review` runs through `RunManager.StartReviewRun`.
- Provider watch capability: extends review providers with status for PR head and latest provider review. CodeRabbit implements this with `gh` REST/GraphQL calls, matching the existing provider implementation style.
- Git push runner: a small daemon-side boundary for read-only git inspection and `git push`. It never stages, cleans, resets, restores, checks out, or removes files.
- Extension hooks and events: expose watch lifecycle seams without allowing extensions to bypass the core provider/fix/push state machine.

### Data Flow

1. The operator runs `compozy reviews watch tools-registry --provider coderabbit --pr 85 --auto-push --until-clean --max-rounds 6 [...reviews fix flags]`.
2. The CLI resolves workspace/config, enforces `auto_commit=true` when `--auto-push` is active, ensures the daemon is ready, and sends `ReviewWatchRequest`.
3. The daemon rejects duplicate active watches for the same workspace, provider, and PR.
4. The parent run records initial git state, including branch, HEAD, dirty state, and unpushed-commit warning metadata.
5. For each round, the coordinator asks the provider for current PR review status.
6. If the provider has not reviewed the current head, the coordinator waits with condition-based polling until the provider reports a current review or the timeout/cancel context fires.
7. Once the current head is reviewed, the coordinator fetches normalized review items in memory.
8. If the fetch result is empty and the provider review is current, the parent run completes clean without creating an empty review directory.
9. If items exist, the coordinator persists the next `reviews-NNN` round through the existing fetch writer path and syncs the workflow.
10. The coordinator starts a detached child `reviews fix` run for that round with the requested runtime and batching flags.
11. The parent waits on the child run stream/snapshot until terminal.
12. On successful child completion, the coordinator verifies the round has zero unresolved issues and verifies HEAD advanced for non-empty rounds.
13. If `auto_push` is enabled, the coordinator runs `git push` for the selected remote/branch.
14. The coordinator waits for the provider to review the new PR head, then repeats until clean or `max_rounds`.

## Implementation Design

### Core Interfaces

The watch request extends the API contract without changing existing fetch/fix request shapes:

```go
type ReviewWatchRequest struct {
    Workspace        string          `json:"workspace"`
    Provider         string          `json:"provider,omitempty"`
    PRRef            string          `json:"pr_ref"`
    UntilClean       bool            `json:"until_clean,omitempty"`
    MaxRounds        int             `json:"max_rounds,omitempty"`
    AutoPush         bool            `json:"auto_push,omitempty"`
    PushRemote       string          `json:"push_remote,omitempty"`
    PushBranch       string          `json:"push_branch,omitempty"`
    RuntimeOverrides json.RawMessage `json:"runtime_overrides,omitempty"`
    Batching         json.RawMessage `json:"batching,omitempty"`
}
```

Providers opt into current-head watch semantics:

```go
type WatchStatusProvider interface {
    Provider
    WatchStatus(ctx context.Context, req WatchStatusRequest) (WatchStatus, error)
}

type WatchStatus struct {
    PRHeadSHA       string
    ReviewCommitSHA string
    ReviewID        string
    State           string // pending, reviewed
    SubmittedAt     time.Time
}
```

The daemon uses a narrow git boundary:

```go
type ReviewWatchGit interface {
    State(ctx context.Context, workspace string) (GitState, error)
    Push(ctx context.Context, workspace string, remote string, branch string) error
}
```

### Data Models

#### Runtime and transport models

- Add `ReviewWatchRequest` to `internal/api/contract`.
- Add `ReviewWatchRequest` aliases and service method signatures to `internal/api/core`.
- Add client method `StartReviewWatch(ctx, workspace, slug string, req apicore.ReviewWatchRequest)`.
- Add daemon run mode string `review_watch` for parent runs.
- Add `WatchStatusRequest`, `WatchStatus`, and `WatchStatusProvider` to `internal/core/provider`.
- Add `WatchReviewsConfig` to workspace config:

```toml
[watch_reviews]
max_rounds = 6
poll_interval = "30s"
review_timeout = "30m"
quiet_period = "20s"
auto_push = false
until_clean = true
push_remote = "origin"
push_branch = "feature-branch"
```

Config precedence is:

1. CLI flags.
2. `[watch_reviews]` for watch-only loop behavior.
3. `[fix_reviews]`, `[fetch_reviews]`, and `[defaults]` for existing child fetch/fix behavior.
4. Built-in defaults.

Validation rules:

- `max_rounds` must be greater than zero when `until_clean` is true.
- `poll_interval`, `review_timeout`, and `quiet_period` must be positive durations.
- `auto_push=true` requires `auto_commit=true`; the CLI should force this before transport and the daemon should reject contradictory requests.
- `push_remote` and `push_branch` are optional only when the current branch has an upstream that can be resolved.

#### Persistent state

No new v1 database table is required. The parent watch run persists state through the existing daemon run row and run event journal. Review rounds remain stored as Markdown issue files under `.compozy/tasks/<slug>/reviews-NNN/` and mirrored into `global.db` through existing sync.

If implementation later needs resumable watch recovery after daemon crash, add a follow-up ADR and schema migration. Do not add a speculative watch table in v1.

#### Watch events

Add event kinds with payload structs under the existing events package:

- `review.watch_started`
- `review.watch_waiting`
- `review.watch_round_fetched`
- `review.watch_fix_started`
- `review.watch_fix_completed`
- `review.watch_push_started`
- `review.watch_push_completed`
- `review.watch_push_failed`
- `review.watch_clean`
- `review.watch_max_rounds`

Payloads must include `provider`, `pr`, `workflow`, `round` when applicable, `run_id`, `child_run_id` when applicable, `head_sha`, and relevant error text.

### Coordinator State Machine

The parent run uses an explicit state machine:

| State | Entry Condition | Exit Condition |
|-------|-----------------|----------------|
| `initializing` | request accepted | provider/git/config validation succeeds |
| `waiting_for_review` | provider has not reviewed current head | provider reports current completed review, timeout, or cancellation |
| `fetching_round` | current review exists | actionable items are persisted or clean state is detected |
| `fixing_round` | persisted round has actionable items | child run completes, fails, or is cancelled |
| `verifying_round` | child run completed successfully | round has zero unresolved issues and HEAD advanced when required |
| `pushing` | `auto_push=true` and verified round produced a commit | push succeeds or fails |
| `clean` | provider reviewed current head and fetch has zero actionable items | terminal |
| `max_rounds` | configured round limit reached before clean | terminal |
| `failed` | provider, child, verification, timeout, or push failure | terminal |
| `cancelled` | parent context cancelled | terminal |

The coordinator must use context-bound timers or tickers for polling and quiet periods. It must not use unowned goroutines or `time.Sleep()`-based orchestration.

### API Endpoints

#### Start review watch

- Method: `POST`
- Path: `/api/reviews/:slug/watch`
- Request: `ReviewWatchRequest`
- Response: `RunResponse`
- Success: `201 Created`
- Errors:
  - `422 invalid_runtime` for invalid runtime overrides.
  - `422 invalid_watch_request` for `auto_push=true` with `auto_commit=false`.
  - `409 review_watch_already_active` when the same workspace/provider/PR already has an active watch.
  - `404 review_round_not_found` only when a caller requests a fixed starting round that does not exist.
  - `503 review_service_unavailable` when the daemon lacks review services.

#### Existing run APIs

The existing run APIs handle `review_watch` without new routes:

- `GET /api/runs/:run_id`
- `GET /api/runs/:run_id/snapshot`
- `GET /api/runs/:run_id/stream`
- `POST /api/runs/:run_id/cancel`

### CLI Contract

Add:

```text
compozy reviews watch [slug]
```

Primary flags:

- `--provider`
- `--pr`
- `--name`
- `--until-clean`
- `--max-rounds`
- `--auto-push`
- `--push-remote`
- `--push-branch`
- `--poll-interval`
- `--review-timeout`
- `--quiet-period`
- existing fix flags: `--ide`, `--model`, `--reasoning-effort`, `--access-mode`, `--timeout`, `--max-retries`, `--retry-backoff-multiplier`, `--concurrent`, `--batch-size`, `--include-resolved`, `--add-dir`
- attach flags: `--attach`, `--ui`, `--stream`, `--detach`

Behavior:

- `--auto-push` forces runtime override `auto_commit=true`.
- `--auto-commit=false` with `--auto-push` fails before contacting the daemon.
- Dirty worktrees do not block watch startup.
- `--format json` and `--format raw-json` stream watch events through existing daemon event streaming behavior.

## Integration Points

### GitHub and CodeRabbit

The CodeRabbit provider continues to use `gh` as the process boundary. Watch status adds GitHub PR metadata and review metadata queries:

- PR head SHA.
- Latest CodeRabbit review id, submitted time, state, and commit SHA when available.
- Review threads for unresolved inline comments.
- Existing review-body parsing for outside-diff nitpick/minor/major findings.

Authentication remains the existing `gh` authentication model. Provider errors must wrap the failing `gh` operation and surface through provider call events.

### Git push

The git runner supports:

- Inspect current branch and HEAD.
- Detect dirty worktree for warning events only.
- Detect unpushed commits for warning events.
- Resolve push remote and branch from flags or upstream configuration.
- Run `git push <remote> HEAD:<branch>`.

It must not support destructive operations. If upstream cannot be resolved and flags are absent, fail with a clear validation error.

Safety invariants:

- Never run `git restore`, `git checkout`, `git reset`, `git clean`, `git rm`, or manual staging commands.
- Never push after a failed child run.
- Never push after a non-empty round if HEAD did not advance.
- Never mark clean until provider status is current for the PR head.
- Always emit the initial dirty/unpushed state as warning metadata instead of blocking startup.

### Extensions

Add extension SDK hooks:

- `review.watch_pre_round`: mutable; can stop or adjust round-level fetch/fix options.
- `review.watch_post_round`: observer; reports round fetch/fix/push outcome.
- `review.watch_pre_push`: mutable; can veto push or adjust remote/branch.
- `review.watch_finished`: observer; reports terminal reason.

Hooks must not be able to skip provider-current clean detection. They can veto push or stop the loop with an explicit reason.

## Impact Analysis

| Component | Impact Type | Description and Risk | Required Action |
|-----------|-------------|----------------------|-----------------|
| `internal/cli/reviews_exec_daemon.go` | modified | Adds watch command and validation. Medium risk because it shares review fix flags and daemon attach behavior. | Reuse existing command state helpers and daemon event streaming. |
| `internal/api/contract` and `internal/api/core` | modified | Adds watch request and route. Low-medium risk due to public API expansion. | Add typed request/response tests and route smoke coverage. |
| `internal/api/client` | modified | Adds `StartReviewWatch`. Low risk. | Mirror existing review run client patterns. |
| `internal/daemon` | modified | Adds parent watch orchestration and run mode. High risk due to long-running lifecycle, child runs, cancellation, and push boundaries. | Keep coordinator small, context-bound, and tested with fake provider/git/child run services. |
| `internal/core/provider` | modified | Adds optional watch status capability. Medium risk for provider abstraction. | Keep existing `Provider` interface backward-compatible and require watch status only for watch mode. |
| `internal/core/provider/coderabbit` | modified | Adds CodeRabbit current-head status queries. Medium risk because GitHub/API shape drives correctness. | Add fixture tests for current, stale, pending, and clean states. |
| `internal/core/workspace` | modified | Adds `[watch_reviews]` config. Low risk. | Extend config merge/validation/docs tests. |
| `pkg/compozy/events` and docs | modified | Adds watch lifecycle events. Low risk. | Document payloads and add event kind tests. |
| `sdk/extension` and `sdk/extension-sdk-ts` | modified | Adds watch hooks and types. Medium risk across Go and TS SDK parity. | Add smoke, fluent helper, and template tests. |

## Testing Approach

### Unit Tests

- CLI:
  - Parses `reviews watch [slug]` and all loop flags.
  - Applies config precedence from `[watch_reviews]`, `[fetch_reviews]`, `[fix_reviews]`, and `[defaults]`.
  - Rejects `--auto-push --auto-commit=false`.
  - Forces `auto_commit=true` in daemon runtime overrides when `--auto-push` is set.
  - Allows dirty worktree state without preflight failure.

- Provider:
  - CodeRabbit watch status detects pending review for a new head.
  - CodeRabbit watch status detects stale review for an older head.
  - CodeRabbit watch status detects reviewed current head.
  - Existing inline and review-body comments remain normalized.
  - Provider without watch status returns an explicit unsupported error for `--until-clean`.

- Git runner:
  - Reports branch, head, dirty state, upstream, and unpushed count.
  - Pushes with `git push <remote> HEAD:<branch>`.
  - Has no code path for `git restore`, `git checkout`, `git reset`, `git clean`, `git rm`, or manual staging commands.
  - Fails clearly when no upstream or push target is available.

- Coordinator:
  - Does not create an empty review round when provider is current and fetch result is empty.
  - Fails if child fix succeeds for a non-empty round but HEAD does not advance.
  - Stops at `max_rounds`.
  - Propagates child failure, cancellation, provider failure, timeout, and push failure.

### Integration Tests

- Daemon review watch starts a parent run, emits watch events, starts a child review run, and reaches terminal clean state with fake provider/git dependencies.
- Parent cancellation stops provider waiting and prevents new child runs.
- Duplicate active watch for the same workspace/provider/PR returns `409`.
- Watch run remains observable through existing run snapshot and stream endpoints.
- Existing `reviews fetch`, `reviews fix`, `reviews list`, and `reviews show` tests continue to pass unchanged.

### End-to-End QA

- Use a temporary git repository with fake provider responses to validate a two-round loop:
  1. Provider reports reviewed current head with issues.
  2. Watch writes `reviews-001`.
  3. Child fix marks all issues resolved and creates a commit.
  4. Watch pushes.
  5. Provider reports reviewed new head with no issues.
  6. Parent completes clean.
- Final implementation must run and pass `make verify`.

## Development Sequencing

### Build Order

1. Add API/config types - no dependencies.
2. Add provider watch status interface and unsupported-provider errors - depends on step 1 for request semantics.
3. Implement CodeRabbit watch status with fixtures - depends on step 2.
4. Add git runner abstraction and tests - depends on step 1 for watch push config.
5. Add daemon watch coordinator and parent run mode - depends on steps 1, 2, and 4.
6. Wire daemon review service and API route - depends on step 5.
7. Add API client and CLI command - depends on step 6.
8. Add watch events, docs, and stream formatting - depends on step 5 so payloads match coordinator behavior.
9. Add extension hooks in Go and TS SDKs - depends on step 8 for hook payload/event shape.
10. Add integration tests and QA fixtures - depends on steps 3, 5, 6, and 7.
11. Run full `make verify` and fix production/test issues at root cause - depends on all implementation steps.

### Technical Dependencies

- `gh` must be authenticated with repository read access and review-thread resolution permissions already required by the CodeRabbit provider.
- The selected agent runtime must be available for child `reviews fix` runs.
- Push target must be resolvable from upstream config or explicit `--push-remote` and `--push-branch`.
- Existing daemon bootstrap must be healthy because watch is daemon-only.

## Monitoring and Observability

- Parent watch run emits structured run events for every state transition.
- Provider calls emit `provider.call_started`, `provider.call_completed`, and `provider.call_failed` with method names such as `watch_status`, `fetch_reviews`, and `resolve_issues`.
- Push emits explicit started/completed/failed events with remote, branch, old/new head, and error text.
- Logs include `workspace`, `workflow`, `provider`, `pr`, `round`, `head_sha`, `child_run_id`, and `run_id`.
- Failure terminal states preserve the last provider status, last child run ID, and last actionable round in event payloads.

## Technical Considerations

### Key Decisions

- Decision: model watch as a daemon parent run.
  - Rationale: watch is long-running, cancellable, observable work and should share daemon lifecycle and attach semantics.
  - Trade-off: adds parent-child orchestration complexity.
  - Alternatives rejected: foreground-only CLI loop and CI automation.

- Decision: require provider current-head status before clean completion.
  - Rationale: zero fetched issues is ambiguous until the provider has reviewed the current head.
  - Trade-off: provider implementations need an additional capability.
  - Alternatives rejected: fetch-only polling and webhook-dependent local automation.

- Decision: `--auto-push` forces `auto_commit=true` and allows dirty worktrees.
  - Rationale: the accepted workflow prioritizes fully automated round progression.
  - Trade-off: existing unpushed commits may be included in a push.
  - Alternatives rejected: clean-only start and manual-push default.

### Known Risks

- CodeRabbit may change body/comment formatting. Mitigate with fixture tests from real review examples and fail-visible parsing errors.
- Provider status may lag after push. Mitigate with condition-based polling, timeout, and stale/current head reporting.
- Child fix may resolve Markdown but fail to create a commit. Mitigate by requiring HEAD advancement before push for non-empty rounds.
- Existing unpushed commits may be pushed. Mitigate with explicit warning events and final summary.
- Duplicate watches can race pushes. Mitigate with active duplicate rejection by workspace/provider/PR.

## Architecture Decision Records

- [ADR-001: Use a Daemon-Owned Parent Run for Review Watching](adrs/adr-001.md) - Implements watch as an observable, cancellable daemon parent run over existing fetch/fix flows.
- [ADR-002: Require Provider Watch Status Before Declaring Reviews Clean](adrs/adr-002.md) - Prevents premature clean completion by requiring provider review state for the current PR head.
- [ADR-003: Force Auto-Commit and Allow Dirty Worktrees for Auto-Push Watch Runs](adrs/adr-003.md) - Captures the accepted auto-push policy and its git safety boundaries.
