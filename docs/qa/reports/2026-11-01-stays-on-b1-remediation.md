# Stays-on Track B1 remediation evidence

Branch: `feat/stays-on-fix-b1`. Implementation head: `886eb98eb`.
Scope: the approved always-on-agent `impl-fix.md` Track B1, including daemon W-4/N-6.
All findings below were confirmed and fixed; none were rejected. Track B1 is complete: every finding has a committed fix and owning regression coverage; all required package checks have successful results.

## Finding ownership and regression coverage

Test names below identify existing canonical suites, with cases added or corrected for the stated invariant. The commit records the precise case and production change.

| Finding | Commit(s) | Invariant and owning suite |
| --- | --- | --- |
| ASTRA-001; daemon W-4/N-6 | `9850caf8c`, `07a8c109e` | Check fields, cap/count and fatigue limit survive shared HTTP projection and SSE schemas: core `TestAgentInstanceCheckProjection`; spec `TestDocumentPreservesSchemaSemantics`. |
| ASTRA-002; D-8/D-10 | `ad70fa09d` | Agent HTTP/UDS create/archive require approval admission; authenticated approve-all still enforces ownership: core `TestAgentInstanceApprovalAdmission`, daemon `TestNativeInstanceAuthority`. |
| ASTRA-003; D-7 | `dffed84c3` | Only eligible operator creation grants unrestricted access: daemon `TestInstanceCreateAdmission`. |
| ASTRA-004 | `a56fa939c` | An intervening manual/fatigue pause cannot be bypassed by agent resume; authorization executes in the SQLite transaction: globaldb `TestInstanceResumeAuthority`. |
| ASTRA-010; D-9 | `5823e5bf3` | Input-sensitive approve-reads exemptions retain deny rules, post-hook approval and native ownership: tools `TestInstanceCallApproval`, daemon `TestNativeInstanceAuthority`. |
| D-1 | `2ce737f0d` | Draft instructions remain app-token denied and classified mutating: spec `TestAppAccessPolicy`, builtin catalog suite. |
| D-3 | `b5d214e80` | First create returns created=true, idempotent repeat false: core `TestAgentInstanceCreateResult`, complete spec schema suite. |
| D-4 | `720103b3b`, `79065ed77`, `886eb98eb` | Fresh root policy contains enabled skills, MCP servers and directories; explicit resource grants survive; fork/continue retain native tools; directory skill identity survives ceiling filtering: session `TestCreateManualSessionProducesRootLineage`, `TestDeriveLineageReadModel`, `TestManagerSpawnResolvesPermissionSelection`; extension session-create integration suites. |
| D-6 | `cb08c8cbd` | Resolved AdditionalDirs belong to the definition ceiling: daemon `TestInstanceDefinitionPaths` with real workspace resolution. |
| D-24 | `f8a0f250d` | Triage is hidden from public lists and denied by public by-ID operations: session `TestManagerListForWorkspace`, core `TestBaseHandlersHiddenTriage`, daemon `TestNativeHiddenTriage`, extension `TestHostAPIHiddenTriage`. |
| N-20 | `720f11191`, `1bdbc7298` | Archived workspace yields 409 with stable code and approved message: core `TestAgentInstanceErrorResponses`. |
| N-21 | `4ccf63a9e` | Paused instances reject real checks before capacity admission; dry-run remains allowed: globaldb `TestInstanceCheckGates`. |
| N-23 | `5b753cc11` | Canonical draft CLI and session_pending_interactions native names: CLI `TestAgentDraftCommand`, daemon `TestNativeSessionPending`, generated builtin catalog checks. |
| N-44 | `6fcd40820` | Catalog SSE excludes foreign-instance events before serialization: core `TestBaseHandlersStreamSessionCatalog` with SQLite authority. |
| N-49 | `1bdbc7298` | Stable error codes carry approved messages, hooks emit reason, CLI outputs label/scope and report proposal count; archive accepts HTTP 204: core `TestAgentInstanceErrorResponses`, hooks `TestInstanceHookDispatch`, CLI `TestAgentInstanceCommands`, globaldb lifecycle hook suite. |
| N-51 | `770e3982f` | Filters apply before LIMIT; attention aggregation covers instance roots/descendants without ListAll enrichment: globaldb `TestInstanceWorkspaceIsolation`, `TestInstanceCallerLineage`, core `TestAgentInstanceCheckProjection`. |

## Verification receipts

- `make codegen` and `make codegen-check`: passed after installing the existing locked dependencies with `bun install --frozen-lockfile`. Final contract check terminal `term-f007e1e36485` exited 0. No generated artifacts were hand edited.
- `go build ./...`: passed, including the build preceding the successful final session/extension/daemon run after the latest D-4 correction.
- Full race package suites passed: agentinstance 7.070s, heartbeat 28.100s, api/core 69.375s, api/spec 126.583s, tools 2.318s, tools/builtin 4.729s, cli 74.935s, hooks 9.082s (`term-9a2b323e62e5`). That combined run failed in session/extension and on a daemon boot timeout; it is not an overall pass.
- Full globaldb package passed in 278.232s without race (`term-25fc8aa4b525`, exit 0), after full race runs timed out. Focused changed SQLite invariants also passed with race.
- D-4 follow-up focused race command covering root policy, derivation, spawn inheritance, extension session create and boot reconciliation passed: session 15.152s, extension 8.567s, daemon 15.780s (`term-9e5220285470`, exit 0). Existing failing assertions were preserved; production code was repaired. Boot timeout did not reproduce in the isolated retry.
- Passed: `go build ./...` followed by `go test -p=1 -parallel=4 -timeout=20m ./internal/session ./internal/extension ./internal/daemon`, terminal `term-6d6fbb7a14d2`, monitor `mon-2def890e083b`. Exited 0: session 221.452s, extension 20.933s, daemon 53.719s. Uses sanitized COMPOZY environment and canonical TMPDIR. No required validation remains pending.

No global gate, E2E, repo-wide tests, push, or live provider/browser walkthrough was run. B4 owns real-provider triage runs. Web consumers of the delivered contract fields remain Track F scope. QA scenario and official runtime reference changes accompany the remediation commits. The orchestrator owns the upstream cross-surface impact entry.
