---
name: 07-tools-sandbox
description: QA child report — tool dispatch, tool runtime registry, sandbox profile resolution, MCP server lifecycle, path security, secret redaction, hook deny/narrow, external-call timeout discipline.
type: qa-child
module: tools-sandbox
sources:
  - internal/tools
  - internal/tools/builtin
  - internal/toolruntime
  - internal/sandbox
  - internal/sandbox/local
  - internal/sandbox/daytona
  - internal/sandbox/providertest
  - internal/mcp
  - internal/mcp/auth
  - internal/fileutil
  - internal/acp/handlers.go
  - internal/acp/client.go
  - internal/acp/process_tree_unix.go
  - internal/acp/process_tree_windows.go
---

# Tools / Tool Runtime / Sandbox / MCP — Final QA Plan

## 1. Module Surface

This child owns the runtime contract that turns a model's `tool_call` into an observable, redacted, hook-narrowed, sandbox-bounded process — and the matching MCP path that exposes daemon-owned tools to ACP-speaking agents. Every claim cites `file:line`.

### 1.1 Packages and entry points

| Package                                     | Responsibility                                                                                                                                                                                                                          |
| ------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `internal/tools`                            | Descriptor / Provider / Registry / dispatch pipeline; policy evaluator; approval-token store; result limiter (byte cap + redaction); tool-event emission contract. Composition root: `internal/tools/registry.go:32-48`.                |
| `internal/tools/builtin`                    | Native-Go (`agh__*`) descriptors and handlers — catalog/skills/network/sessions/workspace/memory/observe/bridges/tasks/autonomy/config/hooks/automation/extensions/mcp_auth.                                                            |
| `internal/toolruntime`                      | Process registry + interrupts. `Registry.Register/Checkpoint/Complete/Interrupt/ReconcileBoot` (`internal/toolruntime/registry.go:128-358`). Default interrupter sends SIGTERM → poll → SIGKILL with PID/start-time validation.        |
| `internal/sandbox`                          | Provider-neutral types: Backend (`local`/`daytona`/`e2b`), SyncMode, PersistenceMode, Resolved, NetworkPolicy, ToolHost interface (`types.go:230-304`). Provider Registry (`registry.go:23-82`).                                       |
| `internal/sandbox/local`                    | Daemon-host provider; wires `acp.NewLocalToolHost` and `acp.NewLocalLauncher` per session (`provider.go:85-132`).                                                                                                                       |
| `internal/sandbox/daytona`                  | Remote sandbox provider — Daytona SDK launcher, sidecar transport, SSH token manager, archive sync (`tar.go:145-313`), provider-side EvalSymlinks containment.                                                                          |
| `internal/sandbox/providertest`             | Generic compliance suite for `sandbox.Provider`; called from each provider's tests.                                                                                                                                                     |
| `internal/mcp`                              | Hosted MCP (`agh tool mcp` stdio proxy) lifecycle service: launch nonce, peer credential bind, projection digest, call/release. Remote-MCP `CallExecutor` for stdio + http/sse servers with timeouts (`executor.go:115-150`).            |
| `internal/mcp/auth`                         | OAuth/PKCE service for remote MCP servers; HTTP client with explicit timeout (`service.go:31-53`); token store contract.                                                                                                                |
| `internal/fileutil`                         | Atomic write/replace/remove primitives — `AtomicWriteFile` (`atomic.go:14-46`), `AtomicRemoveFile` (`atomic.go:49-67`). Used by every config/state writer in this module.                                                                |

> NOTE — Path-security helpers `sanitizePathKey` / `realpathDeepestExisting` referenced in the parent prompt do **not** live in `internal/fileutil`. The two production places that implement the same invariant are `internal/skills/path_security.go:9-36` (`ensurePathWithinRoot`) and `internal/sandbox/daytona/tar.go:177-313` (`archiveTargetPath` / `safeArchiveName` / `safeSymlinkTarget` / `ensureSafeParent`). Both use `filepath.EvalSymlinks` + path-prefix check, exactly per `internal/CLAUDE.md` Security Invariants.

### 1.2 Dispatch pipeline (canonical order)

`RuntimeRegistry.dispatch` (`internal/tools/dispatch.go:20-94`) walks every call through this exact ordering. A scenario that doesn't observe these gates in this order is asserting against a regressed runtime.

1. `contextErr` (line 22) — refuse if ctx is nil/canceled/timed-out.
2. `normalizeCallRequest` (line 25) — fold `Scope.SessionID` / `WorkspaceID` / `AgentName` defaults.
3. `ToolID.Validate` (line 26).
4. `resolveDispatchTarget` (line 29) — index lookup, evaluator decision, handle resolution, conflict detection (`registry.go:438-453`).
5. Normalize input (line 36).
6. `ensureDispatchTargetCallable` (line 37) — denies before any side effects when `Decision.Callable=false` or handle is nil.
7. Emit `ToolCallStarted` with redacted input (line 40).
8. `validateCallInput` against descriptor schema (line 46).
9. `runPreCallHook` (line 49) — hook may return a patched `CallRequest` and a `Decision{Callable, ReasonCodes}`; tool ID change is forbidden (`dispatch.go:258-266`).
10. ctx re-check (line 53).
11. `requestApproval` (line 56) — if `Decision.ApprovalRequired`, calls the configured `ApprovalBridge`; missing bridge → `ReasonApprovalUnreachable`.
12. `target.handle.Call` (line 59) — provider-side execution.
13. `resultLimiter().Apply` (line 67) — descriptor byte cap + redaction (`internal/tools/result_limit.go:35-104`).
14. `runPostCallHook` (line 75) — may rewrite the result; second `Apply` runs.
15. Emit `ToolCallCompleted` (line 79); if truncated, also emit `ToolResultTruncated` (line 86).
16. Failure paths: `failDispatch` emits `ToolCallFailed` or `ToolCallDenied` with `ToolEventData.Err` (line 408-424).

### 1.3 Tool-runtime registry contract

`internal/toolruntime/registry.go` — durable + in-memory tracking of tool/agent/terminal subprocesses.

| Surface                                 | Behavior                                                                                                                                                                              |
| --------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `Register(ctx, RegisterConfig)`         | Lines 165-214. Generates `proc_<hex>` id, derives `StartedAt` from `procutil.StartedAt(pid)`, validates record, upserts to store, holds in-memory `activeProcess{record, interrupt}`. |
| `Handle.Checkpoint(ctx, ProcessCheckpoint)` | Lines 224-229 → `checkpoint` (lines 360-402). Mutates owner / pid / pgid / state under lock and re-validates before upsert.                                                          |
| `Handle.Complete(ctx, ProcessCompletion)` | Lines 232-242. `sync.Once`-gated terminal write, `Failed` if `Err != nil`, else `Completed`.                                                                                          |
| `ReconcileBoot(ctx)`                    | Lines 244-286. Lists active records, calls `validateRecovered` (PID + start-time match via `procutil.MatchesStartTime`), marks recovered or stale, returns `BootReconcileReport`.    |
| `Interrupt(ctx, InterruptScope)`        | Lines 288-358. Pulls candidates by scope (live + durable), transitions to `interrupting`, fires owner-supplied `InterruptFunc` for live procs, falls back to `defaultInterrupter` for recovered procs. |
| `defaultInterrupter`                    | `internal/toolruntime/interrupt.go:13-78`. **SIGTERM → 250ms poll → SIGKILL → 1s poll**, signaling process group when `ProcessGroupID > 0` (`signalRecord`, lines 52-57). Validates `MatchesStartTime` before AND after each signal. |

**ACP wiring** — `internal/acp/client.go:296-331` registers the agent process with `Source=ProcessSourceACPAgent`, `ProcessGroupID = process.PID` (the agent process is a session leader); `internal/acp/handlers.go:735-756` registers each terminal subprocess with `Source=ProcessSourceACPTerminal` carrying `Owner.{SessionID,TurnID,TerminalID}`. Both register an `Interrupt` callback that owns cooperative shutdown.

### 1.4 Sandbox profile resolution

| Type                                | File:line                                       | Behavior                                                                                          |
| ----------------------------------- | ----------------------------------------------- | ------------------------------------------------------------------------------------------------- |
| `Resolved`                          | `internal/sandbox/types.go:118-130`             | Profile, Backend, SyncMode, Persistence, RuntimeRootDir, DestroyOnStop, Env/SecretEnv, NetworkPolicy, optional Daytona block. |
| `Provider.Prepare`                  | local: `provider.go:85-132`; daytona: `provider.go`. | Returns `Prepared{State, RuntimeRootDir, RuntimeAdditionalDirs, Launcher, LaunchSpec, ToolHost}`. |
| `ToolHost`                          | `types.go:292-304`                              | ACP-facing surface — read/write text file, resolve path, authorize permission op, create/kill/wait/release terminal. |
| `Registry.Provider(backend)`        | `registry.go:58-67`                             | Returns provider; `ErrProviderNotRegistered` when missing.                                        |
| `PermissionOperation` constants     | `types.go:262-274`                              | `fs/read_text_file`, `fs/write_text_file`, `terminal/create`, `session/request_permission`.       |
| `PermissionDecision`                | `types.go:276-290`                              | `pending` / `allow-once` / `allow-always` / `reject-once` / `reject-always`.                       |

### 1.5 MCP lifecycle surface

Two distinct subsystems — keep them straight.

- **Hosted MCP** — `internal/mcp/hosted.go:23-180` plus `hosted_proxy.go`. The daemon mints a single-use bind nonce, ACP launches `agh tool mcp` as an MCP stdio sidecar pointed at the daemon's UDS, the proxy binds with `(SessionID, Nonce)`, peer credentials and binary path are verified (`hosted.go:38-39, 164-167`). Server name is `agh-hosted-tools`. Failures are fail-closed.
- **Remote MCP** — `internal/mcp/executor.go:115-150` and the auth service in `internal/mcp/auth/`. Daemon connects to user-configured stdio / http / sse MCP servers via `mcp-go`; default call timeout is `30s` (`executor.go:25`); both the executor (`executor.go:129-150`) and the metadata client (`auth/service.go:21-53`) construct `*http.Client{Timeout: ...}` — `http.DefaultClient` is forbidden in this code path.

### 1.6 Secret redaction surface

