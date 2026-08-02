# Fresh Go 1.26 Modernization Synthesis

## Freshness, scope, and evidence standard

This synthesis replaces every earlier package conclusion after the 2026-08-01 `golang-master` update. It uses only the eight fresh slice artifacts in this directory plus direct checks against the current source tree and the installed Go 1.26.4 standard library. Earlier analysis artifacts were not used as evidence. The existing implementation diff was preserved, but no preserved change is treated as proven merely because it previously passed a gate.

The eight slices cover the requested tree exactly:

| Slice | Go files | Fresh artifact |
| --- | ---: | --- |
| Command, config, update, codegen | 636 | [`01_analysis_command-config.md`](01_analysis_command-config.md) |
| API, transports, SSE, window manager, identity | 823 | [`02_analysis_api-transports.md`](02_analysis_api-transports.md) |
| Daemon, subprocess, process utilities, support, test runtime | 732 | [`03_analysis_daemon-lifecycle.md`](03_analysis_daemon-lifecycle.md) |
| ACP, sessions, transcripts, workspaces, client state | 364 | [`04_analysis_sessions-workspaces.md`](04_analysis_sessions-workspaces.md) |
| Stores, memory, resources, settings, model catalog | 800 | [`05_analysis_persistence-memory.md`](05_analysis_persistence-memory.md) |
| Automation, task, loop, scheduler, observe, HEARTBEAT, SOUL | 587 | [`06_analysis_orchestration-domain.md`](06_analysis_orchestration-domain.md) |
| Extensions, bridges, bundles, hooks, marketplace | 643 | [`07_analysis_extensions-bridges.md`](07_analysis_extensions-bridges.md) |
| MCP, registry, tools, security boundaries, sandbox, Go SDK | 512 | [`08_analysis_tools-security-sdk.md`](08_analysis_tools-security-sdk.md) |
| **Total** | **5,097** | **All `cmd/**`, `internal/**`, and `sdk/go/**` Go files** |

The repository currently has 147 buildable Go packages across the root and `sdk/go` modules. The current Go 1.26 `modernize` analyzer reports zero automatic findings in both modules. That negative result is important: the remaining modernization work is semantic and ownership-sensitive, not a bulk linter rewrite. The `dupl` diagnostic reported 117 raw runtime candidates across 60 files and no SDK candidates; these remain hypotheses until one responsibility and one canonical owner can be proved.

## Executive conclusion

The codebase already uses a substantial portion of modern Go correctly. The restart does not justify a repository-wide syntax churn. The strongest opportunities fall into four semantic categories:

1. **Make authoritative transitions atomic.** Fire-limit evaluation must commit with reservation; task lease mutation must commit with task reconciliation and audit; notification/cache identities must include their actual scope.
2. **Make lifecycle ownership explicit and retryable.** Every owner must close admission, cancel, join, retain terminal state/result, and allow later callers to observe the same operation with their own context.
3. **Use capabilities at I/O boundaries.** Rooted filesystem operations, bounded readers, origin-bound credentials, and exactly-once response cleanup are correctness and security changes, not cosmetic API adoption.
4. **Apply modern APIs after the invariant is correct.** `WaitGroup.Go`, `OnceFunc`, `b.Loop`, `AsType`, typed dialing, sequence helpers, and `synctest` are valuable only where they preserve the owning state machine, benchmark contract, or I/O lifetime.

The implementation sequence must therefore start with critical atomicity/security/lifecycle defects, complete the already-preserved changes, then perform structural cleanup and low-risk language modernization. Generated contract files remain evidence-only and must change through their generator owner.

## Global disposition of the 20 requested features

