# Compozy E2E Testing Assessment & Plan

## Executive Summary

- The only E2E test (`test/e2e/orchestrate_e2e_test.go:1`) short-circuits the HTTP server, injects real OpenAI credentials, and exercises just one orchestrator handler; it never boots Temporal, Postgres, Redis, or the Bun runtime, so it misses the execution paths users rely on when they run `make dev EXAMPLE=<name>`.
- `make test-e2e` (`Makefile:210-216`) merely runs that single package, so regressions in examples under `examples/` go unnoticed until manual verification.
- Spinning up the development stack requires rich infrastructure—Temporal, Postgres, Redis, MCP proxy, Bun—wired together by the CLI entrypoint (`cli/cmd/dev/dev.go:18-87`) and server lifecycle (`engine/infra/server/lifecycle.go:17-44`), yet the current suite provides no automation to provision or validate those dependencies.
- We must replace the current “mocked E2E” approach with a deterministic harness that boots the real server, replays the HTTP flows from each `api.http`, asserts database state via existing helpers, and captures structured logs—without hitting external LLMs or MCP backends.
- The plan below proposes a phased implementation: baseline current pain points, introduce a reusable test harness, add an HTTP scenario runner capable of interpreting `api.http` files, stand up ephemeral infrastructure (Postgres, Redis, Temporal, Bun, MCP stubs), build deterministic LLM fixtures, and roll out coverage across the examples.

## Current State Analysis

### Existing E2E Coverage

- `test/e2e/orchestrate_e2e_test.go:30-104` requires `OPENAI_API_KEY`, constructs in-memory stores, directly calls `orchestrate.Definition(env).Handler`, and asserts JSON output. No HTTP traffic, no database, no runtime/tool execution, and the test is skipped unless an OpenAI key is present.
- `Makefile:210-216` shows `make test-e2e` simply runs `go test -tags e2e ./test/e2e/...`, so the entire target equates to that single orchestrator test.

### Manual Workflow Expectations

- Developers rely on `make dev EXAMPLE=<example>` (`Makefile:104-115`) followed by the example’s `api.http` file (e.g., `examples/agentic/api.http`) to validate behaviour.
- `cli/cmd/dev/dev.go:18-88` creates a full server, resolves `.env`, and expects usable Temporal, Postgres, Redis, and Bun binaries.

### Server Requirements

- Server startup (`engine/infra/server/lifecycle.go:17-45`) calls `setupDependencies` and `buildRouter`, then blocks on an OS signal; the current tests never reach this code.
- Dependency set-up (`engine/infra/server/dependencies.go:30-189`) initializes Redis, resource stores, monitoring, the Postgres repository provider, MCP proxy, and the Temporal worker. Temporal reachability is mandatory (`engine/infra/server/worker.go:106-131`)—if `cfg.Temporal.HostPort` isn’t reachable, startup aborts.
- `cluster/docker-compose.yml:34-160` documents the dev stack: Redis, Temporal (with its own Postgres), application Postgres, Temporal UI, etc. None of these services are orchestrated in tests today.

### Available Helper Infrastructure

- `test/helpers/database.go:38-183` and `test/helpers/testcontainer_retry.go:19-117` already provide resilient Postgres containers with migrations and retry logic.
- `test/helpers/server/server.go:42-114` builds an in-memory Gin engine with a real config manager, Postgres connection, and registered routes—useful but limited to in-process HTTP handlers (no actual network listener or worker thread supervision).
- Database assertions (`test/helpers/db_verifier.go:19-173`) poll workflow/task repositories and can be reused for state validation.
- Redis helpers (`test/helpers/redis.go`) and miniredis provide lightweight caching substitutes.

### Example Landscape

- All examples run on Bun (`examples/*/compozy.yaml`), so the Bun binary must be discoverable (`engine/runtime/bun_manager.go:274-320`). Missing Bun leads to immediate runtime failure.
- Models reference external providers (OpenAI, Groq, etc.)—see `examples/agentic/compozy.yaml:8-26` and `examples/github/compozy.yaml:13-35`. Deterministic tests need to override these with `core.ProviderMock` (`engine/llm/adapter/providers.go:19-125`) or scripted LLM responses.
- Some examples integrate MCP servers (`examples/github/compozy.yaml:10-35`), requiring accessible MCP endpoints or faithful stubs.
- The `api.http` scripts range from simple POST/GET sequences (`examples/weather/api.http`) to flows with variable substitution and response chaining (`examples/sync/api.http`).

### Logging & Observability

- Tests default to `logger.NewForTests()` which discards output (`pkg/logger/mod.go:150-218`), so today’s suite cannot assert log content. Any harness must supply a configurable logger to capture structured logs without polluting stdout.

