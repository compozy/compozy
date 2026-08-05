# Analysis: command-config

**Scope question:** Re-audit the command/config slice from first principles under the updated `golang-master` doctrine: identify correctness, resource-lifetime, concurrency, context, safety, interface, duplication, dead-code, test, benchmark, and Go 1.21–1.26 modernization opportunities.

**Primary sources:** Every Go production file, test, and benchmark under `cmd/**`, `internal/cli/**`, `internal/codegen/**`, `internal/config/**`, `internal/update/**`, `internal/version/**`, `internal/doctor/**`, `internal/demoseed/**`, `internal/e2elane/**`, `internal/logger/**`, `internal/frontmatter/**`, `internal/listcursor/**`, and `internal/workref/**`; the updated Go and Compozy engineering doctrine; and `/home/pedronauck/.codex/attachments/36398ca5-d255-4ce7-92d3-0d3429f3838f/pasted-text-1.txt:1-65`.

**Coverage mode:** Exhaustive source inventory plus full-scope lexical/structural scans across 636 Go files in 23 package directories (162,085 lines). Thirty high-signal sources were read end-to-end; the remaining 606 files were inspected through package-wide searches and candidate-driven excerpts. This is a full static surface audit, not a claim that all 162,085 lines received equal semantic depth. No prior analysis artifact was read.

**Candidate count:** 14 evidence-backed findings (4 high, 5 medium, 5 low) plus 20 explicit feature decisions. No production code, test, generator, gate, or benchmark was executed because this dispatch was read-only except for this analysis file.

## Overview

The slice is already substantially modern: it uses `errors.AsType`, `b.Loop`, `omitzero`, `os.OpenRoot`, sequence-based string APIs, `WaitGroup.Go`, range-over-int, and `slices`/`maps`. The updated doctrine nevertheless changes the priority order. Correctness and lifetime issues must precede cosmetic modernization.

The four high-severity items are:

1. `internal/doctor/doctor.go:105-125` can return after a probe timeout while the spawned probe goroutine remains alive indefinitely if the probe does not cooperate with cancellation.
2. HTTP response bodies and filesystem cleanup errors are not consistently drained, propagated, or joined in `internal/update/github.go:100-188`, `internal/update/apply.go:19-50`, `internal/update/manager.go:186-194`, `internal/cli/client_loops.go:128-152`, and `internal/cli/loop_support.go:310-319`.
3. The canonical CLI integration suite contains 18 duplicated daemon-stop/wait cleanup pairs that discard both errors, beginning at `internal/cli/cli_integration_test.go:230-237`, plus additional discarded cleanup errors in CLI tests.
4. `internal/config/file_io.go:11-46` validates a pathname with `Lstat` and later opens it with `ReadFile`, leaving a symlink-swap TOCTOU window that the already-used `os.Root` pattern can close.

The primary architectural pressure is in `internal/cli`: 376 Go files and 109,569 lines, including a 213-method transport interface at `internal/cli/client.go:26-470`. The interface can remain a root composition facade, but command consumers should not accept all 213 methods when they use one to three. `loopCommandClient` and `automationClientAPI` repeat the same issue with 18 methods each (`internal/cli/loop_support.go:23-81`, `internal/cli/automation_client_api.go:13-48`).

No production source exceeds the project's 500-line cap. Seven are at or above 480 lines, so any change to them must extract responsibility rather than append: `internal/config/automation.go` (495), `internal/cli/client_extensions_bridges.go` (494), `internal/cli/hooks.go` (491), `internal/cli/install.go` (485), `internal/cli/skill_output.go` (482), `internal/config/hooks.go` (481), and `internal/cli/skill_marketplace.go` (480). This is a growth risk, not a present violation.

The dead-code scan found no unexported production package-level function whose identifier appears only at its declaration. That negative result is lexical, not a whole-program call graph. One removable dead contract was verified: `doctor.NewRunner` returns an error even though it has no error path (`internal/doctor/doctor.go:54-59`). Exported dead-code deletion remains a whole-repository question.

## Mechanisms / Patterns

The matrix applies the 20 features from the supplied Go 1.26 review to this slice. Status values are deliberately scoped: `adopt` means a concrete candidate exists here; `already` means the relevant scoped idiom is already modern; `defer` means the owning implementation lies outside this slice or needs a cross-package design; `reject` means the feature is a poor fit; and `not applicable` means no matching responsibility or use exists here.

