# Workflow Memory

Keep only durable, cross-task context here. Do not duplicate facts that are obvious from the repository, PRD documents, or git history.

## Current State
- Task 01 completed a new public `pkg/compozy/events` API surface and validated it with clean package tests plus full `make verify`.

## Shared Decisions
- Public event payloads in `pkg/compozy/events/kinds` must not expose `internal/core/model` types; session/content/usage structures are duplicated as public event-facing types with snake_case JSON contracts.
- Task 01 sized the payload structs for downstream task_05/task_06 emit/translate work, including job index/attempt fields plus public session, usage, shutdown, task, review, and provider metadata.
- Downstream work should follow the explicit ADR-003/techspec kind lists, which enumerate 33 public event kinds across 9 domains; the stale prose count of `27` in some task/ADR text is not authoritative.
- Task 02 changed ACP session ingress to a bounded `1024`-entry buffer with `5s` timed backpressure, explicit per-session slow/drop counters, and rate-limited drop warnings exposed through the `agent.Session` interface via `SlowPublishes()` and `DroppedUpdates()`.
- Task 04 introduced `agent.Registry` plus `agent.DefaultRegistry()` as a lightweight façade over the package-level ACP registry helpers so `KernelDeps` can carry the agent runtime registry dependency without rewriting the existing validation/availability logic.
- Task 03 resolved its reader-library dependency cycle by landing the minimal public `pkg/compozy/runs/` replay foundation early (`RunSummary`, `Open`, `Summary`, `Replay`) instead of adding a task-local test helper.
- Task 07 completed the public reader surface in `pkg/compozy/runs`; public run summaries normalize terminal statuses to `completed` / `cancelled` and backfill workflow-run status from `result.json` plus `events.jsonl` because workflow `run.json` does not persist terminal state.

## Shared Learnings
- Later emitter work should translate internal executor/session state into the public `kinds` payloads instead of reusing internal structs directly, otherwise the `pkg/` API becomes unusable to external consumers.
- Later journal and execution tasks can assume the upstream ACP ingress no longer silently drops on a full channel; any loss after the 5-second backpressure window is counted and logged per session.
- `golangci-lint --fix` can rewrite `"cancelled"` string literals to `"canceled"`, so any code that must accept both spellings should normalize input rather than relying on two literal switch cases.
- Task 05 made the shared journal the canonical writer for workflow and exec `events.jsonl` output; exec mode still keeps a separate stdout JSON projection for CLI-facing resume/status consumers.
- Task 08 resolved the `core` ↔ `kernel` import-cycle risk by having `internal/core/kernel` register dispatcher-backed legacy adapters through `core.RegisterDispatcherAdapters(...)`, while kernel handlers keep calling the preserved `*Direct` core functions.
- Task 08 made workflow runs persist `.compozy/runs/<run-id>/result.json` even in human/text CLI mode, so later reader, daemon, and SDK work can rely on that artifact across workflow modes instead of only in JSON output mode.

## Open Risks
- `DroppedFor(id)` only reports counters for active subscriptions because counters live on the subscription entry; callers that need a terminal drop count must read it before unsubscribe/close or persist it elsewhere.
- Review provider lifecycle events can only report best-effort `status_code` values until the provider abstraction exposes transport-level response details consistently.

## Handoffs
- Task 05 should wire executor and logging emission into the existing payloads rather than redefining event shapes.
- Task 06 can translate directly from `events.Event` payloads because task 01 already includes the fields required to rebuild `jobStartedMsg`, `jobRetryMsg`, `jobFailureMsg`, `usageUpdateMsg`, and shutdown state; executor/logging no longer publish those messages directly.
- Task 03 and task 05 can use `agent.Session.DroppedUpdates()` / `SlowPublishes()` for observability assertions without reaching into `sessionImpl`.
- Task 07 should extend the shipped replay foundation with `List`, `Tail`, `WatchWorkspace`, and the `nxadm/tail` / `fsnotify` dependency work, rather than rebuilding `Open`/`Replay`.
- Phase B daemon and later SDK tasks can now consume `pkg/compozy/runs.List/Open/Tail/WatchWorkspace` directly instead of re-parsing `.compozy/runs/` artifacts.
