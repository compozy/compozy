# Always-on agents: Track B4 remediation

Date: 2026-11-01. Branch: `feat/stays-on-fix-b4`.
Scope: the B4 findings in the approved always-on-agent remediation brief.
This is focused backend/provider evidence, not a full release or Web walkthrough.

## Findings and commits

| Finding | Commit | Result |
| --- | --- | --- |
| D-22 | `39aaa5bec` | Stopped-session worktree binding tolerates absent live permission state. |
| N-47 | `1d3240676` | Replacement validates the resulting entry length without changing rejected notes. |
| N-52 | `d2df8eb56` | Memory retirement failure preserves the definition for retry. |
| D-23 | `5c5a648ee` | Missing workspace roots preserve instance identity and read projection. |
| N-54 | `b669fa463` | Derived sessions inherit the instance ceiling before first launch. |
| D-26 | `b56a108e5` | Authenticated Global callers resolve Global memory without a project lookup. |
| ASTRA-006, N-45, N-46, N-53 | `6471c1bd7` | Serialized lifecycle operations, retryable stop failure, and replacement of stopped main attempts. |
| D-11 | `af4d88f00` | Active instances block profile deletion; archived instances and profile memory are inventoried and removed recoverably. API, SDK, CLI and Web previews agree. |
| N-48 | `e97f98ade` | Instance lineage gates agent memory; entries use workspace names; document/history digests use `sha256:`. |
| D-5 | `43f0a96ca` | Verified provider-scoped planning modes and confirmed config-only mode negotiation. |

## D-5 live provider evidence

The probe used the production `acp.Driver`, real provider executables, an isolated
bootstrap runtime home, and separate provider working directories. Native-CLI
operator login state was preserved as required by the bootstrap home policy.
No credentials were copied, changed, or logged. No main daemon was started or changed.

Manifest: `/Users/pedronauck/dev/qa-labs/compozy-b4-provider-triage-20261101-162557-256434-lab/qa-artifacts/qa/bootstrap-manifest.json`.
Runtime home: `/var/folders/7x/xg204hnd04b81fczcxvjlhzr0000gn/T/compozyqa-d008086f9801/runtime`.
Evidence beside the manifest: `provider-observations.json`, `provider-attempt.json`,
`journey-log.jsonl`, `focused-verification.txt`, `qa-audit-report.json`.
Probe source retained at the worktree's ignored `.cache/b4-provider-probe/main.go`.

| Provider | Observed mode IDs | Result |
| --- | --- | --- |
| Claude | `default`, `acceptEdits`, `plan`, `auto`, `bypassPermissions` | Confirmed `plan`; read `signal.txt`; returned `NOTHING_TO_REPORT` on a digest prompt. A separate write request returned `WRITE_BLOCKED`; `denied.txt` remained absent. |
| Codex | `read-only`, `workspace-write`, `agent`, `agent-full-access` | Confirmed `read-only`; returned `NOTHING_TO_REPORT`. File-read request without commands was declined because its available reader needed a command; no relaxation was added. |
| OpenCode | Config-only `build`, `plan`; no legacy modes list | Confirmed `plan` through `session/set_config_option`; returned `NOTHING_TO_REPORT`. Other launches intermittently returned `ServiceUnavailableError`, including the separate write-refusal probe, which is not claimed as passed. |
| Kimi CLI | Config-only `default`, `plan`, `auto`, `yolo` | Candidate `plan` selection succeeded but the prompt required authentication. Mapping was removed; provider remains unverified and fail-closed. |
| Cursor | Unavailable | `session/new` returned `Authentication required`. No mapping added. |
| Pi | `low`, `high`, `max` | These are reasoning modes, not read-only execution modes. Triage returned `triage_unsupported`; no mapping added. |
| Gemini, Copilot, Qwen Code, Antigravity | Unavailable | `gemini`, `copilot`, `qwen`, and `agy_acp_server.par` were absent from PATH. No installation or login was attempted. |

Successful digest sessions: Claude `345c1f42-f50b-4d99-a7d2-76bcd297e2d1`,
Codex `01a197f4-265e-79b0-a208-a9f505a4bd91`,
OpenCode `ses_e680b9557ffeMcbfcJdOs2GWbV`.
Claude write-refusal session: `a6dd8df9-1364-478e-9663-deba96fabba8`.
The allowlist contains only Claude and OpenCode's `plan` aliases. Explicit
read-only IDs remain supported. Unknown planning modes remain rejected.

The strict targeted QA evidence audit passed. The manifest's sibling
`teardown.json` records `clean: true`; teardown reported `TEARDOWN_ALL_CLEAN=true`.

## Verification

### Current-head verification continuation

The continuation validates production HEAD `43f0a96ca`. On 2026-11-01,
`make codegen-check` passed (exit 0, 78.276 s) and `go build ./...` passed
(exit 0, 26.630 s). Local command receipts and full output are retained under
`.cache/b4-verification/` (`results.json`, `1.log`, and `2.log`).

The broad changed-package race-test run finished with exit 1. Its initial invocation
incorrectly included `./sdk/go/client` from the root module; that package is a
separate module, so its setup failure is not a production regression. A separate
`go test -race -count=1 -timeout=10m ./client` from `sdk/go` passed (exit 0,
47.783 s wall time; package test time 37.597 s), with output in `sdk.log` and its
terminal receipt in `sdk-result.json`.