| # | Feature | Status | Scoped evidence and decision |
|---:|---|---|---|
| 1 | `errors.AsType[T]` | **adopt** | The slice has 13 modern calls, for example `internal/config/dotenv.go:81` and `internal/cli/tool_errors.go:49-72`, but 20 legacy `errors.As` calls remain in 12 files. Convert the five production sites at `internal/cli/root.go:366`, `internal/cli/extension_install.go:84`, `internal/cli/workspace_resolution.go:272-276`, and `internal/cli/session_event_output.go:178`, then update tests where the target type/interface satisfies the generic constraint. |
| 2 | `b.Loop()` | **adopt** | Six benchmark files already contain 16 `b.Loop` calls. The only two `b.N` loops are `internal/workref/ref_bench_test.go:23-27` and `:42-46`; replace them and remove sink/`runtime.KeepAlive` ceremony that `b.Loop` makes unnecessary. |
| 3 | JSON `omitzero` | **already** | `internal/config/agent.go:24-30` correctly uses `omitzero` for the value-typed `AgentSkillsConfig`, and the TypeScript generator recognizes it at `internal/codegen/sdkts/generate.go:243-259` with coverage at `internal/codegen/sdkts/generate_test.go:218-224`. Do not bulk-replace scalar/slice `omitempty`; zero and empty have different wire semantics. |
| 4 | `os.OpenRoot` / rooted filesystem access | **adopt** | Rooted access already exists at `internal/config/agent_delete.go:87` and in code generators. Apply it to the validation/open race in `internal/config/file_io.go:17-42`, using a trusted owner root plus a relative path and validating the opened descriptor. Creating a root from the untrusted path's own directory would not establish a security boundary. |
| 5 | `strings.SplitSeq` / `FieldsSeq` / `Lines` | **already** | Nine sequence uses exist in eight files, including `internal/update/manager_archive.go:26-34` and `internal/config/agent.go:419-422`; no remaining `for range strings.Split(...)` materialization was found. `internal/update/release_track.go:22-38` only needs the first token and should use `strings.Cut`, not a sequence. |
| 6 | `sync.WaitGroup.Go` | **already** | Both scoped `WaitGroup` declarations already use `Go`: `internal/config/agent_test.go:388-390` and `internal/cli/daemon.go:169-175`. No manual `Add`/`Done` pair remains in scope. |
| 7 | Range over integer | **already** | The idiom is present at `internal/cli/window_manager_test.go:307-310`. Remaining C-style loops need or mutate their index, including `internal/config/persistence_document_ranges.go:189` and `:235`, reflection field/index loops at `internal/config/tool_surface_reflection.go:49` and `:115`, and CLI argument parsing. The two benchmark loops are better replaced by `b.Loop`. |
| 8 | `slices` / `maps` / builtin `min` and `max` | **adopt** | The scope has 85 modern uses in 46 files, but 42 production `sort.Strings`/`sort.Slice`-family calls remain in 36 files, e.g. `cmd/compozy-codegen/sdk_go_contracts.go:69-75` and `internal/cli/config_display.go:139-145`. Replace simple sorts with `slices.Sort`/`SortFunc`; replace manual map-key collection with `slices.Sorted(maps.Keys(m))`. |
| 9 | `testing/synctest` | **adopt** | `internal/version/version_test.go:32-48` starts a goroutine and waits on a real 250 ms timer solely to detect blocking; it is a direct `synctest.Test` candidate. The three sleeps in `internal/cli/cli_integration_test.go:993-1018`, `:4192-4203`, and `:4348-4358` observe integration boundaries and need explicit observable completion/coordination, not virtual time. |
| 10 | `iter.Seq` / range-over-function | **adopt** | There are no direct `iter` APIs in scope. Adopt iterators internally where they remove materialization boilerplate, starting with `internal/doctor/doctor.go:246-261` as `slices.Sorted(maps.Keys(r.probes))`. Do not change public list-return contracts without consumer evidence. |
| 11 | `os.Process.WithHandle` / `ErrNoHandle` | **defer** | CLI process operations are injected from `internal/cli/root_defaults.go:77-84` and delegated to `internal/procutil`; tests already defend against PID reuse at `internal/cli/daemon_wait_test.go:254-281` and `internal/cli/lifecycle_test.go:370-405`. A stable-handle primitive belongs in `procutil` and must be designed cross-platform before CLI call sites change. |
| 12 | `sync.OnceFunc` / `OnceValue` / `OnceValues` | **adopt** | `internal/version/version.go:39-52` is an exact `sync.OnceFunc` candidate for its idempotent restore closure. Do not mechanically convert `internal/update/types.go:178-183` / `internal/update/detect.go:15-22`: that cache currently captures the first caller's context and needs a context-independent initialization design first. |
| 13 | `math/rand/v2` | **not applicable** | No `math/rand` or `math/rand/v2` use exists in the 636-file scope. Detection order is deterministic rather than randomized, for example `internal/update/detect.go:138-167`; introducing randomness would be counterproductive. |
| 14 | `cmp.Or` | **adopt** | Use for cheap, side-effect-free fallback expressions such as permissions and command resolution at `internal/config/provider_resolve.go:78-86`. Do not collapse the source-tracking branches at `:120-170`, which update provenance in addition to choosing a value, and avoid eager fallback evaluation where it is expensive. |
| 15 | `T.ArtifactDir` / `T.Attr` / `T.Output` | **not applicable** | No scoped test retains diagnostic artifacts or emits test metadata. Existing temporary directories are fixtures/scratch that should remain `t.TempDir`, e.g. `cmd/compozy-codegen/openapi_temp_storage_test.go:11-41`. Add artifact APIs only when retained failure evidence becomes an explicit test contract. |
| 16 | `http.CrossOriginProtection` | **defer** | `internal/cli/mcp_serve.go:100-117` delegates HTTP serving to `internal/mcp`, outside this slice. The origin policy must be audited and installed at the handler/server owner, not wrapped speculatively in the CLI. The WebSocket HTTP server at `internal/cli/window_manager_test.go:181-190` is test-only. |
| 17 | `runtime/trace.FlightRecorder` | **not applicable** | There is no trace recorder/import in scope. `internal/doctor/doctor.go:62-102` is a bounded per-command diagnostic runner, not the long-lived daemon observation owner; flight recording belongs to a daemon/observability slice if operational requirements justify it. |
| 18 | Typed `DialUnix` / `DialTCP` | **adopt** | Replace the two raw UDS `net.Dialer.DialContext` callbacks at `internal/cli/client_settings_vault.go:35-39` and `internal/cli/client_window_manager_stream.go:38-43` with the typed Unix dial API supported by the repository baseline. Keep the Gorilla WebSocket dial at `internal/cli/client_window_manager_stream.go:52-59`; it is a different abstraction. |
| 19 | `bytes.Buffer.Peek` | **reject** | Scoped buffers are write-side render/generation buffers, while interactive input uses complete-line readers (`internal/cli/bridge_setup_wizard.go:55-57`, `internal/cli/support.go:135-136`, `internal/cli/mcp_auth_flow.go:182-183`). No framing/lookahead parser would become clearer with `Peek`. |
| 20 | `unique` interning | **reject** | No repeated, long-lived high-cardinality value set has profile evidence. Small reference values such as `internal/workref/ref.go:7-15` are short structs; interning would add process-lifetime retention and obscure ownership without a measured benefit. |

