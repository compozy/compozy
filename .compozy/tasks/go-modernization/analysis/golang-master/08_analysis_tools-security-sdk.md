# Analysis: tools-security-sdk

- **Ordinal / slug:** `08_analysis_tools-security-sdk`
- **Slice:** all Go production sources, tests, benchmarks, and packages under `internal/mcp/**`, `internal/registry/**`, `internal/skills/**`, `internal/skillscan/**`, `internal/tools/**`, `internal/toolmeta/**`, `internal/toolruntime/**`, `internal/network/**`, `internal/sandbox/**`, `internal/fileutil/**`, `internal/providers/**`, `internal/providerauth/**`, `internal/providerenv/**`, `internal/vault/**`, `internal/retry/**`, `internal/redact/**`, `internal/notifications/**`, and `sdk/go/**`.
- **Baseline:** root module and `sdk/go` both declare Go `1.26.4` (`go.mod:3`, `sdk/go/go.mod:3`). The supplied review and the current `golang-master`, `eng-code-guidelines`, `architectural-analysis`, `refactoring-analysis`, `security-review`, and `eng-cleanup-failure-paths` doctrine were applied. Project-specific path-hardening and workspace-isolation rules take precedence over generic modernization advice.
- **Method:** fresh analysis; previous analysis artifacts were not opened. All 512 scoped Go files were inventoried and pattern-scanned; the security-, lifecycle-, concurrency-, I/O-, workspace-, SDK-, and modernization-critical implementations and their canonical tests were read deeply. The 31 generated `sdk/go/contracts/*_gen.go` files were used only as contract evidence, not as refactoring targets.
- **Write/verification constraint:** static analysis only. The dispatch prohibited `git`, builds, tests, formatters, generators, and package-manager commands; therefore no dynamic result is claimed.

## Overview

The slice is large but generally well factored: 512 Go files, 112,253 total lines, 122 test files (five of them benchmarks), 31 generated contract files, and 359 non-test/non-generated production files totaling 58,672 lines. It spans 35 package directories. No production file exceeds the 500-line hard cap, although `internal/tools/schema.go` is exactly 500 lines and six other files are at 473–492 lines, so those files cannot absorb new responsibilities.

The strongest implementation patterns are the MCP `securehttp` DNS-to-dial policy, collision-safe MCP target/Vault identities, workspace-scoped tool artifacts, single-use approval tokens, bounded SDK framing, bounded archive extraction, error-joining cleanup in `fileutil`/Vault/Daytona, and the existing `os.OpenRoot` Daytona extractor. These are reusable reference implementations.

The highest-priority mismatches are not cosmetic modernization opportunities:

1. Notification cursor and delivery identities are not injective across global/workspace events, enabling cross-workspace interference.
2. The provider pre-start cache omits `HomePaths`, `CommandEnv`, and any workspace identity, and never prunes expired distinct keys.
3. MCP callers discard every cleanup returned by `RegisterDynamicSecret`; one path registers the bearer token on every HTTP request.
4. Registry extraction checks symlinks before path-based opens, leaving a check/use race that the Daytona `os.OpenRoot` implementation already avoids.
5. The git source builds an entire compressed repository in memory and enforces the compressed limit only afterward.
6. Registry outbound-network policy is weaker than MCP `securehttp`; in particular, GitHub credentials are attached to response-provided URLs without an origin check.

### Exhaustive package coverage

`Tests` includes benchmark files; `Bench` identifies that subset. `Prod` excludes tests and generated contracts.

| Package directory | Go files | Prod | Tests | Bench | Generated | Primary review surface |
|---|---:|---:|---:|---:|---:|---|
| `internal/fileutil` | 11 | 6 | 5 | 1 | 0 | Atomic replace/fsync, deterministic tar/gzip, cleanup |
| `internal/mcp` | 38 | 32 | 6 | 0 | 0 | MCP execution, hosted proxy, stdio/HTTP serving, workspace binding |
| `internal/mcp/auth` | 20 | 16 | 4 | 0 | 0 | OAuth/DCR/PKCE, scope binding, Vault ownership, response cleanup |
| `internal/mcp/securehttp` | 5 | 4 | 1 | 0 | 0 | SSRF, redirects, DNS rebinding, body limits, timeouts |
| `internal/network` | 40 | 27 | 13 | 0 | 0 | Envelope validation, routing, durability, workspace membership |
| `internal/network/participation` | 9 | 8 | 1 | 0 | 0 | Bounds, normalization, ownership, numeric safety |
| `internal/network/rules` | 1 | 1 | 0 | 0 | 0 | Channel rules |
| `internal/network/usage` | 1 | 1 | 0 | 0 | 0 | Public usage parsing/bounds |
| `internal/notifications` | 2 | 2 | 0 | 0 | 0 | Cursor contracts and normalization |
| `internal/notifications/presets` | 10 | 9 | 1 | 0 | 0 | Fanout, cursor/delivery identity, detached dispatch |
| `internal/providerauth` | 1 | 1 | 0 | 0 | 0 | Native-CLI boundary and isolated login environment |
| `internal/providerenv` | 2 | 1 | 1 | 0 | 0 | Provider-home isolation and symlink rejection |
| `internal/providers` | 7 | 5 | 2 | 0 | 0 | Probe execution/classification and global pre-start cache |
| `internal/redact` | 8 | 7 | 1 | 0 | 0 | Dynamic-secret registry and structured redaction |
| `internal/registry` | 23 | 12 | 11 | 1 | 0 | Multi-source fanout, installer, archive confinement, atomic replace |
| `internal/registry/clawhub` | 11 | 9 | 2 | 0 | 0 | HTTP/retry/body limits/archive spool |
| `internal/registry/github` | 10 | 8 | 2 | 0 | 0 | GitHub API/download/publish, credentials, response ownership |
| `internal/registry/gitsrc` | 4 | 3 | 1 | 0 | 0 | Git subprocess, repository reference validation, archive memory |
| `internal/retry` | 3 | 2 | 1 | 0 | 0 | Context-aware retry and jitter backoff |
| `internal/sandbox` | 5 | 2 | 3 | 0 | 0 | Provider registry and typed-nil handling |
| `internal/sandbox/daytona` | 38 | 26 | 12 | 1 | 0 | SSH/sidecar lifecycle, archive sync, HTTP boundaries |
| `internal/sandbox/daytona/cmd/compozy-daytona-sidecar` | 5 | 4 | 1 | 0 | 0 | Process ownership, bounded streams, loopback server |
| `internal/sandbox/local` | 3 | 1 | 2 | 0 | 0 | Local sandbox provider |
| `internal/sandbox/providertest` | 4 | 1 | 3 | 0 | 0 | Provider contract test helpers |
| `internal/skills` | 49 | 32 | 17 | 1 | 0 | Discovery, loading, watches, provenance, workspace caches |
| `internal/skills/marketplace` | 11 | 10 | 1 | 0 | 0 | Install/update/remove, realpath policy, visibility |
| `internal/skillscan` | 4 | 3 | 1 | 0 | 0 | Bounded discovery and symlink confinement |
| `internal/toolmeta` | 12 | 8 | 4 | 0 | 0 | Safe previews and rendering |
| `internal/toolruntime` | 9 | 7 | 2 | 0 | 0 | Durable process ownership and interrupt semantics |
| `internal/tools` | 60 | 45 | 15 | 1 | 0 | Dispatch, approvals, policy, schema, result offload, artifacts |
| `internal/tools/builtin` | 42 | 41 | 1 | 0 | 0 | Native descriptors and schemas |
| `internal/vault` | 6 | 4 | 2 | 0 | 0 | AEAD/key-file lifecycle and MCP ref ownership |
| `sdk/go` | 25 | 20 | 5 | 0 | 0 | Public extension SDK, JSON-RPC transport, lifecycle |
| `sdk/go/contracts` | 31 | 0 | 0 | 0 | 31 | Generated public contract evidence only |
| `sdk/go/extensiontest` | 2 | 1 | 1 | 0 | 0 | External extension harness |

