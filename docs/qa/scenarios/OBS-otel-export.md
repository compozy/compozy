---
id: OBS-otel-export
area: OBS
title: Export turn traces and metrics to a real OTLP collector
persona: Dora
journey: J-export-telemetry-to-collector
expected: With export enabled against the Compose Collector, each prompt is one session.turn trace in Jaeger with spawn, ACP, and hosted tool spans nested, the Prometheus p95 query returns a value, and status reports export health identically over CLI, HTTP, and UDS.
entry_points: compozy config set observability.otel.*; compozy status; compozy status --json; GET /api/status over HTTP; GET /api/status over UDS; web session composer; desktop app; compozy__config_set; http://localhost:16686; http://localhost:9090
qa_status: blocked-verify
bug_ids: BUG-20261111-web-adapter-requests-skip-traceparent; BUG-20261111-otel-model-exports-other; BUG-20261111-otel-model-probe-spawn-provider-other; BUG-20261111-otel-header-refs-redacted
fix_status: fixed
retest_status: pending
fix_commits: 47b40a7b4; 82347190f
evidence: docs/qa/evidence/2026-11-11-otel-telemetry-export/
last_report: docs/qa/reports/2026-11-11-otel-telemetry-export.md
overlaps: OBS-otel-export; RT-001
---

story: As the runtime administrator I point the daemon at the OpenTelemetry collector I already run and see, per turn,
where the time went, without the export carrying any prompt, tool, path, or credential content.