The governing patterns from the updated doctrine are concrete here:

- Structured concurrency requires an owner that can stop and wait for every goroutine (`.agents/skills/golang-master/references/concurrency.md:3-13`). A timeout around an in-process goroutine does not make non-cooperative work stoppable.
- Context begins at entry points and is propagated; mid-path `context.Background()` silently detaches work (`.agents/skills/golang-master/references/context.md:9-21`).
- Cleanup errors are independent failures and are joined; HTTP responses are drained on all statuses before close (`.agents/skills/eng/eng-cleanup-failure-paths/SKILL.md:28-38`, `.agents/skills/eng/eng-cleanup-failure-paths/references/cleanup-table.md:15-18`, `:45-48`).
- Consumer-owned interfaces should normally expose one to three methods (`.agents/skills/golang-master/references/interfaces-generics.md:3-20`).
- External numeric narrowing is bounds-checked before conversion (`.agents/skills/golang-master/references/safety.md:80-83`).
- Modernization priority is correctness/safety, then readability, then gradual test/benchmark cleanup (`.agents/skills/golang-master/references/modernize.md:38-44`).

## Relevant Sources

| ID | Finding | Evidence | Severity / confidence | Fowler refactoring technique and recommended direction |
|---|---|---|---|---|
| F01 | Probe timeout does not own goroutine lifetime | `internal/doctor/doctor.go:105-125` launches `probe.Run` in a goroutine, selects the timeout, and returns without joining. A probe that ignores `probeCtx` survives the result. | **High / High** | **Substitute Algorithm**: if probes are trusted and context-cooperative, run synchronously under the timeout. If hard timeouts are a contract, **Extract Class** into a subprocess/supervisor boundary whose owner can terminate and wait. A goroutine cannot be force-stopped safely. |
| F02 | Cleanup and HTTP-drain failures are lost | `internal/update/github.go:100-115` and `:149-188` close only and keep cleanup errors only when the primary error is nil; non-OK bodies are not drained. The same primary-error replacement appears at `internal/update/manager.go:186-194`, `internal/cli/client_loops.go:128-152`, and `internal/cli/loop_support.go:310-319`. `internal/update/apply.go:19-24` and `:45-50` silently discard file-close errors. `internal/update/manager_archive.go:39-78` has comments justifying three discarded closes, but the updated cleanup contract requires propagation/join. | **High / High** | **Extract Function** / **Move Statements into Function**: create package-local drain-and-close and cleanup-join helpers; preserve the primary error with `errors.Join` rather than overwriting or suppressing it. Keep helpers local to each ownership boundary instead of creating a generic cross-package utility. |
| F03 | Canonical tests discard shutdown/cleanup errors | Eighteen identical pairs in `internal/cli/cli_integration_test.go` discard daemon-stop and runner-wait errors: `:235-236`, `:448-449`, `:717-718`, `:1034-1035`, `:1351-1352`, `:1425-1426`, `:1506-1507`, `:1578-1579`, `:1632-1633`, `:1683-1684`, `:1839-1840`, `:1893-1894`, `:1952-1953`, `:2013-2014`, `:2107-2108`, `:2260-2261`, `:2446-2447`, and `:2755-2756`. `:4294-4300` also discards `RemoveAll`. Additional confirmed error discards occur at `internal/cli/client_test.go:3659`, `:3671`, `:3874`, `:3882`, `:4102`; `internal/cli/render_test.go:240`; and `internal/cli/skill_test.go:3036`, `:3063`, `:3084`. | **High / High** | **Extract Function**: add one harness-owned `t.Cleanup` helper in the existing canonical integration suite, attempt both stop and wait, join/report all failures, and reuse it. Do not create duplicate standalone regressions. The correctly handled cleanup at `internal/cli/cli_integration_test.go:160-172` is the local model. |
| F04 | Cancellation is detached or nil contexts are silently normalized | Production has 11 `context.Background()` calls; only `cmd/compozy/main.go:15` and `cmd/compozy-codegen/main.go:38` are process entry points. Suspect mid-path uses are `cmd/compozy-catalog/validate.go:60`, `internal/update/detect.go:15-22`, `internal/cli/daemon_process.go:20-21`, `internal/cli/daemon_status.go:24-32`, `internal/cli/daemon.go:214-230`, `internal/cli/support.go:202-219`, `internal/cli/update.go:349-364`, and `internal/cli/loop_support.go:103-113` / `:146-151`. | **Medium / High** | **Change Function Declaration**: require a non-nil `ctx` as the first parameter, thread command/root context through parse/validation helpers, and create deadlines without detaching. Let nil be a caller bug rather than silently converting it to immortal work. |
| F05 | Regular-file validation has a TOCTOU symlink race | `internal/config/file_io.go:11-46` performs `os.Lstat(path)`, validates mode, then independently calls `os.ReadFile(path)`. The pathname can change between checks. Rooted access is already demonstrated at `internal/config/agent_delete.go:80-103`. | **High / High** | **Substitute Algorithm**: resolve from a trusted owner root, open the relative path once, inspect the opened descriptor, and read that same descriptor. Preserve optional-not-found semantics without reintroducing path-based validation. |
| F06 | Reflection-based config projection lacks the required adjacent justification | `internal/config/tool_surface_reflection.go:13-31`, `:33-43`, `:46-83`, `:86-110`, and `:113-142` use reflection throughout a public tool-surface projection. The project rule requires an adjacent written performance justification and lint reason for genuinely required reflection (`.agents/skills/eng/eng-code-guidelines/SKILL.md:52-55`, `.agents/skills/eng/eng-code-guidelines/references/coding-style.md:16-19`). | **Medium / High** | **Substitute Algorithm**: prefer typed projection or generated field walkers from the owning config schema. If tag-driven reflection is the deliberate decoder boundary, document why it is required adjacent to the entry point and benchmark the representative config shape before claiming a performance exception. |
| F07 | External duration conversions can overflow or narrow silently | `internal/cli/scheduler.go:284-299` parses an external duration and returns `int(timeout / time.Second)` without checking target width. `internal/cli/extension_output.go:204-222` multiplies external `int64` seconds by `time.Second`, which can overflow before it is formatted, then converts derived values to `int`. | **Medium / High** | **Substitute Algorithm**: validate the seconds against the target integer range before conversion; format uptime with checked `int64` division/modulo rather than overflowing through `time.Duration`. Return validation errors where input is user-controlled. |
| F08 | Four polling loops duplicate lifecycle policy | `internal/cli/daemon_status.go:19-55`, `internal/cli/support.go:202-249`, `internal/cli/update.go:349-389`, and `internal/cli/daemon.go:214-267` repeat nil-context normalization, default deadline selection, ticker ownership, cancellation selection, and terminal-state handling. | **Medium / High** | **Extract Function**: introduce a small context-safe polling skeleton that owns deadline/ticker lifecycle and accepts a domain callback that returns continue/success/failure. Preserve immediate-vs-first-tick behavior explicitly; do not hide domain status classification in a generic reflection-based helper. |
| F09 | Command consumers depend on oversized provider interfaces | `DaemonClient` spans `internal/cli/client.go:26-470` with 213 methods; `loopCommandClient` has 18 at `internal/cli/loop_support.go:23-81`; `automationClientAPI` has 18 at `internal/cli/automation_client_api.go:13-48`. Many command helpers accept `DaemonClient` directly, e.g. `internal/cli/support.go:202-205` and `internal/cli/update.go:349-353`, forcing test doubles such as `internal/cli/helpers_test.go:465` to satisfy the entire transport. | **Medium / High** | **Extract Class** plus **Change Function Declaration** (Go adaptation: consumer-owned micro-interfaces): keep one root composition facade if useful, but let each command/function accept the one-to-three methods it consumes. Compose domain interfaces only at wiring boundaries. |
| F10 | `doctor.NewRunner` exposes an impossible error path | `internal/doctor/doctor.go:54-59` always returns `&Runner{...}, nil`; callers and tests nevertheless carry error handling, e.g. `internal/doctor/doctor_test.go:91-94`. | **Low / High** | **Change Function Declaration**: return `*Runner` only and delete unreachable error handling after a whole-repository call-site search. If construction will later validate, add that behavior when it exists rather than preserving a speculative error today. |
| F11 | Release-track token extraction is duplicated and allocates | `internal/update/release_track.go:17-29` and `:32-40` repeat `strings.Split(prerelease, ".")[0]`. | **Low / High** | **Extract Function**: one `prereleaseTrack` helper using `strings.Cut` centralizes classification and avoids materializing all tokens. |
| F12 | Legacy typed-error extraction remains despite a Go 1.26 baseline | Five production sites use declared-target `errors.As`: `internal/cli/root.go:366`, `internal/cli/extension_install.go:84`, `internal/cli/workspace_resolution.go:272-276`, and `internal/cli/session_event_output.go:178`; 15 test sites remain. Modern local examples already exist at `internal/cli/root.go:253-266` and `internal/cli/tool_errors.go:49-72`. | **Low / High** | **Substitute Algorithm**: migrate a focused batch to `errors.AsType[T]`, retaining `errors.Is` where identity rather than extraction is intended. |
| F13 | Two benchmarks retain pre-1.24 loop/sink ceremony | `internal/workref/ref_bench_test.go:19-27` and `:38-46` use `b.N`, package-level sinks, and `runtime.KeepAlive`. | **Low / High** | **Substitute Algorithm**: use `for b.Loop()` and delete the sinks if benchmark behavior remains observable under the new loop semantics. Compare with the already-modern benchmarks in `internal/version/version_bench_test.go:5-28`. |
| F14 | A unit test uses wall time for an in-process concurrency invariant | `internal/version/version_test.go:32-48` starts a goroutine and fails after a real 250 ms timer. Separately, 217 `context.Background()` calls occur in 34 scoped test files, many of which are eligible for `t.Context()` when test-lifetime cancellation is the intended contract. | **Low / Medium** | **Substitute Algorithm**: move the version invariant into `synctest.Test`; migrate test contexts case-by-case, excluding process-lifetime/integration contexts whose work intentionally outlives the test request. Replace integration sleeps with explicit completion signals rather than applying `synctest` across external boundaries. |

