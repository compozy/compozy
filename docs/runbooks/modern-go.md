# Modern Go compliance

## Checked modules

`make go-lint` runs the root source policy and golangci-lint v2.13.1 using the
repository `.golangci.yml`. `make lint` and the CI Go lint job call that same lane.
An unset `COMPOZY_GO_LINT_SCOPES`, or `./...`, checks every row below.
The Mage suite compares the dispatch list with tracked nested `go.mod` files,
so adding a module requires updating lint coverage.

| Directory | Declared Go version | Package scope / build tag |
| --- | --- | --- |
| Repository root | 1.27.1 | `./...`, default and `integration` |
| `magefiles` | 1.27.1 (root module) | `./...`, `mage` |
| `sdk/go` | 1.26.4 | `./...` |
| `sdk/examples/notes-commands` | 1.26.4 | `./...` |
| `sdk/examples/clarify-tool` | 1.26.4 | `./...` |
| `internal/extension/testdata/command-fixture-go` | 1.26.4 | `./...` |
| `internal/extension/testdata/palette-fixture-go` | 1.26.4 | `./...` |

Fixture authoring tests copy the module and replace the SDK with `sdk/go`.
Lint resolves that same local SDK through a disposable Go workspace containing
only the fixture and SDK modules. The workspace Go directive is the maximum of
the member module requirements. Its workspace and sum file are removed after
lint. The checked-in fixture modules and their runtime build behavior are unchanged.
A `GOFLAGS=-modfile=...` override is unsuitable here: the bundled `go/packages`
queries the Go version with modules disabled, which rejects that flag.

## Local checks and gate selection

Run commands from the repository root, with the pinned tools on PATH:

```sh
mise exec -- make go-lint
COMPOZY_GO_LINT_SCOPES='./sdk/go/...' mise exec -- make go-lint
COMPOZY_GO_LINT_SCOPES='./sdk/examples/notes-commands/...' mise exec -- make go-lint
COMPOZY_GO_LINT_SCOPES='./sdk/examples/clarify-tool/...' mise exec -- make go-lint
COMPOZY_GO_LINT_SCOPES='./internal/extension/testdata/command-fixture-go/...' mise exec -- make go-lint
COMPOZY_GO_LINT_SCOPES='./internal/extension/testdata/palette-fixture-go/...' mise exec -- make go-lint
COMPOZY_GO_LINT_SCOPES='./magefiles/...' mise exec -- make go-lint
```

Scopes are space-separated repository-relative package patterns. A nested-module
scope is translated to a pattern relative to that module. A root package scope
stays narrow; a recursive ancestor also selects supported modules below it.
Root scopes run both the full default-tag lint set and an `integration`-tag pass
restricted to `modernize,forbidigo,depguard,copyloopvar` with `--enable-only`.
This includes integration-tagged tests and `scripts/gate_integration_test.go`
when their root scopes are selected; default `./...` covers them in CI.
Nested modules and Mage retain their existing tag sets (no integration-only
source files exist there). The first failing pass returns a nonzero exit status.

`COMPOZY_GO_LINT_CONCURRENCY` applies to each sequential invocation. The existing
cache override remains supported, including relative paths resolved from the
repository root. `COMPOZY_GO_LINT_FORMATTERS=run` checks formatters with each
module; `split` checks changed Go files across module boundaries before analysis.
The default is `run` in CI and `split` locally.

`bash scripts/gate.sh plan` reports the selected commands. SDK Go changes select
module lint and race tests. Go source, `go.mod`, and `go.sum` changes under SDK
examples select that module's Go lanes. Non-document assets in Go-only examples
also select Go lanes (for example, embedded templates); examples with `package.json` retain
JavaScript lanes. Fixture changes select fixture lint and
`make go-fixture-check GO_FIXTURE_MODULE=<module>` (build + vet against the local
SDK, with a disposable binary). Their runtime consumers are
`internal/daemon/daemon_extension_commands_e2e_integration_test.go`
(`integration && !windows`) and `desktop/e2e/_electron/__tests__/shell.spec.ts`;
those heavy E2E journeys remain owned by CI. Mage changes select tagged lint plus the existing
Mage/script tests. Each module lane keeps its own content-keyed evidence record. When root lint
uses `./...`, module/Mage lint records reference its current passing log instead
of rerunning lint. Module race tests and fixture build/vet still run.
CI classifies nested module manifests as backend changes, and its lint cache key
includes every module manifest and sum file.

