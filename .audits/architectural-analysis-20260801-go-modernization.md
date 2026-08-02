# Architectural Analysis Report

**Date**: 2026-08-01
**Files Analyzed**: 5,094 Go files
**Dead Code Files**: 0 proven
**Duplication Groups**: 9 confirmed

---

## Executive Summary

- **Dead Code**: no file or exported symbol met the repository-wide proof threshold for deletion
- **Duplicated Functionality**: 9 confirmed groups, 5 suitable for focused consolidation and 4 requiring staged architectural work
- **Architectural Anti-Patterns**: 9 confirmed ownership/cohesion issues
- **Type Issues**: 3 primitive or mirrored-contract risks; no type-erasure bypass comparable to TypeScript `any`
- **Code Smells**: 19 prioritized instances/groups

**Estimated Cleanup**: no speculative dead-code deletion; focused consolidation can remove several parallel mappers/helpers while the route/dependency work should optimize change safety, not line count.

---

## Dead Code

### Completely Dead Files (DELETE)

None found. The package and symbol survey did not establish a hand-owned production file with no compiler, registry, generated-descriptor, reflection, command-wiring, manifest, or public-API reachability.

**Total Lines**: 0 lines proven safe to delete.

### Dead Exports (REMOVE)

None proven. Candidate exports found by local reference counts cross package, command, generator, or extension boundaries and require whole-repository reachability proof before deletion.

### Possibly Dead (VERIFY)

| File                             | Export                  | Reason                                                       | Verification Needed                                                                                                    |
| -------------------------------- | ----------------------- | ------------------------------------------------------------ | ---------------------------------------------------------------------------------------------------------------------- |
| `internal/registry/installer.go` | `PrimeInstallDetection` | Sparse direct references were reported in the surface slice. | Trace daemon/CLI composition and registry interface wiring; retain unless compiler/static analysis proves unreachable. |

### Internal Dead Code

None proven. Commented examples and test fixtures were not classified as dead production code.

---

## Duplicated Functionality

### CRITICAL: Exact Duplicates

#### Duplication Group 1: Task-run row projections and mappers

**Instances**: 3 read paths
**Files**:

- `internal/store/globaldb/global_db_task_runs.go:38`
- `internal/store/globaldb/task_core_mapping.go:62`
- `internal/store/globaldb/global_db_task_run_scan.go:14`
- generator owner `internal/store/globaldb/queries/task_core.sql:152`

**Analysis**: `GetTaskRun` and status listing depend on separate generated row types and parallel 40-plus-field mappings, while the dynamic list path already has a canonical projection/scanner. Workspace identity, network participation, review lineage, claim-token redaction, and ordering can drift when the schema changes.

**Recommendation**: make one hand-owned projection/scanner the read boundary, remove redundant named queries at the SQL source, and regenerate sqlc output. Do not edit generated Go.

#### Duplication Group 2: Deterministic extension map-key collection

**Instances**: 2 exact same-package helpers
**Files**:

- `internal/extension/manifest_tool_config.go:171`
- `internal/extension/manager_clone.go:14`

**Analysis**: both collect map keys and sort the result for deterministic validation/resource loading.

**Recommendation**: retain one helper implemented as `slices.Sorted(maps.Keys(values))`; update callers and delete the other.

### HIGH: Similar Logic

#### Duplication Group 3: HTTP and UDS route declaration

**Instances**: 2 large route registrars
**Files**:

- `internal/api/httpapi/routes.go:6`
- `internal/api/udsapi/routes.go:5`

**Analysis**: most domain route method/path wiring is repeated, while HTTP-only webhooks/OAuth/OpenAI/static policy and UDS-only privileged identity/MCP routes are intentional deltas.

**Recommendation**: after HTTP security policy is stable, introduce a shared ordinary-operation registrar or descriptor checked against `api/spec`; keep transport-only routes and middleware explicit.

#### Duplication Group 4: Surface dependency bags

**Instances**: 3 large service graphs
**Files**:

- `internal/api/httpapi/server.go:35`
- `internal/api/httpapi/handlers.go:24`
- `internal/api/udsapi/server.go:52`