## Mechanisms / Patterns

### Boundary map

- **Registry/install:** a `Source` resolves metadata/downloads, the installer bounds the compressed stream, extracts with decompressed-size and entry-count limits, validates a root manifest/content, computes provenance, and atomically replaces the final directory. The core safety gap is that `internal/registry/extract.go` still performs path checks and path-based writes rather than root-relative handle operations.
- **MCP/auth:** a normalized `auth.Target` carries scope, workspace, and server identity; HTTP catalog endpoints use `securehttp`; stdio servers receive an isolated temporary home and exact environment; OAuth material is stored behind scope-qualified Vault refs; the executor projects failures into stable tool errors. Dynamic redaction registration is the missing lifecycle edge.
- **Tools:** caller `Scope` is bound into approval tokens and provider lists; policy and approval precede execution; oversized results are offloaded to a content-addressed store whose workspace directory is a SHA-256 of the workspace ID; foreign reads return the same not-found identity.
- **Network/notifications:** network envelopes carry workspace identity through routing and durable acceptance. Notification presets derive cursor and bridge delivery identifiers independently; those identifiers currently use ambiguous string concatenation and omit workspace in delivery IDs.
- **Sandbox/providers:** provider homes reject symlink components and enforce `0700`; Daytona owns SSH/sidecar processes and has a root-handle tar extractor; provider pre-start probes cache a classification globally for 30 seconds.
- **SDK:** newline-delimited JSON-RPC frames are capped at 10 MiB, pending calls are correlated under a mutex, handlers run concurrently, and initialization starts asynchronous `OnReady` callbacks. The callback lifecycle is not tied to `Run` or shutdown.

### Strong patterns to preserve

- `internal/mcp/securehttp/client.go:119-176`, `policy.go:13-95`, and `transport.go:18-84` validate URL scheme/host, re-resolve immediately before dialing, reject mixed public/private DNS answers, disable proxies, strip credentials on cross-origin redirects, apply timeouts, and cap response bodies.
- `internal/mcp/auth/types.go:52-77` rejects illegal global/workspace combinations and constructs an injective NUL-separated target key. `internal/vault/mcp_refs.go:25-124` adds collision-safe owner segments and prevents cross-workspace or daemon-managed OAuth subtree access.
- `internal/tools/approval_token.go:120-328` binds one random single-use token to tool/session/workspace/agent/input digest, compares digests in constant time, and retains replay evidence only until expiry.
- `internal/tools/artifact_store.go:76-193,233-253` requires workspace identity, hashes it to an opaque directory, bounds writes/pages, and hides foreign/missing/expired artifacts behind `ErrToolArtifactNotFound`; `artifact_store_test.go:17-75` proves foreign-read isolation.
- `internal/network/manager_send.go:25-145` preserves caller values while detaching commit cancellation only under a bounded deadline and daemon lifecycle cancellation.
- `internal/sandbox/daytona/tar_extract.go:13-114` is the local model for root-confined extraction with `os.OpenRoot` and joined close failures.
- `sdk/go/transport_framing.go:12-35` rejects an oversized unterminated frame as soon as the bound is crossed; `transport_lifecycle_test.go:94-140` proves the adversarial case.

### Go 1.26 modernization matrix