## Pain Points & Constraints

1. **False E2E coverage:** No HTTP requests, no Temporal worker, no runtime tasks, no persistence verification. Manual testing remains the only guard rail.
2. **External service coupling:** Current “E2E” depends on live LLM credentials and assumes a developer has already started Temporal/Redis/Postgres/Bun; CI cannot meet those expectations.
3. **Example drift:** Each example evolves independently; lack of automated execution means breaking changes surface late.
4. **Complex flows:** `api.http` scripts rely on variables like `{{runAgentAsync.response.body.data.exec_id}}`; we need an executor that mirrors the VS Code REST client semantics to reuse these assets.
5. **Asynchronous orchestration:** Workflows often queue async executions; tests must poll both HTTP endpoints and DB state to confirm completion.
6. **Artifact visibility:** Without structured logs and database traces, diagnosing CI flakiness or regression roots is painful.

## Goals & Success Criteria

1. **Automate the full path a user exercises**—start the server in example context, execute the documented HTTP calls, and confirm database + log outcomes.
2. **Deterministic, offline execution**—no calls to external LLMs or MCP services; substitute scripted mocks with reproducible payloads.
3. **Self-provisioned dependencies**—Postgres, Redis, Temporal (ideally via temporalite or a slim container), Bun availability checks, and optional MCP stubs must be orchestrated within the harness.
4. **Fixture-driven validation**—reuse existing helpers (`test/helpers`), record Golden JSON/logs per scenario, and surface diffable artifacts.
5. **CI-grade reliability**—parallel-friendly, bounded runtimes, failure triage, and easy retries via `make test-e2e`.
6. **Metric instrumentation with telemetry hooks**—baseline current manual duration/failure rates using structured timing/log counters, and keep emitting the same metrics from the automated suite so improvements are measurable over time.
7. **Service-level targets**—keep full-suite runtime under 5 minutes on CI hardware and sustained flake rate below 2%; any regression triggers automated alerting.

## Proposed Architecture

### 1. Test Harness Skeleton

- **Example Workspace Manager**: Materialise each scenario in a temp directory using immutable overlays—leave the source tree untouched, render `compozy.yaml` overlays via a merge step, and project `.env` overrides through environment variables. This keeps manual `examples/` flows and automated tests in lockstep while still allowing scenario-specific tweaks.
- **Config Overrides**: Build a layered override system that:
  - Forces `cfg.Server.Port = 0` for ephemeral listeners.
  - Sets `cfg.Server.SourceOfTruth = repo` and disables auth/rate limits as in `NewServerHarness`.
  - Points `cfg.Database.ConnString` at the shared test container (`test/helpers/database.go:38-61`).
  - Injects deterministic runtime config (e.g., logging, timeouts, ToolExecutionTimeout).
- **Logger Injector**: Instantiate `logger.NewLogger` with a `bytes.Buffer` output and attach via `logger.ContextWithLogger` so we can persist scenario-specific logs.

### 2. Dependency Orchestration

- **Postgres**: Reuse shared container helpers for fast startup (`test/helpers/database.go:38-183`); allocate unique schemas per parallel worker (or per scenario) and run within transactions so tests never contend for shared tables; clean schemas on teardown.
- **Redis**: Leverage `test/helpers/redis.go` (miniredis) or an ephemeral container for features that depend on caching/rate limiting.
- **Temporal**: Adopt a hybrid strategy decided up front. Phase 1 will prototype Temporalite (`go.temporal.io/server/temporalite`) as the default harness backend for fast feedback, while nightly/pre-release CI drives against a Dockerised Temporal stack to ensure parity (namespaces, search attributes, worker registration). The harness exposes a switch so the same orchestration path satisfies both modes, and `cfg.Temporal.HostPort` is always resolved through the shared startup routine (`engine/infra/server/worker.go:106-131`).
- **MCP Proxy**: For MCP-dependent examples, provide a local stub that mimics `/admin/mcps` and tool streaming used by the GitHub scenario. We can reuse patterns from `test/integration/llm/mcp_strict_mode_test.go`.
- **Bun Runtime**: Detect the Bun binary via `$PATH` or `mise`. If absent, mark affected scenarios as `Skip` with a clear remediation.

### 3. Server Runner

- Start the server exactly the way production and `make dev` do: invoke the CLI command path (`cli/cmd/dev`) so `config.ContextWithManager`, dependency setup, worker startup, and reconciler wiring all follow the same sequence. For tests we only swap the listener by providing a pre-bound `net.Listener` (e.g., through a small adapter or `http.Server.BaseContext`) that captures the ephemeral port while leaving `Server.Run` untouched.
- Provide harness helpers to request shutdown through the existing `Server.Shutdown()` path and to surface the bound address so scenario runners can fill `{{baseUrl}}`.

