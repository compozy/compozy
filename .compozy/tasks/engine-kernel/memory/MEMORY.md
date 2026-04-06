# Workflow Memory

Keep only durable, cross-task context here. Do not duplicate facts that are obvious from the repository, PRD documents, or git history.

## Current State
- Task 01 completed a new public `pkg/compozy/events` API surface and validated it with clean package tests plus full `make verify`.

## Shared Decisions
- Public event payloads in `pkg/compozy/events/kinds` must not expose `internal/core/model` types; session/content/usage structures are duplicated as public event-facing types with snake_case JSON contracts.
- Task 01 sized the payload structs for downstream task_05/task_06 emit/translate work, including job index/attempt fields plus public session, usage, shutdown, task, review, and provider metadata.
- Task 02 changed ACP session ingress to a bounded `1024`-entry buffer with `5s` timed backpressure, explicit per-session slow/drop counters, and rate-limited drop warnings exposed through the `agent.Session` interface via `SlowPublishes()` and `DroppedUpdates()`.

## Shared Learnings
- Later emitter work should translate internal executor/session state into the public `kinds` payloads instead of reusing internal structs directly, otherwise the `pkg/` API becomes unusable to external consumers.
- Later journal and execution tasks can assume the upstream ACP ingress no longer silently drops on a full channel; any loss after the 5-second backpressure window is counted and logged per session.

## Open Risks
- `DroppedFor(id)` only reports counters for active subscriptions because counters live on the subscription entry; callers that need a terminal drop count must read it before unsubscribe/close or persist it elsewhere.

## Handoffs
- Task 05 should wire executor and logging emission into the existing payloads rather than redefining event shapes.
- Task 06 can translate directly from `events.Event` payloads because task 01 already includes the fields required to rebuild `jobStartedMsg`, `jobRetryMsg`, `jobFailureMsg`, `usageUpdateMsg`, and shutdown state.
- Task 03 and task 05 can use `agent.Session.DroppedUpdates()` / `SlowPublishes()` for observability assertions without reaching into `sessionImpl`.