| # | Feature | Repository-wide disposition | Governing constraint |
| ---: | --- | --- | --- |
| 1 | `errors.AsType[T]` | **Adopt selectively** | Convert ordinary typed extraction. Keep `errors.Is` for identity and ordinary `errors.As` where an addressable non-error interface target is genuinely required. |
| 2 | `b.Loop()` | **Adopt after removing `b.N`-dependent setup** | A benchmark must use either `Loop` or a `b.N` loop. The final residual benchmark was redesigned so setup no longer reads `b.N`, then converted without changing its measured prompt-history/event-drain invariant. |
| 3 | JSON `omitzero` | **Already adopted; contract-only additions** | Pointer, string, slice, nil/empty, and wire-absence semantics must not be changed mechanically. Contract/codegen changes co-ship through their owner. |
| 4 | `os.OpenRoot` / `os.Root` | **Adopt at proven filesystem authority boundaries** | Archive extraction, trusted-root config reads, memory-file reads, and rooted managed sidecar/install operations qualify. Keep stricter realpath/no-symlink product policies where they carry distinct semantics. |
| 5 | `SplitSeq`, `FieldsSeq`, `Lines` | **Already common; adopt remaining one-pass cases** | Retain materialization where code indexes, reverses, rejoins, preserves exact newline semantics, or returns a reusable slice. Prefer `strings.Cut` when only the first token is needed. |
| 6 | `sync.WaitGroup.Go` | **Adopt after admission is correct** | `Go` does not make `Add`/`Wait` ordering safe by itself. Close admission under the same lock before waiting; functions passed to `Go` must not panic. |
| 7 | Range over integer | **Adopt selectively** | Use for count-only loops or readable indexed loops. `b.Loop` owns benchmark iteration, and index-dependent mutation remains explicit where clearer. |
| 8 | `slices`, `maps`, `min`, `max` | **Already broad; finish exact semantic matches** | Preserve stable ordering, nil-versus-empty results, alias isolation, and comparator overflow safety. |
| 9 | `testing/synctest` | **Adopt for pure in-process time/concurrency** | Do not virtualize subprocesses, real sockets, filesystem watchers, SQLite contention, or other external systems. Those need observable readiness/completion barriers. |
| 10 | `iter.Seq` / range-over-function | **Internal-only, evidence-driven use; defer public rewrites** | `maps.Keys` pipelines are useful. Public/store list contracts retain materialized ordering, paging, resource cleanup, and error timing unless a profile and consumer contract justify a hard cut. |
| 11 | `os.Process.WithHandle` / `ErrNoHandle` | **Reject blanket migration; defer specialized handle operations** | `*os.Process.Signal` and `Kill` already use pidfd/Windows handles internally when available. `WithHandle` is for custom operations on the native handle; it does not replace process-group or Job Object ownership. |
| 12 | `OnceFunc`, `OnceValue`, `OnceValues` | **Adopt for exact once-actions/result caches** | Do not wrap context-sensitive shutdown or first-caller-context initialization. Repeated callers need independent wait contexts and a shared terminal result. |
| 13 | `math/rand/v2` | **Adopt only for existing non-cryptographic jitter** | Retry jitter in `internal/retry` and `internal/bridgesdk` qualifies. IDs, tokens, PKCE, Vault keys, claim tokens, and correctness-sensitive store jitter remain `crypto/rand`. |
| 14 | `cmp.Or` | **Narrow use only; otherwise reject/defer** | Arguments are eager and the operation is first non-zero, not trim/validate/provenance/lazy fallback. Use only on already-normalized, cheap values with exact semantics. |
| 15 | `T.ArtifactDir`, `T.Attr`, `T.Output` | **Adopt only where retained evidence is the test contract** | `t.TempDir` remains correct for disposable fixtures. Artifact retention requires a redaction/CI-consumer policy; structured attributes can replace stable metric logs without retaining secrets. |
| 16 | `http.CrossOriginProtection` | **Adopt at browser-reachable unsafe HTTP boundaries** | The preserved HTTP API middleware is valid. MCP/provider endpoints require reachability/protocol tests. Non-browser clients without origin/fetch metadata remain allowed by the standard primitive. Public webhooks and safe routes retain their distinct contracts. |
| 17 | `runtime/trace.FlightRecorder` | **Defer** | It needs a daemon-owned memory budget, trigger/dump behavior, redaction/retention policy, config lifecycle, and agent-manageable CLI/HTTP/UDS/native-tool surfaces. A library-local recorder would invert ownership. |
| 18 | Typed `DialUnix` / `DialTCP` | **Adopt at typed UDS and prevalidated IP endpoints** | Keep generic dial seams where they intentionally model policy injection, SSH, WebSocket, or arbitrary networks. Preserve DNS-to-dial validation and redirect policy. |
| 19 | `bytes.Buffer.Peek` | **Not applicable** | Current buffers are render/encode accumulators; framed protocols use `bufio.Reader`/`Scanner`. No current parser becomes safer or clearer with buffer lookahead. |
| 20 | `unique` | **Reject without heap-profile evidence** | Dynamic IDs and user-controlled strings are high-cardinality and sometimes sensitive. Interning would add process-lifetime retention and representation coupling. |