## Transferable Patterns

1. **Make cleanup part of the result, not an afterthought.** A function that acquires a body, file, temporary directory, or subprocess should use a named result or an explicit finalization block that joins cleanup errors. Apply this package-locally to `internal/update/github.go:100-188`, `internal/update/apply.go:19-58`, `internal/update/manager.go:186-199`, and `internal/cli/client_loops.go:128-152`. The invariant is: every exit releases or transfers every acquired resource, and a primary error never hides a secondary cleanup failure.

2. **Separate timeout reporting from cancellation enforcement.** `internal/doctor/doctor.go:105-125` demonstrates why a timer around a goroutine is only reporting: it cannot stop code that ignores context. In-process plugins must have a cooperative contract and synchronous ownership; hard isolation requires a killable subprocess boundary. The same question should be asked before every future `go` statement.

3. **Create context exactly once at a process/request boundary.** The correct entry-point shape is visible at `cmd/compozy/main.go:15`; mid-path helpers should accept and propagate it. Consolidating the four CLI pollers provides a natural place to enforce non-nil context, deadline policy, ticker shutdown, and cancellation without duplicating them.

4. **Put the filesystem trust boundary above the untrusted relative path.** `internal/config/agent_delete.go:80-103` is the transferable pattern. The trusted root should represent a Compozy home/workspace owner; validation and I/O happen through the same rooted descriptor. Path normalization alone is not an authorization boundary.

