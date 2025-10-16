# Workflow & Task Usage Storage Refactor

## Background

- Usage metrics are currently stored in a dedicated `execution_llm_usage` table managed by `UsageRepo` (`engine/infra/postgres/usage_repo.go:105`). Task and workflow state repositories never load that table, so API layers call a second repository to hydrate responses (`engine/task/router/exec.go:61-76`, `engine/workflow/router/execs.go:50-113`, `engine/infra/server/router/usage.go:16-83`).
- Collectors persist usage after each LLM loop via `Collector.Finalize`, which writes to `UsageRepo` (`engine/llm/usage/collector.go:135-214`). Task/workflow state updates happen separately, so persistence is split across two transactions.
- Schemas (`engine/infra/postgres/migrations/20250603124915_create_task_states.sql`, `.../20250603124835_create_workflow_states.sql`) have no usage columns; aggregator queries join task states at read time to summarize workflow totals (`engine/infra/postgres/usage_repo.go:169-220`).

## Pain Points

- **High query cost**: APIs repeatedly join `execution_llm_usage` to `task_states` to build workflow usage summaries, causing N+1 risks and avoidable load on larger workflows.
- **Split transactions**: Usage persistence is decoupled from task/workflow state updates, creating a consistency window and complicating error handling.
- **API plumbing overhead**: Routers, workers, and CLI must resolve a separate repo, making DTO hydration slower and harder to reason about.
- **Limited aggregation strategy**: Workflow totals are computed on demand, and provider/model attribution collapses to `"mixed"` when multiple rows exist.

## Goals & Guardrails

1. Store usage alongside state rows as JSONB (`usage` column) for both `task_states` and `workflow_states`.
2. Populate usage atomically during task/workflow state transitions—no separate repo or joins.
3. Update API/CLI layers to read usage directly from state structs.
4. Keep current metrics/telemetry behaviour.
5. Adopt a greenfield cutover (drop legacy table, no dual writes).
6. Honor repo-wide rules: context-first logging/config (`logger.FromContext`, `config.FromContext`), no functions >50 lines, no backwards compatibility branch.

## Proposed Architecture

### 1. Data Model Changes

- Add `usage JSONB` column to `workflow_states` and `task_states`. Enforce `jsonb_typeof(usage) = 'array'` (or NULL) and validate each element is an object with required token fields. JSON shape (shared by task/workflow):
  ```json
  [
    {
      "provider": "openai",
      "model": "gpt-4o-mini",
      "prompt_tokens": 812,
      "completion_tokens": 265,
      "total_tokens": 1077,
      "reasoning_tokens": 0,
      "cached_prompt_tokens": 120,
      "input_audio_tokens": 0,
      "output_audio_tokens": 0,
      "agent_ids": [
        "AGENT123",
        "AGENT456"
      ],
      "captured_at": "2025-10-16T15:30:00Z",
      "updated_at": "2025-10-16T15:32:40Z",
      "source": "task" // "task" or "workflow"
    }
  ]
  ```
- Each entry represents totals for a single provider/model pair. Workflow rows aggregate across every task entry already persisted; tasks also maintain per-model entries when subtasks or retries invoke different providers.
- Drop `execution_llm_usage`, its trigger, and indexes in the same migration (greenfield cutover).

### 2. Domain & Serialization

- Introduce `usage.Summary` (new file `engine/llm/usage/summary.go`) exposing `Entries []usage.Entry`. Provide helpers to merge token deltas by provider/model (`MergeEntry`, `AddTokens`, `MergeAgentID`) and normalize timestamps.
- Extend `task.State` with `Usage *usage.Summary` and `task.StateDB` with `UsageRaw []byte`. Update `StateDB.ToState` to unmarshal the JSON array and validate each entry (`engine/task/domain.go`).
- Extend `workflow.State` similarly (`engine/workflow/domain.go`).
- Add helpers in `engine/infra/postgres/jsonb.go` to convert summaries to/from `[]byte`.

### 3. Persistence Layer

- Update column lists in `taskStateColumns` / `workflow` equivalents to include `usage`. Modify upsert SQL to write `usage` via `$15` parameter and keep `updated_at` semantics (`engine/infra/postgres/taskrepo.go:162-188`).
- Implement `TaskRepo.UpdateUsage(ctx, execID, entries)` that `SELECT ... FOR UPDATE`, merges provider/model entries, and writes back the JSON array. Mirror for `WorkflowRepo`. Use transactions (`WithTransaction`) to gate concurrency.
- Remove `UsageRepo` and provider wiring: delete `engine/infra/postgres/usage_repo.go`, `engine/infra/repo/provider.go:37-40`, `engine/infra/server/router/helpers.go:457-468`, and associated tests. Replace with lightweight helpers that read `state.Usage`.

### 4. Collector & Execution Flow

- Refactor `Collector.Finalize` to return `(*usage.Finalized, error)` where `Finalized` bundles metadata + entry slice. Persisting becomes caller responsibility. Metrics callbacks stay untouched.
- Direct executor updates (`engine/task/directexec/direct_executor.go:452-540`): after status update, call `collector.Finalize`, attach entries to `task.State`, persist via `TaskRepo.UpdateUsage`, then call `WorkflowRepo.AccumulateUsage` to merge per-model entries into the workflow array.
- Temporal activities (`ExecuteBasic.attachUsageCollector`, `ExecuteSubtask.attachUsageCollector`) adopt the same flow—finalize, persist task entries, accumulate workflow usage.
- Orchestrator middleware remains unchanged; collectors still aggregate snapshots from `conversationLoop.recordLLMResponse`.

### 5. Read Path Simplification

