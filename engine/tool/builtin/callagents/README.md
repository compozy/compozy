# cp\_\_call_agents Development Notes

## Implementation Summary

- Introduced a new native builtin (`cp__call_agents`) that fans out to multiple agent executions in parallel while preserving the existing single-agent implementation.
- Added `NativeCallAgentsConfig` with `enabled`, `default_timeout`, and `max_concurrent` tunables that flow from config providers, schema definitions, and defaults.
- Built a dedicated executor that combines `sync.WaitGroup.Go` with a weighted semaphore to cap concurrency and guard resource usage.
- Captures per-agent telemetry via `builtin.RecordStep` and aggregates invocation metrics with `builtin.RecordInvocation`.
- Returns structured results containing success/error metadata per agent plus aggregate counts/duration for downstream orchestration.

## Concurrency Pattern

- `sync.WaitGroup.Go` manages goroutine lifecycle without manual `Add`/`Done` bookkeeping.
- `golang.org/x/sync/semaphore.Weighted` limits simultaneous agent executions to `max_concurrent`.
- Each agent runs with a context derived from the caller to propagate cancellation and timeout overrides.
- Panics in worker goroutines are recovered, logged, and surfaced as structured failures rather than crashing the process.

## Configuration Reference

| Field                                              | Default | Description                                                                                 |
| -------------------------------------------------- | ------- | ------------------------------------------------------------------------------------------- |
| `runtime.native_tools.call_agents.enabled`         | `true`  | Enables the builtin.                                                                        |
| `runtime.native_tools.call_agents.default_timeout` | `60s`   | Per-agent timeout when `timeout_ms` is not supplied.                                        |
| `runtime.native_tools.call_agents.max_concurrent`  | `10`    | Maximum number of agents executed simultaneously and upper bound for `agents` array length. |

All values support environment overrides and appear in `compozy config show` alongside other native tool settings.

## Testing & Quality

- Unit tests cover request decoding, validation, aggregation, and error reporting across success, invalid input, and partial failure cases.
- Executor tests assert order preservation, panic recovery, and concurrency limits.
- Integration test (`test/integration/tool/call_agents_integration_test.go`) exercises the builtin end-to-end inside the LLM service.
- Run `go test -race ./engine/tool/builtin/callagents/...` to validate for data races before invoking `make test` and `make lint` per repo standards.

## Performance Considerations

- Outputs only clone agent responses when provided, avoiding unnecessary allocations.
- Parallel execution shortens wall-clock latency compared to sequential `cp__call_agent` chains while the semaphore keeps peak load predictable.
- Aggregate duration is reported so orchestrators can monitor how close workloads are to overall SLA budgets.