| Surface                                 | File:line                                                | Behavior                                                                          |
| --------------------------------------- | -------------------------------------------------------- | --------------------------------------------------------------------------------- |
| Result-limit redaction                  | `internal/tools/result_limit.go:17-104`                  | `redactedJSONValue = "[REDACTED]"`; default sensitive fields + descriptor-marked. |
| Event input redaction                   | `internal/tools/dispatch.go:606-624`                     | Before emit, `redactInputForEvents` rewrites the input copy and surfaces redacted paths in `RedactedInputFields`. |
| Result digest                           | `internal/tools/dispatch.go:626-633`                     | `sha256(json(result))` recorded in event; raw bytes never persist on the event.   |
| Diagnostics dynamic-secret registration | `internal/diagnostics/redact.go:35-94` (referenced)      | Daemon-wide registry of runtime-issued secrets (claim_token, MCP tokens, etc).   |

### 1.7 Path-security surface

| Helper                                  | File:line                                                | What it defends                                                                          |
| --------------------------------------- | -------------------------------------------------------- | ---------------------------------------------------------------------------------------- |
| `skills.ensurePathWithinRoot`           | `internal/skills/path_security.go:9-36`                  | `Abs → EvalSymlinks → Rel → reject "../" prefix`. Used by skill file/sidecar containment. |
| `extension.evaluateBundleRoot`          | `internal/extension/bundle.go:741-749`                   | Same pattern for extension bundles.                                                      |
| `extension.install_managed.<resolve>`   | `internal/extension/install_managed.go:355,483,572`      | Same pattern for managed-extension dependency copies.                                    |
| `daytona/tar.archiveTargetPath`         | `internal/sandbox/daytona/tar.go:202-212`                | Tar-extract: `safeArchiveName` (no abs, no `..` segments), `isWithinRoot` check after join. |
| `daytona/tar.safeSymlinkTarget`         | `internal/sandbox/daytona/tar.go:280-295`                | Tar symlink entry: rejects abs links escaping root and relative links resolving outside root. |
| `daytona/tar.ensureSafeParent`          | `internal/sandbox/daytona/tar.go:297-313`                | Reject overwriting symlinked files; re-evaluate parent after `MkdirAll`.                  |

These five helpers — not a single `fileutil` symbol — are the path-security perimeter for tool/sandbox/MCP-driven file operations. Scenarios MUST cite the helper they exercise.

## 2. Existing Test Coverage Map

### 2.1 `internal/tools/`

- `dispatch_test.go` — `TestPolicyDenyAll` (`:228`); redaction (`:624` "Should redact sensitive result fields before post-call hooks and events"; `:716` "started event redacted input fields"); result-budget redaction (`:751`); approval bridge happy/sad paths; pre/post hook patch/deny; tool-id-rewrite-rejection.
- `policy_test.go` — `policyResultDenied` for registry, source, agent, session policy paths (lines 53, 76, 150).
- `policy_resolver_test.go` — scope-aware resolver overlay merging.
- `registry_test.go` — `Should mark duplicate canonical IDs as conflicted and hide from session projection` (`:81`); `Should mark sanitized name collisions as conflicted` (`:164-207`); search/operator vs session projection.
- `approval_token_test.go` — single-use TTL, expiration, random-source injection.
- `boundary_test.go` — registry boundary invariants.
- `mcp_test.go`, `native_test.go` — mcp/native source registration.
- `pattern.go` / `policy.go` glob-tests in `policy_test.go`.
- `resource_test.go`, `result_limit_test.go` — result envelope semantics, byte cap.
- `schema_digest_test.go` — JSON schema canonical hashing.
- `toolset_test.go` — toolset expansion + diagnostics.
- `tool_test.go` — descriptor validation.
- `perf_bench_test.go` — micro-bench (not load-bearing for QA).

### 2.2 `internal/toolruntime/`

- `registry_test.go` — Register / Checkpoint / Complete sync.Once gate; ReconcileBoot recovered/stale split; Interrupt scope filters; in-memory + durable candidate union.
- `memory_store.go` — pure in-memory store implementation (test-only).
- **Coverage gap**: there is no test that asserts `defaultInterrupter` actually walks SIGTERM → SIGKILL with the 250ms grace and 1s kill grace against a **real** child process group. The interrupt-policy logic is unit-tested via fake interrupter; the real-syscall path is exercised only indirectly through `internal/acp/launcher_tool_host_test.go:302,363`.

### 2.3 `internal/sandbox/`

- `registry_test.go` — provider register/lookup, default-backend fallback, nil-provider rejection.
- `types_test.go` — `Backend.Valid`, `SyncMode.Valid`, `PersistenceMode.Valid`.
- `local/provider_test.go` — Prepare returns ToolHost / Launcher / SessionState; permission-mode override per request.
- `daytona/provider_test.go`, `provider_integration_test.go`, `launcher_transport_integration_test.go`, `sdk_test.go`, `ssh_test.go`, `ssh_token_test.go`, `ssh_validation_test.go`, `tar_test.go`, `perf_bench_test.go` — Daytona SDK integration, transport, SSH key issuance, tar safety, perf.
- `providertest/suite.go` + `suite_test.go` — generic compliance suite called from local + daytona.
- **Coverage gap**: tar safety tests cover header path/symlink rejection but do NOT exercise null-byte (`foo\x00.txt`), URL-encoded (`%2e%2e/`), or NFC/NFD homoglyph inputs — these would have to be injected at the tar layer or, for a hosted daemon-host write, at the ACP `fs/write_text_file` boundary.
- **Coverage gap**: `daytona/ssh.go:88` falls back to `http.DefaultClient` when `s.httpClient == nil`. This is reachable only via test injection (production wiring always sets `httpClient`), but a regression test should pin "production NewProvider never leaves httpClient nil".

### 2.4 `internal/mcp/`

- `hosted_test.go` — `TestHostedServiceBindNonceLifecycle` (`:18`), `TestHostedServiceValidatesPeerAndBinaryFailClosed` (`:73`), `TestHostedServiceProjectionAndCallUseRegistryScope` (`:151`), `TestHostedServiceProjectionMatchesRegistrySessionProjection` (`:225`), `TestHostedServiceCallUsesRegistryApprovalBridge` (`:290`), `TestHostedServiceReleaseAndFailureBranches` (`:351`).
- `hosted_proxy_test.go` — `TestHostedProxyHelpers` (`:148`).
- `executor_test.go` — remote MCP stdio + sse + http call dispatch, error mapping; auth refresh hand-off.
- `peer_test.go` + per-platform peer credential variants (`peer_darwin_cgo.go`, `peer_darwin_nocgo.go`, `peer_linux.go`, `peer_unsupported.go`).
- `auth/service_test.go`, `auth/metadata_test.go`, `auth/pkce_test.go` — OAuth/PKCE flow.
- **Coverage gap**: there is no end-to-end test that boots the daemon, spawns a real session against a real Claude Code subprocess, runs a hosted-MCP tool call, and asserts the round-trip ledger contains the call. All hosted-MCP tests are unit-level; the production path through `internal/acp` is exercised only through `acpmock`.
- **Coverage gap**: external HTTP timeout enforcement is structurally enforced (`*http.Client{Timeout: ...}` everywhere), but there is no chaos test that inserts a 60s server delay against `executor.go` and asserts a deterministic timeout error rather than a hang.

### 2.5 `internal/fileutil/`

- `atomic_test.go`, `atomic_remove_test.go`, `atomic_bench_test.go` — happy-path atomic write/replace/remove + dir-sync.
- **Coverage gap**: no test asserts that paths containing a null byte (`foo\x00.txt`) fail cleanly at the `os.CreateTemp(dir, base+".tmp-*")` step rather than producing a truncated filename. Add a regression test.

## 3. Coverage Gaps

For each gap, the load-bearing AGH claim and the missing test.