| # | Candidate | Status | Evidence and decision |
|---:|---|---|---|
| 1 | `errors.AsType` | `adopt` | Already used correctly in `internal/providers/runner.go:74,124,152`, `internal/tools/errors.go:162`, and `sdk/go/errors.go:114`; convert remaining concrete/interface holder boilerplate such as `internal/tools/errors.go:166`, `internal/skills/diagnostics.go:95`, `internal/mcp/securehttp/client.go:186`, and `internal/mcp/executor.go:158,182`. |
| 2 | `b.Loop` | `adopt` | Most benchmarks already use it. Convert the two simple `b.N` holdouts at `internal/fileutil/atomic_bench_test.go:27-31` and `internal/tools/perf_bench_test.go:96-101`; neither needs the index. |
| 3 | `omitzero` | `already` | Correct value-struct usage exists at `internal/skills/resource_spec.go:32` and `internal/network/participation/types.go:77`. Retain `omitempty` for pointers, strings, and slices where emptiness is the intended wire contract. |
| 4 | `os.OpenRoot` | `adopt` | The Daytona implementation at `internal/sandbox/daytona/tar_extract.go:19-32,61-114` is sound. Apply the same root-handle algorithm to registry extraction at `internal/registry/extract.go:157-211,378-430`; do not use it as a blanket replacement for project-mandated realpath validation of user/agent paths. |
| 5 | `strings` sequence APIs | `already` | `SplitSeq` is used in `internal/tools/id.go:99`, `internal/registry/extract.go:401`, `internal/providerenv/env.go:165`, and `sdk/go/types.go:72`. Remaining `Split`/`Fields` calls either return a materialized slice or need indexed/first-element access. |
| 6 | `sync.WaitGroup.Go` | `adopt` | Already used at `internal/mcp/hosted_proxy.go:116-123` and in a skills concurrency test. Replace manual `Add`/goroutine/`Done` pairs in `internal/registry/multi.go:64-94,250-270`. |
| 7 | range-over-integer | `already` | Used in production at `internal/mcp/executor_tools.go:18`; remaining indexed loops (for example `internal/toolmeta/preview_terminal.go:105`) need the index or mutate aligned slices. |
| 8 | `slices` / `maps` / built-in `min`/`max` | `already` | Widespread and semantically appropriate: `internal/registry/multi.go:224,278`, `internal/skills/registry_load.go:117`, `internal/retry/backoff.go:36-48`, `sdk/go/errors.go:72`. |
| 9 | `testing/synctest` | `adopt` | The shutdown assertion at `internal/tools/artifact_sweeper_test.go:11-33` sleeps 3 ms after a ticker-driven worker stops and is a direct virtual-time candidate. The HTTP-timeout test at `internal/registry/clawhub/client_test.go:586-610` should instead use a blocking transport/context because real netpoll timing is the behavior under test. |
| 10 | `iter.Seq` | `defer` | Registry, skills, tools, Vault, and SDK list methods expose/materialize slices (`internal/registry/source.go:40-46`, `internal/skills/registry.go:158-184`, `internal/tools/registry.go:156-205`). Changing public contracts is not justified without a measured allocation/hot-path problem. |
| 11 | `os.Process.WithHandle` | `defer` | `internal/toolruntime/interrupt.go:15-94` intentionally operates from durable PID/start-time/process-group evidence and delegates platform behavior to `internal/procutil`, outside this slice. Modernization must be designed in that owner, not patched locally. |
| 12 | `sync.OnceFunc` / `OnceValue` / `OnceValues` | `adopt` | Convert the local idempotent cleanup closure in `internal/redact/dynamic.go:28-41` to `OnceFunc`. Keep stateful per-instance `sync.Once` fields (SDK transport, sessions, registry clients) where the result/error is stored elsewhere; `processSnapshot` also intentionally depends on first-call input. |
| 13 | `math/rand/v2` | `adopt` | `internal/retry/backoff.go:4,46-48` uses the legacy global `math/rand`; move jitter to `rand/v2` and use a deterministic `rand/v2.PCG` in `internal/retry/retry_test.go`. Keep `crypto/rand` for Vault keys, PKCE, IDs, and approval tokens. |
| 14 | `cmp.Or` | `reject` | Defaults in this slice generally require trimming, validation, lazy work, or duration normalization (for example token type at `internal/mcp/auth/service.go:294-302`). `cmp.Or` would silently change those semantics and make normalization less visible. |
| 15 | `T.ArtifactDir` / `T.Attr` / `T.Output` | `adopt` | Use `T.Attr` for structured Daytona integration evidence currently encoded in `t.Logf` at `internal/sandbox/daytona/ssh_validation_test.go:296-314` and `launcher_transport_integration_test.go:78-96`. Keep `t.TempDir` for disposable fixtures; use `ArtifactDir` only for evidence intended to survive, and `Output` only where test subprocess output belongs in the test stream. |
| 16 | `http.CrossOriginProtection` | `adopt` | Wrap the token-authenticated loopback MCP handler assembled at `internal/mcp/serve_http.go:64-78`; retain constant-time bearer validation at `serve_http.go:117-135`. Add compatibility tests for non-browser clients with absent `Origin`/Fetch Metadata and rejection tests for cross-site unsafe requests. |
| 17 | `runtime/trace.FlightRecorder` | `not applicable` | No trace/diagnostic recorder is owned by the scoped packages. Introducing a recorder in a tool, registry, or SDK package would create the wrong lifecycle boundary; a daemon diagnostics owner must decide it. |
| 18 | typed `DialUnix` / `DialTCP` methods | `reject` | Production dial points intentionally accept generic network/address pairs for policy injection (`internal/mcp/securehttp/transport.go:18-33`) or SSH transports. The only package-level `net.DialUnix` occurrence is test setup at `internal/mcp/peer_test.go:83`. |
| 19 | `bytes.Buffer.Peek` | `not applicable` | SDK framing uses `bufio.Reader.ReadSlice` with an explicit byte cap (`sdk/go/transport_framing.go:17-34`); scoped `bytes.Buffer` values are accumulators, not parsers requiring lookahead. |
| 20 | `unique` | `defer` | Registries contain many repeated string keys (`internal/skills/registry.go:41-49`, `internal/tools/registry.go:24-38`), but interning would add process-global identity/lifetime behavior and alter public-value expectations. Require heap/profile evidence first. |

## Relevant Sources

| Area | Primary sources | Why they matter |
|---|---|---|
| Go/toolchain contract | `go.mod:3`; `sdk/go/go.mod:3`; `/home/pedronauck/.codex/attachments/36398ca5-d255-4ce7-92d3-0d3429f3838f/pasted-text-1.txt` | Establish Go 1.26.4 and the modernization candidate set. |
| MCP workspace/auth identity | `internal/mcp/auth/types.go:30-77,215-240`; `internal/vault/mcp_refs.go:25-124` | Reference collision-safe scope/workspace/server identities and secret ownership. |
| MCP HTTP security | `internal/mcp/securehttp/client.go:104-194`; `policy.go:13-95`; `transport.go:18-84`; `client_test.go:26-329` | Reusable SSRF, redirect, timeout, and response-bound implementation. |
| MCP lifecycle/redaction | `internal/mcp/executor_client.go:25-183,205-350`; `executor_auth.go:157-176`; `internal/redact/dynamic.go:12-73` | Shows the dropped cleanup contract and per-request registrations. |
| MCP server boundary | `internal/mcp/serve.go:70-97`; `serve_http.go:25-137`; `serve_facade.go:20-160` | HTTP/stdio lifecycle, token gate, workspace pinning, dependency error adaptation. |
| Registry concurrency | `internal/registry/multi.go:53-130,218-395`; `multi_test.go:66-390` | Fanout, partial failure, alias ownership, and manual `WaitGroup` sites. |
| Registry extraction | `internal/registry/extract.go:64-254,256-430`; `extract_test.go:113-234,293-326` | Size/count limits, cleanup, symlink checks, and TOCTOU surface. |
| Safe extraction precedent | `internal/sandbox/daytona/tar_extract.go:13-143`; `tar.go:145-181` | Root-relative filesystem operations and archive-name policy. |
| Registry HTTP | `internal/registry/github/http.go:17-250`; `releases.go:17-228`; `internal/registry/http_archive.go:27-151`; `internal/registry/clawhub/client_request.go:16-147` | Credentials, redirects, read bounds, retry, drain/close ownership. |
| Git source memory | `internal/registry/gitsrc/client.go:117-176`; `reference.go:11-29`; `internal/fileutil/targzip.go:17-105` | Unbounded clone/archive materialization and repository scheme policy. |
| Skills path/provenance | `internal/skills/path_security.go:8-39`; `marketplace/paths.go:17-109`; `internal/skillscan/scan.go:42-60`; `scan_directory.go:27-137`; `internal/skills/provenance.go:48-219` | Existing realpath policies, duplicated primitives, and close ownership. |
| Provider isolation/cache | `internal/providers/prestart.go:15-62,123-139`; `probe_env.go:18-64`; `internal/providerenv/env.go:93-196`; `env_symlink_test.go:13-91` | Global cache identity gap versus strong provider-home confinement. |
| Tool authorization/artifacts | `internal/tools/approval_token.go:120-328`; `artifact_store.go:68-419`; `artifact_store_test.go:17-186` | Strong session/workspace binding and artifact isolation. |
| Notification identity | `internal/notifications/presets/service_helpers.go:38-58`; `types.go:235-275`; `service.go:188-225`; `match_test.go:124-211` | Ambiguous durable keys and missing cross-workspace collision tests. |
| Network durability | `internal/network/manager_send.go:25-145,230-300`; `envelope_security.go:12-100`; `validate.go:31-307` | Bounded detached commits, workspace routing, and secret rejection. |
| Vault crypto | `internal/vault/service.go:70-151`; `crypto.go:263-317`; `service_test.go:117-184,489-515` | AES-GCM use and missing associated-data binding. |
| SDK lifecycle/framing | `sdk/go/transport.go:102-248,251-424`; `transport_framing.go:12-35`; `extension_initialization.go:10-79`; `extension_context.go:72-94`; `runtime_test.go:385-436` | Bounded transport, lazy lifecycle, asynchronous callbacks. |
| Generated contracts | `sdk/go/contracts/capabilities_gen.go:1`; `host_contracts_gen.go:1`; `types_026_gen.go:1` | Evidence of generator ownership; not edit targets. |