## Confirmed priorities

### P0 — authoritative correctness, security, and durable identity

1. **Automation fire-limit reservation is non-atomic.** `internal/automation/dispatch_reservation.go:97-124,168-235` locks only the read/count phase and performs `CreateRun` afterward. Concurrent dispatcher instances can both pass. Move the predicate plus insert/CAS into one store transaction and delete the process-local fire-limit mutex. Source: `06`, OD-01.
2. **Daemon shutdown drops ownership before teardown finishes.** `internal/daemon/daemon_shutdown_targets.go:53-102` copies and clears all live handles before cleanup. Concurrent shutdown can return early, a timed-out attempt cannot retry, and boot can observe an empty daemon while old resources remain live. Retain a durable `running/stopping/stopped` runtime state, shared completion, and terminal result. Source: `03`, R1.
3. **Windows Job Object ownership can leak and collide under PID reuse.** The global PID-keyed map is released only through group-wait paths; natural root exit can leave the Job handle/map entry live. Replace PID lookup ownership with a registration handle that the launched process closes on every terminal path. Source: `03`, R2.
4. **Task lease transitions can partially commit.** Claim, heartbeat, and release mutate durable lease state before task reconciliation and canonical audit recording. Completion already has the correct settlement pattern. Add transaction-owned claim/heartbeat/release settlements; raw claim tokens remain one-time return values and never enter settlements/events. Source: `06`, OD-02.
5. **Notification cursor and delivery identities are not injective across scope/workspace.** String concatenation aliases global with workspace `global`, colon-containing fields, and same event IDs across workspaces; delivery IDs omit workspace. Introduce one typed, canonical, collision-safe identity used for cursor, delivered, and skipped IDs. Source: `08`, F1.
6. **Provider pre-start cache identity omits the environment it classifies.** `HomePaths`, `CommandEnv`, and workspace/runtime-home identity are absent, while expired distinct keys are retained indefinitely. Move the cache into a bounded daemon-owned component and key it by explicit non-secret scope/home/environment identity. Source: `08`, F2.
7. **Registry/config/managed-install check-use paths remain path-raceable.** Registry archive extraction and config regular-file reads validate one pathname and later open another resolution of it. Extension managed installs have the same class of risk. Use trusted-root, relative, descriptor-owned operations; keep domain-specific containment diagnostics. Sources: `01` F05, `07` F14, `08` F4.
8. **Extension startup lacks a complete rollback transaction.** Capability grants and source-session state can survive a later startup failure. Register every inverse at acquisition time and settle activation only after all runtime/resource stages succeed. Source: `07`, F-01.

### P1 — lifecycle ownership, bounded cleanup, and adversarial I/O

1. **Converge owned runtimes on close-admission → cancel → join → terminal-result.** Confirmed mismatches span daemon model/task/scheduler/coordinator workers; HTTP serving; automation scheduler; observe retention; session/ACP prompt and watcher tasks; recall/deadentity; marketplace; bridges broker/provider lifecycle; extension SDK callbacks; and SSE reader cancellation. Sources: `02` F-01/F-03, `03` R3/R5/R8, `04` R1-R4, `05` PM-03/PM-04, `06` OD-03/OD-04, `07` F-02/F-03/F-06–F-09, `08` F9.
2. **Complete response/file/row cleanup with one owner and bounded reads.** Update/CLI, registry GitHub/HTTP archive, MCP auth, support bundles, skill provenance, sidecar pipes, tests, and SQLite rows contain missed drain/close/join paths. A primary error never hides cleanup failure; untrusted bodies are bounded before allocation and bounded when drained. Sources: `01` F02/F03, `03` R4/R7, `05` PM-05, `07` F-05/F-10, `08` F7/F11.
3. **Stream git-source archives instead of allocating before the limit.** Clone/archive traversal needs file-count, uncompressed, compressed, context, disk, and cleanup bounds before materialization. Source: `08`, F5.
4. **Unify outbound destination policy and bind credentials to origins.** Registry HTTP/GitHub paths do not match MCP's DNS-to-dial/private-network/redirect policy, and GitHub tokens can be attached to response-provided URLs without an origin allowlist. Source: `08`, F6.
5. **Bind Vault ciphertext to canonical ownership metadata with AEAD AAD.** Ref/kind substitution must fail authentication. This is a greenfield hard cut with no fallback decrypt path. Source: `08`, F8.
6. **Make mutable capability/config snapshots private and cloned.** `AgentProcess.Caps` can bypass its mutex; workspace owns a field-by-field config clone that can silently miss new fields; API operation response variants are not deep-cloned. Sources: `02` F-02, `04` R7/R8.
7. **Remove entropy downgrade fallbacks, entropy panics, and unchecked numeric narrowing.** Session/inputqueue, workspace-resolver, workspace-identity, and store-wide ID generation must propagate `crypto/rand` failure; durable `RunGeneration` remains `int64` end to end; external duration/seconds conversions validate range before conversion. Sources: `01` F07, `04` R6/R14/R19/R20/R21, `07` F-15.