### 4. HTTP Scenario Runner

- Reuse before reimplementing: first evaluate existing `.http`/REST-client parsers (VS Code REST Client CLI, `restclient` Go libraries, etc.). Only if none satisfy our needs do we fall back to a thin interpreter that supports:
  - Variable declarations (`@var = value`).
  - Named requests (`# @name foo`).
  - Template expressions `{{var}}` and `{{request.response.body.path}}`.
  - Auto-binding JSON bodies, headers, and method/path.
  - Recording full request/response payloads for logging.
    Any fallback parser must explicitly document unsupported features so we fail fast when authors use them.
- Execute requests sequentially, honouring delays or retries for async flows, and provide an opt-in polling helper for long running operations.
- Add first-class streaming/SSE support: buffer chunked responses, expose assertions on event order/payload, and archive the raw stream for troubleshooting.
- Expose hooks so scenario specs can add custom assertions (e.g., verifying JSON schema, matching response headers).

### 5. Verification & Artifacts

- **HTTP Assertions**: Validate status codes & payload snippets per step; allow scenario-specific check functions.
- **Database Assertions**: Reuse `DatabaseStateVerifier` (`test/helpers/db_verifier.go:19-198`) to ensure workflows/tasks reach the expected `core.StatusSuccess` or other statuses.
- **Log Capture**: Persist structured logs per scenario (e.g., `artifacts/<example>/logs.jsonl`) and assert on key/value pairs or log levels rather than raw strings.
- **Structured Reports**: Summarize each scenario (requests made, durations, DB verifications) and write to disk for CI artifacts.

### 6. Deterministic LLM & Tooling

- Override example `models` to use `core.ProviderMock` (`engine/llm/adapter/providers.go:19-125`).
- Introduce scenario fixtures that map prompts/actions to canned outputs using `DynamicMockLLM` (`engine/llm/adapter/dynamic_mock.go:19-85`) or test adapters.
- For complex orchestrations, store recorded tool call sequences (e.g., as YAML) and replay via a custom `llm.WithLLMFactory` override injected through the config manager.

## Example Coverage Strategy

| Example                               | Key Paths                         | Special Handling                                   | Test Focus                                                        |
| ------------------------------------- | --------------------------------- | -------------------------------------------------- | ----------------------------------------------------------------- |
| `prompt-only`                         | `examples/prompt-only/api.http`   | Minimal LLM usage—ideal starter for harness PoC.   | Validate server bootstrap, HTTP runner, log capture.              |
| `sync`                                | `examples/sync/api.http`          | Uses sync + async endpoints and response chaining. | Exercise variable interpolation and async polling.                |
| `weather`                             | `examples/weather/api.http`       | Simple workflow with LLM summarization.            | Demonstrate deterministic LLM responses & DB verification.        |
| `agentic`                             | `examples/agentic/api.http`       | Orchestrator, native tools, Bun runtime.           | Validate tool environment, multi-step DB checks.                  |
| `code-reviewer`                       | `examples/code-reviewer/api.http` | Heavy file IO, Bun runtime, large payloads.        | Stress test Bun sandbox & log volume.                             |
| `wait-task` / `schedules` / `signals` | Various                           | Require Temporal scheduling & signals.             | Validate Temporal stub configuration and schedule reconciliation. |
| `github`                              | `examples/github/api.http`        | MCP streams, external APIs.                        | Provide MCP stub service and repository fixture data.             |
| `memory`                              | `examples/memory/*.yaml`          | Uses Redis-backed memory store.                    | Exercise Redis helper and memory assertions.                      |

Roll out coverage incrementally—start with `prompt-only` & `sync`, then expand once the harness stabilizes.

## Implementation Roadmap

### Phase 0 – Baseline & Scoping

1. Instrument the current manual process end-to-end: wrap `make dev` + `api.http` runs with a thin CLI that records duration, exit status, and key logs to NDJSON so the baseline is machine-readable.
2. Document pass/fail history from CI or `main` branch to quantify flakiness.
3. Codify acceptance criteria for each example (expected HTTP status, workflow IDs, structured log keys, and metrics that must be emitted).

### Phase 1 – Harness Foundation

1. Build `ExampleWorkspaceManager` and config override loader.
2. Integrate Postgres/Redis helpers and create a stub Temporal launcher (temporalite or container).
3. Add logger capture plumbing and `make test-e2e` gating that fails fast if Bun is missing.

### Phase 2 – HTTP Scenario Runner MVP

