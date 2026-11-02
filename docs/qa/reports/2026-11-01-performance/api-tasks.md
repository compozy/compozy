# API and Tasks performance audit

Baseline: `de5ab2f4d`. Worktree: `responsiveness-audit-20261101`.

## Retained: task stream cross-query refetch amplification

`web/src/systems/tasks/lib/task-live-refresh-coordinator.ts` invalidates every run-detail and run-inspect query on each task's stream event. Both canonical responses include an immutable task ID. With many active task windows, a change to one task rereads unrelated run endpoints.

Test invariant: a task event refreshes that task's run detail/inspection, preserves unrelated populated run reads, and conservatively refreshes reads whose owner identity is not yet available. Owning layer: Tasks live-query coordination. Canonical suite: `web/src/systems/tasks/hooks/__tests__/use-task-stream.test.tsx`. Existing coalescing, cleanup, pagination, and stream lifecycle cases remain intact.

The coordinator now reads the immutable owner ID from populated canonical run-detail and run-inspect snapshots. Known unrelated owners are excluded; missing snapshots or missing owner IDs still receive a canonical reread. Agent-context and review query behavior stays unchanged because these query shapes do not supply an equally reliable task identity. No timer, persistent cache, new query key, or additional stream owner is introduced.

Five repetitions used 50 active TanStack QueryObserver pairs (run detail plus run inspection), with one stream refresh request for `task_0`. This measures endpoint reread amplification, not 50 rendered windows or user latency.

| Same fixture, Bun 1.4.4 | Before | After |
| --- | --- | --- |
| Run endpoint rereads per event, five samples | 100, 100, 100, 100, 100 | 2, 2, 2, 2, 2 |
| Median and range | 100 (100–100) | 2 (2–2) |

The reduction is 98%. Baseline deliberately ran the new behavioral assertion against unchanged production code: that assertion failed with the five 100-call samples while the other 18 cases passed. After the implementation, all 19 cases pass. Existing burst coalescing, cleanup, pagination, replay and stream lifecycle cases remain in that same suite.

Commands (from repository root; both measurement runs used host Bun 1.4.4):

```sh
python3 .cache/performance/run-bounded.py bunx turbo run test --filter=./web --only -- src/systems/tasks/hooks/__tests__/use-task-stream.test.tsx --maxWorkers=1
python3 .cache/performance/run-bounded.py env PATH="/opt/homebrew/bin:$PATH" /opt/homebrew/bin/bun x turbo run test --filter=./web --only -- src/systems/tasks/hooks/__tests__/use-task-stream.test.tsx --maxWorkers=1
```

Receipts: `.cache/performance/api-tasks/task-stream-baseline-only.log` and `task-stream-after-only.log`. The first invocation preceded the bounded runner's pinned-runtime change; the second explicitly preserves the same runtime. `--only` is focused iteration evidence, not the integrated codegen/typecheck/CI gate. Pinned Bun 1.4.0 final-source checks are tracked below.

## Coordinated: worktree aggregate selection before hydration

The API previously resolved the permitted workspace set after the service hydrated all worktrees in the profile scope. The coordinated service change now accepts the already-resolved workspace set before status/activity/PR hydration; nil or empty selection admits no rows. API ownership covers the internal method argument and its admission order only; the public route and DTO are unchanged. Before/after SQLite measurements and service coverage belong to the [worktree report](worktree-workspace.md).

Test invariant: aggregate reads pass the selected, registered workspace set and profile scope to the service, preserve owner projection, and perform no aggregate read when workspace selection cannot be resolved. Owning layer: API core worktree handlers. Canonical suite: `internal/api/core/worktrees_test.go`, reusing its worktree service stub and the existing workspace service stub. Service filtering and query-count evidence belong to `worktree-workspace.md`.

Transport invariant: the aggregate route keeps byte-identical HTTP/UDS response bodies and success status after the internal service signature change. Owning layer: transport registration and shared payload contract. Canonical suite: `internal/api/testutil/worktree_transport_parity_test.go` (`TestWorktreeHTTPUDSTransportParityIT033`); the existing finite-route table gains the aggregate route. It does not duplicate service filtering coverage.