5. **Keep transport breadth at composition roots and dependency breadth at consumers.** `DaemonClient` may describe the daemon's complete UDS capability surface, which supports the project's agent-manageability premise. Command functions still need only narrow, consumer-owned interfaces. This preserves extensibility while shrinking mocks, compile-time coupling, and accidental authority.

6. **Use semantic modernization batches.** `errors.AsType` and `b.Loop` can be low-risk focused batches. `omitzero`, iterators, `cmp.Or`, and `OnceValue` require semantic review because they can change wire omission, allocation/laziness, provenance, or initialization lifetime. The local generator support at `internal/codegen/sdkts/generate.go:243-259` is a good example of co-shipping wire semantics and generated-language behavior.

7. **Keep tests in their canonical suite while extracting lifecycle helpers.** The integration cleanup issue should be fixed inside `internal/cli/cli_integration_test.go`, reusing one harness helper rather than adding 18 regressions. That both honors test consolidation and makes leaked-process failures visible.

8. **Prefer integer-domain formatting for integer-domain data.** Uptime arrives as seconds; formatting by division/modulo avoids the narrower representable range and overflow behavior of `time.Duration`. Reserve `time.Duration` for durations whose range has already been validated.

## Risks / Mismatches

- **`os.OpenRoot` can be applied cosmetically and still be unsafe.** `os.OpenRoot(filepath.Dir(untrustedPath))` merely blesses an attacker-chosen directory. The caller must supply a trusted root and a validated relative path; the opened descriptor, not a prior pathname stat, owns the regular-file check.

- **Draining has latency and trust implications.** The updated cleanup contract requires HTTP draining for connection reuse, but error bodies may be attacker-controlled. The owning client must pair a bounded body policy with close behavior rather than introducing an unbounded drain that can hang. Successful JSON decode must also drain trailing bytes before close.