| Gap                                                                       | Claim AGH makes                                                                                                                                                                                       | Missing test                                                                                                                                                                                                              |
| ------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Real-LLM tool dispatch end-to-end**                                     | "Tool dispatch goes through hooks for permission decisions (deny/narrow/annotate)." (`internal/CLAUDE.md` Security Invariants + `internal/tools/dispatch.go:49-93`).                                  | Spawn real Claude Code, drive `fs/read_text_file`, `fs/write_text_file`, and `terminal/create` (the canonical "tool.read_file/write_file/run_command" surface in AGH), assert each event is recorded with input digest. |
| **Real interrupt mid-execution**                                          | "interrupted, not success" — `defaultInterrupter` SIGTERM → SIGKILL with PID validation (`internal/toolruntime/interrupt.go:13-78`).                                                                  | Run `terminal/create sleep 60`, fire `Driver.Interrupt(scope{TerminalID})`, assert `ProcessRecord.State == Interrupted`, child PID gone, exit code in observed terminal output is non-zero.                              |
| **Sandbox blocks `/etc/shadow` and `../` escape**                         | `acp.NewLocalToolHost` enforces `Resolved.RuntimeRootDir` containment; `daytona/tar.archiveTargetPath:202-212` rejects abs + traversal headers.                                                       | Drive `fs/read_text_file path=/etc/shadow` and `fs/read_text_file path=../../etc/passwd`; assert ACP error, ledger row written, no agent crash.                                                                          |
| **Null-byte path rejection**                                              | "Path security helpers handle null-byte, URL-encoded traversal, Unicode normalization, symlink-escape." (root prompt).                                                                                | `fileutil.AtomicWriteFile("foo\x00.txt", ...)` returns a deterministic error; ACP `fs/write_text_file` with embedded `\x00` fails before any disk side effect.                                                            |
| **URL-encoded `%2e%2e/` traversal**                                       | Same.                                                                                                                                                                                                  | `fs/write_text_file path="docs/%2e%2e/etc/passwd"` is rejected at the tool-host boundary (no decode-before-check regression).                                                                                             |
| **Unicode NFD/NFC homoglyph rejection**                                   | Same.                                                                                                                                                                                                  | `fs/write_text_file` with a path that NFD-normalizes outside the workspace root (e.g. composed Latin Small Letter A With Diaeresis vs decomposed) is rejected.                                                            |
| **Symlink-escape under EvalSymlinks**                                     | `skills.ensurePathWithinRoot:9-36`; `daytona/tar.ensureSafeParent:297-313`.                                                                                                                            | Place `link -> /etc` inside the sandbox root, drive `fs/read_text_file path=link/passwd`; assert the EvalSymlinks-after-join check rejects.                                                                              |
| **OPENAI_API_KEY etc never persisted in tool logs**                       | `internal/tools/result_limit.go:17-104` + `internal/tools/dispatch.go:606-624` redaction; `internal/diagnostics/redact.go` runtime registry.                                                          | Fixture writes the key to a workspace file, agent reads it via `fs/read_text_file`, assert the persisted SSE event + tool ledger contains `[REDACTED]` (not the literal key) and the `RedactedInputFields` path is set. |
| **Hosted MCP sidecar lifecycle**                                          | `internal/mcp/hosted.go:182-...` Launch → bind → projection → call → release; daemon shutdown propagates SIGTERM to the proxy.                                                                        | E2E: start session, capture proxy PID (PPID == daemon), `agh daemon stop`, assert proxy exits within `defaultShutdownTimeout` (10s), no orphan.                                                                          |
| **Hosted MCP exposes daemon tool through real agent**                     | Same.                                                                                                                                                                                                  | Real Claude Code calls `mcp__agh-hosted-tools__agh__skill_view`, daemon executes through `RuntimeRegistry`, response carries valid skill content; ledger row matches.                                                    |
| **MCP plugin lifecycle hot reload**                                       | Hosted projection digest invalidates on registry change; remote MCP `CallExecutor.servers` re-resolves on every call.                                                                                  | Drive `agh extensions install <ext-with-mcp>`, observe SSE projection-changed event, run an in-flight call across the toggle, assert no in-flight call is dropped.                                                       |
| **External HTTP call timeout — chaos**                                    | `executor.go:129-150` constructs `*http.Client{Timeout: defaultCallTimeout}`; `auth/service.go:21-53` does the same.                                                                                  | Stand up a slow MCP `http` server (60s body delay), call a tool, assert error within `executor.timeout + slack`, error wraps `context.DeadlineExceeded`.                                                                  |
| **Tool registry collision precedence**                                    | `registry.go:288-289` accumulates `conflicts`; `conflictReason:438-444` distinguishes ID vs sanitized-name collisions.                                                                                 | Register two providers exposing the same external `mcp__github__search` and `mcp__github__Search`; assert higher-precedence layer wins, lower is `Conflicted=true`, audit event recorded.                                |
| **Hook deny short-circuits pre-call before spawn**                       | `runPreCallHook:267-275` returns `ErrToolDenied` before `target.handle.Call` runs; failDispatch emits `ToolCallDenied`, not `ToolCallFailed`.                                                          | Configure a hook that returns `Decision{Callable:false}`, drive a real agent call, assert no provider-side process spawn (e.g. no `terminal/create` PID created).                                                        |
| **Hook narrowing on `fs/write_text_file`**                                | `mergeHookCallRequest` (lines 339-371) preserves narrowed `Input`; runtime re-validates the patched input (`dispatch.go:277-281`).                                                                     | Hook narrows allowlist to `<workspace>/notes/`, drive `fs/write_text_file path=<workspace>/secrets/leak.md`; assert deny.                                                                                                  |
| **Concurrent tool dispatch — registry tracks each**                       | In-memory + durable registry path is concurrent-safe (`registry.go:208-212` lock).                                                                                                                     | One agent issues 5 parallel `terminal/create` calls; assert 5 distinct `ProcessRecord` rows, all `state=running`, all complete cleanly, no orphan PIDs after Stop.                                                       |
| **Tool process group cleanup on Unix; forced-exit on Windows**            | `internal/CLAUDE.md` "process-group supervision parity"; `acp/process_tree_unix.go:18-25` vs `process_tree_windows.go:14-30`; `toolruntime/interrupt.go:52-57` signals process group when `pgid > 0`. | Spawn child via `terminal/create` that itself spawns a grandchild; interrupt the terminal; assert grandchild PID also gone (Unix). On Windows, assert forced-exit.                                                       |
| **`http.DefaultClient` is never used in production tool/MCP/sandbox path** | Root prompt invariant.                                                                                                                                                                                 | Code-search gate: `grep -nR 'http\.DefaultClient' internal/tools internal/toolruntime internal/sandbox internal/mcp` returns ZERO matches outside `*_test.go` and `daytona/ssh.go:88` (which is gated on test injection). |
| **Approval bridge unreachable surfaces ReasonApprovalUnreachable**        | `dispatch.go:294-300`.                                                                                                                                                                                 | Wire registry without `WithApprovalBridge`, attempt approval-required tool; assert the typed error and reason; SSE/event captures the reason code.                                                                       |

## 4. Real-LLM / Real-Agent Scenarios

Each scenario is a fenced markdown block with `qa-scenario` + `qa-flow`.
Numbered TOL-01..TOL-17. Live runs use
`agh sessions start --agent claude-code --workspace ./fixtures/<theme>`
against a real Claude Code subagent unless tagged `provider: none`. All runs
use `agh-qa-bootstrap`-issued `bootstrap-manifest.json` with unique
`AGH_HOME`, daemon ports, and provider auth resolved per the provider's
`home_policy`: bound-secret and explicitly isolated-home lanes use
`PROVIDER_HOME` / `PROVIDER_CODEX_HOME`, while `native_cli` lanes with
`home_policy=operator` preserve the operator `HOME` unless the scenario
explicitly validates isolated provider-home behavior.

```markdown
### TOL-01 — Real Claude Code drives fs/read + fs/write + terminal/create end-to-end

```yaml qa-scenario
id: tol-fs-terminal-real-roundtrip
title: Real Claude Code reads, writes, and runs a command; sandbox profile honored; ledger captures every step
theme: tools-sandbox
coverage:
  primary:
    - tools.dispatch.real-roundtrip
  secondary:
    - sandbox.local.tool-host
    - toolruntime.process-record
    - acp.fs.read-write
    - acp.terminal.create
live: true
provider: claude-code
```

```yaml qa-flow
preconditions:
  - $LAB_HOME from agh-qa-bootstrap; PROVIDER_HOME set; default :2123 forbidden.
  - Workspace fixture: ./fixtures/tol01/ contains AGENT.md + a small README.md.
  - Sandbox profile: backend=local, permissions=approve-reads (the local provider default per provider.go:63).
steps:
  - run: agh daemon start && sleep 5
  - run: agh sessions start --agent claude-code --workspace ./fixtures/tol01 -o json | tee sess.json
  - set: SID=$(jq -r '.id' sess.json)
  - run: agh sessions prompt $SID --message "Read README.md, then write a NOTES.md summarizing it (3 lines), then run 'wc -l NOTES.md' and report the count" -o json | tee prompt.json
  - run: agh sessions transcript $SID -o json > transcript.json
  - run: agh tool ledger --session-id $SID -o json > ledger.json
  - run: jq '[.[] | select(.kind=="tool_call_completed") | {tool_id, source_kind, decision, reason_codes, result_bytes}]' ledger.json | tee summary.json
  - run: ls -la ./fixtures/tol01/NOTES.md
expected_behavior:
  - prompt.json: `state == "completed"`; assistant text references the wc -l count.
  - ./fixtures/tol01/NOTES.md exists, ≥1 byte, and is inside the workspace root (no symlink trickery).
  - ledger.json contains:
    - at least one `tool_call_started` + `tool_call_completed` for `fs/read_text_file` (descriptor source kind = acp).
    - at least one for `fs/write_text_file`.
    - at least one for `terminal/create` (the wc command).
  - For each completed event: `result_bytes > 0`; `decision in {allowed, allowed_with_approval}`; `reason_codes` is an array (may be empty).
  - No raw `OPENAI_API_KEY`, `ANTHROPIC_API_KEY`, or `agh_claim_*` literal in transcript.json or ledger.json.
evidence_to_capture:
  - sess.json, prompt.json, transcript.json, ledger.json, summary.json.
  - `ps -o pid,ppid,pgid,command -ax` snapshot showing the agent subprocess + any terminal subprocesses.
failure_signatures:
  - Any of fs/read_text_file, fs/write_text_file, or terminal/create absent from ledger.
  - NOTES.md written outside the workspace root.
  - Raw secret literal in any captured artifact.
cleanup:
  - agh sessions stop $SID && agh daemon stop && rm -f ./fixtures/tol01/NOTES.md
```
```

```markdown
### TOL-02 — Tool interrupt mid-execution: agent runs sleep 60, user cancels, process killed via process group

```yaml qa-scenario
id: tol-interrupt-mid-execution
title: terminal/create sleep 60 + Driver.Interrupt(TerminalID) ⇒ SIGTERM → SIGKILL via process group; ledger reports interrupted
theme: tools-sandbox
coverage:
  primary:
    - toolruntime.interrupt.process-group
  secondary:
    - acp.terminal.kill
    - subprocess.process-group.signal
live: true
provider: claude-code
```

```yaml qa-flow
preconditions:
  - As TOL-01.
  - Build is current (toolruntime/interrupt.go owns SIGTERM 250ms then SIGKILL 1s grace).
steps:
  - run: agh daemon start && sleep 5
  - run: agh sessions start --agent claude-code --workspace ./fixtures/tol02 -o json | tee sess.json
  - set: SID=$(jq -r '.id' sess.json)
  - run: agh sessions prompt $SID --message "Run a 'sleep 60 && echo done' command, do not finish until it completes" --detach -o json | tee prompt.json
  - wait: until ledger.json shows tool_call_started for terminal/create with sleep 60 in args
  - run: agh tool processes --session-id $SID -o json | tee procs.json
  - set: PID=$(jq -r '.[] | select(.source=="acp_terminal") | .pid' procs.json | head -1)
  - set: PGID=$(jq -r '.[] | select(.source=="acp_terminal") | .process_group_id' procs.json | head -1)
  - set: TID=$(jq -r '.[] | select(.source=="acp_terminal") | .owner.terminal_id' procs.json | head -1)
  - run: ps -o pid,ppid,pgid,command -p $PID
  - run: agh tool interrupt --session-id $SID --terminal-id $TID -o json | tee interrupt.json
  - wait: 3s
  - run: kill -0 $PID 2>&1 || echo "child gone"
  - run: agh tool ledger --session-id $SID -o json | jq '[.[] | select(.kind=="tool_call_completed" or .kind=="tool_call_failed") | {kind, tool_id, reason_codes, error_code}]' | tee outcomes.json
  - run: agh sessions transcript $SID -o json | jq '.[] | select(.kind=="tool_result") | .result.is_error' | tee is_error.txt
expected_behavior:
  - procs.json reports a row with state=running, source=acp_terminal, pgid > 0.
  - interrupt.json: `matched >= 1`, `signaled >= 1`.
  - kill -0 $PID exits non-zero (process gone within 1.25s of interrupt; toolruntime/interrupt.go grace constants).
  - outcomes.json shows the terminal/create call as `tool_call_failed` (or `tool_call_completed` with terminal exit_status non-zero) carrying reason that maps to interruption — never `tool_call_completed` with success and result content from sleep.
  - is_error.txt contains true for the interrupted tool result.
  - On Windows (cross-build path) the equivalent CI run uses forced-exit semantics from acp/process_tree_windows.go.