### P2 — architectural debt and greenfield deletion

1. Move narrow consumer interfaces to command/runtime consumers instead of passing 18- or 213-method facades. Keep broad composition capability surfaces only at composition roots. Sources: `01` F09, `03` R11.
2. Consolidate exact path-security mechanics into named low-level policies without flattening distinct realpath, deepest-existing, no-symlink, and root-extraction rules. Sources: `04` R12, `06` OD-05, `08` F13.
3. Delete explicit legacy repair after a repository-wide delete-target audit. `session.RepairLegacyProvider` and observe's repair/skip behavior violate zero-legacy policy. `situation.Service.PromptSection` is not locally dead: it is still required by the `session.PromptProvider` type even though dispatch prefers `PromptStartupSection`; removing it requires an atomic descriptor/interface split. Source: `06`, OD-06, corrected by direct repository reference analysis.
4. Inline the private straight-line coordinator FSM but retain `internal/loop/watch`'s observable branching FSM. Source: `06`, OD-07.
5. Split only proven multi-responsibility/near-cap owners when touched: task wake, event registry, daemon runtime state, support builder phases, session lifecycle state holders, exact-500 tools schema. Do not add structure-only tests. Sources: `03` R10, `04` R17, `06` OD-08, `08` F15.
6. Remove proven dead or test-only production seams only after whole-repository references are checked: impossible `doctor.NewRunner` error, transcript helper used only by tests, unreachable post-`os.Exit` return, and contract wrapper candidates. Fresh function-value tracing proved the workspace ID helper is live resolver behavior, so it belongs to error propagation rather than deletion. Sources: `01` F10, `03` R12, `04` R14/R15/R20, `07` F-12.

### P3 — mechanical modernization and canonical test cleanup

- The residual `internal/extensiontest/perf_bench_test.go` benchmark has been redesigned and converted: setup no longer preallocates from `b.N`, while every measured iteration still appends one real prompt and drains its emitted events.
- Apply `errors.AsType`, `WaitGroup.Go`, `OnceFunc/OnceValues`, string sequences, `slices.Sorted(maps.Keys(...))`, typed dialing, `rand/v2` jitter, and safe comparator changes only in their owning files.
- Replace pure in-process timer/poll tests with `synctest` or explicit channels. Replace integration sleeps with readiness/completion protocols.
- Consolidate cleanup helpers inside canonical suites. The fresh slices identified hundreds of discarded test cleanup errors; tests must fail or report when cleanup fails rather than use `_ =`.
- Split giant canonical test files by behavioral responsibility only when editing those suites; one invariant retains one canonical owner.

## Corrections and divergences resolved by the parent audit

The slice artifacts are evidence reports, not unquestioned implementation instructions. Direct standard-library and call-site checks resolved these divergences:

