# Cursor default reasoning regression (#1788)

## Scope and rule

A fake live Cursor catalog advertises `gemini-3.8-flash-low` and
`gemini-3.8-flash-high` plus matching Fast aliases, with no provider-marked
default. Discovery publishes `low` in the existing `default_reasoning_effort`
field. The session manager uses that default to choose the launch alias; the
Web derives the displayed default from the same field. Explicit `ultra` remains
invalid. Declared defaults and bare normal bindings preserve their behavior.

The fallback preserves unspecified effort when any bare transport exists, so a
model-wide inferred effort cannot block a bare alias in another mode. If every
transport requires an effort, it prefers normal, non-Thinking bindings, then
non-Thinking Fast, normal Thinking, and Fast Thinking bindings, selecting the
lowest supported effort in canonical order from the first available mode.
It never enables Fast or Thinking or changes an explicit effort to obtain
a match. No public runtime-selection or payload shape changes.

## Evidence

- Catalog: `TestLiveProviderSources/Should_resolve_a_missing_Cursor` covers
  fallback, declared default, bare binding, normal-mode precedence, and
  Fast-only/Thinking-only defaults. Existing Cursor aggregation cases also passed.
  The mixed normal-effort/bare-Fast case preserves the unspecified model default.
- Runtime: `TestCreateAppliesRuntimeModelOverride` passed, including normal/Fast
  default alias launch and explicit unsupported-effort rejection.
  It also preserves the existing bare Fast launch when normal aliases require
  an effort. Both new regression assertions failed before the corrective guard.
- Web: focused Turborepo model-catalog suites passed (31 tests), including
  agreement between active-session derivation and runtime-selector defaults.
- Real integration: isolated ACP subprocess and SQLite check passed again after
  the bare-Fast correction (9.965s).
- Go lint: the full configured lint set and the owning integration lint lane
  passed on both changed packages (0 issues).
- Independent review: Claude Opus 5.5 / high approved the initial candidate and
  both corrective versions without blockers, including the bare-Fast guard.
  Review nits were addressed. The reviewer ran no checks independently.

Focused commands (all passed; Go runs use `GOMAXPROCS=2`, `GOFLAGS=-p=2`,
and a task-owned `GOCACHE`):

```sh
go test -p 2 ./internal/modelcatalog ./internal/session -run 'TestLiveProviderSources/Should_(aggregate_Cursor|resolve_a_missing_Cursor)|TestCreateAppliesRuntimeModelOverride' -count=1 -timeout=120s
go test -p 2 -tags=integration ./internal/session -run TestManagerIntegrationCursorDefaultReasoning -count=1 -timeout=120s
bunx --no-install turbo run test --filter=./web -- src/systems/model-catalog/__tests__/derive-active-session-options.test.ts src/systems/model-catalog/lib/__tests__/to-runtime-selector-options.test.ts --maxWorkers=2
bunx --no-install oxlint --deny-warnings web/src/systems/model-catalog/__tests__/derive-active-session-options.test.ts
go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0 run --concurrency 2 --timeout 3m ./internal/modelcatalog ./internal/session
go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0 run --concurrency 2 --timeout 3m --build-tags integration --enable-only modernize,forbidigo,depguard,copyloopvar ./internal/modelcatalog ./internal/session
```

The initial Web attempt failed before tests because dependencies were absent;
`bun install --frozen-lockfile` restored the repository's generator patch, and
Turborepo's codegen prerequisite and both suites then passed. Initial integration
attempts failed on missing shared Go cache files; a private build cache resolved
the compilation failure. A daemon restart interrupted another compilation;
the resumed integration run above passed. No assertion or gate was weakened.
The installed lint binary was built with Go 1.26 and cannot target Go 1.27.3;
running the project's pinned version from source with the current Go succeeded.

The test-convention checker reports existing whole-file findings in canonical
suites (mixed serial/parallel tests, older inline cases and discarded errors).
No new case uses `t.Setenv`, discards errors, or omits required parallelism.
Comparing checker output against HEAD confirmed identical findings in all three
changed test files, after normalizing shifted line numbers.

## Cross-surface impact

- Native tools / CLI / HTTP / UDS: model catalog and session consumers receive
  the same supported default via existing fields. No IDs, routes, flags or DTOs change.
- Extensibility, hooks and config: no new keys, hooks, permissions or SDK shapes.
  Existing curated defaults keep their precedence.
- Workspace/profile isolation and user state: no migration or data rewrite;
  discovery remains scoped to its existing provider execution context.
- Web: existing catalog-backed controls display the resolved default. No layout or
  visual component changes; focused selector coverage verifies data derivation.
- Docs and official skill: provider/catalog docs and runtime operations reference
  document the deterministic rule. Scenario: RT-070.

The owning impact audit is this report and the PR body, following
`docs/_memory/change-impact.md`; the shared audit file is left to the conductor.

## Limits

No live Cursor account or operator daemon is used. Full, race and E2E suites and
repository-wide lint remain CI-owned under the overloaded-host dispatch.
Persisted discovery rows acquire the resolved default on the next successful
live refresh (normally within five minutes); an explicit catalog refresh can
apply it sooner. No user-state migration is needed.
Unavailable speed/Thinking combinations still fail explicitly, including models
with only Fast or Thinking aliases unless those modes are selected. A model-wide
default does not invent a supported combination when normal and Fast bindings
offer disjoint effort sets.

The isolated environment teardown completed with `TEARDOWN_ALL_CLEAN=true`.