## API audit findings

- `internal/api/core`: session catalog has bounded paging, batch health/subagent/owner projection, and one workspace registration read. Process summary decoration uses an O(1) cached lookup with a brief read lock; introducing batching/goroutines here has no measured justification. Runtime status builds counts via `Sessions.ListAll`, including filesystem inventory and recovery semantics that catalog facets do not reproduce. The status/runtime owner found the existing bounded `/api/status/identity` endpoint and is moving identity-only consumers to it. No substitute status aggregate or changed status semantics was added here.
- `internal/api/modelcatalogprojection`: linear bounded conversion, no I/O, no concurrency opportunity; no optimization proposed.

Static package coverage is complete. Focused final-source and integrated verification remain tracked separately below.

## Separate package coverage

This is static call-path coverage, not a claim that every endpoint was load-tested. Package directories were enumerated from the source tree; test-only fixture packages are identified separately.

| Package/system | Inspected paths | Finding and disposition |
| --- | --- | --- |
| `internal/api/core` | `session_catalog.go`, `session_catalog_health.go`, `session_detail.go`, `authored_context_access.go`, `session_profile_owners.go`, `subagent_summaries.go`, `session_monitors.go`, `session_processes.go`, `task_read_handlers.go`, `task_profile_owners.go`, `status.go`, `doctor_payload.go`, `providers.go`, `terminal_catalog.go`, `terminal_aggregate.go`, `worktree_aggregate.go`, `workspace_breadth.go`, `profile_read_scope.go`, `openapi_document.go` | Session list enrichment is already batched; process summaries are cached O(1) reads. OpenAPI serialization/compression is cached with `sync.OnceValues`. Full status semantics preserved; lighter consumers move to identity at the frontend owner. Worktree aggregate selection now reaches the service before hydration. No speculative concurrent payload mutation or new cache added. |
| `internal/api/contract` | `tasks.go`, `agents.go`, `responses.go`, `session_catalog.go` and DTO inventory | DTO/conversion layer; task-run responses already carry the immutable owner identity needed by the frontend improvement. No wire or schema change needed. |
| `internal/api/httpapi` | `middleware.go`, `static.go`, `server_setup.go`, `server_lifecycle.go`, route registration | Request handlers share core owners; static delivery uses seekable content and cache headers. Lifecycle locks cover setup/shutdown, not whole request execution. No measured independent hotspot. |
| `internal/api/udsapi` | `server_handlers.go`, `server_setup.go`, `server_start.go`, `server_shutdown.go`, route registration | Shared core handler paths preserve HTTP/UDS parity. Locks own lifecycle, not catalog queries. No duplicate optimization proposed. |
| `internal/api/ginutil` | `quiet_debug.go` | Global mutex owns Gin setup only; normal requests only decode their bounded route parameters. No hot-path lock optimization justified. |
| `internal/api/modelcatalogprojection` | `projection.go` | Linear model/option projection with no I/O. No evidence supporting concurrency or memoization here. |
| `internal/api/spec` | `operation_registry.go`, `schema_components.go`, `schema_reflection.go`, `render.go`, `openapi_document.go` | Registry built at initialization; component-name computation cached. Runtime release bytes have a separate cached core owner. Generation work should not be treated as per-click API work. |
| `internal/api/testutil` | `session_stub.go`, `session_stub_page.go`, `task_stub_query.go`, `apitest.go` | Test fixtures only; changing stub performance cannot improve production responsiveness. |
| `internal/api/spec/testdata/componentnames/session` | `types.go` | Code-generation component naming fixture only; unchanged. |
| `internal/api/spec/testdata/componentnames/task` | `types.go` | Code-generation component naming fixture only; unchanged. |
| `internal/api/spec/testdata/componentnames/collision/session` | `types.go` | Code-generation collision fixture only; unchanged. |
| `web/src/systems/tasks` | `query-options.ts`, `query-keys.ts`, `task-live-refresh-coordinator.ts`, `task-grouping.ts`, `use-task-stream.ts`, `use-task-detail-stream.ts`, `use-task-detail-reads.ts`, `use-task-run-page.ts`, `use-tasks-page-reads.ts`, `use-task-fan-out-run-results.ts`, API adapters and stream tests | Identified populated run reads belonging to unrelated task windows being refreshed by every stream event. Detail side reads already respect window/tab gates, bounded catalog envelopes and cancellation. Keep those contracts. |
| `internal/cli` | `client_settings_vault.go`, `client_session_prompt.go`, `client_terminal_stream.go`, `client_window_manager_stream.go`, `session_list.go`, `catalog_aggregate.go`, `loop_follow_timeline.go`, `poll.go`, `workspace_detail.go`, `daemon_status.go`, `app_diagnose_bundle_logs.go` | Client transport is shared across JSON/stream clients within an invocation; stream lifetimes differ deliberately. Prompt/error and diagnostic tail reads are bounded. Command formatting is linear over requested data; paging/cursor authority remains the server. No measured CLI hotspot warrants a production change in this slice. |
| `internal/cli/docpost` | `docpost.go`, `subcommands.go`, `docpost_markdown.go` | Build-only Markdown postprocessing validates targets before writes and checks cancellation between files. Child discovery scans the command inventory per document, but no measured generation bottleneck or interaction-path cost justifies another index/concurrency owner. |
| `web/src/systems/support` | `adapters/support-api.ts`, `hooks/use-support-bundle-download.ts`, `hooks/support-bundle-download-store.ts`, query keys/types/tests | Demand-driven support operation; cancellable polling, two-minute lifetime, replacement cancellation, timer cleanup and object-URL revocation already present. No always-on work or retained blob cache found. |
| `internal/codegen/hostapi` | `generate.go`, `catalog.go` | Build-only deterministic contract rendering; does not run on interaction paths. |
| `internal/codegen/jsbin` | `jsbin.go` | Bounded executable lookup during code generation; no runtime cache justified. |
| `internal/codegen/openapits` | `generate.go`, `sdk.go` | Build-only external tool execution, temporary output lifecycle and byte comparison; unchanged. |
| `internal/codegen/sdkgo` | `generate.go`, `type_model.go`, `type_render.go` | Build-only reflection/rendering with deduplication; not daemon click latency. |
| `internal/codegen/sdkts` | `generate.go`, `generate_render.go` | Build-only type graph traversal, pre-sized string builder and emitted-type tracking; unchanged. |
| `internal/codegen/storeschema` | `streams.go`, `atlas.go`, `sqlc.go`, `schema_source.go` | Three ordered schema streams; validation deliberately replays migration truth. No parallel generator added while the machine is resource-sensitive. |
| `cmd/compozy-codegen` | `codegen_all.go`, `codegen_check.go`, `generated_artifact_transaction.go` | Build-only ordered generators and transactional artifact publish/rollback. Determinism and resource bounds take priority; no measured user interaction hotspot. |
| `internal/e2elane` | `lanes.go` | Small immutable lane descriptions and copy-on-return slices; no production runtime cost. |
| `internal/testutil` | `testutil.go`, `hermetic_env.go` | Hermetic environment/test lifecycle helpers only. |
| `internal/testutil/e2e` | `runtime_harness_build.go`, `runtime_harness_environment.go`, `runtime_harness_process.go`, artifact helpers | Builds reused per test process and accept an explicit existing binary; isolated process/environment ownership. No test-harness change presented as application gain. |
| `internal/testutil/acpmock` | `driver_binary.go`, `fixture.go` | Test-only ACP fixture execution; driver build is already reused. |
| `internal/testutil/acpmock/cmd/acpmock-driver` | Driver prompt/control inventory | Test-only ACP protocol process; no production runtime cost. |
| `internal/testutil/mcpfixture` | `fixture.go`, `http.go`, `oauth.go` | Test-only HTTP/stdio fixtures with owned servers; no production hotspot. |
| `internal/testutil/storeseed` | Seed construction and clone implementation | Existing database seed copying amortizes migration cost across tests. No additional generator or cache required. |
| `internal/testutil/storeseed/global` | Global seed construction | Existing global database seed cloning amortizes migration cost; test-only. |
| `internal/testutil/storeseed/internal/seedbase` | Shared seed builder and clone implementation | Shared test database seed lifecycle; no production runtime cost. |