1. Integrate an off-the-shelf `.http` parser if viable; otherwise implement the constrained fallback interpreter defined above.
2. Execute the `prompt-only` scenario end-to-end, asserting HTTP responses and logs.
3. Emit structured artifacts (HTTP transcript, metrics snapshot) to a temp directory; ensure cleanup honours `t.Cleanup`.

### Phase 3 – Deterministic LLM & Tool Fixtures

1. Add LLM factory overrides per scenario, defaulting to `MockLLM`, and enforce deterministic model parameters (temperature = 0, top-p disabled) via config overlays.
2. Create fixture registry (JSON/YAML) describing expected tool outputs.
3. Refactor harness to inject `llm.WithLLMFactory` via config manager overrides.

### Phase 4 – Asynchronous & DB Verification

1. Extend HTTP runner to support response chaining (e.g., `{{runAgentAsync.response.body.data.exec_id}}`).
2. Integrate `DatabaseStateVerifier` to poll workflow/task state after each scenario.
3. Build helper assertions for schedule/async endpoints (e.g., poll `/executions/agents/{id}`).

### Phase 5 – Advanced Dependencies

1. Provide MCP stub server with scripted responses; point `MCP_PROXY_URL` to it.
2. Finalize Temporal integration—ensure worker reconciliation completes (`engine/infra/server/worker.go:133-214`).
3. Add Redis-backed memory validations for the `memory` example.

### Phase 6 – Coverage Rollout & CI Integration

1. Onboard remaining examples incrementally, adding fixtures as needed.
2. Parallelize scenarios using worker-aware resource pools: pre-allocate ephemeral ports via listeners, assign unique DB schemas per worker, and reuse shared containers to avoid startup storms.
3. Update CI to run `make test-e2e`; publish artifacts (logs, HTTP traces, DB snapshots) for failed scenarios.

### Phase 7 – Observability & Maintenance

1. Track runtime metrics (duration, retries, flake rate) and assert thresholds.
2. Document harness usage in `docs/` and ensure new examples ship with fixtures.
3. Add guardrails so PRs modifying `examples/` must update/extend E2E fixtures.

## Validation & CI Strategy

- **Pre-submit**: Developers run `make test-e2e EXAMPLE=<name>` to execute a focused scenario using the shared harness.
- **CI**: Run the full suite in its own job, archiving artifacts on failure. Use tags to optionally skip MCP- or Bun-dependent suites if prerequisites are missing.
- **Metrics**: Emit Prometheus-friendly metrics (duration, success) during test runs for long-term tracking.
- **Alerts**: Flag regressions when scenarios exceed SLA or fail consistently; incorporate into release readiness checks.

## Risks & Mitigations

| Risk                                                                      | Impact                                 | Mitigation                                                                                                                                |
| ------------------------------------------------------------------------- | -------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------- |
| Temporalite or container startup times balloon CI runtime.                | Slower feedback loops.                 | Cache containers between tests, share Temporal per `go test` invocation, and only start it for scenarios that need scheduling.            |
| Bun binary missing or incompatible in CI.                                 | Immediate test failures.               | Add a pre-flight check that surfaces actionable guidance and default to skipping Bun-heavy scenarios until tooling is installed.          |
| Parsing `api.http` misses edge cases (e.g., custom headers, multipart).   | Coverage gaps.                         | Start with core features in existing files; fall back to scenario-specific Go code for exotic cases until the parser is extended.         |
| MCP stubbing diverges from real protocol.                                 | False confidence.                      | Derive stub behaviour from integration tests and optionally allow running the real MCP proxy in a nightly suite.                          |
| Maintaining deterministic LLM fixtures becomes onerous as prompts change. | Fixture drift causing false negatives. | Keep fixtures close to example source, add helper scripts to regenerate fixtures from controlled dry runs, and document required updates. |

## Open Questions

1. How frequently should we exercise the high-fidelity (Docker Temporal) mode—per PR, nightly, or release only?
2. What is the minimum acceptable runtime for `make test-e2e` in CI (per scenario and overall)?
3. What retention policy should we follow for captured artifacts (HTTP transcripts, logs, metrics) to aid debugging without bloating storage?
4. Which team owns fixture maintenance when examples evolve (core vs. feature teams)?
5. Do we need golden snapshots of HTTP responses for regression diffing?

## Immediate Next Steps

1. Prototype the example workspace + config override to run the `prompt-only` scenario with a mocked LLM.
2. Decide on Temporal strategy (temporalite vs. Docker) and spike the setup in the harness.
3. Draft fixture format for deterministic LLM/tool responses and validate against one scenario.
4. Share this plan with stakeholders, gather feedback, and refine phase breakdown before implementation.