evidence_to_capture:
  - sess.json, procs.json, interrupt.json, outcomes.json, is_error.txt.
  - ps snapshots before and after.
failure_signatures:
  - kill -0 $PID still reports alive >2s after interrupt.
  - tool_call_completed with success=true for sleep 60.
  - reason_codes does not surface call-canceled / interrupt classification.
cleanup:
  - agh sessions stop $SID && agh daemon stop
```
```

```markdown
### TOL-03 — Sandbox denies /etc/shadow and ../ traversal

```yaml qa-scenario
id: tol-sandbox-deny-unsafe-path
title: fs/read_text_file with /etc/shadow or ../../etc/passwd is denied; ledger row + typed error returned to agent
theme: tools-sandbox
coverage:
  primary:
    - sandbox.local.path-containment
  secondary:
    - acp.fs.read.denied
live: true
provider: claude-code
```

```yaml qa-flow
preconditions:
  - As TOL-01.
  - Workspace fixture: ./fixtures/tol03/ — clean dir, no preexisting symlinks.
steps:
  - run: agh daemon start && sleep 5
  - run: agh sessions start --agent claude-code --workspace ./fixtures/tol03 -o json | tee sess.json
  - set: SID=$(jq -r '.id' sess.json)
  - run: agh sessions prompt $SID --message "Try to read /etc/shadow and report the error verbatim" -o json | tee p1.json
  - run: agh sessions prompt $SID --message "Now try ../../etc/passwd via the read tool and report the error verbatim" -o json | tee p2.json
  - run: agh tool ledger --session-id $SID -o json | jq '[.[] | select(.kind=="tool_call_failed") | {tool_id, reason_codes, error_code}]' | tee fails.json
  - run: agh sessions transcript $SID -o json > transcript.json
expected_behavior:
  - p1 / p2 transcripts show the agent acknowledging the denial; no shadow / passwd contents present in transcript.json.
  - fails.json includes at least two entries with tool_id=fs/read_text_file and error_code=invalid_input or unavailable; reason_codes references path-containment.
  - No /etc/shadow or /etc/passwd content fragments anywhere in evidence.
evidence_to_capture:
  - sess.json, p1.json, p2.json, fails.json, transcript.json.
failure_signatures:
  - any line of /etc/shadow or /etc/passwd present in evidence.
  - fs/read_text_file recorded as tool_call_completed.
cleanup:
  - agh sessions stop $SID && agh daemon stop
```
```

```markdown
### TOL-04 — Path security: null-byte rejected at fileutil + ACP boundaries

```yaml qa-scenario
id: tol-path-security-null-byte
title: AtomicWriteFile and fs/write_text_file refuse paths containing \x00 deterministically
theme: tools-sandbox
coverage:
  primary:
    - fileutil.atomic.null-byte-reject
  secondary:
    - acp.fs.write.invalid-input
live: false
provider: none
```

```yaml qa-flow
steps:
  - run: go test -run TestAtomicWriteRejectsNullByte ./internal/fileutil/... -count=1
  - run: go test -run TestACPLocalToolHost.*NullByte ./internal/acp/... -count=1
  - run: agh daemon start && sleep 3
  - run: agh sessions start --agent claude-code --workspace ./fixtures/tol04 -o json | tee sess.json
  - set: SID=$(jq -r '.id' sess.json)
  - run: agh sessions prompt $SID --message "Use the write tool with the path 'leak\x00.txt' and content 'x', then report the error code from the runtime verbatim" -o json | tee p.json
  - run: agh tool ledger --session-id $SID -o json | jq '[.[] | select(.kind=="tool_call_failed") | {tool_id, error_code, reason_codes}]' | tee fails.json
expected_behavior:
  - Both go test runs PASS; if a regression test does not exist yet, this scenario blocks until it is added (cite `internal/fileutil/atomic.go:14-46` and `internal/acp/handlers.go:237-247`).
  - fails.json contains a tool_call_failed for fs/write_text_file with error_code=invalid_input.
  - No `leak*.txt` file (or any stub) materializes anywhere under $LAB_HOME, ./fixtures/tol04, or /tmp.
evidence_to_capture:
  - go test output, sess.json, fails.json, full filesystem diff of fixture before/after.
failure_signatures:
  - Any file matching `leak*` written.
  - error_code != invalid_input.
cleanup:
  - agh sessions stop $SID && agh daemon stop
```
```

```markdown
### TOL-05 — Path security: URL-encoded %2e%2e/ traversal rejected

```yaml qa-scenario
id: tol-path-security-urlencoded-traversal
title: fs/write_text_file with %2e%2e/ does not get decoded into ../ — denied at boundary
theme: tools-sandbox
coverage:
  primary:
    - sandbox.local.urlencoded-traversal
live: true
provider: claude-code
```

```yaml qa-flow
steps:
  - run: agh daemon start && sleep 5
  - run: agh sessions start --agent claude-code --workspace ./fixtures/tol05 -o json | tee sess.json
  - set: SID=$(jq -r '.id' sess.json)
  - run: agh sessions prompt $SID --message "Use the write tool with path 'docs/%2e%2e/etc/passwd' and content 'x'; if it errors, paste the error" -o json | tee p.json
  - run: agh tool ledger --session-id $SID -o json | jq '[.[] | select(.kind=="tool_call_failed" or .kind=="tool_call_completed") | {kind, tool_id, error_code, reason_codes}]' | tee out.json
  - run: ls -la ./fixtures/tol05 /etc/passwd
expected_behavior:
  - out.json shows fs/write_text_file as tool_call_failed (path is treated literally, ends up outside workspace, EvalSymlinks-after-join rejects).
  - /etc/passwd is unmodified (mtime + checksum unchanged from a captured baseline).
  - No file under ./fixtures/tol05/docs/%2e%2e/.
evidence_to_capture:
  - sess.json, p.json, out.json, mtime/checksum of /etc/passwd before+after.
failure_signatures:
  - tool_call_completed for fs/write_text_file in this scenario.
  - any change to /etc/passwd.
cleanup:
  - agh sessions stop $SID && agh daemon stop
```
```

```markdown
### TOL-06 — Path security: NFC vs NFD Unicode homoglyph rejected

```yaml qa-scenario
id: tol-path-security-unicode-homoglyph
title: fs/write_text_file with an NFD-decomposed path that escapes the workspace root is denied
theme: tools-sandbox
coverage:
  primary:
    - sandbox.local.unicode-normalization
live: true
provider: claude-code
```

```yaml qa-flow
preconditions:
  - Workspace fixture root is ./fixtures/tol06/composed/ (containing a Latin Small Letter A With Diaeresis: U+00E4).
  - Agent will be asked to target a sibling that, post-NFD-normalization, decomposes back to U+0061 U+0308 — same on disk for case-insensitive filesystems but distinct strings at the API boundary.
steps:
  - run: agh daemon start && sleep 5
  - run: agh sessions start --agent claude-code --workspace ./fixtures/tol06/composed -o json | tee sess.json
  - set: SID=$(jq -r '.id' sess.json)
  - run: agh sessions prompt $SID --message "Use the write tool with path '../decomposed/leak.txt' (where 'decomposed' is the NFD form of the workspace root's last segment) and content 'x'" -o json | tee p.json
  - run: agh tool ledger --session-id $SID -o json | jq '[.[] | select(.kind=="tool_call_failed") | {tool_id, error_code, reason_codes}]' | tee out.json
  - run: ls -la ./fixtures/tol06
expected_behavior:
  - out.json: fs/write_text_file failed with reason citing path containment.
  - No file `decomposed/leak.txt` exists outside the composed root.
evidence_to_capture:
  - p.json, out.json, full directory tree of ./fixtures/tol06.
failure_signatures:
  - leak.txt written anywhere outside the composed/ root.
  - tool_call_completed for fs/write_text_file with this path.
cleanup:
  - agh sessions stop $SID && agh daemon stop && rm -rf ./fixtures/tol06/decomposed
```
```

```markdown
### TOL-07 — Path security: symlink escape rejected (link → /etc inside approved root)

```yaml qa-scenario
id: tol-path-security-symlink-escape
title: A symlink placed inside the sandbox root points to /etc — fs/read_text_file via the link is denied (EvalSymlinks-after-join check)
theme: tools-sandbox
coverage:
  primary:
    - sandbox.local.symlink-containment
  secondary:
    - skills.path-security.parity
live: true
provider: claude-code
```

```yaml qa-flow
preconditions:
  - ./fixtures/tol07/ exists.
  - Pre-step creates `./fixtures/tol07/escape -> /etc` symlink before the daemon resolves the workspace.
steps:
  - run: ln -snf /etc ./fixtures/tol07/escape
  - run: agh daemon start && sleep 5
  - run: agh sessions start --agent claude-code --workspace ./fixtures/tol07 -o json | tee sess.json
  - set: SID=$(jq -r '.id' sess.json)
  - run: agh sessions prompt $SID --message "Read 'escape/passwd' via the read tool and report any error verbatim" -o json | tee p.json
  - run: agh tool ledger --session-id $SID -o json | jq '[.[] | select(.kind=="tool_call_failed") | {tool_id, error_code, reason_codes}]' | tee out.json
  - run: agh sessions transcript $SID -o json | grep -c "root:" || echo "no shadow contents"
expected_behavior:
  - out.json shows fs/read_text_file failed; reason_codes maps to a containment violation.
  - Transcript does not contain `/etc/passwd` content.
  - The skills helper `skills.ensurePathWithinRoot` (`internal/skills/path_security.go:9-36`) enforces the same shape; this scenario asserts the sandbox tool host has parity.
evidence_to_capture:
  - sess.json, p.json, out.json, transcript.json.
failure_signatures:
  - any /etc/passwd content present.
  - tool_call_completed for fs/read_text_file via the escape link.
cleanup:
  - agh sessions stop $SID && agh daemon stop && rm ./fixtures/tol07/escape
```
```

```markdown
### TOL-08 — Secret redaction: OPENAI_API_KEY in tool log stripped, transcript shows [REDACTED]

```yaml qa-scenario
id: tol-secret-redaction-tool-logs
title: A workspace file containing OPENAI_API_KEY=sk-… is read by the agent; the tool ledger and SSE never carry the raw key
theme: tools-sandbox
coverage:
  primary:
    - tools.result-limiter.redaction
  secondary:
    - diagnostics.dynamic-secret.scope
    - sse.redaction