- DTO builders now read from `state.Usage` directly. Example: `newTaskExecutionStatusDTO` maps each entry into the API slice without resolving another repo (`engine/task/router/exec.go:74-99`).
- Workflow listing surfaces aggregated usage arrays from `workflow.State.Usage` and per-task arrays already embedded in `state.Tasks` (populated through repo). Remove `preloadWorkflowUsageSummaries` entirely.
- CLI (`cli/api/services.go`) and swagger generation continue to rely on `apitypes.UsageSummary`; add conversion helpers that expand `usage.Entry` slices.

### 6. Observability

- Metrics emission stays in `Collector` (still records success/failure).
- `usage.Entry` stores timestamps so we can still compute latency deltas if needed.
- No change to Prometheus counters: they consume the summary before persist.
- Keep audit logging on persistence failures using existing logger patterns.

## Migration & Deployment Plan

1. **Migration 20251016170000_embed_usage_jsonb.sql**
   - Add nullable `usage JSONB` (+ check constraint) to `workflow_states` and `task_states`.
   - Drop trigger/function/index definitions tied to `execution_llm_usage`, then drop the table.
2. **Code Refactor**
   - Introduce `usage.Summary`, update domain structs, remove `UsageRepo`, refactor collectors, and rewrite routers.
3. **Schema Validation**
   - Run `goose status` and ensure migrations apply on dev DB.
4. **App Testing**
   - Run `make lint` and `make test` (project standard).
   - Execute targeted suites (`gotestsum -- -race -parallel=4 ./engine/llm/... ./engine/infra/postgres/...`) during development, full `make test` before completion.
5. **Rollout**
   - Deploy migration + code together (greenfield). No feature flag needed.
   - Monitor usage metrics and API latency for regression.

## Implementation Breakdown

1. **Model & Repo foundations**
   - Add `usage.Summary` with per-model `usage.Entry` slices, modify `task/workflow` domain structs & repositories, write unit tests for array serialization and merge semantics.
2. **Collector refactor**
   - Change `Finalize` signature, update direct executor and task activities, introduce helper to persist aggregated per-model usage.
3. **API/CLI adjustments**
   - Remove `ResolveUsageRepository`, map entry arrays in DTO builders, update swagger generation, refresh CLI models.
4. **Cleanup & Tests**
   - Delete legacy repo/tests, update integration suites (worker helpers now stub `TaskRepo`), ensure `make lint && make test` pass.
5. **Docs & Dashboards**
   - Update developer docs (this file + `./tasks/prd-usage`) to reflect embedded model; confirm dashboards can query JSONB if needed.

## Testing Strategy

- **Unit**:
  - `usage.Summary` merge and normalization logic (distinct provider/model entries, agent ID aggregation).
  - `TaskRepo.UpdateUsage`, `WorkflowRepo.AccumulateUsage` ensuring JSONB round-trip.
  - Collector finalization returns expected summaries.
- **Integration**:
  - Direct executor flow writes usage into both state tables.
  - Workflow listing returns usage without extra queries (use pgxmock to assert single SELECT).
  - Temporal activities persist usage via new helpers.
- **API/CLI**:
  - Update router tests to assert usage arrays present on DTOs without stubbing repos.
  - CLI serialization golden tests.
- **Regression**:
  - Run `make lint`, `make test` prior to completion (mandatory project rule).
  - Smoke test for workflows with parallel tasks to confirm aggregation works under concurrent completions.

## Impacted Files / Modules

- `engine/llm/usage/collector.go` (Finalize contract)
- `engine/llm/usage` (new `summary.go`)
- `engine/task/domain.go`, `engine/workflow/domain.go`
- `engine/infra/postgres/taskrepo.go`, `workflowrepo.go`, `jsonb.go`
- `engine/task/directexec/direct_executor.go`, `engine/task/activities/exec_basic.go`, `exec_subtask.go`
- `engine/infra/server/router/...` (helpers, usage, agent/task/workflow routes)
- `cli/api/services.go` + generated swagger/docs
- Legacy deletions: `engine/infra/postgres/usage_repo.go`, `test/integration/repo/usage_test.go`, helper stubs, provider wiring (`engine/infra/repo/provider.go:37-40`), worker dependency injection (`engine/infra/server/worker.go`).

## Risks & Mitigations

- **Concurrent workflow aggregation**: simultaneous task completions could clobber JSON if not locked. Mitigation: use `FOR UPDATE` and pure Go merge helper to ensure atomic increments.
- **JSON drift / validation**: invalid JSON arrays would break DTOs. Mitigation: add schema check constraint and validation helper when unmarshalling.
- **API contract change**: ensure DTOs stay backward-compatible (usage arrays are optional).
- **Dashboard queries**: Dropping `execution_llm_usage` removes direct SQL analytics. Mitigation: document new location and provide sample JSONB query (`usage ->> 'prompt_tokens'`) referencing Postgres JSONB operators.
- **Migration ordering**: dropping table requires coordinated deploy; use migration guard and smoke test after deploy.

## Open Questions

1. Do we need to keep historic per-call usage granularity for analytics? (Current proposal drops it.)
2. Should we persist additional breakdowns (e.g., per-agent token splits or cost estimates) inside each usage entry?
3. Any downstream consumers reading `execution_llm_usage` directly (BI tooling)? Need confirmation before drop.

## References

- PostgreSQL JSON data types and functions (`jsonb`, `jsonb_set`, indexing guidance). citeturn0search2turn0search4
- Existing usage feature spec `./tasks/prd-usage` (workspace context).
- Project coding standards & architecture rules in `.cursor/rules/*.mdc` (reviewed prior to drafting).
