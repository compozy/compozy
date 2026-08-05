# Go 1.21-1.26 Modernization and Architecture Audit — Parent Synthesis

Date: 2026-08-01

## Objective and method

This audit covers every Go source package under `cmd/`, `internal/`, and `sdk/go/`. The supplied review was treated as a set of hypotheses rather than a migration checklist. The parent inventory found 5,094 Go files and 147 buildable packages (144 in the root module and 3 in the SDK module). Four evidence slices mechanically surveyed the complete corpus and deep-read every cited implementation, owner, and canonical test region. Generated and fixture code was classified but is not a hand-edit target.

The evidence slices are:

- `01_analysis_lifecycle-processes.md`: lifecycle, process, daemon, session, retry, sandbox, and support ownership.
- `02_analysis_state-domain-storage.md`: state, storage, memory, task, workspace, scheduler, and configuration boundaries.
- `03_analysis_surfaces-extensions-sdk.md`: commands, HTTP/UDS surfaces, extensions, bridges, registries, tools, code generation, and the public Go SDK.
- `04_analysis_runtime-gaps.md`: the corrective survey for `deadentity`, `events`, `hooks`, `logger`, `loop`, `reasoning`, `speed`, `testutil`, and `transcript`.

The conclusions below require behavior evidence. A standard-library API is adopted only where it exactly preserves ownership, error timing, order, cancellation, and public contracts. A similar-looking expression is not sufficient.

## Executive decision

The repository is already substantially modern. The valuable work is not a blanket syntax rewrite. It is a set of focused correctness, security, lifecycle, and duplication fixes, followed by exact standard-library substitutions:

1. Bound the SDK JSON-RPC reader before allocation.
2. Add daemon ownership and shutdown semantics to support-bundle operations.
3. Join inbound subprocess handler goroutines during shutdown.
4. close the HTTP Host/DNS-rebinding gap and layer `CrossOriginProtection` around ordinary API routes.
5. Make automation backoff overflow-safe.
6. move Daytona extraction to root-relative filesystem capabilities.
7. make E2E runtime teardown retryable after a canceled first caller and bound hook-subprocess cancellation joins.
8. handle production/test-infrastructure cleanup failures instead of discarding them.
9. adopt exact `Once*`, `WaitGroup.Go`, `rand/v2`, `cmp.Or`, typed UDS, deterministic collection, `errors.AsType`, benchmark, and selected `synctest` improvements.
10. consolidate duplication only where one canonical owner is clear: task-run scanning, deterministic key collection, UDS dialing, platform hook wrappers, and low-level bridge JSON normalization.

No production dead code was proven by the complete symbol/package survey. Nothing should be deleted merely because a slice-local textual search has no callers; registries, generated descriptors, reflection, command wiring, and extension manifests are reachability boundaries. Dead-code deletion requires repository-wide compiler/static-analysis proof.

## Twenty-feature decision matrix