## Transferable Patterns

### F1 — Make notification identities injective across scope and workspace

- **Severity:** High
- **Confidence:** High
- **Fowler technique:** Replace Primitive with Object; Introduce Parameter Object.
- **Evidence:** `cursorKeyForTarget` maps an empty workspace to the literal `global`, then concatenates `workspaceID + ":" + event.ID` (`internal/notifications/presets/service_helpers.go:38-47`). Consequently, `(workspace="", event="e")` and `(workspace="global", event="e")` are identical; `(workspace="a:b", event="c")` and `(workspace="a", event="b:c")` are also identical. `skipDeliveryID` and `deliveryIDForTarget` omit workspace entirely (`service_helpers.go:50-58`). `Event.Validate` only requires non-empty ID/type and positive sequence; it imposes no grammar that would make these concatenations safe (`types.go:260-275`).
- **Risk:** one workspace can reuse or advance another workspace/global event's cursor, producing false replay skips, suppressed notifications, or misattributed failure state. Delivery IDs can collide at the bridge idempotency boundary even when cursor keys do not.
- **Recommendation:** introduce a typed `deliveryIdentity` containing explicit scope tag, workspace, event, preset, target hash, index, and reason. Encode it with length-prefixed fields or canonical JSON and hash it; never reserve a user-representable literal for global scope. Use the same identity source for cursor subject, delivered ID, and skipped ID.
- **Canonical invariant/suite:** “distinct scope/workspace/event/target tuples never share cursor or delivery identity”; own it in `internal/notifications/presets/match_test.go`, including empty-vs-`global`, colon-containing, and same-event-across-two-workspaces cases.

### F2 — Scope and bound the provider pre-start cache

- **Severity:** High
- **Confidence:** High
- **Fowler technique:** Extract Class; Encapsulate Collection.
- **Evidence:** `ProbeEnv` carries `HomePaths` and `CommandEnv` (`internal/providers/probe_env.go:23-30`), and the probe executes with `CommandEnv` (`prestart.go:98-106`), but `preStartCacheKey` hashes only provider name, auth mode, commands, and credential-ref metadata (`prestart.go:123-139`). The package-global map retains expired distinct keys indefinitely (`prestart.go:28-54`).
- **Risk:** two workspaces or runtime environments with the same provider configuration but different isolated homes/environment can share an authentication classification for 30 seconds. Distinct configurations also accumulate expired entries for the process lifetime. FNV-64 is unnecessary for an identity/security cache.
- **Recommendation:** move the cache into an injected, daemon-owned component; require explicit workspace/runtime-home identity; hash a canonical, redacted environment identity with SHA-256; prune on access/insertion or use a bounded LRU. Do not include plaintext secrets—include scope, home, key names, and stable non-secret binding/version evidence.
- **Canonical invariant/suite:** “same provider config in different workspace/home/env scopes never shares a probe result, and expired keys are removed”; extend `internal/providers/prestart_test.go` rather than adding a standalone cache test.

### F3 — Give dynamic-secret registrations an owner and cleanup edge

- **Severity:** High
- **Confidence:** High
- **Fowler technique:** Encapsulate Collection; Move Function; Move Statements into Function.
- **Evidence:** `RegisterDynamicSecret` increments a global reference count and returns an idempotent unregister closure (`internal/redact/dynamic.go:26-58`). All four production call sites discard it: secret-env resolution at `internal/mcp/executor_client.go:313,331,345` and bearer construction at `internal/mcp/executor_auth.go:173`. The bearer path executes for every authorized request.
- **Risk:** secrets and their reference counts remain in global memory indefinitely; repeated requests continually increase counts and rebuild/sort the snapshot. Rotated/deleted secrets continue affecting redaction, and error paths before MCP connection completion leak registrations too.
- **Recommendation:** centralize registration at the token/launch lifecycle owner. Return a cleanup bundle from MCP env construction, join it with stdio-home cleanup, and unregister on every connect/launch failure. For HTTP, register once for the managed client/token generation or bracket the round trip with `defer cleanup()`. Implement idempotence with `sync.OnceFunc`.
- **Canonical invariant/suite:** “success, connect failure, cancellation, token rotation, and client close restore the dynamic-secret registry to its baseline”; extend `internal/mcp/executor_test.go` for lifecycle ownership and `internal/redact/redact_test.go` only for the registry contract.

### F4 — Replace registry extraction check/use paths with `os.Root`