live: true
provider: claude-code
```

```yaml qa-flow
preconditions:
  - ./fixtures/tol08/.env contains: `OPENAI_API_KEY=sk-fakeQAA1B2C3D4E5F6G7H8I9` (synthetic key, prefix matches real OpenAI shape but is a deterministic fixture).
  - Agent system prompt explicitly instructs to NOT echo secrets, to prove redaction is structural, not behavioral.
steps:
  - run: agh daemon start && sleep 5
  - run: agh sessions start --agent claude-code --workspace ./fixtures/tol08 -o json | tee sess.json
  - set: SID=$(jq -r '.id' sess.json)
  - run: agh sessions prompt $SID --message "Read .env and tell me how many lines it has; do not repeat any value verbatim" -o json | tee p.json
  - run: agh tool ledger --session-id $SID -o json > ledger.json
  - run: agh sessions transcript $SID -o json > transcript.json
  - run: agh sessions events $SID -o json > events.json
  - run: grep -E 'sk-fakeQAA1B2C3D4E5F6G7H8I9' ledger.json transcript.json events.json $LAB_HOME/logs/agh.log; echo "exit=$?"
  - run: jq '[.[] | select(.kind=="tool_call_completed") | {tool_id, redacted_input_fields, result_redaction_paths}]' ledger.json | tee redactions.json
expected_behavior:
  - grep returns exit 1 (no match) across ALL evidence (ledger, transcript, events, agh.log).
  - redactions.json shows at least one `result_redaction_paths` entry covering the key location for the read result; `redacted_input_fields` is set when the agent passes a secret-bearing argument.
  - Captured tool result value at the key offset is the literal `[REDACTED]` (`internal/tools/result_limit.go:17`).
evidence_to_capture:
  - ledger.json, transcript.json, events.json, $LAB_HOME/logs/agh.log, redactions.json.
failure_signatures:
  - Any of the captured artifacts contains `sk-fakeQAA1B2C3D4E5F6G7H8I9`.
  - result_redaction_paths empty when result clearly contained the key.
cleanup:
  - agh sessions stop $SID && agh daemon stop
```
```

```markdown
### TOL-09 — Hosted MCP sidecar lifecycle: spawn on session start, daemon stop kills sidecar within timeout

```yaml qa-scenario
id: tol-mcp-sidecar-lifecycle
title: agh-hosted-tools sidecar starts with the session and exits when the daemon is stopped (no orphan)
theme: tools-sandbox
coverage:
  primary:
    - mcp.hosted.lifecycle
  secondary:
    - daemon.shutdown.subprocess-cascade
live: true
provider: claude-code
```

```yaml qa-flow
preconditions:
  - config.toml enables hosted MCP (default per HostedConfig.Enabled wiring).
steps:
  - run: agh daemon start && sleep 5
  - run: agh sessions start --agent claude-code --workspace ./fixtures/tol09 -o json | tee sess.json
  - set: SID=$(jq -r '.id' sess.json)
  - run: ps -o pid,ppid,pgid,command -ax | grep -E "agh tool mcp|agh-hosted-tools" | tee proxy.before
  - set: PROXY_PID=$(awk '{print $1}' proxy.before | head -1)
  - run: kill -0 $PROXY_PID && echo "proxy alive"
  - run: agh daemon stop -o json | tee stop.json
  - wait: 12s
  - run: kill -0 $PROXY_PID 2>&1 || echo "proxy gone"
  - run: ps -o pid,ppid,command -ax | grep -E "agh tool mcp|agh-hosted-tools" || true | tee proxy.after
expected_behavior:
  - proxy.before lists at least one `agh tool mcp` (hosted-MCP stdio proxy) child whose PPID is the daemon PID.
  - kill -0 $PROXY_PID after `agh daemon stop` (within defaultShutdownTimeout=10s + slack) reports the proxy gone.
  - proxy.after is empty.
  - `mcp/hosted.go` ErrHostedDisabled is NOT logged unless the config gates it off.
evidence_to_capture:
  - proxy.before, proxy.after, stop.json, $LAB_HOME/logs/agh.log filtered to mcp.
failure_signatures:
  - proxy.after non-empty.
  - PROXY_PID still alive >12s after stop.
cleanup:
  - rm -f proxy.before proxy.after
```
```

```markdown
### TOL-10 — Hosted MCP exposes a daemon tool to the real agent

```yaml qa-scenario
id: tol-mcp-hosted-tool-roundtrip
title: Real agent calls mcp__agh-hosted-tools__agh__skill_view through the hosted MCP proxy; round-trip works; ledger captures it
theme: tools-sandbox
coverage:
  primary:
    - mcp.hosted.call-roundtrip
  secondary:
    - tools.builtin.skills
live: true
provider: claude-code
```

```yaml qa-flow
preconditions:
  - At least one user-level skill installed under $LAB_HOME/skills/ (or bundled fallback).
steps:
  - run: agh daemon start && sleep 5
  - run: agh sessions start --agent claude-code --workspace ./fixtures/tol10 -o json | tee sess.json
  - set: SID=$(jq -r '.id' sess.json)
  - run: agh sessions prompt $SID --message "List your available agh-hosted skills via the hosted MCP, then view the first one and report its title and 5-line summary" -o json | tee p.json
  - run: agh tool ledger --session-id $SID -o json | jq '[.[] | select(.tool_id | startswith("mcp__agh-hosted-tools__")) | {tool_id, kind, decision}]' | tee mcp.json
  - run: agh sessions events $SID -o json | jq '[.[] | select(.event=="mcp.hosted.bind" or .event=="mcp.hosted.call.completed")]' | tee mcphosted_events.json
expected_behavior:
  - mcp.json shows at least one `mcp__agh-hosted-tools__agh__skill_list` (or `..__agh__skill_search`) followed by `..__agh__skill_view` — both `tool_call_completed` with decision=allowed.
  - p.json transcript references skill content the agent could only have obtained from the hosted MCP (cite a bundled-skill marker line).
  - mcphosted_events.json records the bind event with `bind_id` and `digest`.
evidence_to_capture:
  - sess.json, p.json, mcp.json, mcphosted_events.json.
failure_signatures:
  - No mcp__agh-hosted-tools__* call in the ledger.
  - Hosted MCP returns ErrHostedBindNotFound or ErrHostedNonceExpired during the run.
cleanup:
  - agh sessions stop $SID && agh daemon stop
```
```

```markdown
### TOL-11 — MCP plugin lifecycle hot reload: extension toggle without daemon restart

```yaml qa-scenario
id: tol-mcp-plugin-hot-reload
title: An extension exposing an MCP server is installed/disabled/re-enabled mid-session; in-flight calls drain; new calls hit the new projection
theme: tools-sandbox
coverage:
  primary:
    - extension.lifecycle.hot-reload
  secondary:
    - mcp.executor.servers-resolution
    - tools.registry.projection-digest
live: true
provider: claude-code
```

```yaml qa-flow
preconditions:
  - Test fixture extension `qa-mcp-fixture` exposes one MCP stdio server with a single tool `mcp__qa-mcp-fixture__echo`.
steps:
  - run: agh daemon start && sleep 5
  - run: agh extensions install ./fixtures/extensions/qa-mcp-fixture -o json
  - run: agh sessions start --agent claude-code --workspace ./fixtures/tol11 -o json | tee sess.json
  - set: SID=$(jq -r '.id' sess.json)
  - run: agh sessions prompt $SID --message "Use mcp__qa-mcp-fixture__echo with {\"text\":\"hello\"} and report the response" -o json | tee p1.json
  - run: agh extensions disable qa-mcp-fixture -o json
  - wait: 1s
  - run: agh sessions prompt $SID --message "Try mcp__qa-mcp-fixture__echo again with {\"text\":\"world\"}; if it errors paste the error verbatim" -o json | tee p2.json
  - run: agh extensions enable qa-mcp-fixture -o json
  - wait: 1s
  - run: agh sessions prompt $SID --message "Try mcp__qa-mcp-fixture__echo with {\"text\":\"reborn\"}" -o json | tee p3.json
  - run: agh tool ledger --session-id $SID -o json | jq '[.[] | select(.tool_id=="mcp__qa-mcp-fixture__echo") | {kind, decision, reason_codes}]' | tee echo.json
expected_behavior:
  - p1: tool_call_completed (echo returns "hello").
  - p2: tool_call_failed with reason_codes citing source disabled or backend unavailable.
  - p3: tool_call_completed (echo returns "reborn").
  - echo.json shows three rows in expected order.
  - Daemon PID is unchanged across all three prompts.
evidence_to_capture:
  - p1.json, p2.json, p3.json, echo.json, daemon PID before+after.
failure_signatures:
  - Daemon restart required for the toggle.
  - p2 succeeds (disable did not take effect).
  - p3 fails (re-enable did not take effect).
cleanup:
  - agh extensions remove qa-mcp-fixture -o json && agh sessions stop $SID && agh daemon stop
```
```

```markdown
### TOL-12 — External HTTP timeout: chaos delay forces deterministic timeout, not hang

```yaml qa-scenario
id: tol-mcp-external-timeout-chaos
title: A remote MCP http server stalls 60s; CallExecutor returns a deadline error within executor.timeout + slack
theme: tools-sandbox
coverage:
  primary:
    - mcp.executor.timeout
  secondary:
    - sandbox.no-default-http-client
live: false
provider: none
```

```yaml qa-flow
preconditions:
  - Stand up a local stub MCP `http` server at http://127.0.0.1:$STUB_PORT/ that sleeps 60s before responding.
  - config.toml registers the stub as an MCP server with `kind: http`, no auth.
  - executor.timeout left at default 30s (`internal/mcp/executor.go:25`).
steps:
  - run: agh daemon start && sleep 5
  - run: time agh tool call --tool-id mcp__qa-stub__noop --input '{}' -o json 2>&1 | tee out.json; echo "elapsed=${SECONDS}s"
  - run: jq '{error_code, reason_codes, message}' out.json
expected_behavior:
  - elapsed is between executor.timeout (30s) and executor.timeout + 5s.
  - error_code = timed_out or backend_failed; reason_codes contains call_timed_out or backend_unhealthy.
  - Log line in agh.log shows "context.DeadlineExceeded" wrapped error from the mcp-go transport.
evidence_to_capture:
  - out.json, $LAB_HOME/logs/agh.log filtered to the call's correlation_id, time output.
failure_signatures:
  - elapsed >> executor.timeout + 5s (hang regression).
  - error_code reports succeeded.
  - http.DefaultClient diagnostic — search agh.log for any frame mentioning DefaultClient.