**Analysis**: constructor/service changes fan out through parallel server and handler configurations.

**Recommendation**: construct shared core handlers from a validated, cohesive dependency assembly after route consolidation clarifies common ownership. Do not replace three lists with one unbounded god config.

#### Duplication Group 5: Unix-socket dial closures

**Instances**: 3
**Files**:

- `internal/cli/client_settings_vault.go:28`
- `internal/cli/client_window_manager_stream.go:38`
- `internal/testutil/e2e/runtime_harness.go:248`

**Analysis**: stringly typed `DialContext("unix", path)` adapters repeat path/cancellation behavior; generic client construction is misplaced in a settings-vault file.

**Recommendation**: one typed `DialUnix` helper in the CLI transport owner; reuse it for HTTP and websocket adapters. Keep the E2E helper local if importing CLI would invert dependencies.

#### Duplication Group 6: Bridge JSON normalization

**Instances**: 3 variants
**Files**:

- `internal/bridges/json.go:8`
- `internal/bridges/contract/json.go:9`
- `internal/api/contract/bridge_json_payload.go:108`

**Analysis**: clone/compact/empty/object checks overlap, while API validation and error prefixes are distinct public contracts.

**Recommendation**: extract a low-level normalization primitive below both bridge packages; retain package-specific wrappers for validation and errors.

#### Duplication Group 7: Session acceptance adapter

**Instances**: 2 exact transport adapters
**Files**:

- `internal/api/httpapi/handlers.go:202`
- `internal/api/udsapi/server_handlers.go:95`

**Analysis**: both adapt the same session acceptance behavior into transport handlers.

**Recommendation**: move the adapter to shared API core when ordinary route registration is consolidated.

#### Duplication Group 8: Repeated database-row cleanup pattern

**Instances**: 8 production sites
**Files**:

- `internal/resources/kernel_records.go:101`
- `internal/resources/kernel.go:384`
- `internal/memory/catalog_operations.go:138`
- `internal/memory/catalog_query.go:123`
- `internal/memory/observability.go:46`
- `internal/observe/tasks_snapshot.go:178`

**Analysis**: explicit `_ = rows.Close()` repeats a forbidden cleanup pattern. `internal/store/globaldb` already has a canonical joined-close helper.

**Recommendation**: adopt an owning-package helper or named-return `errors.Join` pattern that preserves the primary query/scan error and reports close failure.

#### Duplication Group 9: Hook subprocess platform wrappers

**Instances**: 2 byte-identical build-tagged implementations
**Files**:

- `internal/hooks/executor_subprocess_unix.go:14`
- `internal/hooks/executor_subprocess_windows.go:14`

**Analysis**: configure/signal/kill/force-exit behavior is identical on both platforms because the actual OS policy already lives in `internal/procutil`. Parallel wrappers invite asymmetric fixes without carrying a platform distinction.

**Recommendation**: consolidate the hooks wrapper into one file and keep platform-specific behavior/tests at the `procutil` owner. Validate both platform builds.

### Type Duplication

Mirrored task-run, bridge-domain, and wire DTOs are intentionally distinct ownership boundaries. Consolidate mapping code, not the types, unless generation becomes the canonical owner. Type aliases would couple storage/domain/wire evolution and can introduce slice or `json.RawMessage` aliasing.

---

## Architectural Anti-Patterns

### Unowned Background Work

#### `internal/support/service.go:119`

**Responsibilities**: accepts an operation, detaches its request context, starts bundle construction, and updates operation state.
**Issue**: the goroutine has no daemon owner, admission shutdown, join, or store-lifetime ordering.
**Recommendation**: service owner context + admission state + `WaitGroup` + `Shutdown(ctx)`, composed into daemon shutdown after server drain and before stores close.

### Incomplete Concurrency Ownership

#### `internal/subprocess/transport.go`

**Issue**: inbound request handlers are launched independently of the reader lifecycle and are not joined before completion is published.
**Recommendation**: transport-owned handler `WaitGroup`; stop/join the reader, then join cooperative handlers.

### Layered Security Assumption Gap

#### `internal/api/httpapi/middleware.go:83`