| # | Go feature / review claim | Decision | Evidence-backed boundary |
|---:|---|---|---|
| 1 | `errors.AsType[T]` | Apply mechanically where the target is a normal typed pointer/value extraction. | Convert residual hand-written `var target *T; errors.As(err, &target)` forms across hand-owned production/tests. Retain dynamic/reflection targets and unusual interface-shape cases. Existing owning suites prove behavior; no syntax-only tests. |
| 2 | `b.Loop()` | Apply to exact independent benchmark iterations. | Convert remaining simple `for range b.N` / indexed loops only when `b.N` is not part of preallocation, result indexing, or workload size. Benchmark setup and allocation semantics must remain equivalent. |
| 3 | JSON `,omitzero` | No blanket change. | Adopt only where zero and empty are intentionally equivalent on the wire. Do not change nullable/presence-sensitive API, patch, persistence, or generated contract fields. The current high adoption means remaining `omitempty` sites are presumptively semantic. |
| 4 | `os.OpenRoot` | Apply to Daytona archive extraction. | Replace validate-then-open path checks with root-relative capability operations. This closes the symlink TOCTOU window and keeps archive entries confined beneath the destination. |
| 5 | `strings.SplitSeq`, `FieldsSeq`, `Lines` | Apply to internal single-pass consumers. | Preserve materialized slices where callers reuse, index, count, or expose the result. No public iterator contract changes. |
| 6 | `sync.WaitGroup.Go` | Apply to exact owned goroutines. | Convert session compaction, process watcher, daemon task-role activation, subprocess inbound handlers, and new support work. Retain `Add`/`Done` where counting synchronous callbacks/callers rather than launching a goroutine. |
| 7 | range over integer | No broad action. | Existing adoption is high. Convert only exact counted loops where the index remains the same value and benchmark semantics are unaffected. |
| 8 | `slices`, `maps`, `min`, `max` | Apply to deterministic internal collection and exact arithmetic helpers. | Use `slices.Sorted(maps.Keys(m))` behind materialized stable-order boundaries; preserve public slices, pagination, authorization-before-mutation, and generator determinism. |
| 9 | `testing/synctest` | Apply selectively. | Start with pure in-process timer/goroutine suites such as hooks pool, batching, artifact sweeper, memory/runtime timers, and process-parent watcher logic. Do not place subprocess, filesystem timestamp, SQLite progress, real socket, or external scheduler behavior in a fake-time bubble. |
| 10 | `iter.Seq` / range-over-function | Reject public/storage streaming rewrites; apply collection helpers internally. | Current list APIs encode snapshot, stable-order, authorization, transaction, pagination, and JSON ownership. Exposing laziness would change work/error timing. Internal sorted-map collection is safe. |
| 11 | `os.Process.WithHandle` / `os.ErrNoHandle` | Reject for current process owners. | `exec.Cmd` children already use pidfds internally on supported Unix systems. Compozy owns process groups and Windows jobs, not a single durable `os.Process` identity; `WithHandle` does not model descendants or restart identity. |
| 12 | `sync.OnceValue`, `OnceValues`, `OnceFunc` | Apply narrowly. | Use for zero-argument idempotent cleanup and true sticky result/error memoizers. Do not convert first-call argument/context capture, multi-channel lifecycle, or zero-value struct ownership without redesigning the contract. |
| 13 | `math/rand/v2` | Apply. | Convert non-cryptographic retry jitter in `internal/retry` and `internal/bridgesdk`; preserve injected randomness and bound-based assertions. Keep every secret, token, nonce, and identifier on `crypto/rand`. |
| 14 | `cmp.Or` | Apply narrowly. | Use only after normalization for pure first-nonzero fallback such as `ResolveAgentName`. Retain explicit branches when provenance, pointer overlays, trim semantics, or non-positive validation are observable. |
| 15 | `T.ArtifactDir`, `T.Attr`, `T.Output` | Apply selectively / defer retention-sensitive parts. | Standardize failed diagnostic artifacts only where the runner publishes them; keep `TempDir` for live fixtures. Use low-cardinality, redaction-safe attributes/output only in canonical suites with a named CI consumer. Do not emit paths, IDs, credentials, or daemon payloads casually. |
| 16 | `net/http.CrossOriginProtection` | Apply with explicit Host validation. | Keep existing CORS. Validate request Host against the configured bind identity, then apply CSRF protection to ordinary `/api` routes after public webhooks and the safe OAuth callback. Preserve JSON/OpenAI error envelopes and UDS behavior. |
| 17 | `runtime/trace.NewFlightRecorder` | Defer. | A recorder is process-global, potentially cross-workspace sensitive state. Implementation first needs owner, size, trigger, consent, redaction, retention/deletion, support-bundle integration, config defaults/docs, and CLI/HTTP/UDS management. |
| 18 | `net.Dialer.DialUnix` / `DialTCP` | Apply to UDS only. | Centralize typed, context-aware Unix dialing for CLI HTTP/websocket and the E2E harness. Retain policy-aware TCP dialers that pin validated IPs for SSRF/DNS-rebinding protection. |
| 19 | `bytes.Buffer.Peek` | Reject. | There is no relevant `bytes.Buffer` parser. The real SDK defect is `bufio.Reader.ReadBytes`, which allocates the full attacker-controlled line before enforcing the limit. Fix framing with bounded fragment reads. |
| 20 | `unique` | Reject. | Session/workspace/task/path/auth-derived strings are high-cardinality and process-global interning would retain tenant data for process lifetime. Reconsider only for a small bounded vocabulary after heap-profile evidence. |

## Confirmed correctness and security findings

### P0 — SDK response framing is not allocation-bounded

`sdk/go/transport.go` reads newline-delimited JSON-RPC with `ReadBytes('\n')` and checks `MaxMessageBytes` only after the line returns. A peer can send an arbitrarily long unterminated frame, forcing allocation beyond the configured limit and delaying rejection indefinitely.