cleanup:
  - agh daemon stop && tear down stub server.
```
```

```markdown
### TOL-13 — Tool registry collision: precedence resolves, lower layer becomes Conflicted

```yaml qa-scenario
id: tol-registry-collision-precedence
title: Two providers register the same external tool ID; higher-precedence layer wins; lower layer is marked Conflicted; audit event recorded
theme: tools-sandbox
coverage:
  primary:
    - tools.registry.collision
  secondary:
    - extension.precedence-layer
live: false
provider: none
```

```yaml qa-flow
preconditions:
  - Two fixture extensions `qa-precedence-bundled` and `qa-precedence-user` both declare `mcp__github__search`. Bundled (or higher-layer) MUST win per the five-layer precedence rule (`internal/CLAUDE.md` Memory & Skills RFC).
steps:
  - run: agh daemon start && sleep 3
  - run: agh extensions install ./fixtures/extensions/qa-precedence-user -o json
  - run: agh tool list --tool-id mcp__github__search -o json | tee lookup.json
  - run: agh tool registry diagnostics -o json | jq '[.tools[] | select(.id=="mcp__github__search") | {id, source, availability, decision}]' | tee diag.json
  - run: agh observe events --filter tool_registry_collision --limit 10 -o json | tee audit.json
expected_behavior:
  - lookup.json shows ONE callable view from the bundled (higher-precedence) source; user-layer view is `Availability.Conflicted=true` with `ReasonCodes=[conflicted_id]` (`internal/tools/registry.go:438-444, 480-485`).
  - diag.json reflects the same.
  - audit.json contains at least one shadow / collision audit event per the five-layer rule.
evidence_to_capture:
  - lookup.json, diag.json, audit.json.
failure_signatures:
  - Both views surface as callable.
  - No audit event recorded.
cleanup:
  - agh extensions remove qa-precedence-user -o json && agh daemon stop
```
```

```markdown
### TOL-14 — Hook deny short-circuits dispatch (no provider-side spawn)

```yaml qa-scenario
id: tol-hook-deny-pre-call
title: Pre-call hook returns Decision{Callable:false} ⇒ tool_call_denied emitted, no provider call, agent receives the deny
theme: tools-sandbox
coverage:
  primary:
    - tools.hooks.pre-call.deny
live: true
provider: claude-code
```

```yaml qa-flow
preconditions:
  - Hook script under .agh/hooks/qa-deny-write.sh returns JSON `{decision:{callable:false, reason_codes:["hook_denied"]}}` for any pre_call event whose tool_id == fs/write_text_file.
  - Hook is wired via config.
steps:
  - run: agh daemon start && sleep 5
  - run: agh sessions start --agent claude-code --workspace ./fixtures/tol14 -o json | tee sess.json
  - set: SID=$(jq -r '.id' sess.json)
  - run: agh sessions prompt $SID --message "Write a one-line CLAIMED.md saying 'pwned' to the workspace and confirm" -o json | tee p.json
  - run: agh tool ledger --session-id $SID -o json | jq '[.[] | select(.tool_id=="fs/write_text_file") | {kind, decision, reason_codes, error_code}]' | tee out.json
  - run: ls -la ./fixtures/tol14
  - run: agh hooks runs --session-id $SID -o json | jq '[.[] | select(.tool_id=="fs/write_text_file") | {hook, decision, error}]' | tee hooks.json
expected_behavior:
  - out.json shows fs/write_text_file as `tool_call_denied` with reason_codes including `hook_denied`; NO `tool_call_started` for fs/write_text_file makes it past the pre-hook (hooks fire BEFORE tool_call_started in `dispatch.go:40-49`; check ordering carefully).
  - No CLAIMED.md exists.
  - hooks.json records the deny.
evidence_to_capture:
  - sess.json, p.json, out.json, hooks.json, fixture directory listing.
failure_signatures:
  - CLAIMED.md materializes.
  - tool_call_completed for fs/write_text_file in this scenario.
cleanup:
  - agh sessions stop $SID && agh daemon stop
```
```

```markdown
### TOL-15 — Hook narrowing: write allowlist confined to subdir

```yaml qa-scenario
id: tol-hook-narrow-allowlist
title: Pre-call hook narrows fs/write_text_file allowlist to <workspace>/notes/; write outside the allowlist denied at boundary
theme: tools-sandbox
coverage:
  primary:
    - tools.hooks.pre-call.narrow
live: true
provider: claude-code
```

```yaml qa-flow
preconditions:
  - Hook .agh/hooks/qa-narrow-write.sh returns a patched CallRequest whose Input.path is rewritten to add an allowlist annotation, AND a Decision that injects an additional reason if path is outside notes/.
  - The hook follows mergeHookCallRequest semantics (`dispatch.go:339-371`) — does NOT rewrite tool_id (forbidden, lines 258-266).
steps:
  - run: agh daemon start && sleep 5
  - run: agh sessions start --agent claude-code --workspace ./fixtures/tol15 -o json | tee sess.json
  - set: SID=$(jq -r '.id' sess.json)
  - run: agh sessions prompt $SID --message "Write 'inside notes' to notes/note1.md (this should succeed) then write 'outside' to secrets/leak.md (this should fail)" -o json | tee p.json
  - run: agh tool ledger --session-id $SID -o json | jq '[.[] | select(.tool_id=="fs/write_text_file") | {kind, input_digest, reason_codes}]' | tee out.json
  - run: ls -la ./fixtures/tol15/notes ./fixtures/tol15/secrets 2>&1
expected_behavior:
  - notes/note1.md exists; secrets/leak.md does not exist.
  - out.json shows two fs/write_text_file events: first kind=tool_call_completed, second kind=tool_call_denied with reason_codes containing hook_denied (or path-containment).
evidence_to_capture:
  - sess.json, p.json, out.json, fixture directory listing.
failure_signatures:
  - secrets/leak.md exists.
  - hook attempted to rewrite tool_id (`dispatch.go:258-266` should have rejected; if not, this is a bypass bug).
cleanup:
  - agh sessions stop $SID && agh daemon stop && rm -rf ./fixtures/tol15/notes ./fixtures/tol15/secrets
```
```

```markdown
### TOL-16 — Concurrent tool dispatch: 5 parallel calls, registry tracks each, zero orphans

```yaml qa-scenario
id: tol-concurrent-dispatch
title: One agent issues 5 parallel terminal/create calls; toolruntime registry tracks each; cleanup leaves zero orphans
theme: tools-sandbox
coverage:
  primary:
    - tools.dispatch.concurrent
  secondary:
    - toolruntime.registry.concurrency
live: true
provider: claude-code
```

```yaml qa-flow
steps:
  - run: agh daemon start && sleep 5
  - run: agh sessions start --agent claude-code --workspace ./fixtures/tol16 -o json | tee sess.json
  - set: SID=$(jq -r '.id' sess.json)
  - run: agh sessions prompt $SID --message "Run these five commands in parallel via the terminal tool: 'sleep 2 && echo a', 'sleep 2 && echo b', 'sleep 2 && echo c', 'sleep 2 && echo d', 'sleep 2 && echo e'. Wait for all of them and report their outputs in order." --detach -o json | tee p.json
  - wait: 1s
  - run: agh tool processes --session-id $SID -o json | tee proc.during.json
  - wait: 5s
  - run: agh tool processes --session-id $SID -o json | tee proc.after.json
  - run: agh tool ledger --session-id $SID -o json | jq '[.[] | select(.tool_id=="terminal/create" and .kind=="tool_call_completed") | .tool_call_id] | length' | tee count.txt
  - run: agh sessions stop $SID && wait_for_session_stop $SID
  - run: ps -ax | grep -v grep | grep -E 'sleep 2 && echo [a-e]' || echo "no orphans"
expected_behavior:
  - proc.during.json: 5 rows with state=running, distinct PIDs and PGIDs, source=acp_terminal.
  - proc.after.json: rows transition to completed (or are gone), no row in interrupting state.
  - count.txt >= 5 (one tool_call_completed per terminal/create).
  - Final ps grep prints "no orphans".
evidence_to_capture:
  - p.json, proc.during.json, proc.after.json, count.txt, ps grep output.
failure_signatures:
  - proc.during.json shows fewer than 5 distinct PIDs (registry race).
  - any sleep child remains after sessions stop (orphan leak).
cleanup:
  - agh daemon stop
```
```

```markdown
### TOL-17 — Real LLM scenario: multi-step refactor inside sandboxed workspace; full audit trail

```yaml qa-scenario
id: tol-real-llm-multistep-refactor
title: Real Claude Code performs read→propose→write→run-tests refactor in a real sandbox; every tool call is auditable end-to-end
theme: tools-sandbox
coverage:
  primary:
    - tools.real-llm.multistep
  secondary:
    - sandbox.local.workspace-bounded
    - tools.observability.coverage-matrix
live: true
provider: claude-code
```

```yaml qa-flow
preconditions:
  - ./fixtures/tol17 contains a small Python module `calc.py` with a buggy `divide` function (returns int division), a failing `test_calc.py`, and an AGENT.md describing the refactor goal.
  - Local sandbox provider; permissions=approve-reads.
steps:
  - run: agh daemon start && sleep 5
  - run: agh sessions start --agent claude-code --workspace ./fixtures/tol17 -o json | tee sess.json
  - set: SID=$(jq -r '.id' sess.json)
  - run: agh sessions prompt $SID --message "Read calc.py and test_calc.py, fix the divide function so the tests pass, then run pytest -q and report PASSED/FAILED" -o json | tee p.json
  - run: agh tool ledger --session-id $SID -o json > ledger.json
  - run: jq '[.[] | select(.kind=="tool_call_completed") | .tool_id] | group_by(.) | map({tool: .[0], count: length})' ledger.json | tee mix.json
  - run: agh sessions events $SID -o json > events.json
  - run: cat ./fixtures/tol17/calc.py
  - run: (cd ./fixtures/tol17 && pytest -q) | tee pytest.txt
  - run: grep -E '\bsk-|\bagh_claim_|\bANTHROPIC_API_KEY' ledger.json events.json transcript.json $LAB_HOME/logs/agh.log; echo "exit=$?"
expected_behavior:
  - p.json: state=completed, assistant text references PASSED.
  - mix.json: at least one fs/read_text_file, one fs/write_text_file, one terminal/create entry.
  - calc.py: divide returns float division (or correct rounding per AGENT.md spec); test passes locally.
  - pytest.txt ends with PASSED.
  - secret grep returns exit 1 (no leak).
  - Every tool call in ledger.json carries: correlation_id, session_id, agent_name, decision, reason_codes (may be empty), input_digest, result_bytes, started_at, duration_ms.
