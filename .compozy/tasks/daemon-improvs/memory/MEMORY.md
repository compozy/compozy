# Workflow Memory

Keep only durable, cross-task context here. Do not duplicate facts that are obvious from the repository, PRD documents, or git history.

## Current State

- Task `01` completed: `internal/api/contract` is now the canonical transport surface for daemon DTOs, error envelopes, SSE/cursor helpers, timeout classes, route inventory metadata, and compatibility notes.
- Task `02` tracking is marked completed, but the current `daemon-improvs` worktree does not contain the `internal/testutil/e2e` package or the referenced runtime-harness symbols from earlier memory notes; task `03` kept transport parity coverage in `internal/api/httpapi/transport_integration_test.go` until that harness lands or the tracking is corrected.
- Task `03` completed: shared handler and transport parity tests now decode canonical `internal/api/contract` envelopes for daemon, workspaces, tasks, reviews, runs, sync, and exec endpoints, and assert equivalent HTTP/UDS SSE `event`, `heartbeat`, and `overflow` frames.
- Task `04` completed: `internal/api/client` now owns canonical request/response decoding, route timeout-class routing, and reconnecting run-stream behavior, while `pkg/compozy/runs` consumes those canonical payloads through explicit compatibility adapters with focused coverage at `82.0%` (`internal/api/client`) and `81.2%` (`pkg/compozy/runs`).
- Task `06` completed: session liveness metadata is now persisted through the global `sessions` index, recovery consumers classify interrupted ACP work via `session.ClassifyInactiveMetaForRecovery`, Unix/Windows supervision paths are explicit, and ACP fault coverage now includes blocked-cancel teardown behavior.
- Task `07` completed: daemon health/metrics now expose the richer readiness, reconcile, integrity, and Prometheus contract; `run.db` persists sticky `run_integrity` state and bounded transcript projections for canonical cold snapshots; transport/client/public-reader surfaces now preserve `Incomplete` reason codes consistently.
- Task `08` completed: reusable QA planning artifacts now live under `.compozy/tasks/daemon-improvs/analysis/qa/`, including the feature test plan, regression suite, and execution-ready cases covering transport parity, timeout classes, lifecycle/logging, ACP fault handling, and observability handoff for `task_09`.

## Shared Decisions

- Session snapshot transport DTOs also live in `internal/api/contract`; later tasks must convert at runtime/UI boundaries instead of importing `internal/core/run/transcript` or `internal/core/model` into the contract layer.
- In the current `daemon-improvs` worktree, the repository-wide verification gate is `make verify`. There is no `make test-integration` target yet, and most daemon integration-style suites still run in the default `go test` lane without `//go:build integration` tags.

## Shared Learnings

- Public run-reader adoption can expose import cycles quickly because `pkg/compozy/runs` is consumed by tests under runtime packages; keeping contract types free of runtime package imports is a cross-task constraint.
- Runtime package tests should avoid importing `pkg/compozy/runs` just to reuse small sentinels or helpers, because the public run-reader now depends on daemon client/core packages and can close cycles back into `internal/core/run/*`.
- Prompt/exec SSE consumers cannot rely on explicit `event:` frames alone; the shared harness now infers semantic event names from JSON `data:` payloads when transports omit explicit SSE event labels.
- Repeated daemon boot cycles can hit HTTP bind races in CI; the shared harness now captures process logs, removes stale singleton artifacts, reseeds the HTTP port, and retries startup before surfacing readiness failures.
- Transport request-ID parity depends on sending the same `X-Request-Id` header to both transports; the middleware echoes that value in the response header and canonical error envelope, so parity tests should seed the same request ID rather than ignore it.
- Directory watch refresh must happen before persisted sync when renames or deletes change the watch set; otherwise a fast write into the renamed directory can land after the DB view updates but before the new filesystem watcher is attached.
- `core.RunStreamOverflow` is service-local state only; the canonical wire shape for heartbeat/overflow frames stays owned by `internal/api/contract`.
- Future recovery/reporting work should reuse `session.ClassifyInactiveMetaForRecovery` and the persisted `subprocess_pid`, `subprocess_started_at`, `last_update_at`, `stall_state`, and `stall_reason` fields instead of re-deriving crash/orphan/stall heuristics locally.
- ACP runtime tests that prompt or stop immediately after session creation should keep the `createFixtureBackedSession` readiness wait, otherwise parallel runs can race session indexing and fail with `404 session not found`.
- Blocked ACP cancellation currently produces a terminal prompt-stream `error` event from the disconnect path while the durable session stop is still `user_canceled`; operator/reporting surfaces should treat stream framing and persisted stop state as distinct signals.
- `run_integrity` reason codes must merge durably across later snapshot audits instead of short-circuiting after the first incomplete read; otherwise cold readers lose why a run stayed incomplete.
- Daemon journal-drop metrics need per-run delta accounting because snapshot reads can persist runtime integrity multiple times for the same live run; accumulating absolute drop counts on every read double-counts the contract metric.
- The task-generation ledger saved paths under `.compozy/tasks/daemon-improvs/analysis/`, but in this worktree the authoritative TechSpec, ADRs, and task files currently live under `.compozy/tasks/daemon-improvs/`; only the QA artifact output for `task_08`/`task_09` is being rooted under `.compozy/tasks/daemon-improvs/analysis/qa/`.
- The current branch has no daemon web UI surface and no browser automation harness. QA planning and execution should mark browser validation as blocked or out of scope unless that surface is added later.
- Stream-attached CLI observers must wait for the durable terminal snapshot after seeing a terminal daemon event before returning control to the operator; otherwise immediate post-run inspection and extension teardown can race the final state mirror.
- The low-level daemon API client reconnects run streams on EOF by design; cross-task operator tests that consume raw `OpenRunStream` should stop after the observed terminal event or use the higher-level watch helpers instead of treating EOF as the terminal contract.
- Real-binary daemon QA on shared development machines should set `COMPOZY_DAEMON_HTTP_PORT=0` or another free port, because the default `127.0.0.1:2323` can already be occupied and cause detached `daemon start` to fail before writing a healthy `daemon.json`.

## Open Risks

## Handoffs

- Later transport migration tasks should consume `contract.RouteInventory`, `contract.TimeoutClassForRoute`, `contract.TransportErrorEnvelope`, and the contract-owned run snapshot/page helpers rather than redefining route or timeout semantics locally.
- Future daemon-facing readers and CLI surfaces should keep `internal/api/client` as the single owner of daemon SSE parsing and reconnect rules; adapter packages should translate semantics, not reimplement transport parsing.
- Until a shared runtime harness package actually exists in this worktree, later CLI parity and ACP fault-injection tasks should either land that harness first or extend the existing transport integration suite without assuming `RuntimeManifest`, `CaptureTransportOutput`, or `CaptureCLIOutput` symbols are already available.
- In-process daemon test harnesses must shut down the daemon `RunManager` before closing `global.db`; `GlobalDB` now marks itself closed atomically to avoid close-vs-update races during teardown, but orderly manager shutdown remains the intended lifecycle.