Root fix: add a focused bounded-line reader using `ReadSlice` fragments, reject immediately when the cumulative frame crosses `max+1`, preserve legal boundary behavior, blank-line handling, and the existing EOF-partial-frame contract.

Invariant / owner / canonical suite: the SDK transport rejects terminated and unterminated oversized frames before consuming the attacker-controlled remainder; valid maximum-size and final EOF-partial frames retain current JSON-RPC behavior. Owner: `sdk/go` transport. Canonical suite: `transport_lifecycle_test.go` plus existing public runtime contract tests.

### P0 — Support-bundle work is detached but daemon-unowned

`support.Service.Create` detaches the request context and launches a raw goroutine. The concrete service is not retained as a shutdown target, there is no admission close, and daemon shutdown can close stores while accepted work is still running.

Root fix: give the service an owner context, admission state, `WaitGroup`, and idempotent `Shutdown(ctx)`. Accepted work survives request cancellation while the daemon is active; shutdown closes admission, cancels service-owned work, waits for settlement, and leaves every accepted operation terminal. Shut the service down after HTTP/UDS handlers drain and before persistent resources close.

Invariant / owner / canonical suite: request cancellation does not cancel accepted work; service shutdown rejects new work, cancels active builders, waits, and produces a terminal status; daemon shutdown cannot close dependencies before support work settles. Owner: `internal/support` plus daemon composition. Canonical suites: existing support service tests and daemon shutdown-order tests.

### P0 — HTTP same-origin logic trusts attacker-controlled Host

The review's “no CORS” premise is false: Compozy already has strict origin/port response handling. The actual gap is the branch that allows `Origin == request Host`; a hostname controlled by an attacker can resolve to loopback and make both headers match. `CrossOriginProtection` alone also treats matching Host/Origin as same-origin, so it is defense in depth rather than the root fix.

Root fix: require the request target to be compatible with the configured bind identity (loopback aliases remain compatible only with a loopback bind), retain exact origin/port checks, and apply `CrossOriginProtection` to ordinary API routes. Public signed webhooks and safe OAuth GET registration remain outside that middleware. Unconditional proxy-header trust must not be expanded.

Invariant / owner / canonical suite: an unapproved non-loopback Host is rejected even when Origin matches it; allowed local same-origin requests and preflight remain; browser cross-site unsafe requests fail; machine-to-machine webhooks, OAuth, OpenAI error shape, and UDS remain unchanged. Owner: `internal/api/httpapi`. Canonical suites: `middleware_refac_test.go`, existing handler error tests, and HTTP integration tests.

### P1 — Inbound subprocess handler goroutines are not joined

The transport reader launches one raw goroutine per inbound request. Shutdown waits for the reader and closes pending outbound calls, but it does not join handlers before the process `Done` channel closes.

Root fix: own handler launches in a transport `WaitGroup`, stop new launches by terminating/joining the sole reader, then wait for cooperative context-aware handlers before closing transport state.

Invariant / owner / canonical suite: lifecycle cancellation reaches active handlers, and process completion is not reported until cooperative handlers return. Owner: `internal/subprocess`. Canonical suite: existing process shutdown cancellation and inbound-routing tests.

### P1 — Automation exponential backoff can overflow

The retry delay multiplies `baseDelay` by `1 << (attempt-1)` without checked shifting/multiplication, while configuration accepts any positive retry count. Large attempts can wrap a positive backoff into zero or negative duration.

Root fix: perform checked exponential growth and return a descriptive runtime error before overflow. Do not introduce an arbitrary public maximum or change valid schedules.

Invariant / owner / canonical suite: attempt one equals the base delay; all valid delays remain identical, positive, and monotonic; overflow is rejected. Owner: `internal/automation`. Canonical suite: existing retry helper/context-aware sleep test.

### P1 — Daytona extraction has a validate/open TOCTOU window

Archive extraction evaluates parent symlinks and later calls path-based `OpenFile`. A concurrent symlink swap can escape the validated destination.

Root fix: open the destination with `os.OpenRoot` and perform mkdir, lstat, open, remove, and symlink operations relative to that capability. Reject escaping or absolute symlink targets and preserve valid relative round trips. Split extraction from archive writing rather than growing the current mixed-responsibility file.