evidence_to_capture:
  - sess.json, p.json, ledger.json, events.json, mix.json, pytest.txt, calc.py before/after.
failure_signatures:
  - Any secret literal present in evidence.
  - Tool call missing correlation_id, session_id, or input_digest.
  - pytest.txt FAILED.
cleanup:
  - agh sessions stop $SID && agh daemon stop && cd ./fixtures/tol17 && git checkout -- calc.py test_calc.py
```
```

## 5. Edge Cases (gates, not full scenarios)

These must be exercised by short asserts inside the QA gate suite or as additional unit tests.

- **Result limiter applies twice when post-call hook rewrites the result.** `dispatch.go:67,75,386-390` runs `Apply` once, then the post-call hook may return a fresh result, then `Apply` runs again. A regression that elides the second `Apply` allows post-hook bypass — assert via mock hook in `dispatch_test.go`.
- **`runPreCallHook` rejects tool_id rewrite.** `dispatch.go:258-266` returns `ReasonHookDenied`; assert hook attempt to rewrite tool_id is denied AND no event is emitted with the rewritten id.
- **Approval bridge unreachable.** `dispatch.go:294-300` — `r.approvalBridge == nil` while `Decision.ApprovalRequired=true` ⇒ `ReasonApprovalUnreachable`. Add a regression test.
- **Result digest is sha256 of canonical JSON, not raw bytes.** `dispatch.go:626-633` — assert deterministic digest across two identical results.
- **Conflicted descriptors never become callable.** `registry.go:178-186, 480-485` — assert `Decision.Callable=false` whenever `Availability.Conflicted=true`, even with a permissive policy evaluator.
- **`procutil.MatchesStartTime` fail closes the interrupter.** `interrupt.go:22-29` — if the recovered process's PID is recycled (start time mismatches), interrupter returns `ErrOwnershipValidationFailed` and does NOT signal the unrelated PID. Add a regression test that injects a mismatching start time.
- **`toolruntime.Registry.Interrupt` returns `ErrProcessNotFound` when no candidate matches.** `registry.go:305-307`. Confirm with empty scope vs scope that matches nothing.
- **`mcp/hosted.go` ErrHostedNonceExpired honors `BindNonceTTL`.** `hosted.go:160-163, 32-37` — wait past TTL, attempt bind, assert exact error.
- **`mcp/hosted_proxy.go` exits with non-zero when `Release` fails.** `hosted_proxy.go:85-86` logs the error; the wrapper should not silently continue. Cite this as a DX cliff if it does.
- **`internal/sandbox/daytona/ssh.go:88` `http.DefaultClient` fallback is dead code in production.** Add a unit test that the production constructor wires a non-nil `httpClient`. If the fallback is reachable, replace with an explicit error or a packaged client.
- **`mcp/executor.go:148-151` clones the http client when its timeout is unset.** Confirm via test that even a caller-injected `http.Client{Timeout: 0}` ends up with `executor.timeout`.
- **`fileutil.AtomicWriteFile` rejects empty path.** `atomic.go:15-17`. Already covered.
- **`fileutil.AtomicWriteFile` cleans up the temp file when rename fails.** `atomic.go:27-32`. Add a test that injects a rename failure (read-only target) and asserts the temp file is gone.
- **External-extension MCP server registration without explicit timeout** — if config allows a `timeout=0` value, it must be replaced with `defaultCallTimeout`. Cite `executor.go:142-144`.
- **macOS `/private/var/folders` canonicalization** — sandbox tests that use temp dirs MUST `EvalSymlinks` the workspace root before assertions, otherwise a `/var/folders` -> `/private/var/folders` mismatch flakes the containment check.
- **NFC vs NFC again under HFS+** — case-insensitive filesystems silently fold homoglyphs; the rejection must happen at the API boundary BEFORE filesystem syscalls.

## 6. Integration Surfaces

Matrix of dependencies and obligations.

| Other module                  | Tools/Sandbox/MCP obligation to module                                                                                              | Module obligation to tools/sandbox/MCP                                                                          | Citation                                                       |
| ----------------------------- | ----------------------------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------- |
| `internal/acp`                | Provide `RegisterConfig.Interrupt` callback; pass PID + PGID; ProcessSource ACPAgent / ACPTerminal.                                  | Construct `LocalLauncher` and `LocalToolHost`; carry the `ProcessRegistry` through the toolHost adapter.        | `internal/acp/client.go:296-331`, `handlers.go:735-756`        |
| `internal/session`            | Tool dispatch is invoked through the session's HookSet; session manager owns approval bridge wiring.                                | Provide Scope (session_id/workspace_id/agent_name) on each call.                                                | `internal/tools/dispatch.go:96-107`, `registry.go:127-180`     |
| `internal/hooks`              | Honor pre/post/error hook ordering; hooks may deny/narrow but cannot rewrite tool_id.                                                 | Implement `HookRunner` with explicit deny/annotate semantics; never tail event tables to fire hooks.            | `dispatch.go:245-405`, `internal/CLAUDE.md` "Hooks are typed dispatch" |
| `internal/observe`            | Emit `ToolCallStarted/Completed/Failed/Denied/Truncated` events with redacted input + result digest.                                 | Persist to event store before broadcasting; honor `after_seq` replay; index by correlation_id.                  | `dispatch.go:537-604`                                          |
| `internal/diagnostics`        | Use `RegisterDynamicSecret` for runtime tokens (claim_token, MCP tokens, OAuth codes).                                                | Provide regex-driven redaction layered with the result limiter.                                                 | `internal/diagnostics/redact.go` (via `internal/CLAUDE.md`)    |
| `internal/extension`          | Honor extension-supplied MCP server entries; carry `secret_env` validation; respect manifest precedence.                              | Provide validated MCPServer slice; never inject raw secrets into env at registration time.                       | `internal/extension/manifest.go:286-295,654,786,891-922`       |
| `internal/skills`             | Skills are not tools but use the same `ensurePathWithinRoot` invariant; skill-bound tools surface through the registry.              | Honor five-layer precedence; emit shadow events on collision.                                                    | `internal/skills/path_security.go:9-36`                        |
| `internal/store/globaldb`     | Tool ledger rows persist via the registry's `events` sink; tool process records persist via toolruntime store.                       | Update the global domain schema fragment, append Goose SQL via codegen, refresh `atlas.sum`/sqlc, and honor `agh-schema-migration`. | `toolruntime/registry.go:548-555` (UpsertProcessRecord)        |
| `internal/api/core`           | Expose tool list/get/search/call + tool ledger + tool processes through `BaseHandlers`.                                              | Share parsing/validation; HTTP and UDS choose registration only.                                                 | `internal/api/contract` types                                  |
| `internal/api/httpapi`        | Mount `/api/tools/*`, `/api/tool/processes`, `/api/tool/interrupt`, `/api/tool/ledger`.                                              | Honor SSE redaction; never broadcast raw input.                                                                  | `internal/api/httpapi/routes.go`                                 |
| `internal/api/udsapi`         | Mount UDS parity for the same surface; agents use UDS by default.                                                                    | Same.                                                                                                            | `internal/api/udsapi/routes.go`                                  |
| `internal/cli`                | `agh tool *` subcommands invoke the UDS client; `agh tool mcp` is the hosted-MCP stdio proxy entry.                                  | Use `commandDeps` injection for tests; never log raw tool input.                                                 | `internal/cli/...` (tool subcommand tree)                       |
| `internal/automation`         | Automation jobs that invoke tools dispatch through the same registry; `secret_env` resolution is identical.                          | Honor scheduled-tool detached lifetime via `context.WithoutCancel`.                                              | `internal/automation/...`                                       |
| `internal/network`            | Network turns may block writes (`acp/handlers.go:242` `ErrToolBlockedForNetworkTurn`); MCP calls during network turns are governed.   | Surface `ErrToolBlockedForNetworkTurn` cleanly.                                                                  | `internal/acp/handlers.go:241-242, 380-384`                    |

## 7. DX Cliffs

1. **Hosted MCP failure produces ambiguous tool errors.**
   - Symptom: when the hosted proxy's bind expires (e.g. clock skew, missed heartbeat), the agent sees a generic "tool unavailable" rather than "rebind required".
   - Repro: set `BindNonceTTL=1s`, sleep 2s, attempt hosted MCP call.
   - Fix surface: `internal/mcp/hosted.go:160-163` — surface a distinct reason code mapped through to the registry view (`ReasonBackendUnhealthy` is too generic).

2. **Tool ledger does not include the redacted input plaintext for diff-based debugging.**
   - Symptom: operator wants to know which key in the input was redacted, but `RedactedInputFields` only shows the JSON path.
   - Fix surface: add a `redaction_reasons` map to `dispatch.go:606-624`, mirroring the result-side `Redaction.Reason` (`internal/tools/result_limit.go`).

3. **Sandbox provider error messages don't always carry the resolved path.**
   - Symptom: `daytona/tar.go` and `acp/handlers.go` raise containment errors that don't include the resolved path that escaped — operator sees only the input.
   - Fix surface: include both `requested` and `resolved` in the wrapped error (`tar.go:286, 293, 307`).

4. **`http.DefaultClient` fallback in `daytona/ssh.go:88` is reachable in tests but never exercised in production.**
   - Symptom: a contributor copies the pattern thinking it is the standard.
   - Fix surface: replace with `errors.New("ssh access requires explicit http client")` or remove the fallback; tighten with a regression test pinning the production constructor's wiring.

5. **`agh tool ledger` paginated output truncates without a marker.**
   - Symptom: a long-running session produces hundreds of entries; the CLI truncates without indicating "more results available".
   - Fix surface: add a `next_after_seq` field in the JSON shape (parity with `internal/observe`).

6. **`Tool.execution.cleanup` is implicit on `Stop`, not on session crash.**
   - Symptom: kill -9 the daemon mid-tool; on restart, `ReconcileBoot` marks the process stale, but operator does not see a structured "tool was running when daemon died" record in `agh sessions transcript`.
   - Fix surface: surface stale-tool records as transcript entries, not just registry rows (`toolruntime/registry.go:276-283`).

7. **`PermissionMode` change mid-session has no observable confirmation.**
   - Symptom: operator switches a session from `approve-all` to `deny-all`; the next tool call respects the new mode but no event records the transition.
   - Fix surface: emit a `permission_mode_changed` event when `provider.permissionModeFor(req)` changes between calls (`internal/sandbox/local/provider.go:163-168`).

