# Harness dogfood corrections: focused verification

Date: 2026-10-23. Baseline: `74f33b7d460866b8061544e7eb98b73dbe972901`.
Branch: `fix/harness-dogfood-2026-10-22`. Findings and disposition:
[research synthesis](../../research/harness-dogfood/analysis/summary.md).

## Executed checks

| Owning boundary | Executed command / suite | Result |
| --- | --- | --- |
| Delegation policy and legacy recovery | `CGO_ENABLED=1 go test -race ./internal/session -run 'Test(DeriveLineageReadModel\|CreateAllowedToolsOverrideNarrowsAgentProfile\|AllowedToolsOverridePolicyHelpers\|CreateSpawnedAndCoordinatorSessionsValidateLineage\|CreateUserSessionRecordsProvenanceLineage)' -count=1` | Pass. New fork/continue and persisted empty-grant resume reproduced before the repair; inherited child narrowing remains covered. |
| Native history and monitor validation | `CGO_ENABLED=1 go test -race ./internal/daemon -run 'TestDaemonNativeTools/Should_(read_assistant_answers\|explain_invalid_monitor)' -count=1` | Pass. Real SQLite with two tool-heavy turns; both cursor directions retain complete Unicode answer chunks, no artifact or hydrated tool payload. Real monitor service rejects invalid descriptions without creating a monitor. |
| Shared schema/API/MCP projection | `CGO_ENABLED=1 go test -race ./internal/tools ./internal/api/core ./internal/mcp -run 'Test(RuntimeRegistryDispatchValidationAndPolicy\|ToolErrorResponses\|HostedToolResultContract\|HostedProxy)' -count=1` | Pass. Stable codes, actionable safe details, no input-secret/unknown-key exposure; provider not called for malformed input. |
| Log routing and hook identity | Existing HTTP request/client identity/redaction suites; `TestRequestLoggingLevels`; `TestHookDispatchLogging`, with race detector | Pass. INFO omits successful polling/hooks; DEBUG includes them; denied/failed requests remain visible. Hook records retain available correlation IDs without payload content. |
| Hosted diagnostic round trip | `CGO_ENABLED=1 go test -tags integration ./internal/daemon -run '^TestDaemonE2EHostedMCPProjectsAndCallsNonBootstrapNativeTool$' -count=1 -timeout=120s` | Pass. A real hosted stdio client receives the monitor description/200-character correction and recovery fields without the submitted sentinel; persisted hook runs/events remain intact. |
| Idle-release contract | Worker: `CGO_ENABLED=1 go test -race ./internal/session -run 'TestSupervisionReleasesIdleRuntime\|TestPromptActivity\|TestSupervision' -count=1`; `CGO_ENABLED=1 go test -race ./internal/acp -count=1` | Pass. The canonical budget/exit-ownership/session-load tests and real ACP subprocess suite pass. The [worker handoff](../../research/harness-dogfood/analysis/04_idle_release_fix.md) records the red/green boundary and platform checks. |
| Late reply after idle release | `CGO_ENABLED=1 go test -tags integration ./internal/daemon -run '^TestReplyWatchLongWaitIntegration$' -count=1 -timeout=120s` | Pass. A disposable daemon retains the waiting parent and delivers one late reply after target stop/resume, with the runtime release/rebind lifecycle enabled. |
| Real derived delegation journey | `CGO_ENABLED=1 go test -tags integration ./internal/daemon -run 'TestSubagentRootHostedMCPDaemonIntegration/Should_inherit_native_delegation' -count=1 -timeout=180s` | Pass. Root/fork/continue start real fixture ACP and hosted MCP subprocesses; a child calls status, delegates a grandchild and publishes catalog state. |

Tests extend existing invariant owners, without new standalone suites. The manager owns grant
materialization/persistence; native registry tests own filtered storage reads and validation;
shared transport suites own error projection; daemon integration owns real CLI/HTTP/MCP wiring.
The denied-call/allowlist suites remain authoritative for authorization; no grant bypass was added.

## Scenario coverage and limits

- `RT-subagent-delegate`: derived-parent permission branch and nested hosted native calls verified
  through the existing real daemon fixture journey. Child-answer retrieval verified on real SQLite.
- `ET-cli-session-continue`: CLI-created continued parent can delegate with its intended tool budget.
  The existing historical cross-provider replay evidence is separate.
- `ET-compozy-native-tool-invocation`: validation and transport checks above own the new safe diagnostics.
- `RT-session-message-reply`: truncated answer hints retain the actual session/turn IDs and use the
  existing answer filter. Delivery/acknowledgement semantics are unchanged.
- `RT-session-native-stop`: canonical idle-release budget, verified exit and rebind contracts pass; the daemon late-reply integration covers waiting/recovery. The observed live Codex six-second stop distribution was not re-benchmarked.
- No production mutations, fresh browser walk, or new live-provider behavioral score is claimed.
  The skill change was reviewed as an instruction contract; embedded-resource checks cannot prove
  model compliance or measure token savings.

Full scenario headers retain their historical verdicts; the bounded checks above do not silently
replace earlier persona walks. Required `make gate` and exact-head CI results are recorded in the
PR after all implementation changes, not inferred from these focused checks.