Invariant / owner / canonical suite: no entry or symlink can escape the extraction root, including through a concurrently replaced parent; valid files/directories/relative symlinks round-trip. Owner: `internal/sandbox/daytona`. Canonical suite: existing tar tests.

### P1 — E2E runtime teardown memoizes an incomplete first attempt

`RuntimeHarness.Stop` wraps the entire stop attempt in `sync.Once`. `stopWithContext` treats a signaled process plus any non-deadline wait error—including `context.Canceled`—as success. A canceled first caller can therefore return nil before exit, consume the once, and prevent mandatory cleanup from retrying.

Root fix: model confirmed terminal completion separately from an in-flight stop attempt. A canceled caller either completes a bounded force/join path or returns its error while leaving a later caller able to finish cleanup. Cache success only after process exit is confirmed.

Invariant / owner / canonical suite: repeated successful stops are idempotent; a canceled first stop never reports false success or disables a later cleanup; the final attempt proves child termination and closes idle transports. Owner: `internal/testutil/e2e`. Canonical suite: `runtime_harness_lifecycle_test.go`.

### P1 — Hook subprocess cancellation can block before force cleanup

The cancellation and failed-start paths request group termination, wait for a grace timer, issue a kill, and then receive from `waitCh` without a bound. If termination fails or a descendant keeps the process alive, the code cannot reach the later process-group cleanup helper.

Root fix: keep the graceful deadline, but after the first timeout run bounded group/process force termination before the final join. Join errors with the original execution/termination/checkpoint failures. Consolidate the byte-identical Unix and Windows hook wrappers because platform behavior already belongs to `internal/procutil`.

Invariant / owner / canonical suite: cancellation always reaches a bounded terminal outcome; descendants are terminated; original and cleanup errors remain visible; Unix and Windows behavior stays equivalent. Owner: `internal/hooks` with `internal/procutil`. Canonical suites: existing Unix/Windows subprocess lifecycle tests and cross-build gate.

### P1 — Production cleanup errors are discarded

Rows, files, archive writers, and temporary-file removal errors are explicitly assigned to `_` in multiple hand-owned production paths. This violates repository error policy and can hide durability, resource, and cleanup failure.

Root fix: use explicit close paths or named returns with `errors.Join`, preserving the primary error while surfacing cleanup failure. Where an API contract proves that `rows.Err` owns close failures, encode that rationale in one shared helper rather than repeating ignored calls.

The corrective `testutil` slice also found fourteen discarded errors in production-like test helpers and lifecycle tests, including artifact removal, response-body close, process wait, encoder/writer, and blocker-close failures. Repair them in their owning helpers/suites rather than applying a blind assertion rewrite.

## Confirmed duplication and refactoring backlog

### Implement in this workstream

- **Task-run read mapping:** converge `GetTaskRun`, status lists, and dynamic lists on one hand-owned projection/scanner; edit SQL generator sources and regenerate rather than touching sqlc output. Preserve workspace, network, review-lineage, ordering, and claim-token redaction fields.
- **Deterministic map keys:** replace same-package duplicate map-to-sorted-slice helpers with one `slices.Sorted(maps.Keys(...))` implementation. Keep stable materialized output.
- **Typed UDS transport:** one CLI helper owns address validation and `DialUnix`; HTTP and websocket adapters reuse it. Move generic client construction out of the settings-vault file.
- **Bridge JSON normalization:** extract only clone/compact/empty/object primitives below daemon and contract packages; keep layer-specific validation and exact error prefixes.
- **Hook platform wrappers:** Unix and Windows wrappers are byte-identical because `internal/procutil` already owns platform behavior. Consolidate the wrapper and retain platform-specific process-owner tests/cross-build evidence.
- **Exact cleanup/memoization helpers:** replace local `sync.Once` wrappers with `OnceFunc`, and true sticky value/error caches with `OnceValues`, while retaining lifecycle-dependent once fields.

### Separate high-risk architecture work; do not combine with security hardening