- **Severity:** High
- **Confidence:** High
- **Fowler technique:** Substitute Algorithm; Move Function.
- **Evidence:** registry extraction validates components with `Lstat` (`internal/registry/extract.go:378-430`) and later calls `MkdirAll`/`OpenFile` by absolute path (`extract.go:147-211`). A concurrent actor can replace a checked parent with a symlink between those operations. Daytona already performs the same class of operation through `os.OpenRoot` and root-relative methods (`internal/sandbox/daytona/tar_extract.go:19-143`).
- **Risk:** archive writes can escape the extraction root under a local symlink-swap race, despite traversal and static-symlink tests passing.
- **Recommendation:** port the registry extractor to an `*os.Root`-owned implementation; preserve registry sentinels, compressed/decompressed limits, file-count bounds, mode policy, and partial-file cleanup. Use `OpenRoot` for archive extraction only. User/agent-supplied path resolution still requires the project `sanitizePathKey` + `realpathDeepestExisting` policy.
- **Canonical invariant/suite:** “an adversary swapping an extraction parent cannot redirect any create/chmod/remove outside the root”; extend `internal/registry/extract_test.go`, using the existing Daytona tests as the behavioral reference.

### F5 — Stream and bound git-source archives before materialization

- **Severity:** High
- **Confidence:** High
- **Fowler technique:** Substitute Algorithm; Extract Class.
- **Evidence:** `TarGzipDirectory` writes the complete archive to a `bytes.Buffer` (`internal/fileutil/targzip.go:17-49`). The git source clones first, calls that helper, and only then compares `len(archive)` with `MaxArchiveSize` (`internal/registry/gitsrc/client.go:117-173`). A shallow clone is not a size bound.
- **Risk:** a large or highly compressible repository can consume unbounded disk during clone and unbounded heap during archive construction before the advertised compressed limit is checked. The later installer limit cannot protect this producer-side allocation.
- **Recommendation:** stream tar/gzip into an owned temp file through a counting/limited writer, enforce file-count and uncompressed-byte limits during traversal, honor context during walk/copy, rewind on success, and return a close/remove owner. Add an explicit clone-output budget or documented sandbox/quota boundary.
- **Canonical invariant/suite:** “repository content beyond any configured count/uncompressed/compressed limit fails without proportional heap growth and removes temp state”; own end-to-end behavior in `internal/registry/gitsrc/client_test.go`, with the streaming primitive's mechanics in the existing `internal/fileutil` tar/gzip suite.

### F6 — Reuse one outbound-network policy and bind credentials to origins

- **Severity:** High
- **Confidence:** Medium (the unsafe code paths are certain; exploitability depends on which catalog/config inputs are trusted by callers outside this slice)
- **Fowler technique:** Extract Class; Move Function.
- **Evidence:** MCP `securehttp` blocks private/special destinations at URL and dial time and strips credentials on cross-origin redirects (`internal/mcp/securehttp/client.go:119-176`, `transport.go:18-84`). In contrast, curated HTTP archives validate only absolute HTTPS and redirects remaining HTTPS (`internal/registry/http_archive.go:114-145`). GitHub download URLs come from release metadata (`internal/registry/github/http.go:85-104`), while `newRequest` attaches `GITHUB_TOKEN` to every raw URL without checking origin (`http.go:165-174`). Git repository normalization validates HTTP(S) credentials/hostname only when those prefixes are present and accepts every other git/local/SCP-style form unchanged (`internal/registry/gitsrc/reference.go:11-29`).
- **Risk:** compromised/configurable metadata can target internal HTTPS services; response-provided GitHub asset/tarball URLs can receive the operator token on the initial request; git sources may bypass an intended remote-only/network boundary.
- **Recommendation:** extract a lower-level outbound policy usable by both MCP and registry code without introducing a registry→MCP dependency. Require an explicit destination policy per source, revalidate every redirect and resolved IP, and attach credentials only to an allowlisted origin. Replace raw repository strings with a validated `RepositoryRef` whose allowed schemes are an explicit product decision.
- **Canonical invariant/suite:** extend `internal/registry/http_archive_test.go`, `internal/registry/github/client_test.go`, and `internal/registry/gitsrc/client_test.go` with private/mixed-DNS, redirect, cross-origin-token, and disallowed-scheme cases; keep the DNS-rebinding contract in `internal/mcp/securehttp/client_test.go`.

### F7 — Standardize bounded read, drain, and exactly-once close

- **Severity:** Medium
- **Confidence:** High
- **Fowler technique:** Extract Function; Consolidate Duplicate Conditional Fragments.
- **Evidence:** three GitHub release endpoints use unbounded `io.ReadAll` (`internal/registry/github/releases.go:38,120,190`) and both defer a direct close and explicitly call `closeResponseBody`, causing double ownership (`releases.go:27-39,109-121,179-191`). `closeResponseBody` only closes (`release_selection.go:210-218`). MCP authorization challenges close without draining (`internal/mcp/executor_client.go:134-149`) even though `internal/mcp/auth/response_body.go:9-23` already defines drain-and-close. HTTP archive status/oversize branches also close without draining (`internal/registry/http_archive.go:73-103`). Daytona's SSH token source falls back to forbidden, timeout-free `http.DefaultClient` if its field is nil (`internal/sandbox/daytona/ssh.go:80-90`).
- **Risk:** adversarial JSON can exhaust memory; unread bodies prevent connection reuse; double close obscures ownership and can surface misleading errors with custom bodies; the default-client fallback can hang indefinitely.
- **Recommendation:** define a bounded JSON decoder/read helper and one drain-close owner per response. Normalize every injected client at construction with an explicit timeout; never fall back to `http.DefaultClient`. Share behavior, not package coupling, across registry/MCP/Daytona.
- **Canonical invariant/suite:** each owning HTTP client suite should prove oversized success/error bodies, exactly one close, drain failure joined with the operation error, retry reuse, and nil-injected-client timeout normalization.

### F8 — Bind Vault ciphertext to its ref/kind with AEAD associated data

- **Severity:** Medium
- **Confidence:** High
- **Fowler technique:** Introduce Parameter Object; Change Function Declaration.
- **Evidence:** `PutSecret` normalizes ref/kind but calls `encryptValue(key, plaintext)` (`internal/vault/service.go:70-103`); `ResolveRef` calls `decryptValue(key, record.EncryptedValue)` (`service.go:116-151`). AES-GCM seals and opens with nil associated data (`internal/vault/crypto.go:263-317`).
- **Risk:** anyone able to tamper with the SQLite rows can swap a valid encrypted value between refs/kinds; the ciphertext authenticates itself but not its ownership metadata, causing secret substitution without knowing the key.
- **Recommendation:** pass a versioned canonical ownership object (`ref`, `kind`, format version) as AEAD additional data for both seal/open. This is a greenfield hard cut: update the canonical storage/seed expectations rather than adding a fallback decrypt path.
- **Canonical invariant/suite:** “ciphertext copied to another ref or kind fails authentication”; extend `internal/vault/service_test.go`.

### F9 — Tie SDK `OnReady` work to extension lifecycle

