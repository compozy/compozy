# Modern Go compliance

## Checked modules

`make go-lint` runs the root source policy and golangci-lint v2.13.1 using the
repository `.golangci.yml`. `make lint` and the CI Go lint job call that same lane.
An unset `COMPOZY_GO_LINT_SCOPES`, or `./...`, checks every row below.
The Mage suite compares the dispatch list with tracked nested `go.mod` files,
so adding a module requires updating lint coverage.

| Directory | Declared Go version | Package scope / build tag |
| --- | --- | --- |
| Repository root | 1.27.1 | `./...` |
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
The first failing module returns a nonzero exit status.

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

Controller: fill this section from all worker reports after integration, recording
`rule | file:line or pattern/count | semantic/generated/justified-exclusion | reason`.
Any retained prohibited call needs a site-specific documented lint exception;
no blanket Modern Go suppression is permitted.

## Change impact

The owning impact audit is issue #482's Compozy Impact Audit. Native tools,
extension/hook interfaces, persisted configuration, workspace isolation, and
`skills/compozy/` are unchanged. This slice changes developer verification and
behavior-preserving Go idioms; it changes no Web or public wire contracts.