Full package race tests passed for `internal/acp`, `internal/agentinstance`,
`internal/agentmemory`, `internal/api/contract`, `internal/api/core`,
`internal/cli`, and `internal/profile`. The broad daemon suite failed in boot,
watcher, and harness cases. Session tests reported canonical temporary-path
differences and a stop deadline, then timed out at ten minutes; globaldb also
timed out in migration/reopen tests. Full output is retained in `3.log`; these
three full suites are not claimed as passed. Separate focused B4 integration
runs and diagnosis of the named failures completed, using canonical
`TMPDIR=/private/tmp`, `-p=1`, and `-parallel=2` without changing test assertions.

The focused integration run passed `internal/session` (27.273 s). It also
exposed an outdated fixture in `TestInstanceApprovalCrashRestartDaemonIntegration`:
the adopted instance had an empty permission ceiling, so N-54 correctly refused
the child's workspace at launch, and the fixture dereferenced the absent child ID.
The approval and crash-recovery fixtures now explicitly authorize only the parent
workspace path, matching the existing instance payload fixture. Recovery and
approval assertions are unchanged; a missing child now reports the delegation
result instead of panicking. Production permission enforcement is unchanged.
The invariant belongs to daemon integration: preserve child approval and recover
approval reports exactly once after a crash. The existing suites remain the owner.
The parent fixture must also declare the workspace path: effective authority is
the intersection of the instance ceiling and the parent's policy. Both fixtures
now use the same explicit parent-path setup as the suite's existing instance
creator case. The test-shape checker passed. The final SQLite/process regression
run passed (exit 0, daemon 71.652 s; `fixture-final.log`):

```text
go test -race -parallel=2 -tags=integration ./internal/daemon -run '^(TestInstanceApprovalCrashRestartDaemonIntegration|TestSubagentPermissionDaemonIntegration|TestInstancePayloadIntegration)$' -count=1 -timeout=5m
```

All changed Go packages therefore have passing full-package or focused owning-suite
evidence, including the separate Go SDK module. The only changes after production
HEAD `43f0a96ca` are the verified integration fixture setup and this report;
the passing codegen-check/build evidence remains applicable to production sources.

The controlled diagnosis of all named broad-suite failures passed (exit 0,
82.028 s): daemon 29.638 s and session 31.186 s. Focused instance coverage in
globaldb also passed (78.636 s). These results do not turn the earlier timed-out
full-suite run into a pass.

### Earlier slice verification

All Go regression runs used `CGO_ENABLED=1`, the writable temporary Go cache,
and an environment without inherited `COMPOZY_*` variables.

- `go test -race -tags=integration ./internal/session -run 'TestManagerIntegrationWorktreeBindingLifecycle/Should_bind_stopped' -count=1` — passed.
- `go test -race ./internal/agentmemory -count=1` — passed; later N-48 `TestStore` run also passed.
- `go test -race ./internal/api/core -run 'TestBaseHandlersAgentDefinitionMutations/Should_delete_only' -count=1` — passed.
- `go test -race -tags=integration ./internal/daemon -run '^TestInstancePayloadIntegration$' -count=1` — passed, including missing-root and stopped-predecessor cases.
- `go test -race ./internal/session -run '^TestInstancePermissionMode$' -count=1` — passed.
- `go test -race ./internal/store/globaldb -run '^TestInstance(Service|Lifecycle|EmptyStart|Definition)' -count=1` — passed (13.841 s), superseding the earlier duplicate-variable compilation failure.
- `go test -race ./internal/profile -run TestManagerProfileLifecycle -count=1` — passed.
- `go test -race ./internal/cli -run '^TestProfileCommandOutputContract$' -count=1` — passed.
- `go test -race ./internal/agentmemory ./internal/api/core ./internal/daemon -run '^(TestStore|TestAgentMemoryRoutes|TestAgentMemoryNativeTools)$' -count=1` — passed.
- `go test -race ./internal/agentinstance -count=1` and the real SQLite `TestInstanceCallerLineage` suite — passed.
- `go test -race ./internal/acp -run 'TestTriage|TestStart.*Mode|TestStart.*Config' -count=1` — passed (5.469 s).
- Root Turbo, filtered to `compozy-web`: profile lifecycle dialog test file (5 tests) and typecheck — passed.
- `make codegen`, `make codegen-check`, and `go build ./...` — passed; the latter two rerun after D-5/N-48 production changes.

Pre-commit checks run through `lint-staged --no-stash` to avoid prohibited automatic
Git stashes. The test-shape heuristic reports pre-existing top-level test cases in
the ACP suites; new cases reuse `TestTriageStartup` subtests. Fast package lint also
reports existing unrelated complexity/line-length findings and the SDK spelling
`Cancelled`; this report does not claim a clean global lint gate.

Impact owner: the approved always-on-agent `_impact-entry.md`. No schema migration,
new native tool ID, hook, or config key was introduced. Updated official agent/profile
references and QA scenarios describe the changed boundaries. Global gates and E2E
were intentionally not run, as explicitly required by the remediation brief.