## Rule enforcement map

The version-pinned CLI returns 54 IDs for Go 1.27.1 and 48 for Go 1.26.4.
The first six rows below are root-only. The map is based on the installed binary's
`go version -m` output (`golang.org/x/tools v0.49.0`) and that version's
[modernize suite](https://pkg.go.dev/golang.org/x/tools@v0.49.0/go/analysis/passes/modernize).
The golangci adapter uses `modernize.Suite`, not every analyzer exported by that
package: `bloop` and `fmtappendf` exist but are explicitly omitted upstream.
`stditerators` handles standard-library Len/At APIs, not arbitrary collection loops.

“Review” means no complete mechanical check: the reviewer must establish lifetime,
aliasing, nil/empty, evaluation-order, or API ownership equivalence. Analyzer
coverage applies only to the patterns it recognizes; it does not waive review of
other applicable patterns. Generated outputs are excluded from direct migration;
their generators and drift gates retain ownership.

| Guideline ID | Enforcement |
| --- | --- |
| `generic_methods` | Review: receiver ownership and exported API compatibility |
| `json_v2` | Not applicable to this migration; existing v1 encoding is preserved |
| `promoted_field_literals` | modernize `embedlit`; review pointer embedding and mixed fields |
| `strings_bytes_cut_last` | Review: bundled `stringscut` does not implement CutLast |
| `stdlib_uuid` | depguard rejects `github.com/google/uuid` in root-module source only |
| `url_clone` | Review: deep versus shallow copy ownership |
| `new_expression` | modernize `newexpr`; review pointer helper calls and aliasing |
| `errors_as_type` | modernize `errorsastype` plus type-aware forbidigo on `errors.As` |
| `sync_waitgroup_go` | modernize `waitgroupgo`; review admission locks and batched Adds |
| `testing_t_context` | modernize `testingcontext` for cancel/cleanup idioms; review other Background/TODO uses and cleanup lifetime |
| `json_omitzero` | modernize `omitzero` for struct fields; review scalar omission and wire contracts |
| `testing_b_loop` | Review: upstream omits `bloop` because conversion can change benchmark measurements |
| `strings_split_seq` | modernize `stringsseq` |
| `maps_keys_values_iter` | Review: streaming versus materialization and map mutation |
| `slices_collect` | Review: iterator and nil/empty contracts |
| `slices_sorted` | Review: collection nilness and ordering |
| `time_tick_gc` | Review: Stop/Reset and lifecycle ownership |
| `range_over_int` | modernize `rangeint` |
| `loopvar_capture` | modernize `forvar` plus default copyloopvar self-copy checks; renamed copies can preserve required mutation semantics |
| `cmp_or` | Review: arguments are evaluated eagerly |
| `reflect_type_for` | modernize `reflecttypefor` |
| `http_servemux_patterns` | Review: method, malformed-path, wildcard and route precedence contracts |
| `min_max` | modernize `minmax` |
| `clear` | Review: deletion/zeroing loops and mutation semantics |
| `slices_contains` | modernize `slicescontains` (also ContainsFunc) |
| `slices_index` | Review: -1 versus custom absent result |
| `slices_index_func` | Review: predicate effects and absent result |
| `slices_sort_func` | type-aware forbidigo on `sort.Slice`, `SliceStable`, `Sort`, `Stable`; preserve stability and comparator ties |
| `slices_sort` | modernize `slicessort` for simple Slice comparisons; forbidigo on `sort.Strings`, `Ints`, `Float64s` |
| `slices_max_min` | Review: empty-slice result and custom comparison |
| `slices_reverse` | Review: swap-loop bounds and aliases; `slicesbackward` only handles backward iteration |
| `slices_compact` | Review: adjacent duplicates, tail clearing, aliases |
| `slices_clip` | modernize `slicesclip` |
| `slices_clone` | Review: append-to-nil returns nil for an empty nonnil input; Clone does not |
| `maps_clone` | modernize `mapsloop` for nil-preserving patterns; review others |
| `maps_copy` | modernize `mapsloop` |
| `maps_delete_func` | Review: predicate side effects and traversal |
| `sync_once_func` | Review: reset, multiple Do sites, and re-panic behavior |
| `sync_once_value` | Review: memoization ownership and reset behavior |
| `context_after_func` | Review: cancellation versus joining the callback |
| `context_timeout_deadline_cause` | Review: observable error causes |
| `strings_clone` | Review: backing-memory ownership |
| `bytes_clone` | Review: nil versus empty and alias ownership |
| `strings_cut_prefix_suffix` | modernize `stringscutprefix` |
| `errors_join` | Review: error identity, formatting and wrapping contracts |
| `context_cancel_cause` | Review: observable cancellation causes |
| `fmt_appendf` | Review: upstream intentionally omits `fmtappendf` from its default suite |
| `atomic_types` | modernize `atomictypes`; review layout and ownership |
| `any` | modernize `any`; gofmt rewrite rule |
| `bytes_cut` | modernize `stringscut` also handles bytes |
| `strings_cut` | modernize `stringscut` |
| `errors_is` | Review: wrapped matching versus intentional identity comparisons |
| `time_until` | staticcheck S1024 |
| `time_since` | staticcheck S1012 |

`errcheck.check-blank` remains enabled. Its `exclude-functions: [errors.AsType]`
entry permits discarding the matched error value while checking the returned bool.
The bundled errcheck v1.20.0 strips generic type arguments before matching the
fully qualified function name, so the entry has no `[T]` suffix. Ordinary ignored
operation errors, including `_ = os.Chdir(path)`, remain failures. The pre-existing
`std-error-handling` preset excludes `os.Remove`; this change preserves that policy.

The full bundled suite additionally includes `importcomment`, `plusbuild`,
`reflecttypeassert`, `stringsbuilder`, and `unsafefuncs`. These checks remain active.
`appendclipped` and `slicesdelete` are also exported but omitted from the suite
because their transformations do not preserve nilness.

The central config has narrow tooling exceptions: literal schema keys and path
matrices do not need goconst extraction; boundary/release orchestration retains its
existing function size; Mage filesystem operations accept trusted developer paths
and preserve artifact modes. These exceptions do not disable Modern Go analyzers,
forbidigo, depguard, copyloopvar, type checking, or formatting.

## Retained exclusions

These sites still match a Modern Go guideline pattern after the issue #482 migration.
Each is kept because the modern form would change behavior, ownership, or a contract,
or because the file is generated output or a test input. Classifications are
`semantic` (the modern form is not equivalent), `generated` (owned by a generator), and
`justified-exclusion` (a pattern match where the rule does not apply). Counts come
from the per-slice audits and are approximate; recount before relying on one.

### Site-specific lint exceptions

A retained call or tag that a configured linter rejects carries a line-local
`//nolint:<linter> // <reason>` comment. No blanket Modern Go suppression exists, and
there are no `depguard` exceptions. List the current set with
`git grep -n -E 'nolint:(forbidigo|modernize|copyloopvar|depguard)' -- '*.go'`.

| Site | Linter | Guideline | Reason |
| --- | --- | --- | --- |
| `internal/cli/gateway_pairing_profile.go:112` | forbidigo | `errors_as_type` | Target interface has only `errorPayload()`, not `Error()`; `errors.AsType` cannot take it |
| `internal/cli/gateway_profile_recovery.go:346` | forbidigo | `errors_as_type` | Same `errorPayload` target |
| `internal/cli/gateway_client_test.go:1310` | forbidigo | `errors_as_type` | Same `errorPayload` target |
| `internal/worktree/exit_plan.go:351` | forbidigo | `errors_as_type` | Target is `interface{ ExitCode() int }`, which does not implement `error` |
| `internal/automation/schedule_test.go:1535` | forbidigo | `slices_sort_func` | The fake's less returns true for nil/nil `ScheduledAt`; a valid three-way comparator changes its order |
| `internal/api/contract/loops.go:387` | modernize | `json_omitzero` | `Runtime` keeps `omitempty,omitzero`: OpenAPI reflection needs `omitempty`, JSON uses `omitzero` |
| `internal/api/contract/loops_runtime.go:44` | modernize | `json_omitzero` | Same dual tag on `Worker` |
| `internal/api/contract/loops_runtime.go:46` | modernize | `json_omitzero` | Same dual tag on `Judge` |
| `sdk/go/types.go:136` | modernize | `json_omitzero` | `omitempty` is a no-op on the `DescribeResources` value struct; `omitzero` would drop the zero object from the wire |


### Review-enforced retained patterns

These patterns pass lint, either because no analyzer covers them or because the
analyzer does not flag the retained form. Keep them unless the stated reason no longer
holds. During integration, 21 `errors.As` sites moved to `errors.AsType`, and bare
`context.WithoutCancel` cleanup calls moved to `testutil.Context(t)` (fresh 45-second
timeout); neither is retained.

| Guideline | Scope (counts / representative sites) | Classification | Reason |
| --- | --- | --- | --- |
| `loopvar_capture` | ~40 renamed per-iteration copies: `internal/cli/config_flatten.go:21` (`nextPath := key`), `internal/automation/list_resource_catalog.go:36,101`, `internal/task/manager_test.go:2748`, store value normalizers | justified-exclusion | Not redundant self-copies: the copy is mutated, normalized, or kept as a snapshot while the loop variable is still used. Default copyloopvar and modernize `forvar` flag only `x := x` |
| `testing_t_context` | ~196 `context.Background()` calls in or captured by `t.Cleanup` callbacks and returned cleanup functions (daemon 56, api 28, settings 26, session 23, tools 19, store 13, task 9, platform 9, cli 7, extension 6): `internal/settings/config_apply_service_test.go`, `internal/api/udsapi/server_test.go`, `internal/terminal/journal/service_test.go` | semantic | `t.Context()` is canceled before cleanup runs; Close, Shutdown, drain, and ROLLBACK need a live context |
| `testing_t_context` | 9 `TestMain` seed setups: `internal/cli/testmain_test.go`, `internal/session/testmain_test.go`, `internal/store/globaldb/global_db_test.go:101` | justified-exclusion | No `testing.TB` exists in `TestMain`; the seed has process lifetime |
| `testing_t_context` | ~33 helpers, fakes, and interface implementations without `testing.TB` (cli integration daemon 15, task fake store 7, api 2, daemon 2, tools 2, extension 2, session 1, loop 1, e2e `testContext` 1): `internal/cli/cli_integration_test.go`, `internal/task/manager_test.go`, `internal/loop/action_test.go:1610` | justified-exclusion | Threading `testing.TB` would change test-double interfaces or non-test signatures |
| `testing_t_context` | ~19 process and subprocess lifetimes: helper entrypoints `internal/acp/client_process_lifecycle_test.go:75`, stdio MCP servers `internal/mcp/executor_test.go:2607`, process-group kill `internal/toolruntime/interrupt_unix_test.go:23` | semantic | The process runs until EOF, exit, or an ordered cleanup kill; test cancellation would kill it early through `CommandContext` or reorder shutdown |
| `testing_t_context` | 4 runtimes stopped by cleanup: terminal reapers `internal/api/udsapi/udsapi_integration_test.go:2854` and `internal/api/core/terminal_wire_integration_test.go:87`, MCP server `internal/mcp/executor_test.go:1874`, finalization hook `internal/session/manager_prompt_contract_test.go:1277` | semantic | Cleanup Shutdown or Session.Close must run before the runtime stops; the hook keeps an independent 45-second context |
| `testing_t_context` | Non-test lifetimes: CLI/Mage command roots, SDK runtimes, example and fixture mains; compiled source strings in `internal/extension/tool_provider_test.go`; outer captures `internal/daemon/perf_bench_test.go:350` (already `WithTimeout(WithoutCancel(ctx), time.Second)`) and `internal/testutil/testutil_test.go:22` (asserts cancellation) | justified-exclusion | Not a test-scoped context, or already detached and bounded |
| `sync_waitgroup_go` | 17 `Add` calls under an admission lock or with `Done` owned by another method: `internal/session/manager_delete_reconciliation.go:64`, `internal/memory/extractor/runtime_queue.go:170`, `internal/extension/manager_startup_transaction.go:211` | semantic | Admission must happen under the lock before unlock or commit; `wg.Go` moves it to goroutine start and changes shutdown ordering |
| `sync_waitgroup_go` | `Add(n)` batches: `wg.Add(2)` in `internal/daemon/clarify_keepalive_test.go` and `internal/daemon/daemon_test.go`; batched launches in memory runtimes | semantic | Completion bookkeeping is admitted as one batch before launch |
| `sync_waitgroup_go` | 198 `.Add(1)` grep hits on typed atomics (extension 34, session 30, tools 30, api 24, cli 23, daemon 18, store 17, platform 17, loop 2, gates 2, task 1): `internal/codegen/storeschema/atlas.go:263` | justified-exclusion | Atomic counters, sequences, and gauges; not `sync.WaitGroup` |
| `sync_once_func`, `sync_once_value` | ~173 struct-owned `sync.Once` fields and multi-callback guards (session 79, daemon 29, extension 16, platform 15, store 13, api 7, cli 7, tools 5, task 2): `internal/terminal/pty/pipe.go`, `internal/cli/connect_ssh_transport.go:176`, `internal/modelcatalog/service_refresh.go:175` | semantic | Zero-value lifecycle gates shared by several methods or `Do` sites, often memoizing a Close error; `OnceFunc`/`OnceValue` need constructor changes and alter zero-value and panic behavior |
| `sync_once_func`, `sync_once_value` | 5 per-call captures: `internal/daemon/agent_skill_sync_staged.go:52`, `internal/gateway/mutation_gate.go:98`, `internal/memory/dream_test.go:1317`, `internal/mcp/executor_test.go:1593`, `sdk/go/transport.go:85` | semantic | The first caller's context, argument, or error decides the result; an argumentless wrapper cannot bind it |
| `sync_once_func` | Driver registration: `internal/store/migrate_test.go:661` | semantic | Keeps `sync.Once` one-shot panic behavior if global SQL driver registration fails |
| `new_expression` | Behavioral and exported pointer helpers in every slice: `cloneProviderModelPtr` (cli), `budgetExceededPtr` (daemon), `cloneTimePointer` (task), `nullStringPtr` in `internal/store/globaldb/global_db_loop_normalize.go:158` | semantic | Nil guards, defaulting, trimming, UTC conversion, SQL-null mapping, or deep copy; exported signatures stay, and bodies use `new(value)` where equivalent |
| `new_expression` | Mutated or read-back temporaries: `internal/skills/resource_test.go:322`, `sdk/go/extension_command.go:31-37`, store `after`/`call`/`merged` locals | semantic | The value is mutated or read after its address is taken, or shared across assertions; a fresh allocation changes identity |
| `slices_clone`, `bytes_clone` | 705+ `append([]T(nil), s...)` copies (tools 176, cli 166, loop 117, api 113, task 67, store 63, gates 3; daemon, session, platform, extension uncounted): `sdk/go/tool_request.go:57`, `magefiles/gotest_lane.go:328`, `sdk/go/extensiontest/harness.go:25` | semantic | Append-to-nil returns nil for an empty non-nil input; `Clone` keeps it non-nil, which changes JSON `null` versus `[]`, test equality, and `exec.Cmd.Env` (nil inherits) |
| `slices_clone` | `internal/marketplace/store_source.go` `sourceContentRevision` make/copy | semantic | Produces non-nil content from nil before JSON hashing; `Clone` could change the digest |
| `maps_clone`, `maps_copy` | `internal/providers/prestart_cache.go:392`, `internal/outboundpolicy/policy.go:183`, `internal/gateway/connection_registry.go:73`, `internal/extension/manifest_tool_toml.go` `copyTable`, `internal/automation/trigger_clone.go` | semantic | Must return a non-nil map for nil input, or performs a deep or transformed copy |
| `url_clone` | `internal/cli/profile_read_scope.go` `cloneQueryValues`, `internal/cli/client_transport.go` `withFreshStreamTicket`, `internal/testutil/mcpfixture/oauth.go:350` | semantic | Returns a writable non-nil map and collapses empty value slices to nil; `url.Values.Clone` preserves both |
| `slices_sorted`, `slices_collect` | ~17 `slices.AppendSeq(make(...), maps.Keys(m))` plus `slices.Sort` (loop 9, extension 6, cli 2): `internal/config/persistence_helpers.go` `sortedStringKeys`, `internal/config/builtin_agents.go`; preallocated results in `internal/acp/negotiation_error.go` | semantic | Helper and DTO boundaries return a non-nil empty slice; `Sorted` and `Collect` return nil for empty input |
| `slices_sorted`, `slices_collect` | Trimmed, filtered, or projected keys: settings MCP environment-key collectors, `internal/daemon/model_catalog_staging.go`, task accumulators | semantic | Not a raw map-key collection |
| `slices_sort` | 5 `"sort"` imports: `internal/automation/list_resource_catalog.go:7` (`sort.Search` :183), `internal/automation/model/list.go:6` (:359, :373), `internal/memory/header_list.go:7` (:229), `internal/daemon/harness_reentry_queue.go:4` (:15), `internal/automation/schedule_test.go:8` (the `sort.Slice` exception above) | semantic | `sort.Search` is an upper-bound search: pagination needs the first key strictly greater than the cursor, and the reentry queue inserts after equal wake times to keep FIFO ties. `slices.BinarySearchFunc` returns the lower bound |
| `slices_sort_func` | `internal/providers/prestart_cache.go` `rebaseAccessOrderLocked` | semantic | Custom less over unique cache keys, not a legacy sort call; eviction and tie order stay |
| `slices_index_func` | `internal/task/live.go` `splitTreeView` | semantic | No match falls back to index 0 (the root), not -1 |
| `errors_as_type` | 4 non-error interface targets (lint exceptions above); `internal/agentidentity/identity_test.go:307` only names `errors.As` in a message | semantic | `errors.AsType` requires a target type that implements `error` |
| `errors_is` | 5 strict `err != io.EOF` checks: `internal/loop/coordinator_goal_control.go`, `internal/loop/dsl/strategy.go`, `internal/marketplace/entry_input.go` | semantic | JSON decoder end-of-input checks; `errors.Is` would also accept errors that wrap EOF |
| `context_cancel_cause`, `context_timeout_deadline_cause`, `errors_join` | Existing cancellation and error contracts (platform slice) | semantic | Adding causes or joining errors changes observable error text and matching |
| `range_over_int` | 10 index-consuming loops: `internal/cli/output_format_args.go:17`, `internal/worktree/git_porcelain.go:200`, `internal/terminal/mode_preamble.go:31` | semantic | The body advances the index to consume a flag value, escape bytes, or a record |
| `range_over_int` | Custom stride, nonzero or reverse start, or dynamic end (loop 15 in 13 files; daemon, platform, extension): YAML key/value pairs, chunked hashing, reverse cleanup | semantic | Not a `0..n-1` unit-step loop |
| `strings_split_seq` | 11 `Split`/`Fields` sites in 11 `internal/loop` files; api, cli, extension, and store keep similar materialized results | semantic | The result is indexed, counted, sliced, joined, or used for line numbers |
| `strings_cut`, `strings_bytes_cut_last` | Offset-dependent parsers: store Markdown and snippet scanners, `internal/toolmeta/preview_terminal.go:29`, `internal/extension/extension_validation.go` `lineStart`, `internal/codegen/storeschema/sqlite_index_expression.go` | semantic | Need numeric offsets for line/column, nesting, or ordering, or match either slash (`LastIndexAny`) |
| `time_tick_gc` | `time.NewTicker` sites with an owner (daemon 15 and session 16 production sites, plus other slices): `internal/retention/periodic.go`, `internal/scheduler/scheduler.go`, `magefiles/verifylock.go:70` | semantic | Each ticker is stopped, reset, or injected by a bounded owner; none is a process-lifetime loop |
| `generic_methods` | Free generic helpers and exported generic APIs: `internal/listcursor/cursor.go`, `internal/resources/{codec,typed,projector}.go`, `internal/api/spec/response_body.go`, `retry.DoValue`, `frontmatter.Format` | justified-exclusion | No single owned receiver type; exported APIs have callers across packages and keep their signatures |
| `promoted_field_literals` | Named fields such as `ReadScope`, `Capabilities`, `AcceptedCapabilities`; pointer embeds; existing embedded value expressions | justified-exclusion | Named fields are not promoted, pointer-embedded paths are out of scope, and an embedded value expression would have to be rebuilt |
| `http_servemux_patterns` | `internal/api/httpapi/gateway_auth.go` (Gin auth classification), `extensions/spec-cycle/rpc.go` (JSON-RPC method switch) | justified-exclusion | Not HTTP route dispatch |
| `http_servemux_patterns` | `internal/testutil/mcpfixture/http.go`, `oauth.go` | semantic | ServeMux would change method dispatch, path cleaning, HEAD handling, and the custom 405 body |
| `cmp_or` | Lazy fallbacks in `internal/cli` (`connect_ssh.go`, `root.go`, and seven more files) | semantic | `cmp.Or` evaluates every argument; these call the clock, environment, or a function only when needed |
| `maps_delete_func` | `magefiles/gotest_census_update.go:164-179` | semantic | Pruning can return a filesystem error mid-iteration; a bool predicate cannot |
| `maps_keys_values_iter` | Direct single-pass map loops (task slice) | justified-exclusion | Nothing is materialized; mutation and snapshot loops stay explicit |
| `reflect_type_for` | `internal/mcp/serve_projection.go:103` | semantic | The type comes from a runtime value |
| `min_max` | `internal/extension/host_api_rate_limit.go` `minFloat` | semantic | Built-in `min` differs for NaN and signed zero |
| `json_omitzero` | 2,924 strings, 730 slices, 126 maps (1,238 bool, numeric, and pointer tags were migrated) | semantic | The guideline keeps `omitempty` for empty strings, slices, and maps, including `json.RawMessage` and named enum strings |
| `json_omitzero` | 183 fields whose type has `IsZero`, including `*time.Time` and `SessionEventPayload` via `EventCorrelation.IsZero` | semantic | `omitzero` calls `IsZero`; a non-nil pointer to zero or a domain-zero value would disappear from the wire |
| `json_omitzero` | 16 interface fields | semantic | Keeps nil interface distinct from an interface holding a typed nil or zero value |
| `json_omitzero` | 1 struct no-op (`sdk/go/types.go:136`) and 3 dual-tag contract fields (lint exceptions above) | semantic | See the lint exception reasons |
| `json_omitzero` | 1,467 source tags in the Go SDK generator's reachable type graph | generated | `internal/codegen/sdkgo/type_render.go` copies tag text verbatim into generated SDK code |
| `json_omitzero` | 2,984 tags in generated Go; 30 test-input fields; 6 testdata fields | generated | Generated output and fixture or schema inputs stay unchanged |
| All rules (generated files) | 66 generated Go files plus SQL and `atlas.sum` in the persistence slice, `internal/terminal/wire/opcodes_generated.go`, `sdk/go/contracts/capabilities_gen.go:20` | generated | Never hand-edited. `capabilities_gen.go` uses append-to-nil because the template `internal/codegen/sdkgo/contracts_render.go:109` emits it; change the template and regenerate |
| `any` | `internal/codegen/storeschema/sqlc.go`, `sqlc_test.go` | generated | Literal `interface{}` is generator normalization input and its fixture, not a declaration |
| All rules (test inputs) | `internal/extension/testdata/**` (for example `secret-guard/main.go:182`, `secret-guard/main_test.go:61`) and golden inputs | justified-exclusion | Their bytes are the contract. The two `*-fixture-go` modules are linted modules, not test inputs |
| `json_v2` | All `encoding/json` imports | justified-exclusion | Not applicable to this migration; v1 encoding is preserved |

## Change impact

The owning impact audit is issue #482's Compozy Impact Audit. Native tools,
extension/hook interfaces, persisted configuration, workspace isolation, and
`skills/compozy/` are unchanged. This slice changes developer verification and
behavior-preserving Go idioms; it changes no Web or public wire contracts.