- **`sync.OnceValue` and context do not mix automatically.** Replacing `Manager.installOnce` mechanically would preserve the more serious flaw that the first caller's deadline/cancellation decides cached initialization. Resolve installation from context-independent inputs or store a result/error under an explicit lifecycle before adopting `OnceValue`.

- **`cmp.Or` evaluates arguments eagerly.** It is appropriate for cheap trimmed strings at `internal/config/provider_resolve.go:78-86`; it is not a replacement for branches that set provenance or invoke an expensive fallback.

- **`synctest` does not virtualize external systems.** It fits `internal/version/version_test.go:32-48`, where all concurrency and timers are in-process. Applying it to daemon, filesystem, network, or subprocess integration sleeps would create false confidence; those paths need observable readiness/completion.

- **Process-handle modernization is cross-platform and cross-package.** CLI tests already mitigate PID reuse with start-time matching, but replacing that with stable handles changes `internal/procutil`, dependency injection, and platform implementations. A CLI-only patch would split the invariant and is therefore deferred.

- **Interface splitting can accidentally duplicate the transport surface.** The goal is not 213 one-method interfaces declared beside the implementation. Define a small interface at each consumer or cohesive command family, let the concrete UDS client satisfy them implicitly, and keep the root facade only for composition.

- **Iterator adoption can leak into public contracts.** Use `maps.Keys`/`slices.Sorted` internally first. Returning `iter.Seq` from public CLI/config APIs changes traversal lifetime and error semantics and needs consumer evidence even in a greenfield alpha.

- **Reflection removal can cause wire drift.** `tool_surface_reflection.go` likely derives fields/tags generically. A typed or generated replacement must be checked against the config schema, CLI/HTTP/UDS tool descriptors, and generated SDK expectations; replacing it by hand without a contract test risks silently omitting fields.

- **Near-cap files are change constraints, not automatic split targets.** Splitting a 480–495-line file without a behavioral reason can worsen cohesion. Any task that touches one should identify a real responsibility boundary before editing; unrelated proactive churn is not justified by line count alone.

## Open Questions

1. **Doctor probe contract:** Are all `Probe` implementations trusted to return promptly after `ctx.Done()`, or is the timeout advertised as a hard bound against non-cooperative probes? The answer decides synchronous cooperative execution versus a subprocess/supervisor boundary.

2. **Stable process-handle ownership:** Is a Go 1.26 handle-based process primitive already planned in `internal/procutil`, and which platforms must preserve equivalent behavior? This slice cannot safely change only the CLI half.

3. **MCP origin policy:** Does the out-of-scope `internal/mcp.ServeHTTP` already enforce an equivalent origin policy, and is cross-origin browser access intentionally supported? Audit that owner before deciding whether `http.CrossOriginProtection` is required.

4. **Reflection source of truth:** Is `internal/config/tool_surface_reflection.go` intentionally the canonical generic serializer, or can the declarative config/codegen source emit a typed projection? If reflection remains, what measured workload and lint justification should be adjacent?

5. **Exported dead code:** A slice-local identifier scan found no safely removable unexported functions, but it cannot prove exported functions unused outside these directories. A whole-repository call graph is required before generating delete targets.

6. **Supported integer widths:** Does the CLI formally support 32-bit targets? `internal/cli/scheduler.go:299` is definitely unsafe on 32-bit for sufficiently large valid durations; even if the support matrix is 64-bit-only, an explicit bound makes the contract portable and testable.

## Evidence

### Package/source coverage

`Prod` counts non-test `.go` files, `Tests` counts ordinary `_test.go` files, and `Bench` counts benchmark-focused `_bench_test.go` files. `Test LOC` includes both ordinary tests and benchmark files. Every row received an exhaustive file inventory and full-scope pattern scan; “deep” marks packages with one or more end-to-end source reads.