8. **External MCP timeout is global, not per-server.**
   - Symptom: a slow MCP server on one network blocks unrelated calls when callers share `executor.httpClient`.
   - Fix surface: per-server timeout overrides in config; `executor.go:142-150` already supports per-call timeout via context, but there is no per-server config knob.

9. **`fs/read_text_file` truncation policy is opaque to the agent.**
   - Symptom: agent reads a large file, `ReadTextFileResponse.Content` is sliced (`acp/handlers.go:226-235`), but the agent has no signal that the file was truncated.
   - Fix surface: add a `truncated:bool` field to the response or surface the truncation through a tool event.

## 8. Failure Modes QA Must Catch

If any of these slip, we ship a broken tool runtime.

1. **Raw secret literal in any tool ledger / SSE / agh.log entry.** `grep -E '\bsk-[A-Za-z0-9]{20,}\b'` and `grep -E '\bagh_claim_[A-Za-z0-9]+\b'` over all evidence in TOL-08, TOL-10, TOL-17 returns exit 1.
2. **Orphan tool subprocess after sessions stop.** TOL-16 ps grep returns "no orphans".
3. **Interrupt does not kill grandchild on Unix.** TOL-02 cleanup verifies the entire tree.
4. **Path containment bypass via null-byte / URL-encoded / Unicode / symlink.** TOL-04 / TOL-05 / TOL-06 / TOL-07 all reject and leave no artifacts.
5. **Hook deny does not short-circuit; provider-side process spawned anyway.** TOL-14 checks no `tool_call_started` for fs/write_text_file.
6. **Hosted MCP proxy outlives daemon.** TOL-09 proxy.after must be empty.
7. **External MCP call hangs.** TOL-12 elapsed must be within executor.timeout + slack.
8. **Concurrent dispatch loses a row.** TOL-16 proc.during.json must have 5 distinct PIDs.
9. **Conflicted tool callable.** TOL-13 lookup.json must show only one callable view.
10. **Result limiter applied only once after post-call hook rewrite.** Asserted via dispatch_test.go regression.
11. **`http.DefaultClient` reachable in production tool/MCP/sandbox path.** Code-search gate over `internal/tools internal/toolruntime internal/sandbox internal/mcp` returns zero non-test, non-fallback hits.
12. **Tool process group fails to track ProcessGroupID for ACP terminals.** `internal/acp/handlers.go:743` carries `ProcessGroupID = term.cmd.Process.Pid`; if regression sets it to 0, `signalRecord` (`interrupt.go:52-57`) falls back to single-PID signaling and grandchildren leak.
13. **Approval bridge missing crashes the daemon.** TOL gate must verify `ReasonApprovalUnreachable` is returned cleanly, not panic.
14. **MCP plugin disable does not propagate to the executor's server resolver.** TOL-11 p2 must fail; if it succeeds, the resolver is caching stale config.

## 9. Fixtures / Bootstrap Requirements

The QA harness for this child must:

- Use `agh-qa-bootstrap` with unique `AGH_HOME`, daemon HTTP/UDS ports, `tmux-bridge` socket, and `PROVIDER_HOME` / `PROVIDER_CODEX_HOME`. Default `:2123` is forbidden.
- Provide a real Claude Code agent path: `agh sessions start --agent claude-code --workspace ./fixtures/tol<NN>`.
- Provide a deterministic mock-ACP path for non-LLM scenarios (TOL-04, TOL-12, TOL-13): the existing `internal/acp/acpmock` test binary.
- Provide a stub MCP HTTP server for TOL-12 — Go test binary that sleeps 60s before responding, exposes a single tool `mcp__qa-stub__noop`.
- Provide fixture extensions:
  - `qa-mcp-fixture` (TOL-11) — exposes one MCP stdio server with `mcp__qa-mcp-fixture__echo`.
  - `qa-precedence-bundled` + `qa-precedence-user` (TOL-13) — both declare `mcp__github__search`.
- Provide hook scripts:
  - `qa-deny-write` (TOL-14) — pre_call returns `Decision{Callable:false}` for fs/write_text_file.
  - `qa-narrow-write` (TOL-15) — pre_call rewrites Input.path with allowlist annotation; rejects outside notes/.
- Provide fixture workspaces under `./fixtures/tol01..tol17/` with the AGENT.md content the scenario describes (calc.py for TOL-17, .env with synthetic key for TOL-08, etc.).
- Provide an artifact layout under `.artifacts/qa/<run-id>/tools-sandbox/tol-<NN>/`:
  - `qa-report.md` — Worked / Failed / Blocked / Follow-up.
  - `qa-summary.json` — machine-readable.
  - `qa-output.log` — combined stdout/stderr.
  - `qa-observed-events.json` — SSE events captured (redacted by default).
- Provide a **secret-leak grep gate** (`scripts/qa/secret-grep.sh`) that runs after each scenario over every captured artifact, checking for `\bsk-[A-Za-z0-9]{20,}\b`, `\bANTHROPIC_API_KEY=`, `\bagh_claim_[A-Za-z0-9]+\b`, `OPENAI_API_KEY=sk-`. Zero matches mandatory.
- Provide a **tool-event coverage matrix gate**: for each scenario, assert that every tool call emits exactly the canonical event sequence per `dispatch.go:40-93` (`tool_call_started` then either `tool_call_completed`/`tool_call_failed`/`tool_call_denied`, plus optional `tool_result_truncated`).
- Provide a **windows cross-build gate** specifically for `internal/toolruntime`, `internal/acp/process_tree_unix.go` vs `process_tree_windows.go`, `internal/sandbox/local`: `GOOS=windows GOARCH=amd64 go build ./internal/toolruntime/... ./internal/acp/... ./internal/sandbox/local/...`.
- Provide a **bakeoff order** for live runs: GPT (when supported) → Claude Code → Gemini (when supported); tool dispatch is provider-symmetric so most scenarios can run on Claude Code alone, but TOL-01, TOL-02, TOL-08, TOL-17 should be replayed against any second supported live agent.

## 10. Citations

- `internal/tools/dispatch.go:1-653` — canonical dispatch pipeline; pre/post hook contract; emit/redaction discipline; approval bridge contract.
- `internal/tools/registry.go:1-549` — RuntimeRegistry composition root; Provider validation; conflict detection (lines 268-289, 438-453); operator vs session projection.
- `internal/tools/policy.go:1-450+` — policy evaluator with deny/narrow rules; permission-mode constants.
- `internal/tools/policy_resolver.go` — scope-aware resolver overlay merging.
- `internal/tools/approval_token.go` — single-use TTL approval token store.
- `internal/tools/result_limit.go:1-200` — result byte cap + redaction (`redactedJSONValue = "[REDACTED]"`).
- `internal/tools/builtin/descriptors.go:1-108` — native_go descriptor surface; native MVP toolset list.
- `internal/tools/builtin_ids.go:1-220+` — canonical built-in tool IDs.
- `internal/toolruntime/registry.go:1-565` — process registry + interrupts; ReconcileBoot; in-memory + durable candidate union.
- `internal/toolruntime/interrupt.go:1-79` — defaultInterrupter SIGTERM(250ms)→SIGKILL(1s) with PID/start-time validation; process-group signaling when pgid > 0.
- `internal/toolruntime/types.go` — ProcessRecord / ProcessSource / ProcessOwner / InterruptScope / canonical states.
- `internal/sandbox/types.go:1-305` — provider-neutral types; ToolHost interface; permission ops + decisions.
- `internal/sandbox/registry.go:1-83` — provider registry with default-backend fallback.
- `internal/sandbox/local/provider.go:1-187` — daemon-host provider; ToolHost + Launcher wiring.
- `internal/sandbox/daytona/tar.go:1-330` — tar safety (path containment, null-byte/abs/traversal rejection, symlink containment, parent re-eval after MkdirAll).
- `internal/sandbox/daytona/ssh.go:75-135` — SSH access token request; production wires explicit http client; line 88 fallback flagged as DX cliff.
- `internal/sandbox/providertest/suite.go` — shared compliance suite.
- `internal/mcp/hosted.go:1-180+` — hosted MCP launch nonce, bind nonce TTL, projection digest, peer/binary validation; fail-closed errors.
- `internal/mcp/hosted_proxy.go:1-200+` — `agh tool mcp` stdio proxy lifecycle; release-on-error.
- `internal/mcp/executor.go:1-800+` — remote MCP CallExecutor; default 30s timeout; `*http.Client{Timeout: ...}` discipline; per-call deadline via context.
- `internal/mcp/peer.go:1-60` — UDS peer credential surface for hosted MCP.
- `internal/mcp/auth/service.go:1-60+` — OAuth metadata discovery + token store; explicit http client timeout.
- `internal/mcp/auth/pkce.go` — PKCE pair generation.
- `internal/skills/path_security.go:1-36` — `ensurePathWithinRoot` (skill containment; pattern parity for sandbox).
- `internal/extension/bundle.go:741-749` — extension bundle root EvalSymlinks.
- `internal/extension/install_managed.go:355-572` — managed-extension dependency copy path resolution.
- `internal/fileutil/atomic.go:1-86` — atomic write/replace/remove + dir sync.
- `internal/acp/client.go:296-331,594-624,640-701` — agent process registration; Cancel; Interrupt; Stop.
- `internal/acp/handlers.go:85-720+` — terminal manager; fs read/write handlers; permission gating; terminal subprocess registration.
- `internal/acp/process_tree_unix.go:1-30` — Unix process-tree signaling.
- `internal/acp/process_tree_windows.go:1-30` — Windows forced-exit fallback.
- `internal/CLAUDE.md` — backend rules; security invariants (claim_token redaction, symlink hardening, path security helpers, external-call timeouts, load-time security scan); concurrency invariants (Manager-WaitGroup, detached lifetime, subprocess managed-stop, process-group parity).
- `_references/openclaw-qa-patterns.md` — scenario shape (qa-scenario + qa-flow), four-artifact contract, real-LLM vs mock policy, tool-call evidence as pass criterion, forbidden-needle pattern (applied to secret grep).
- `_references/hermes-qa-patterns.md` — hermetic env discipline, async/cancel rigor (applies to interrupt scenarios), subprocess home isolation, secret redaction adversarial inputs.
- `CLAUDE.md` (root) — greenfield zero-legacy rule; subagent read-only; worktree-isolation; provider-home isolation; commit style; `make verify` gate; multi-LLM bakeoff pipeline.