- **HTTP/UDS route duplication:** both transports repeat most route registration, but their middleware, identity, callbacks, OpenAI, webhook, MCP, and static behavior intentionally differ. A future shared operation registrar must be checked against `api/spec` transport metadata and prove full route/middleware parity.
- **Surface dependency bags:** HTTP server, UDS server, and handler configuration repeat a large service graph. Consolidate only after shared route ownership identifies true common dependencies; do not create a replacement god struct.
- **CLI aggregate interface:** keep the concrete client aggregate, but give commands narrow, domain-owned consumer interfaces with compile-time assertions.
- **Dead-entity state retention:** the daemon-global service inserts workspace/entity state indefinitely. Define cardinality, idle lifetime, and a safe unregister/eviction event before changing it; deleting an entry while callers hold the old state could split counters for one key.
- **Flight recording:** requires a dedicated product/security design, not an opportunistic import.

## Rejected false positives and business-rule traps

- `HeaderListQuery.WorkspaceID` is cursor/storage identity, not necessarily a row filter. A naive predicate would break the existing global-memory catalog contract.
- Similar trim/deduplicate helpers differ on ordering, empties, case folding, and rejection semantics; a universal normalizer would erase domain rules.
- Public iterators would move database work and errors outside transactions, change authorization timing, and remove JSON snapshot semantics.
- Process handle wrapping at signal call sites would not model Unix process groups or Windows job descendants.
- `T.ArtifactDir` is not a replacement for live `TempDir` fixtures; retention behavior belongs to the test runner contract.
- `DialTCP` must not replace secure resolvers that validate and pin exact addresses.
- `unique` must not retain workspace, auth, or user-derived identity globally.

## Implementation sequence and gates

1. Land security/correctness fixes with their existing canonical suites: SDK framing, support lifecycle, subprocess join, HTTP Host/CSRF, retry overflow, Daytona `OpenRoot`, retryable E2E teardown, and bounded hook-subprocess cancellation.
2. Land behavior-neutral standard-library substitutions: `rand/v2`, narrow `cmp.Or`, `Once*`, exact `WaitGroup.Go`, sorted map keys, typed UDS, `errors.AsType`, safe benchmarks/string sequences.
3. Land focused duplication/cleanup: task-run scanner/query consolidation, bridge JSON primitive, production close-error handling.
4. Pilot `synctest` only in pure in-process suites; production changes are permitted only when the deterministic test exposes a real bug.
5. Run `make gate` after each mutation batch. User-visible HTTP security behavior requires a QA scenario reset/addition and real walk. Finish with deslop, final verification, review remediation, and one current `make gate-full` record.

## Compozy Impact Audit

- **Native tools:** no tool IDs, toolsets, descriptors, schemas, digests, risk flags, or capability gates are intended to change. Checked `internal/toolmeta`, `internal/tools`, built-ins, HTTP/UDS route owners, and SDK generator ownership. Any task-run query refactor must keep native task response fields byte-equivalent.
- **Extensibility and hooks:** extension protocol, hooks, capabilities, resources, bundles, registries, bridge SDK, and MCP sidecars remain behaviorally unchanged. HTTP CSRF middleware is deliberately placed after public webhook/OAuth registration. Typed TCP security adapters remain intact. Flight recording is deferred because it would require support-source, config lifecycle, consent, and management surfaces.
- **Workspace data isolation:** every changed runtime datum remains in its existing scope. Support operation state is daemon-global but its artifacts retain existing redaction and source rules. HTTP protection is transport policy, not datum ownership. Task-run scanner consolidation must preserve `workspace_id` on every read/list path. `unique` is rejected because it would create process-global retention of workspace/auth identity.
- **Official Compozy skill:** behavior-neutral refactors and internal Go syntax changes have no skill impact after checking public CLI/HTTP/UDS/native-tool semantics. If the HTTP origin rejection becomes documented operator behavior, update the official skill only where it describes HTTP access/security. Flight recording would require a future skill update together with its management surfaces.

## Open design decisions

- Whether `T.Attr`, `T.Output`, and standard artifact retention have a named CI consumer and redaction-safe vocabulary.
- Whether a future flight recorder is an explicitly global support artifact or must be filtered by workspace; this blocks implementation.
- Whether shared route registration should be metadata-driven from `api/spec` or hand-authored and parity-checked against it.
- Whether a trusted reverse-proxy model is supported. Current work must not trust forwarding headers beyond the existing behavior without a configured trust boundary.
- Whether asynchronous hooks are intentionally request-cancelled (as current code/tests specify) or should be pool-owned beyond an HTTP/UDS request. This is a public runtime decision, not a mechanical context change.
- The maximum cardinality/lifetime and safe eviction signal for daemon-global dead-entity state.