| Package directory | Prod | Tests | Bench | Prod LOC | Test LOC | Total LOC | Review depth |
|---|---:|---:|---:|---:|---:|---:|---|
| `cmd/compozy-catalog` | 3 | 1 | 0 | 352 | 34 | 386 | Full inventory/scan; 3 production files deep |
| `cmd/compozy` | 1 | 0 | 0 | 23 | 0 | 23 | Full inventory/scan; entry point deep |
| `internal/frontmatter` | 1 | 1 | 1 | 274 | 256 | 530 | Full inventory/scan; candidate-driven |
| `cmd/compozy-manifest-check` | 1 | 0 | 0 | 94 | 0 | 94 | Full inventory/scan; candidate-driven |
| `internal/e2elane` | 1 | 1 | 0 | 143 | 226 | 369 | Full inventory/scan; candidate-driven |
| `cmd/compozy-codegen` | 5 | 2 | 0 | 895 | 877 | 1,772 | Full inventory/scan; modernization candidates inspected |
| `internal/workref` | 1 | 1 | 1 | 33 | 212 | 245 | Full inventory/scan; production and benchmark deep |
| `internal/doctor` | 5 | 1 | 0 | 844 | 516 | 1,360 | Full inventory/scan; runner/registry deep |
| `internal/update` | 11 | 7 | 0 | 1,698 | 1,604 | 3,302 | Full inventory/scan; lifecycle, GitHub, archive, detection, track deep |
| `internal/version` | 1 | 1 | 1 | 57 | 78 | 135 | Full inventory/scan; all sources deep |
| `internal/logger` | 2 | 1 | 1 | 239 | 260 | 499 | Full inventory/scan; candidate-driven |
| `internal/demoseed` | 6 | 1 | 0 | 1,165 | 253 | 1,418 | Full inventory/scan; candidate-driven |
| `internal/codegen/jsbin` | 1 | 1 | 0 | 32 | 78 | 110 | Full inventory/scan; candidate-driven |
| `internal/codegen/storeschema` | 4 | 2 | 0 | 806 | 244 | 1,050 | Full inventory/scan; rooted-I/O candidates inspected |
| `internal/codegen/openapits` | 1 | 1 | 0 | 217 | 494 | 711 | Full inventory/scan; test-storage candidate inspected |
| `internal/codegen/sdkts` | 6 | 1 | 1 | 1,044 | 479 | 1,523 | Full inventory/scan; JSON-tag generator deep |
| `internal/codegen/sdkgo` | 6 | 1 | 0 | 661 | 80 | 741 | Full inventory/scan; candidate-driven |
| `internal/listcursor` | 1 | 1 | 0 | 80 | 88 | 168 | Full inventory/scan; candidate-driven |
| `internal/config` | 125 | 37 | 1 | 17,095 | 18,113 | 35,208 | Full inventory/scan; merge, bootstrap, file I/O, reflection, deletion, provider resolution deep |
| `internal/config/defaults` | 1 | 0 | 0 | 6 | 0 | 6 | Full inventory/scan; candidate-driven |
| `internal/config/lifecycle` | 1 | 1 | 0 | 251 | 302 | 553 | Full inventory/scan; candidate-driven |
| `internal/cli` | 293 | 83 | 1 | 56,511 | 53,058 | 109,569 | Full inventory/scan; command, daemon, transport, loop, context, cleanup candidates deep |
| `internal/cli/docpost` | 5 | 2 | 0 | 1,057 | 1,256 | 2,313 | Full inventory/scan; sort candidates inspected |
| **Total** | **482** | **147** | **7** | **83,577** | **78,508** | **162,085** | **636 files / 23 package directories** |

### Static scan facts

| Scan | Result |
|---|---|
| Typed error extraction | 13 `errors.AsType` matches in 6 files; 20 legacy `errors.As` matches in 12 files, including 5 production sites. |
| Benchmarks | 16 `b.Loop` matches in 6 files; exactly 2 `b.N` loops, both in `internal/workref/ref_bench_test.go`. |
| JSON omission | 4 `omitzero` matches in 3 files; generator support and a semantic generator test are present. |
| Rooted I/O | 5 `os.OpenRoot` matches in 3 files; one confirmed Lstat/open TOCTOU candidate remains. |
| String sequences | 9 sequence API matches in 8 files; no loop over a materialized `strings.Split` remains. |
| Wait groups | Exactly 2 scoped `WaitGroup` declarations and 2 `.Go` calls; no manual `.Add`/`.Done` pair. |
| Collections/sorting | 85 `slices`/`maps` matches in 46 files; 42 legacy production sort calls in 36 files. |
| Time/concurrency tests | 0 `testing/synctest` uses; 3 `time.Sleep` calls, all in `internal/cli/cli_integration_test.go`; one direct wall-time unit candidate in `internal/version/version_test.go`. |
| Iterators | 0 direct `iter` API uses; internal `maps.Keys`/`slices.Sorted` opportunities exist. |
| Once primitives | 3 raw `sync.Once` declarations; 0 `OnceFunc`/`OnceValue`/`OnceValues` uses. One exact `OnceFunc` candidate; one context-sensitive cache that must not be mechanically converted. |
| Randomness | 0 `math/rand` and 0 `math/rand/v2` imports/usages. |
| Fallbacks | 0 `cmp.Or` uses; simple cheap-string candidates exist. |
| Test diagnostics | 0 `T.ArtifactDir`, `T.Attr`, or `T.Output` uses; no retained-artifact contract was found. |
| HTTP origin protection | 0 `CrossOriginProtection` uses; one production CLI entry point delegates serving to an out-of-scope package. |
| Flight recorder | 0 `FlightRecorder` and 0 `runtime/trace` imports/usages. |
| Network dialing | 2 raw UDS `net.Dialer.DialContext` callbacks; the third `DialContext` match is Gorilla WebSocket's API. |
| Buffer lookahead | 0 `bytes.Buffer.Peek` uses and no matching lookahead parser. |
| Interning | 0 `unique` package uses and no profiled long-lived candidate. |
| Context creation | 11 production `context.Background()` calls; 2 are entry points and 9 are mid-path or nil-normalization candidates. Tests contain 217 `context.Background()` calls in 34 files and require lifetime-by-lifetime review. |
| Error discards | 5 production close-error discards (3 with old adjacent justifications, 2 without); at least 46 confirmed error-return discards in tests, including 36 lines from 18 duplicated daemon cleanup pairs. |
| Interface breadth | `DaemonClient`: 213 methods; `loopCommandClient`: 18; `automationClientAPI`: 18. |
| Dead-code heuristic | No unexported production package-level function identifier appears only at its declaration. This does not prove exported reachability. |
| File-size invariant | 0 production files over 500 lines; 7 production files at 480–495 lines. |