- **Severity:** Medium
- **Confidence:** High
- **Fowler technique:** Extract Class; Replace Function with Command.
- **Evidence:** initialize starts callbacks with `context.Background()` (`sdk/go/extension_initialization.go:77`); late registrations do the same in an untracked goroutine (`sdk/go/extension.go:196-212`). Callback errors are printed to stderr and no join/cancel handle exists (`extension_context.go:72-94`). Shutdown only flips state and invokes the shutdown handler (`extension_shutdown.go:30-64`).
- **Risk:** extension-provided callback work can outlive `Run`, ignore shutdown/deadline, race late `OnReady` registration, and become impossible for embedders to join or observe structurally.
- **Recommendation:** create an extension lifecycle context at `Run`, cancel it on transport termination/shutdown, launch callbacks through an owned `WaitGroup.Go`, and define error aggregation/reporting semantics. Preserve asynchronous initialization response ordering.
- **Canonical invariant/suite:** “Run cancellation/shutdown cancels and joins all ready callbacks; late callback registration has deterministic semantics”; extend `sdk/go/runtime_test.go` and `transport_lifecycle_test.go` only where transport ownership itself is involved.

### F10 — Stop classifying behavior by dependency error strings

- **Severity:** Medium
- **Confidence:** High
- **Fowler technique:** Replace Primitive with Object; Extract Function.
- **Evidence:** stdio MCP accepts a normal termination by matching a dependency's formatted prefix/suffix (`internal/mcp/serve.go:85-97`). Marketplace lookup converts arbitrary errors to not-found by substring (`internal/skills/marketplace/helpers.go:59-78`), despite registry already providing `ErrPackageNotFound` and `PackageNotFoundError` (`internal/registry/source.go:10-37`).
- **Risk:** dependency wording changes break normal shutdown; unrelated errors containing “not found” are mapped to a false absence and can suppress actionable backend failures.
- **Recommendation:** isolate the MCP compatibility quirk in one dependency adapter with a tracked upstream issue and characterization test, then remove it when the SDK retains `io.EOF`. Make every registry source wrap `ErrPackageNotFound`; delete marketplace substring classification.
- **Canonical invariant/suite:** extend `internal/mcp/serve_test.go` (or its existing canonical serve suite) for wrapped EOF identity and `internal/skills/marketplace/service_test.go` for typed not-found versus message-only failures.

### F11 — Make cleanup failures observable, including in tests

- **Severity:** Medium
- **Confidence:** High
- **Fowler technique:** Move Statements into Function; Extract Function.
- **Evidence:** post-commit registry backup deletion silently returns on failure (`internal/registry/extract.go:299-311`); skill provenance ignores `file.Close` (`internal/skills/provenance.go:176-184`); Daytona sidecar stdout/stderr pipe closes are ignored (`internal/sandbox/daytona/cmd/compozy-daytona-sidecar/main.go:196-223`). Test code explicitly discards close/remove/kill/reply/copy errors in `internal/mcp/peer_test.go:41-87`, `internal/skills/loader_test.go:902-911`, `internal/sandbox/daytona/ssh_test.go:248-255`, and multiple registry HTTP tests.
- **Risk:** partial cleanup becomes invisible, tests pass while leaking resources or truncating server output, and post-commit backups accumulate without an operator-visible diagnostic.
- **Recommendation:** return joined cleanup errors where the caller still owns the operation; where commit already succeeded, emit a structured warning/partial-success diagnostic with the retained path. In goroutines, report close/copy failures through an error channel or synchronized test recorder. Reuse small test cleanup/write helpers rather than `_ =`.
- **Canonical invariant/suite:** keep each invariant in its existing owning suite; do not create static tests that merely grep for `_ =`.

### F12 — Consolidate registry concurrency/retry and stop mutating source-owned values

- **Severity:** Medium
- **Confidence:** High
- **Fowler technique:** Substitute Algorithm; Change Reference to Value; Replace Inline Code with Function Call.
- **Evidence:** `MultiRegistry` manually manages two parallel fanouts (`internal/registry/multi.go:64-94,250-270`) and logs source failures before returning the same joined errors when all sources fail (`multi.go:104-127`). `resolveSource` mutates a `*Detail` returned by a source, and `normalizeListings` mutates the returned slice in place (`multi.go:267-273,335-355`). ClawHub reimplements context sleep/exponential backoff and a retry loop (`internal/registry/clawhub/client_retry.go:8-39`, `client_request.go:16-82`) despite `internal/retry/retry.go:1-118` owning those semantics.
- **Risk:** duplicate retry semantics drift; all-failure paths are handled twice (log and return); source-owned cached values can be mutated or raced by the aggregator.
- **Recommendation:** use `WaitGroup.Go`; clone detail/listing values before normalization; log only degraded partial success and let the outer boundary log terminal errors; adapt ClawHub/GitHub retry policies to `internal/retry.DoValue` while preserving status policy, jitter, close-before-retry, and injected sleep.
- **Canonical invariant/suite:** extend `internal/registry/multi_test.go` with source-value immutability/concurrent reuse and retain existing concurrency/priority cases; keep source-specific retry policy assertions in their current client suites.

### F13 — Centralize path-policy primitives without flattening distinct policies

- **Severity:** Medium
- **Confidence:** High
- **Fowler technique:** Extract Class; Move Function.
- **Evidence:** similar containment/symlink logic exists in `internal/skills/path_security.go:8-39`, `internal/skills/marketplace/paths.go:17-109`, `internal/skillscan/scan.go:42-60`, `internal/registry/extract.go:331-430`, and `internal/providerenv/env.go:93-196`. Their existence/error rules differ: existing skill payloads require `EvalSymlinks`, install destinations resolve the deepest existing ancestor after repeated URL decoding/NFC normalization, extraction must safely create nonexistent descendants, and provider homes reject every symlink component.
- **Risk:** copying a nearly-correct helper into a new boundary is likely to omit encoding normalization, nonexistent-tail handling, platform canonicalization, or stable error identity.
- **Recommendation:** define named policy primitives in a low-level package—e.g. existing-path containment, deepest-existing containment, no-symlink creation, and root-handle extraction—with explicit options and stable sentinels. Keep small domain wrappers that translate errors. Do not collapse all four into a permissive generic helper.
- **Canonical invariant/suite:** retain domain-specific adversarial tests; share a conformance table only for truly identical path cases.

### F14 — Finish low-risk test/benchmark modernization