**Issue**: same-origin comparison trusts request `Host`, so DNS rebinding can make attacker-controlled Host and Origin agree. Standard CSRF protection shares that assumption.
**Recommendation**: validate the request target against configured bind identity and then apply cross-origin mutation checks at the ordinary API group.

### Mixed Responsibilities

#### `internal/sandbox/daytona/tar.go`

**Responsibilities**: archive writing, extraction, path validation, symlink policy, file metadata, and cleanup.
**Issue**: path validation and path-based opens are separated, producing a TOCTOU boundary; future root-relative extraction work would further mix concerns.
**Recommendation**: split archive writing from extraction and make `os.Root` the extraction capability.

### Large Transport Registrars

- `internal/api/httpapi/routes.go` is 467 lines.
- `internal/api/udsapi/routes.go` is 484 lines.
- `internal/tools/schema.go` is exactly 500 lines.

No hand-owned production file currently exceeds the cap, but the two route files duplicate a change surface and `schema.go` cannot grow. Split by responsibility before adding behavior.

### Unbounded Public-SDK Framing

#### `sdk/go/transport.go:250`

**Issue**: `ReadBytes` allocates the full newline-delimited frame before enforcing the advertised maximum.
**Recommendation**: bounded fragment reader in a focused file, with EOF-partial and exact-boundary contract tests.

### Memoized Incomplete Teardown

#### `internal/testutil/e2e/runtime_harness_process.go:20`

**Issue**: `sync.Once` memoizes the first stop attempt, while a signaled process plus `context.Canceled` is treated as success before confirmed exit. Later cleanup cannot retry.
**Recommendation**: cache confirmed terminal completion, not the first attempt; make a canceled/incomplete attempt retryable and preserve bounded force/join cleanup.

### Unbounded Hook Cancellation Join

#### `internal/hooks/executor_subprocess_lifecycle.go:61`

**Issue**: after a grace timeout and kill request, cancellation receives from `waitCh` without another bound, so failed termination can block before the later force-group cleanup.
**Recommendation**: perform bounded force termination before final join and return joined execution/checkpoint/cleanup errors.

### Unbounded Workspace-Keyed State

#### `internal/deadentity/service.go:230`

**Issue**: a daemon-global map inserts one mutable state per workspace/entity key and has no observed eviction lifecycle.
**Recommendation**: defer mutation until a cardinality, idle lifetime, and safe unregister event are defined. Eviction must not split one key into two live state objects.

### Circular Dependencies

None detected at the package level by the Go build graph. Proposed shared helpers must preserve that property; specifically, testutil must not import CLI merely to reuse a UDS helper, and a bridge JSON helper must sit below both consumers.

### Tight Coupling

The HTTP/UDS servers and daemon boot graph have high efferent coupling by design as composition roots. The risk is shotgun surgery when one service or route changes. Narrow registrars and dependency assemblies are appropriate; pushing domain behavior into the composition root is not.

### Layer Violations

None confirmed. Generated SDK contracts correctly originate in internal registries/generators, and storage query changes must continue to originate in SQL sources.

---

## Type Issues

### Primitive Identity and Scope

| File                                           | Line | Context                                                                                                   | Severity                                                                |
| ---------------------------------------------- | ---- | --------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------- |
| `internal/memory/header_list.go`               | 34   | `WorkspaceID` participates in cursor/storage identity but looks like an ordinary row filter.              | MEDIUM — a naive refactor breaks global-memory behavior.                |
| `internal/api/httpapi/middleware.go`           | 83   | host, origin, bound host, scheme, and port begin as strings with different trust/normalization semantics. | HIGH — attacker-controlled and configured identities can be conflated.  |
| `internal/store/globaldb/task_core_mapping.go` | 62   | large generated row types are converted manually into one domain aggregate.                               | HIGH — field drift, especially workspace and security-sensitive fields. |

**Recommendation**: keep explicit domain types/normalization boundaries, centralize mapping, and name trust/scope in helper signatures. Do not create a generic string normalizer.

### Type Assertions

No systemic unsafe type assertion pattern was confirmed. Go interface assertions in transport/domain adapters are generally validated or compile-time checked.

### Error Suppression