### End-to-end deep-read set

The 30 sources read in full were:

- `cmd/compozy-catalog/main.go`
- `cmd/compozy-catalog/package.go`
- `cmd/compozy-catalog/validate.go`
- `internal/cli/automation_client_api.go`
- `internal/cli/client_settings_vault.go`
- `internal/cli/client_window_manager_stream.go`
- `internal/cli/config.go`
- `internal/cli/daemon.go`
- `internal/cli/daemon_process.go`
- `internal/cli/loop_support.go`
- `internal/cli/mcp_auth_input_file.go`
- `internal/config/agent_delete.go`
- `internal/config/bootstrap.go`
- `internal/config/file_io.go`
- `internal/config/merge.go`
- `internal/config/persistence_document_ranges.go`
- `internal/config/provider_resolve.go`
- `internal/config/tool_surface_reflection.go`
- `internal/doctor/doctor.go`
- `internal/update/detect.go`
- `internal/update/github.go`
- `internal/update/manager.go`
- `internal/update/manager_archive.go`
- `internal/update/release_track.go`
- `internal/update/types.go`
- `internal/version/version.go`
- `internal/version/version_bench_test.go`
- `internal/version/version_test.go`
- `internal/workref/ref.go`
- `internal/workref/ref_bench_test.go`

### Deduplicated cited source paths

- `/home/pedronauck/.codex/attachments/36398ca5-d255-4ce7-92d3-0d3429f3838f/pasted-text-1.txt`
- `.agents/skills/eng/eng-cleanup-failure-paths/SKILL.md`
- `.agents/skills/eng/eng-cleanup-failure-paths/references/cleanup-table.md`
- `.agents/skills/eng/eng-code-guidelines/SKILL.md`
- `.agents/skills/eng/eng-code-guidelines/references/coding-style.md`
- `.agents/skills/golang-master/references/concurrency.md`
- `.agents/skills/golang-master/references/context.md`
- `.agents/skills/golang-master/references/interfaces-generics.md`
- `.agents/skills/golang-master/references/modernize.md`
- `.agents/skills/golang-master/references/safety.md`
- `cmd/compozy/main.go`
- `cmd/compozy-catalog/validate.go`
- `cmd/compozy-codegen/main.go`
- `cmd/compozy-codegen/openapi_temp_storage_test.go`
- `cmd/compozy-codegen/sdk_go_contracts.go`
- `internal/cli/automation_client_api.go`
- `internal/cli/bridge_setup_wizard.go`
- `internal/cli/client.go`
- `internal/cli/client_loops.go`
- `internal/cli/client_settings_vault.go`
- `internal/cli/client_test.go`
- `internal/cli/client_window_manager_stream.go`
- `internal/cli/cli_integration_test.go`
- `internal/cli/config_display.go`
- `internal/cli/daemon.go`
- `internal/cli/daemon_process.go`
- `internal/cli/daemon_status.go`
- `internal/cli/daemon_wait_test.go`
- `internal/cli/extension_install.go`
- `internal/cli/extension_output.go`
- `internal/cli/helpers_test.go`
- `internal/cli/lifecycle_test.go`
- `internal/cli/loop_support.go`
- `internal/cli/mcp_auth_flow.go`
- `internal/cli/mcp_serve.go`
- `internal/cli/render_test.go`
- `internal/cli/root.go`
- `internal/cli/root_defaults.go`
- `internal/cli/scheduler.go`
- `internal/cli/session_event_output.go`
- `internal/cli/skill_test.go`
- `internal/cli/support.go`
- `internal/cli/update.go`
- `internal/cli/window_manager_test.go`
- `internal/cli/workspace_resolution.go`
- `internal/codegen/sdkts/generate.go`
- `internal/codegen/sdkts/generate_test.go`
- `internal/config/agent.go`
- `internal/config/agent_delete.go`
- `internal/config/agent_test.go`
- `internal/config/dotenv.go`
- `internal/config/file_io.go`
- `internal/config/persistence_document_ranges.go`
- `internal/config/provider_resolve.go`
- `internal/config/tool_surface_reflection.go`
- `internal/doctor/doctor.go`
- `internal/doctor/doctor_test.go`
- `internal/update/apply.go`
- `internal/update/detect.go`
- `internal/update/github.go`
- `internal/update/manager.go`
- `internal/update/manager_archive.go`
- `internal/update/release_track.go`
- `internal/update/types.go`
- `internal/version/version.go`
- `internal/version/version_bench_test.go`
- `internal/version/version_test.go`
- `internal/workref/ref.go`
- `internal/workref/ref_bench_test.go`