## Concurrency decision

The session API enrichment paths already batch their I/O. Adding goroutines around shared payload decoration would add coordination and load to the same SQLite owners without a measured benefit. Tasks excludes irrelevant reads; the worktree service owner batches its remaining hydration after applying the API's selection. CLI/build/test-only packages do not justify increasing process concurrency under the user's machine-resource constraint.

## Cross-surface and QA impact

The HTTP/UDS route and payload shapes remain unchanged. The worktree method argument is an internal-code hard cut coordinated with its only production caller. Profile and workspace selection still use the existing admission and registry owners; the same request context reaches selection and hydration. Tasks retains canonical query keys, profile partitions, cancellation, authoritative server rereads, stream fences and catalog reconciliation. There are no native-tool ID, CLI, hook, config, persisted-state, official-skill or site changes in this slice. The root report owns the consolidated change-impact entry.

Affected existing scenarios: `TA-task-inspect-single-live-stream` and `RT-worktree-api-surface-parity`. No browser/daemon walkthrough was run by this worker; historical scenario passes are not evidence for this revision. Root owns the integrated validation and current-head CI.

## Final-source verification

- Host Bun 1.4.4 diagnostic comparison: Tasks stream suite passes 19/19 after the change.
- Pinned Bun 1.4.0 final-source Tasks stream suite: 19/19 pass, Turbo cache miss (`.cache/performance/api-tasks/task-stream-pinned-final.log`). Command: `python3 .cache/performance/run-bounded.py bunx turbo run test --filter=./web --only -- src/systems/tasks/hooks/__tests__/use-task-stream.test.tsx --maxWorkers=1`.
- `gofmt` and `oxfmt` applied to the changed files. Test-shape checker passes `tasks_internal_test.go` and `worktree_transport_parity_test.go`. It reports the same single pre-existing direct-assertion function, `TestCreateSessionWorktreeBinding`, in both baseline `de5ab2f4d` and current `worktrees_test.go`; that unrelated function is unchanged. Receipt: `.cache/performance/api-tasks/test-conventions-final.log`. New aggregate cases use named subtests.
- API core and HTTP/UDS parity race check: pass after SQLC generation (`core` 2.223 s; `testutil` 2.462 s). Command: `python3 .cache/performance/run-bounded.py env CGO_ENABLED=1 go test -race ./internal/api/core ./internal/api/testutil -run 'TestWorktreeAggregate|TestProfileAwareWorktreeHandlers|TestWorkspaceBreadth|TestWorktreeHTTPUDSTransportParityIT033' -count=1`. Receipt: `.cache/performance/api-tasks/api-race-final.log`. These are test durations, not performance measurements.
- Integrated codegen, affected typecheck/lint and broader integration/platform/CI validation: root-owned; not claimed by this focused report.

## Measurement attempts

- A direct Bun import of the production task coordinator was unsuitable because the query-key profile barrel reaches a Vite `import.meta.glob` module. This failed before measuring; it is not a baseline.
- The first root-Turbo test attempt stopped at `codegen-check` because a concurrent store change required generated `extension_tool_visibility.sql.go`. The root agent owns generation; this failed gate is retained in `.cache/performance/api-tasks/task-stream-baseline-test.log` and is not test evidence.
- Formatting succeeded for the changed core/Tasks files, but the first convention-check command incorrectly supplied two paths to a one-file checker. It exited before checking; the corrected one-file invocations are tracked in `.cache/performance/api-tasks/test-conventions.log`.
- The first final API race invocation stopped during compilation because concurrent store work referenced the not-yet-generated `ListTaskEventsForTasks` method. No API test ran, and the chained pinned Tasks command was not reached. Receipt: `.cache/performance/api-tasks/final-focused.log`. Root regenerated SQLC; Tasks passed independently and the API check was requeued after generation.