- **Severity:** Low
- **Confidence:** High
- **Fowler technique:** Substitute Algorithm.
- **Evidence:** two benchmark loops remain on `b.N` (`internal/fileutil/atomic_bench_test.go:27-31`, `internal/tools/perf_bench_test.go:96-101`); the artifact sweeper relies on wall-clock sleep (`internal/tools/artifact_sweeper_test.go:11-33`); Daytona integration evidence is unstructured log text (`internal/sandbox/daytona/ssh_validation_test.go:296-314`, `launcher_transport_integration_test.go:78-96`).
- **Recommendation:** use `b.Loop`, `testing/synctest` for the in-process ticker lifecycle, and `T.Attr` for stable metrics. Do not mechanically replace `t.TempDir`, and do not virtualize the real HTTP timeout test.
- **Canonical invariant/suite:** modify the existing benchmark/lifecycle/integration suites only.

### F15 — Preserve the 500-line boundary and document unavoidable reflection

- **Severity:** Low
- **Confidence:** High
- **Fowler technique:** Extract Class; Extract Function.
- **Evidence:** production high-water marks are `internal/tools/schema.go` (500), `internal/mcp/auth/manager.go` (492), `internal/tools/builtin/memory_admin.go` (489), `internal/tools/policy.go` (487), `internal/tools/registry.go` (474), and `internal/mcp/executor.go` / `internal/mcp/auth/service.go` (473 each). Reflection is used for JSON-schema equality, typed-nil detection, generated host schema, and structured redaction (`internal/tools/schema.go:205`, `internal/tools/errors.go:183`, `internal/sandbox/registry.go:89`, `internal/mcp/serve_projection.go:101`, `internal/redact/json.go:35`, `slog.go:93`) without adjacent performance rationale.
- **Recommendation:** `schema.go` is at the ceiling and must be split before any growth; other near-cap files should receive new behavior through focused files. Keep reflection where it expresses unavoidable dynamic contracts, but add the required adjacent rationale and benchmark/profile reference; consolidate identical typed-nil logic only if a natural low-level owner exists.
- **Canonical invariant/suite:** behavior tests remain canonical; the file cap and generated-boundary gates, not prose/static snapshot tests, should enforce structure.

## Risks / Mismatches

- **`os.OpenRoot` is not a universal path sanitizer.** It is the correct primitive for archive extraction and other root-owned filesystem operations. It does not replace the project requirement to repeatedly decode/NFC-normalize agent paths and resolve the deepest existing ancestor, nor the `/private`/platform canonicalization handled by realpath-based policies.
- **Public SDK slice contracts should not be changed to iterators opportunistically.** `iter.Seq` would affect external consumers, error timing, ordering, and reuse. Generated `sdk/go/contracts` outputs must be changed only through their generator owner.
- **`Process.WithHandle` cannot be adopted only in `toolruntime`.** Durable recovery uses PID, start time, and process-group identity across restarts; the platform abstraction is `internal/procutil`, outside this slice. A local handle patch would regress group signaling or recovered-process support.
- **`cmp.Or` is a semantic mismatch for normalized defaults.** Eager evaluation and zero-value selection do not perform trimming, validation, or lazy initialization.
- **`unique` and `iter.Seq` require evidence.** Interning adds global lifetime behavior; iterators can retain locks or shift errors into consumption. Neither should precede heap/CPU profiles.
- **Flight recorder ownership is architectural.** A recorder should be daemon-owned with explicit buffer, trigger, privacy/redaction, artifact-retention, and agent-management surfaces. Placing it in a library package would be an ownership leak.
- **`CrossOriginProtection` needs protocol tests, not a blind wrapper.** Place it outside bearer authentication so cross-site unsafe requests are rejected early, while confirming ordinary MCP clients with no browser headers still pass.
- **Retry consolidation must preserve response ownership.** Moving ClawHub/GitHub to `internal/retry` is safe only if every retryable response is drained/closed exactly once before delay and status-specific behavior remains visible.
- **Vault AAD is a hard storage cut.** Do not add a fallback decrypt path for old ciphertext; greenfield policy favors regenerating/rebinding alpha state and deleting the old format.
- **No current 500-line violation exists.** Treat exactly-500 `internal/tools/schema.go` as blocked from growth, not as evidence that all large files are god files.
- **Static-analysis limitation:** no compile, race, unit, integration, fuzz, or benchmark command was allowed. Every proposed fix therefore needs its canonical suite plus `make gate`/`make gate-full` under the implementation workflow.

### Compozy Impact Audit for implementing these recommendations

- **Native tools:** notification, registry/marketplace, extension-install, MCP, approval, and artifact behaviors are exposed through native tool descriptors/executors; schema IDs need not change for internal hardening, but error reasons, availability diagnostics, and end-to-end contract tests must be checked.
- **Extensibility and hooks:** changes affect MCP transports/auth, registry sources, marketplace skill installation, the public Go extension SDK, provider probes, and notification bridge delivery. Hook/event payloads should remain stable; lifecycle semantics and error identity require documentation.
- **Workspace data isolation:** notification cursor/delivery identity and provider pre-start caching are directly affected. Prove workspace propagation through caller scope, service, store keys, events/SSE, caches, and tests; foreign-workspace behavior must remain indistinguishable from absence where applicable.
- **Official Compozy skill:** if public native-tool behavior, CLI paths, MCP auth diagnostics, marketplace error reasons, or SDK lifecycle guarantees change, update `skills/compozy/`; otherwise document exact checked surfaces before declaring no impact.

## Open Questions

1. The `security-review` skill referenced a Go-specific `languages/go.md`, but that resource was absent from the installed skill bundle. The project and `golang-master` rules were applied; should the missing guide be restored before implementation review?
2. Will the upstream MCP Go SDK preserve `io.EOF` (or expose a typed normal-close error), allowing deletion of `isExpectedStdioServerTermination` string matching?
3. Which browser-origin combinations must the loopback MCP HTTP endpoint support? A compatibility matrix is needed before enabling `CrossOriginProtection`.
4. Is `gitsrc` intentionally allowed to accept local paths, `file://`, SSH, and SCP-style references, or should the agent-manageable `git` source be HTTPS-only with an explicit host/network policy?
5. What canonical workspace/runtime-home identifier is available at provider `PreStart` composition time? `ProbeEnv` currently has no explicit workspace field, so a safe cache cannot infer scope.
6. May Vault ciphertext be invalidated wholesale for the AAD hard cut, or is there alpha state that the operator wants exported/rebound first?
7. Should `Process.WithHandle` be evaluated in a dedicated `internal/procutil` cross-platform slice that includes recovered processes and process groups?
8. Do heap/CPU profiles identify string-key duplication or slice materialization in tools/skills/registry as meaningful? Without that evidence, `unique` and `iter.Seq` remain deferred.
9. Should Daytona/registry integration metrics be retained as CI artifacts, or are structured `T.Attr` values sufficient? That decision determines whether `T.ArtifactDir` is useful.