1. **Resolved `b.Loop` precondition:** `internal/extensiontest/perf_bench_test.go` originally preallocated with `b.N`, so a mechanical conversion would have violated Go 1.26's benchmark contract. The implementation first removed that setup dependency, preserving the measured prompt-history/event-drain work, and only then adopted `b.Loop`.
2. **No blanket `WithHandle` migration:** launched `*os.Process` values already retain and use pidfd/Windows handles internally when available. `WithHandle` is relevant only for custom native-handle operations and does not own Unix PGIDs, Windows Jobs, or recovered PID/start-time records.
3. **Active coalescer has no proven wait-group race:** `Add` occurs under the same mutex that sets `closed`, and `Wait` begins only after admission closes. `WaitGroup.Go` is a clarity change, not a correctness repair.
4. **Situation prompt compatibility is active through typing:** production prefers `PromptStartupSection`, but descriptors are typed as `session.PromptProvider`, which still requires `PromptSection`. Deletion must redesign the descriptor interface in the same hard cut.
5. **Snapshot literals serve two different roles:** production diagnostics should format `SnapshotVersion`; selected test literals should remain literal when they intentionally prove the external version contract rather than follow implementation automatically.
6. **Session DB identity is a boundary-design decision, not yet a proven cross-workspace exploit.** Production paths are normally derived from the session owner, but the database file cannot attest the supplied session/workspace identity. Treat persisted attestation as a workspace-isolation architecture task with schema/migration evidence, not an isolated quick patch.
7. **Timing constants are not automatically config keys.** Inject constructor/runtime options for deterministic ownership first. Add `config.toml` only for behavior operators must manage, then co-ship CLI/HTTP/UDS/defaults/docs/skill lifecycle.
8. **Testing artifact APIs are not a replacement for `t.TempDir`.** Adopt them only for evidence intentionally retained and governed by redaction and CI-consumer policy.

## Fresh revalidation of the preserved implementation diff

| Preserved change | Fresh status | Required follow-up |
| --- | --- | --- |
| SDK bounded newline framing | **Keep — validated** | The incremental `ReadSlice` loop rejects an unterminated oversized frame before unbounded allocation and preserves partial EOF behavior. Complete SDK-owned callback/request lifecycle separately. |
| HTTP Host/CORS/CSRF protection | **Keep — validated** | `CrossOriginProtection` plus bound-host validation closes the same-origin Host/DNS-rebinding assumption while preserving non-browser clients. Perform route audit and user-visible QA before completion. |
| Daytona `os.Root` tar extraction | **Keep — validated** | Rooted operations close archive escape races; preserve archive limits and strict symlink-overwrite policy. |
| Automation retry-delay overflow check | **Keep — validated** | Range check preserves backoff semantics and rejects unrepresentable delays. |
| Subprocess handler join before completion | **Keep — validated** | Reader completion closes admission before `handlerWG.Wait`; active accepted handlers now settle before process completion. |
| Support service admission/cancel/join | **Keep and complete** | Lifecycle gate is sound, but builder early failures still leave tar/gzip/file owners open and same-second operations still collide on `.tmp`/final names. The daemon shutdown owner also still detaches all runtime state prematurely. |
| Hook subprocess bounded TERM/kill/wait and wrapper consolidation | **Keep and complete** | Bounded process join and shared `procutil` owner are valid. Registry completion still uses unbounded background context, and current tests use wall time for an in-process timeout invariant. |
| RuntimeHarness retryable stop | **Keep with focused review** | Caching only terminal completion fixes the canceled-attempt bug. Verify concurrent callers honor their own contexts rather than blocking behind a long attempt; this is test infrastructure, not daemon lifecycle authority. |
| SQLite/file/resource cleanup error joining | **Keep — validated direction** | Named-result joins correctly preserve close/remove errors. Finish independent close joining in helpers such as `writeTempFile`; add no static implementation tests. |
| Shared fileutil temp cleanup | **Keep and complete** | Temp removal failure is now observable. `writeTempFile` still suppresses close failure when write/chmod/sync already failed; rooted atomic operations are still needed for managed sidecar boundaries. |

## Implementation waves and canonical test ownership

The implementation should proceed in reviewable semantic waves. Each test change must name the invariant, owning layer, and existing canonical suite before editing.