Explicit `_ = cleanup()` is the Go equivalent of suppressing a meaningful effect. Confirmed production sites are listed in the cleanup duplication group and must use a joined-error or explicit policy.

---

## Code Smells

### Long Functions / Large Files

| File                              | Function / area                   | Lines    | Issue                                                        |
| --------------------------------- | --------------------------------- | -------- | ------------------------------------------------------------ |
| `internal/api/httpapi/routes.go`  | `RegisterRoutes` + route families | file 467 | broad repeated change surface with UDS registrar             |
| `internal/api/udsapi/routes.go`   | route families                    | file 484 | broad repeated change surface with HTTP registrar            |
| `internal/tools/schema.go`        | schema responsibilities           | file 500 | at hard production cap; must split before growth             |
| `internal/sandbox/daytona/tar.go` | write + extract                   | file 356 | mixed responsibilities and security-sensitive path lifecycle |

### Complex Conditionals

| File                                    | Line | Issue                                                                                                                                       |
| --------------------------------------- | ---- | ------------------------------------------------------------------------------------------------------------------------------------------- |
| `internal/api/httpapi/middleware.go`    | 83   | origin/request/bind trust policy is encoded as permissive equality branches rather than an explicit request-target compatibility predicate. |
| `internal/automation/dispatch_retry.go` | 132  | shift/multiply arithmetic assumes the configured attempt cannot overflow.                                                                   |

### Magic Numbers

No actionable magic-number duplication was confirmed. Important limits such as request size, support bundle caps, and retry defaults are already named constants or configuration fields.

### Commented-Out Code

None found that qualifies as deletable production implementation.

### Other Smells

- **Long Parameter Lists / Data Clumps**: repeated surface service graphs in HTTP/UDS construction.
- **Shotgun Surgery**: route and task-run schema changes require parallel edits.
- **Primitive Obsession**: host/origin/scope identities begin as strings with different contracts.
- **Speculative Generality risk**: blanket iterators, interning, and flight recording were proposed without an owning measurable contract and are rejected/deferred.
- **Inappropriate Intimacy risk**: domain/wire mapper consolidation must not become type aliasing across storage, domain, and public contract layers.

---

## Statistics

**Dead Code**:

- Files: 0 proven
- Exports: 0 proven
- Lines: 0 safe to delete

**Duplication**:

- Groups: 9
- Files affected: at least 20 hand-owned sources plus generated outputs downstream of SQL
- Duplicated lines: not used as the optimization target; contract drift is the primary cost

**Architectural Issues**:

- Unowned/incomplete lifecycle boundaries: 6
- Security/validation ownership gaps: 2
- Mixed-responsibility or near-cap areas: 3
- Circular package dependencies: 0 detected

**Type Issues**:

- Primitive/scope ambiguity groups: 2
- Mirrored mapper drift group: 1
- Unsafe type-erasure pattern: none found

**Code Smells**:

- Bloaters/mixed responsibility: 4
- Change preventers/shotgun surgery: 3
- DRY groups: 9
- Conditional/correctness hazards: 2

---

## Impact Assessment

### Code Cleanup Potential

- **Dead code removal**: 0 proven lines
- **Duplication consolidation**: focused helpers/mappers can reduce repeated logic, but no responsible total is claimed before implementation
- **Total reduction**: secondary to one-owner correctness and smaller change surfaces

### Maintainability Improvement

- one task-run scanner prevents schema-field drift;
- one deterministic-key helper and one UDS dial seam remove local duplication;
- explicit lifecycle ownership makes daemon shutdown auditable;
- layered Host/CSRF policy separates configured identity from browser claims;
- root-relative extraction removes a whole class of path races;
- generated ownership remains intact.

### Risk Areas

- HTTP/UDS registration and middleware order;
- daemon boot/shutdown ordering;
- task-run workspace and claim-token fields;
- SDK framing EOF and maximum-size boundaries;
- archive symlink semantics across platforms;
- retryable E2E teardown and bounded hook-process joining;
- dead-entity state cardinality and safe eviction;
- test-only fake time around external I/O;
- trace/artifact evidence that may contain cross-workspace or secret data.