Contract: spec `otel-telemetry-export` (#1361) — `_dx.md` (golden path, status lines, errors, provider forwarding),
`_spec.md` Safety Invariants and ADR-001..003, `_tests.md` QA section. Automated proof already exists for every
hermetic behavior (see Reused evidence and Regression hot spots); these walks cover only what needs a real collector,
a real provider, a real browser, or the desktop app.

## Lab

- Build the daemon and web from the branch under test; run in an isolated lab from `eng-qa-bootstrap` (unique
  `COMPOZY_HOME`, HTTP port, UDS socket). From a managed Compozy session, unset inherited `COMPOZY_*` variables first
  so the lab never talks to the live daemon.
- Start the `_dx.md` Compose stack verbatim (`~/otel-local/{compose.yaml,collector.yaml,prometheus.yaml}`; Collector
  contrib 0.138.0, Jaeger 2.11.0, Prometheus 3.7.3): `cd ~/otel-local && docker compose up -d`. If 4317/4318/16686/9090
  are taken on the host, remap the host ports and use the remapped endpoint below; record the mapping.
- Teardown on every exit: `docker compose down -v` and the bootstrap manifest teardown (`teardown.json` `clean: true`).
- Evidence goes to `docs/qa/evidence/<date>-otel-telemetry-export/`. Never capture a resolved header value; redact
  `OTEL_EXPORTER_OTLP_HEADERS` values in any process-environment capture.

## 1. Golden path: Collector → Jaeger + Prometheus

Preconditions: lab daemon running, export disabled (fresh home), Compose stack up, one agent on a real provider whose
hosted MCP tools are enabled (claude or codex).

1. `compozy status` → `OTel:               disabled`.
2. `compozy config set observability.otel.enabled true` → record shows `Lifecycle: restart-required`,
   `Next Action: restart-daemon`, `Restart Required: true`.
3. `compozy config set observability.otel.endpoint http://localhost:4318` and
   `compozy config set observability.otel.metric_interval 5s` (lab speed; the default 60 s works the same, slower).
4. `compozy daemon stop && compozy daemon start`, then `compozy status` →
   `OTel:               ok · http/protobuf → localhost:4318 · traces pending, metrics pending · sample 1.0` until
   the first export, then `traces ok, metrics ok`.
5. `compozy session new --agent <agent>` (cold session), then
   `compozy session prompt <session-id> "Call the compozy__session_list tool and summarize my open sessions."`
6. Jaeger: `curl -s 'http://localhost:16686/api/traces?service=compozy-daemon&operation=session.turn&limit=5'`, or
   the UI (service `compozy-daemon`, operation `session.turn`).
7. Prometheus: `curl -s http://localhost:9090/api/v1/query --data-urlencode
   'query=histogram_quantile(0.95, sum by (le) (rate(compozy_session_turn_duration_seconds_bucket[5m])))'`
   after a second turn inside the 5 m window and one scrape (15 s) past the next metric push; after a single turn
   `rate()` sees no increase and the query returns `NaN`.
8. Privacy spot check: send a second prompt containing `QA-CANARY-otel-<date>`, then
   `curl -s http://localhost:16686/api/traces/<trace-id> | grep -c QA-CANARY` → `0`.

Expected:

- One trace for the prompt: `POST /api/workspaces/:workspace_id/sessions/:session_id/prompt` (server) →
  `session.turn` → {`acp.process.spawn` (cold session), `acp session/prompt` with a `first_token` event}, and
  `mcp.hosted.call` → `tool.execute` (`compozy.tool = compozy__session_list`, same `compozy.run_id` as the turn)
  nested inside `session.turn`. `session.turn` carries `compozy.session_id`, `compozy.turn_id`, `compozy.agent`,
  `gen_ai.system`, `compozy.outcome = ok`, `compozy.db.flushes`, `compozy.db.write_ms_total`.
- The p95 query returns a non-empty vector; `compozy_tokens_total`, `compozy_tool_calls_total`, and
  `http_server_request_duration_seconds_bucket` series exist.
- The canary never appears in the exported trace.
- `session.turn` and `compozy.session.turn.duration` carry `gen_ai.request.model` = the catalog name the session
  reports (for example `claude-opus-5-5`), `custom` for a configuration-only model, and never a raw model string
  (BUG-20261111-otel-model-exports-other, fixed in `82347190f`). The classifier reads a catalog snapshot rebuilt
  every 5 minutes, and about 30 s after a lookup misses (`db86be3de`), so on a fresh home the first turns may
  export `other` until the catalog learns the model. Check the value on a turn at least a minute after the
  first boot, or after a restart with the catalog already persisted. Metrics classify when they record, so a
  series keeps the label it started with.

Evidence: status text before and after, Jaeger trace JSON and a screenshot of the expanded trace, Prometheus query
JSON, canary grep output.

Reuses: E2E-001 `TestDaemonE2EOTelGoldenJourney`, IT-001 `TestDaemonE2EOTelExportsOneTracePerTurn`, IT-007
`TestDaemonE2EOTelNestsHostedToolCallsInTheirTurn`, IT-011/IT-012 `TestDaemonE2EOTelExportsTurnMetrics`, IT-014/E2E-003
`TestDaemonE2EOTelExportCarriesNoContent` (all against hermetic receivers and acpmock). Remains: real Collector
protocol and Prometheus naming, a real provider's tool call through the hosted proxy, Jaeger rendering, and the
`_dx.md` p95 query as written.

Also check the exported catalog against `packages/site/content/docs/operations/telemetry-export.mdx` ("What is
exported"): every span and metric seen in Jaeger and Prometheus is listed there, with the documented attributes
(task_07 left this check for the finished branch).

Short-header regression: configure `X-Scope-OrgID` through an environment ref with value `1`, then repeat with
`OTEL_EXPORTER_OTLP_HEADERS=X-Scope-OrgID=1` and no file headers. Send a prompt containing
`I used compozy version 0.6.1 for task 1.`; the saved transcript must preserve it verbatim. Ordinary resource
and agent labels containing `1` must remain intact, while an exported attribute exactly equal to the header
value is masked. `TestInitSendsResolvedHeaders` owns the hermetic regression; final-head CI executes it.

## 2. Status over CLI, HTTP, and UDS: ok, metrics off, degraded, forwarding

Preconditions: walk 1 finished with status `ok`. For each state below, restart the daemon after the config change,
wait one metric interval, then read all four surfaces: `compozy status`, `compozy status --json`,
`curl -s <http-base>/api/status`, and `curl -s --unix-socket <uds-path> http://localhost/api/status` (HTTP base,
auth, and socket path from the lab manifest, as in RT-001). Compare the `otel` object across the three JSON reads.

| State | Change | Expected human line | Expected `otel` JSON |
| --- | --- | --- | --- |
| ok | none (walk 1) | `ok · http/protobuf → localhost:4318 · traces ok, metrics ok · sample 1.0` | `state: ok`, both signals `enabled: true`, `last_export_at` set, `failed_exports: 0`, `dropped_spans: 0`, `forward_to_providers: false` |
| metrics off | `config set observability.otel.metrics false` | `… · traces ok, metrics off · sample 1.0` | `metrics.enabled: false`, `state: ok` |
| both signals off | `traces false`; `metrics false` | `disabled` | both signals `enabled: false`; no telemetry model-catalog read at boot |
| degraded, unreachable | `metrics true`; `config set observability.otel.endpoint http://127.0.0.1:1` | `degraded · http/protobuf → 127.0.0.1:1 · traces unavailable (N failed), metrics unavailable (N failed) · sample 1.0` | `state: degraded`, `last_error_class: unavailable` on both signals |
| degraded, unresolved header | endpoint back to `http://localhost:4318`; `compozy config edit` adds `[observability.otel.headers] "api-key" = "env:QA_OTLP_MISSING"` with the variable unset | both signals `header_ref_unresolved` | `state: degraded`, `failed_exports: 0`; the daemon log masks the ref as `[REDACTED]` and never shows a value |
| degraded, 401 (optional) | Collector override with a `bearertokenauth` receiver on 4319, daemon header `"Authorization" = "env:QA_OTLP_AUTH"` set to a wrong token | `… metrics http_401 (N failed)` (and traces) | `last_error_class: http_401` |
| recovered | remove the header, endpoint `http://localhost:4318` | back to `ok` | error classes cleared, `failed_exports` totals reset by the restart |
| forwarding | `config set observability.otel.forward_to_providers true` | the `ok` line ends with `· forwarding to providers` | `forward_to_providers: true` |

Also: during every degraded state, a prompt completes normally (export never blocks a turn); `compozy status` exits 0.

Evidence: the four reads per state (human text and three JSON files), daemon log excerpt for the unresolved ref.

Reuses: UT-025/UT-026 (`TestStatusCommandReturnsDaemonStatus` renders every `_dx.md` row), UT-015/UT-016/UT-028
(`internal/telemetry` runtime and export-status tests), E2E-002 `TestDaemonE2EOTelExportReportsDegradedMetrics`
(401 on metrics, header sent, key absent from the log), `TestBootedDaemonWiresTelemetryThroughBothTransports`
(HTTP and UDS carry the status). Remains: the real daemon across restarts, real connection refusal, and cross-surface
equality; the 401 leg is optional because E2E-002 owns it.

With export disabled, start the isolated daemon with a fictitious credential-bearing environment endpoint
(`OTEL_EXPORTER_OTLP_ENDPOINT=https://collector.example/?auth=QA-ENDPOINT-CANARY`). Boot must succeed;
`compozy config get observability.otel.endpoint` and `compozy__config_get` must mask the endpoint and
never expose the canary. Repeat with URL userinfo and fragments. Restore the environment before walk 3.
The config boundary owns automated coverage in `TestApplyOTelEnvironment`; the daemon composition suite
`TestTelemetryModelClassifierOnlyWhenExportIsRequested` owns the both-signals-off catalog invariant.

## 3. Web and desktop requests join their trace

Preconditions: status `ok`, `sample_ratio` 1.0, lab web UI served against the lab daemon, desktop app built from the
branch and attached to the lab daemon.

1. Web (Playwright via `browser-use:browser`, `agent-browser` fallback): open a session, record network requests, send
   a prompt from the composer. Read the `traceparent` request header of the `POST …/sessions/<id>/prompt` request and
   of one ordinary API read (for example the session list `GET`).
2. For each captured header `00-<trace-id>-<span-id>-00`, run `curl -s http://localhost:16686/api/traces/<trace-id>`.
3. Desktop: with the app attached, capture one daemon-bound runtime health request (`GET /api/status/identity`) through
   the desktop logs or a proxy, and look its trace id up the same way. Send one prompt from the desktop window and
   repeat step 2 for it.
4. Set `sample_ratio 0`, restart, send a web prompt: no trace for that request appears.

Expected: every daemon-bound request carries a `-00` header with a fresh trace id; Jaeger's trace for that id has the
HTTP server span as its root, and for the prompt it continues into `session.turn`. Requests to non-daemon origins
(desktop update feed) carry no `traceparent`. At ratio 0 nothing is exported for client requests.

History: the composer prompt POST bypassed `apiClient` before e434ff7fd; every web daemon request now goes through
`daemonFetch` (47b40a7b4).

Evidence: HAR or Playwright request log with the headers, Jaeger trace JSON per trace id, screenshot of the web trace.

Reuses: UT-060–UT-064 (web `api-client.test.ts`, desktop `runtime-health-monitor` tests), IT-013
`TestRequestTracingHonorsTraceParentOverHTTPAndUDS`, UT-038 `TestSamplerHonorsSampledParentsAndTheRatioForTheRest`.
Remains: a real browser and the desktop app hitting the real daemon, and the prompt send path.

## 4. Provider forwarding with an OTel-capable provider

Preconditions: forwarding on (walk 2 last row), status `ok · … · forwarding to providers`. Use Claude Code
(`claude` provider): its telemetry needs `CLAUDE_CODE_ENABLE_TELEMETRY=1` and `OTEL_METRICS_EXPORTER=otlp` set in the
agent's environment. For this walk only, add `resource_to_telemetry_conversion: { enabled: true }` to the Collector's
`prometheus` exporter so resource attributes become labels (or read them from `target_info`).

1. Start a new session on that agent and send one prompt.
2. Read the provider process environment (`pgrep -af` for the provider command, then
   `tr '\0' '\n' < /proc/<pid>/environ | grep -E '^(OTEL_|TRACEPARENT)'`), redacting header values.
3. Query Prometheus for the provider's own series (`{__name__=~"claude_code_.*"}`) and check their
   `compozy_session_id` label.
4. Set an agent-level `OTEL_EXPORTER_OTLP_ENDPOINT` to a different value, start another session, repeat step 2.
5. Turn forwarding off, restart, start a session, repeat step 2.

Expected: step 2 shows `OTEL_EXPORTER_OTLP_ENDPOINT`, `OTEL_EXPORTER_OTLP_PROTOCOL`, `OTEL_RESOURCE_ATTRIBUTES` with
`compozy.session_id` and `compozy.agent`, `TRACEPARENT` equal to the turn's `acp.process.spawn` span, and
`OTEL_EXPORTER_OTLP_HEADERS` only when headers are configured. Step 3 finds provider series tagged with the session id.
Step 4 keeps the agent's value. Step 5 shows no `OTEL_*`/`TRACEPARENT` from the daemon. Docs check (US-008.AC-4): the
"Forward to providers" section of `telemetry-export.mdx` states in bold that forwarding is an operator policy not
covered by the daemon privacy guarantee, that the provider receives resolved header values, and that nothing is
forwarded while export is disabled, inactive, or degraded at start.

Evidence: redacted environment captures, Prometheus query JSON, docs excerpt with line numbers.

Reuses: UT-070–UT-073 (`TestTelemetryForwarding`, `TestProviderEnvForwardsOnlyAStartedExport`,
`TestProviderTelemetryForwardingYieldsToAgentEnvironment`), acp `TestDriverTurnSpans` (TRACEPARENT from the spawn
span). Remains: a real provider consuming the variables and exporting through the same Collector.

## 5. Background traces

Preconditions: status `ok`.

1. Scheduler: wait 30 s; Jaeger operation `scheduler.tick` shows root spans about every 15 s.
2. Automation: create an agent-target job (`compozy automation jobs create --help`), then
   `compozy automation jobs trigger <job-id>`.
3. Loop: `compozy loop run --name <two-node loop> --no-prompt`, wait for the run to finish (`compozy loop status`).
4. Hook: with a configured session hook (`compozy hooks list`), send one prompt.

Expected: `automation.run` is a root span with `compozy.automation.id` and the job's `session.turn` descends from it;
`loop.run` is a root with `compozy.loop.id` and both node turns descend from it; `hook.dispatch` spans with
`compozy.hook.event` sit under the turn that fired them; no background span carries prompt text.

Evidence: one Jaeger trace JSON per kind.

Reuses: IT-009/IT-010 `TestDaemonE2EOTelTracesBackgroundWork`. Remains: real scheduler cadence and real jobs and Loops
in Jaeger.

## 6. Agents cannot change telemetry settings

Preconditions: lab daemon running; any agent session.

1. Ask the agent to call `compozy__config_set` with `{"path":"observability.otel.enabled","value":"false"}`, and with
   `observability.otel.endpoint`.
2. Structured parity: `compozy tool invoke compozy__config_set --input
   '{"path":"observability.otel.endpoint","value":"http://evil.example:4318"}' -o json`.
3. `compozy tool invoke compozy__config_get --input '{"path":"observability.otel.headers.api-key"}' -o json` with a
   vault header configured.
4. `compozy config get observability.otel` and `compozy status`.

Expected: steps 1–2 refuse with `config_trust_root_forbidden`; nothing changes in `config.toml` and the status is
unchanged. Step 3 returns `[REDACTED]`, never the ref or the resolved value; step 4 (`compozy config get`) shows the ref. If the operator-side
`tool invoke` path is allowed instead of refused, record which caller identity the refusal applies to and compare with
`_dx.md` ("refuses every `observability.otel.*` key").

Evidence: tool results JSON, `config get` output before and after.

Reuses: `TestDaemonNativeTools` (`observability.otel.enabled`/`endpoint` refused with
`ReasonConfigTrustRootForbidden`), `TestToolConfigPathPolicy`, `TestObservabilityOTelConfigIsGlobalOnly`. Remains: the
refusal through a real agent turn.

## 2026-11-11 run

Walked against the `_dx.md` Compose stack (report `docs/qa/reports/2026-11-11-otel-telemetry-export.md`).

- §1 pass: trace shape, p95 vector (after a second turn), required series, canary absent, catalog matches the docs.
  `gen_ai.request.model = other` reproduced → BUG-20261111-otel-model-exports-other; the model-option inspection spawn
  exports `gen_ai.system = other` → BUG-20261111-otel-model-probe-spawn-provider-other.
- §2 pass on all four surfaces; the daemon log masks the unresolved ref, which the contract now documents
  (BUG-20261111-otel-header-refs-redacted, resolved by an operator decision).
- §3 web pass after fixing BUG-20261111-web-adapter-requests-skip-traceparent (`47b40a7b4`): the prompt POST joins its
  trace (e434ff7fd); ratio 0 exports nothing. The EventSource stream cannot carry headers. Desktop leg blocked (no
  packaged build; disk full); desktop unit tests pass.
- §4 forwarding pass (env, `TRACEPARENT` = spawn span, agent value wins, nothing when off); Claude Code 2.1.321 exported
  no series to the Collector even when run directly, so step 3 is blocked.
- §5 pass. §6 refusal pass; `compozy__config_get` masks the header ref as now documented
  (BUG-20261111-otel-header-refs-redacted, resolved).
- State: `blocked-verify` on the two human verifications (desktop §3, provider series §4 step 3). `fix_status` stays
  `pending` for the open BUG-20261111-otel-model-probe-spawn-provider-other; the fixed traceparent and model bugs were
  re-walked and pass.

## Regression hot spots

Automated guards for the `_spec.md` Safety Invariants and ADRs. A QA walk does not re-prove these; a failure in a walk
that touches one of them goes back to the named suite.

| Invariant / decision | Guarding tests | Status |
| --- | --- | --- |
| SI-1 hosted run span context only under `HostedService.s.mu` | UT-031 in `internal/mcp` `TestHostedCallSpans` (50 sessions, race detector) | covered |
| SI-2 span context lives exactly as long as its run identity; ADR-001 proxy-only replacement keeps the parent | UT-035, UT-045, IT-016, IT-020 in `TestHostedCallSpans`; mutants "no bindLaunch copy" and "no ReleaseRun move" killed | covered |
| SI-3 parent kind and tool `RunID` agree (ADR-001 late call follows `RunID`) | UT-031–UT-034 in `TestHostedCallSpans`; IT-007 `TestDaemonE2EOTelNestsHostedToolCallsInTheirTurn` (real proxy) | covered |
| SI-4 `activePromptState.traceCtx` set before publish, read under `promptMu.RLock` | UT-036 `internal/acp` `TestActiveTraceContextFollowsTheActivePrompt` | covered |
| SI-5 `session.turn` single end owner | UT-037, IT-004, IT-005, IT-017, IT-018 (session part), IT-019 in `internal/session` `TestPromptTurnTrace`; `TestPromptTurnOutcome` | covered; IT-018's HTTP span error status is not asserted (otelgin maps the missing binary to 404) |
| SI-6 ADR-002 non-coalesced writes keep caller cancellation; attribution is a value only | IT-021 `internal/store/sessiondb` `TestSessionWriterAttribution`; UT-043 `internal/store` `TestWriteObservation` | covered |
| SI-7 coalesced batch updates each turn account once; `Add` after `Close` ignored | IT-015 `TestSessionWriterAttribution`; UT-044 `TestTurnWriteAccount`; UT-040–UT-042 `internal/telemetry` `TestRecordDBWrite` | covered |
| SI-8 export is asynchronous; nothing waits on an exporter | `BenchmarkTurnTelemetryFootprint` (+0.26 % elapsed), `BenchmarkSpanOverhead`; walk 2 degraded legs | partial: no test blocks the exporter during a turn |
| SI-9 `Init` failure degrades and never blocks boot | UT-015/UT-016 `internal/telemetry` `TestInitDegradesWhenExportCannotStart`, `TestInitInstallsNoopProvidersWhenExportCannotRun`; E2E-002 | covered |
| SI-10 shutdown after servers stop, bounded to 5 s | UT-019 `TestRuntimeShutdownIsBoundedByItsContext`; `internal/daemon` `TestBootTelemetryStartsAfterTheServersMountTheirRoutes`; task_03 mutants (order, 5 s bound) | covered |
| SI-11 only the sanitizing exporter is constructible | `internal/telemetry` `TestPackageBoundary`; depguard rule `otel-sdk-telemetry-only` | covered |
| ADR-003 observe parity (tokens/cost/events from the observe funnel, gauges never `Health()`, inactive when observability off) | UT-050–UT-055 `internal/observe` `TestObserverEventMetrics`; IT-012 `TestDaemonE2EOTelExportsTurnMetrics` (tokens equal `token_stats`, gauges equal status); UT-017 | covered |
| Sanitizer fallbacks (`compozy.unknown`, `other`, `custom`, `invalid`, omitted ids, no exemplars, empty descriptions) | UT-022 `TestSanitizingSpanExporterRebuildsSpanFields`; UT-046 `TestSanitizingSpanExporterAppliesPerKeyFallbackPolicy`, `TestSanitizingMetricExporterAppliesPerKeyFallbackPolicy`; UT-039 `TestSanitizingSpanExporterEnforcesNameAndScopeRegistries`; `TestSanitizingExportersCheckEveryExportedStringForSecrets`; `TestSanitizingSpanExporterFailsClosedWithZeroRegistries`; UT-024 `TestSanitizingMetricExporterExportsNoExemplars`; IT-014/E2E-003 `TestDaemonE2EOTelExportCarriesNoContent` | covered |
| Client propagation (unsampled `-00`, daemon-origin only) | UT-060–UT-064 (`api-client.test.ts`, desktop `runtime-health-monitor` tests); `terminal-api.test.ts` daemon-request traceparent; IT-013 `TestRequestTracingHonorsTraceParentOverHTTPAndUDS` | covered: every web daemon request goes through `daemonFetch` (e434ff7fd, 47b40a7b4); EventSource streams carry none |
| Model attribute | UT-046 (policy), `TestTelemetryModelClassesFollowTheCatalog`, `TestModelLabelClassifiesWhenMetricsRecord` | covered: catalog classifier wired (BUG-20261111-otel-model-exports-other, fixed); first turns on a fresh home may export `other` |