| Wave | Invariant and owner | Canonical evidence |
| --- | --- | --- |
| 1A | At most the configured automation fires commit in one window; durable automation run store owns atomic reservation | GlobalDB automation integration suite for transaction/concurrency; existing automation dispatch suite for service error/retry/reserved-run shape |
| 1B | Lease mutation, reconciled task projection, and canonical audit commit together; task store owns settlement | Existing store/task settlement integration; `internal/task/lease_test.go` for raw-token, fencing, hook, and network cleanup behavior |
| 1C | Daemon/process-tree shutdown has one retained operation/result; daemon and procutil own it | Existing daemon lifecycle suite; procutil Unix/Windows platform suites for natural exit, descendants, timeout, and registration release |
| 1D | Workspace/scope tuples produce distinct notification/cache identities | `internal/notifications/presets/match_test.go`; `internal/providers/prestart_test.go` |
| 1E | Archive/config/install operations cannot escape or race the trusted root | Existing registry extraction, config file I/O, extension install, and Daytona extraction suites; one owner per boundary |
| 2A | HTTP serve error and restart exclusion remain observable through shutdown | Existing HTTP server lifecycle suite, using UDS lifecycle behavior as the reference |
| 2B | API spec results are graph-isolated across calls | `internal/api/spec/operations_refac_test.go`, extended for outer and nested `Bodies` mutation |
| 2C | SSE cancellation joins close exactly once and preserves cancellation plus close failures | Existing `internal/sse/decode_context_test.go` / decoder suite |
| 2D | Accepted support/hook/marketplace/bridge/SDK work is rejected after close and joined before completion | Each component's existing lifecycle suite; no cross-package generic lifecycle test |
| 2E | Every HTTP/file/row response is bounded, drained where appropriate, and closed exactly once | Each owning client/store suite; test helpers remain local to canonical integration suites |
| 3 | Structural extraction, legacy deletion, and narrow interfaces preserve behavior | Existing behavior suites plus build/boundary/codegen gates; no filename, line-count, config snapshot, or prose tests |
| 4 | Modern API substitutions preserve semantics | Existing package suites and benchmarks; use `synctest` only for pure in-process clocks |

## Open decisions that affect design but do not block the completed analysis

1. Whether the same `*Daemon` instance supports boot after successful shutdown or must reject it deterministically.
2. Whether descendants are always terminated/awaited when the direct child exits naturally, and how that composes with Unix groups and Windows Jobs.
3. Which concrete task/automation store interfaces can own the new transaction commands and whether job/trigger IDs require explicit workspace predicates.
4. Which canonical workspace/runtime-home identity is available to provider pre-start caching without hashing plaintext secrets.
5. Whether provider HTTP and loopback MCP HTTP are browser-reachable, which trusted origins they support, and which handlers are safe methods in practice.
6. Whether registry git sources intentionally support local paths, `file://`, SSH, and SCP syntax or should be restricted to an explicit remote policy.
7. Whether `FlightRecorder` is always-on bounded diagnostics, on-demand support evidence, or disabled unless configured.
8. Which integration metrics/artifacts are safe and useful to retain in CI after redaction.

## Compozy Impact Audit

- **Native tools:** The analysis changes no tool ID, descriptor, schema digest, risk flag, or capability gate. Implementation waves affecting automation dispatch, task leases, notifications, registry/marketplace, MCP, support bundles, process status, or error reasons must audit their native tool executors/descriptors and CLI/API fallbacks. Internal lifecycle refactors alone require no descriptor change, but changed terminal timing or reason codes are public behavior.
- **Extensibility and hooks:** Direct impact exists for extension activation rollback, hook subprocess completion, bridge broker/provider lifecycle, bundle compensation, marketplace refresh, public Go SDK `OnReady`, MCP auth/transport, provider probes, registry sources, and managed skill installation. Preserve typed hook payloads, post-commit ordering, capability snapshots, resource/bundle registries, codegen ownership, and scaffold/reference-provider contracts.
- **Workspace data isolation:** Direct impact exists for notification cursor/delivery identity, provider pre-start cache keys, session DB owner attestation, tool artifacts, task/automation transaction predicates, and filesystem roots. Every changed datum must be classified global/workspace/session/agent-scoped and proven through store keys, caches, events/SSE, CLI/HTTP/UDS/core propagation, and foreign-workspace absence behavior.
- **Official Compozy skill:** Analysis-only artifacts require no skill change. Update `skills/compozy/` when implementation changes public CLI/API/native-tool behavior, tool IDs, MCP auth diagnostics, hook events, capability/bundle/resource semantics, provider lifecycle, marketplace errors, SDK lifecycle guarantees, config keys, or memory/network/task behavior.

## Completion boundary

This document completes the restarted analysis and supersedes earlier conclusions. It does not claim implementation completion. Completion still requires implementing the confirmed waves, revalidating every preserved change, handling user-visible QA scenarios, running deslop and final review, and obtaining one current `make gate-full` record after the final mutation.