## Evidence

### Coverage and constraints

- Scoped inventory: 512 Go files / 112,253 lines; 122 test files; five benchmark files; 31 generated contract files; 359 non-test/non-generated production files / 58,672 lines.
- Production high-water mark: `internal/tools/schema.go:1-500`; no scoped production file is over 500 lines.
- Toolchain: `go.mod:3`; `sdk/go/go.mod:3`.
- Review baseline: `/home/pedronauck/.codex/attachments/36398ca5-d255-4ce7-92d3-0d3429f3838f/pasted-text-1.txt`.
- Generated evidence only: `sdk/go/contracts/capabilities_gen.go:1`; `sdk/go/contracts/hook_contracts_gen.go:1`; `sdk/go/contracts/host_contracts_gen.go:1`; `sdk/go/contracts/types_001_gen.go:1`; `sdk/go/contracts/types_026_gen.go:1`.

### Deduplicated source evidence

- `internal/notifications/presets/service_helpers.go:38-58,86-90` — global/workspace cursor identity, delivery IDs, detached context.
- `internal/notifications/presets/types.go:235-275` — event normalization/validation does not constrain workspace or event-ID grammar.
- `internal/notifications/presets/task_observer.go:28-73` — synchronous dispatch on an unbounded `WithoutCancel` context.
- `internal/providers/probe_env.go:18-64` — environment/home inputs omitted by cache identity.
- `internal/providers/prestart.go:15-62,98-106,123-139` — global cache, command env use, incomplete FNV key.
- `internal/redact/dynamic.go:12-73` — ref-counted global registry and returned idempotent cleanup.
- `internal/mcp/executor_client.go:25-183,205-350` — managed-client cleanup, undrained auth challenges, isolated stdio home, discarded redaction cleanup.
- `internal/mcp/executor_auth.go:157-176` — bearer token registered on each header construction.
- `internal/mcp/auth/types.go:30-77` — collision-safe target validation/key precedent.
- `internal/mcp/auth/response_body.go:1-23` — existing drain-and-close behavior.
- `internal/mcp/securehttp/client.go:104-194` — URL/redirect/timeout policy and credential stripping.
- `internal/mcp/securehttp/policy.go:13-95` — blocked IP/scheme/host policy.
- `internal/mcp/securehttp/transport.go:18-84` — DNS-to-dial enforcement and response-size wrapper.
- `internal/mcp/securehttp/client_test.go:26-329` — private/mixed DNS, rebinding, redirect, header, body, and redaction cases.
- `internal/mcp/serve.go:70-97` — dependency error-string normal-close classification.
- `internal/mcp/serve_http.go:25-137` — loopback server, owned shutdown, bearer handler, CrossOriginProtection insertion point.
- `internal/registry/extract.go:64-254,256-430` — extraction limits, path-based create/check race, backup cleanup.
- `internal/registry/extract_test.go:113-234,293-326` — static traversal/symlink and cleanup tests; no concurrent swap case.
- `internal/sandbox/daytona/tar_extract.go:13-143` — safe `os.OpenRoot` extraction precedent.
- `internal/fileutil/targzip.go:17-105` — full in-memory archive materialization and correct close joining.
- `internal/registry/gitsrc/client.go:117-204` — clone timeout, post-materialization limit, temp cleanup.
- `internal/registry/gitsrc/reference.go:11-39` — partial repository-reference validation.
- `internal/registry/github/http.go:17-250` — metadata-provided URLs, unconditional token header, retry/body ownership.
- `internal/registry/github/releases.go:17-228` — three unbounded reads and double-close structure.
- `internal/registry/github/release_selection.go:170-218` — bounded error preview and close-only helper.
- `internal/registry/http_archive.go:27-151` — HTTPS-only URL/redirect policy without dial-time IP validation.
- `internal/registry/clawhub/client_request.go:16-147` — duplicated retry loop and bounded-but-not-drained error reads.
- `internal/registry/clawhub/client_retry.go:1-39` — duplicate context sleep/backoff.
- `internal/retry/retry.go:1-118` — canonical context-aware retry abstraction.
- `internal/registry/multi.go:53-130,218-395` — manual fanout, log-and-return, source-owned mutation.
- `internal/skills/path_security.go:8-39` — existing-path realpath containment.
- `internal/skills/marketplace/paths.go:17-109` — repeated URL decode, NFC, deepest-existing realpath containment.
- `internal/skillscan/scan.go:42-60` — skill definition symlink containment.
- `internal/providerenv/env.go:93-196` — no-symlink provider directory creation and resolved containment.
- `internal/skills/provenance.go:48-219` — deterministic payload hash and ignored file-close error.
- `internal/skills/marketplace/helpers.go:59-78` — message-substring not-found classification.
- `internal/tools/approval_token.go:120-328` — workspace/session/agent/input-bound single-use token.
- `internal/tools/artifact_store.go:68-419` — workspace-hashed storage, bounded read/write, retention.
- `internal/tools/artifact_store_test.go:17-186` — foreign-workspace absence and retention cases.
- `internal/network/manager_send.go:25-145` — bounded detached durable commit and lifecycle cancellation.
- `internal/network/envelope_security.go:12-100` — raw-secret rejection.
- `internal/vault/mcp_refs.go:25-124` — collision-safe MCP secret ownership.
- `internal/vault/service.go:70-151` — ref/kind lifecycle not passed into encryption.
- `internal/vault/crypto.go:263-317` — AES-GCM with nil associated data.
- `sdk/go/transport.go:102-248,251-424` — lazy transport lifecycle, bounded dispatch, untracked request goroutines.
- `sdk/go/transport_framing.go:12-35` — bounded line reader.
- `sdk/go/extension_initialization.go:10-79` — background ready-callback launch.
- `sdk/go/extension.go:196-212` — late `OnReady` background goroutine.
- `sdk/go/extension_context.go:72-94` — callback cloning and stderr-only error handling.
- `internal/tools/artifact_sweeper_test.go:11-33` — wall-clock shutdown assertion suitable for `synctest`.
- `internal/fileutil/atomic_bench_test.go:16-33` — `b.N` holdout.
- `internal/tools/perf_bench_test.go:84-104` — `b.N` holdout.
- `internal/mcp/peer_test.go:35-90` — discarded close errors in tests.
- `internal/skills/loader_test.go:883-914` — discarded remove/close errors in test injection.
- `internal/sandbox/daytona/ssh.go:38-101` — explicit default client plus forbidden nil fallback.
- `internal/sandbox/daytona/cmd/compozy-daytona-sidecar/main.go:185-230` — ignored stdout/stderr pipe close errors.
