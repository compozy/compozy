# Daemon profiling — 2026-11-03

Scope: issue #1224, experimental UDS and CLI profile capture. Persona: Dora investigating CPU
and heap growth through the daemon CLI. The profile bytes and inspect output remain in the
isolated lab; this report records the observed results without profile internals.

| Journey | Status | Evidence |
| --- | --- | --- |
| Capture CPU with active session and filter its labels | Pass | 30.03 s capture; 920 ms total CPU; session filter selected 140 ms (15.22%); tool, route and hosted projection tags present |
| Capture and inspect heap | Pass | CLI output artifact read by go tool pprof; 20,471.55 kB in-use samples |
| Reject concurrent CPU and invalid durations; exclude HTTP | Pass | Owning UDS/CLI suite, including pprof parser for all seven kinds |
| Stop isolated daemon | Pass | Targeted teardown reported clean=true; no lab survivors or provider-resource errors |

Impact: new UDS-only `GET /api/daemon/profile/{kind}` and `daemon profile <kind> --file`
CLI command, both experimental. OpenAPI and generated consumers document UDS-only transport.
HTTP/Gateway listeners, Web UI, native tool IDs, hooks, extension SDK, config keys and durable
state do not change. Existing CLI provides agent access; the official runtime skill documents
capture and inspection. Labels are capture-gated and use IDs, fixed operations and route templates.
No prompt, file body, arguments or query strings are added to labels. No daemon restart or
persistent profiling configuration is required.

The first attempt captured readable profiles while Claude model negotiation was still pending;
it had no session-tagged CPU samples and was not accepted as label evidence. The provider
reported `model_unavailable`. A real Codex `gpt-6.1-sol` session then answered and executed native
read-only tools. The repeated capture produced session and tool labels and a nonempty session
filter. No mock provider or synthetic profiler handler was used for this proof.

Final candidate run (`compozy-1224-final`): 30.05 s, 460 ms total CPU samples; the same
session filter selected 50 ms (10.87%), including `session_turn` and hosted projection.
Both profiles parsed successfully. The earlier warmed-session run additionally sampled
native tool dispatch. Files were mode 0600. The candidate daemon was restarted only inside
the owned lab; no production runtime was changed.

The first broader touched-package run passed UDS, HTTP, core, CLI, tools and MCP. It exposed
the new operation missing from the app-access oracle (corrected; focused oracle test passed),
plus existing ACP startup-deadline sensitivity and a session test-double map race at
`manager_busy_input_test.go:4623` versus `subagent_test_support_test.go:380`. Those session test
lines are unchanged from the base. Final focused hot-path checks and CI own the delivery verdict;
the earlier broader run is not recorded as green.

Targeted teardown finished with `TEARDOWN_ALL_CLEAN=true`. Graceful stop did not settle
within the helper's budget; its verified PID/open-file sweep stopped only lab-owned processes.
Provider resources were removed by the owning helper. Raw profiles and inspect output are
retained under the lab artifact directory. No production process was signaled.

The final focused owning-function selection passed in seven packages with race enabled:
UDS, spec, CLI, ACP, session, tools and MCP. Both focused lint groups completed with zero issues.
After that run, label placement was narrowed to resolved session/tool identities; the follow-up
checks run one function at a time under the operator's reduced-load instruction. Broad gates and
E2E remain CI-only. The initial session test-double race is not represented as a passing check.

The operator's host-load intervention prohibited further local gates. No heavy task owned by
this work remained running when that instruction arrived. Subsequent validation is limited to
single test functions, one process at a time; current-head CI owns the remaining broad checks.
The QA evidence auditor is deferred under that restriction; raw observation and clean teardown
receipts above remain available independently.
