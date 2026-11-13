# CI efficiency implementation

Owner: `ci/efficiency-t3`. Reference: T3 b8c2cfad6. No measured speedup yet.

| Item | Implementation | Validation |
| --- | --- | --- |
| Workspace selection | Manifest dependents and explicit Turbo root inputs; deletions use path ownership; renames disable rename detection; unknown metadata/path/diff selects full checks | Classifier self-test covers closure, external inputs, deletion and fallback |
| Frontend lanes | Independent lint, web/packages and site/client jobs; concurrency capped at two Turbo tasks per runner; canonical aggregators preserved | Actionlint and strict skipped/failure/cancelled gate self-tests |
| Chromium cache | OS/architecture/installed version key; exact hits skip download; Linux dry-run checks libraries before installation; full Chromium retained | Installed Playwright CLI implementation inspected; Linux CI pending |
| Integration census | Separate integration census and qualified package/test identity; one LPT partition; trusted main push receipts; source run recorded | Focused partition/observation tests and go vet passed |
| Turbo audit | Current-run receipts uploaded and summarized; one SHA-deterministic cached test package rerun with cache reads and writes disabled; failure propagates | Real cache hit followed by cold execution: 40 tests passed |
| Synchronization enforcement | Existing testing plugin rejects numeric unit sleeps, unconditional skips and per-test timeout/retry overrides; real process spec tests retained | 40 lint-plugin tests passed via Turbo |
| Setup and artifacts | Lint avoids Go setup; frontend cache retains task outputs without stale run reports; unique lane cache keys; integration receipts uploaded on success and failure; PR cancellation retained | Workflow lint passed; hosted CI pending |

## Selection and trust

`ci-changes.sh` owns broad areas and the required-check contract. `ci-workspaces.py`
reads workspace manifests and `turbo.json`, expands downstream dependents, and emits
only package names. Execution passes filters as argument arrays, never shell text.
Main, nightly and manual runs select all workspaces and heavy checks. CI pipeline
changes and uncertain classification opt PRs into heavy checks. Root or manifest
changes select every frontend workspace. Unknown metadata returns the full sentinel.

Bun, Turbo and current runners remain. Browser cache writes remain main-push only;
PRs consume remote Turbo cache read-only. A cold audit disables both reads and writes (`--cache=`). SQLite paired load and environmental acceptance remain outside Turbo caching.
No timeout, threshold, assertion or retry policy was relaxed.

## Timing interpretation

The `Frontend (web)` check is a single `always()` aggregate over the selected
frontend matrix. Four independent web Vitest shards run alongside lint,
site/SDK checks and the web/package typecheck, build and non-web tests. A failed,
canceled or skipped selected matrix fails the aggregate; `Verify (fmt, lint,
unit, build)` continues to consume the same `frontend` dependency. The current
main ruleset requires review-thread resolution and declares no required status
contexts; the existing frontend check name is nevertheless preserved.

Web shards pass `--shard=i/4 --maxWorkers=75% --retry=2` through Turbo, so each
shard has its own task hash and retains retries on its own runner. Local Vitest
defaults stay at `maxWorkers: "50%"`. Each Turbo invocation must still produce
exactly one fresh receipt, and a cached web test is cold-audited with the same
shard, workers and retries. Artifact and GitHub cache names distinguish matrix
entries. `scripts/ci-changes.sh --self-test` owns the selection, receipt and cold
audit contracts; the PR's CI run supplies runtime validation and lane wall time.
The separate investigation into web test import time remains out of scope.

Integration logs separate package/source discovery from each invocation's wall time.
Invocation wall time includes remaining compilation, process setup and execution;
gotestsum JSON retains individual test/package elapsed times. These are not presented
as a precise compiler-only measurement. Historical unit timings are never reused as
integration timings. Until trusted integration receipts exist, uniform defaults apply.
Census refresh preserves unobserved values and records the source run ID; raw logs and
signed artifact URLs are not committed. Turbo reports cache hits and observed execution
times. Avoided runtime is explicitly unavailable without cold-duration evidence.

## Cross-surface impact

CI tooling only. Native tools, extensibility/hooks/config, workspace data isolation,
user state, official `skills/compozy/`, and product Web/site contracts are unchanged.
No product QA scenario changes are required. Root checkout remains untouched.

## Verification and limits

- `scripts/ci-changes.sh --self-test`: passed.
- `actionlint` on CI, census refresh and Release workflows: passed.
- Focused mage census/partition/safety tests: passed.
- `go vet -tags=mage ./magefiles`: passed.
- `bunx turbo run test --filter=@compozy/lint-plugins --only`: 40 tests passed.
- Changed lint plugin files: oxlint passed; formatting and diff whitespace checks passed.
- Test-convention heuristic flags the existing independent serial `t.Setenv` test and
  parallel tests in `gotest_lane_test.go`. The added pure subtest does not call Setenv;
  focused execution passed. Census test file passes the heuristic.
- System golangci-lint was built with Go 1.26 and cannot lint Go 1.27; pinned Go 1.27-compatible tool passed with zero issues after formatting repairs.
- Full/race/E2E suites and Linux browser installation remain CI-only. Existing SQLite
  paired-load, runtime/desktop E2E and shard timeouts are not claimed fixed.

## First CI repair round

The first site/SDK lane succeeded with 21 cache hits across 23 receipts, a real
uncached extension-SDK audit and three uploaded summary artifacts. Frontend lint
exposed the new rule incorrectly rejecting stricter per-test deadlines. The rule now
uses the owning suite budget (Web: 20 seconds), retains stricter limits and requires
local explanations for increases. Eight existing DOM/lazy-import/real-process
contracts retain their exact limits with occurrence-specific rationale; no test
assertion, timeout or synchronization was changed. Focused lint covered every failing
file and the plugin suite passed again.

Metadata selection now reuses the canonical shell classifier after Turbo root-input
matching; instruction-only edits skip frontend, while glossary edits select site.
Integration refresh searches up to 40 trusted successful runs for the complete
eight-artifact set, reports absent data and includes both source run IDs in its PR.
The implementation PR has `ci:full` for the changed integration shards.

Deadline review repair: positional and object timeout overrides must be finite,
strictly positive and within the owning suite budget. Zero, negative and Infinity
cannot disable the deadline. The canonical plugin suite passes 41 tests; an actual
oxlint parser run rejects all six disabled-deadline forms and accepts both positive
controls. Existing test deadlines and assertions remain unchanged.

The SDK's two real stdio integration tests use the existing positive 180s
`integrationTimeoutMs` constant. Each now documents its process/RPC lifecycle
exception locally; the rule still rejects arbitrary constant overrides. Their
event-driven response synchronization, deadline and assertions are unchanged.
Focused Turbo execution of that canonical integration file passes all three tests
with a cache miss; focused lint and formatting pass.

Review repair extends deadline/retry enforcement to modified and parameterized
test declarations, including concurrent and tagged-template `each` forms, without
treating data factories as declarations. The canonical plugin suite passes 42
tests; actual-parser checks reject four chained overrides and accept two positive
controls. Focused lint passes for the 207 files containing affected call forms.
The installer's existing 20s subprocess contract now has a local exception; all
three selected cosign cases pass via Turbo after restoring the required generated
install template through its owning task. The other 13 cases were filtered out.

Desktop's stalled MIME-registration subprocess test now documents its real
timeout/SIGKILL contract locally, preserving the existing 15s deadline and all
assertions. Focused lint passes for all 15 desktop files using chained test calls.
The single-file iteration disables coverage collection only for that local run;
CI coverage configuration and thresholds remain unchanged.
All 13 deep-link tests pass via Turbo on a cache miss, including the real stalled
child termination (about five seconds).

## CodeRabbit repair round

Unit `.spec` files now receive the same discipline checks as `.test` files;
Playwright specs retain their existing exemption. Chained skips and inline option
spreads cannot hide retry or deadline overrides, and unknown spreads fail closed.
The canonical lint-plugin suite passes all 43 tests with Turbo caching disabled.

Integration-first census creation initializes valid root defaults. Refresh output
reports the selected lane, and source parsing errors abort before changing the
persisted census or its provenance. The existing census suite covers both bootstrap
and failed-refresh preservation; the partition suite now includes regular packages
alongside split test invocations. Focused Go tests and vet pass.

Census input is lane-specific: the refresh workflow downloads unit artifacts into
`/tmp/gotest-json` and integration artifacts into `/tmp/integration-census`, then
invokes the updater separately. Raw gotestsum events and their filenames do not
encode lane identity; callers must not combine these directories. Filtering raw
events by filename would not establish provenance.

Each selected Turbo invocation must emit a fresh summary; missing or stale summaries
fail the lane. Malformed Turbo task metadata conservatively selects all workspaces.
The selected prompt-enhancer workspace now runs its build. Classifier self-tests
cover these failure paths and missing/skipped heavy integration jobs.

The next review round adds literal computed member/key handling and rejects dynamic
option keys whose value cannot be established. The canonical plugin suite now
covers these forms alongside positive controls. Frontend invocations record their
new receipt paths in a per-lane manifest; reporting reads only that manifest, so
older reports cannot inflate hit counts or select an unrelated cold audit. The
existing self-test injects an invalid stale receipt and verifies it is ignored.
Heavy-gate tests now exercise web-only, desktop-only and disabled-heavy selections.
The named census bootstrap subtest checks the exact source-parsing failure.

Static test-key resolution follows immutable lexical bindings and no-interpolation
templates, respects shadowed variables, and stops on cycles or mutable bindings.
Constant `skipIf(true)` and `runIf(false)` declarations are rejected; dynamic
conditions remain supported. Canonical regressions and real-parser controls cover
these cases. The CI self-test also covers an entirely skipped disabled-heavy
projection and verifies that cached test receipts invoke the cache-disabled audit.

Suite-level retry/deadline options now use the same checks as individual tests,
preventing inherited overrides from bypassing local rationale. Static unary
negation is resolved for conditional skips; unresolved operands remain unknown.
The canonical report self-test injects a failed cold audit and requires the exact
subprocess failure to propagate. Real-parser checks cover suite overrides and
negated skips with bounded-deadline and executing-condition controls.

Static condition evaluation distinguishes known global `undefined` from unknown
expressions and respects local shadowing. Suite checks include `suite` and renamed
Vitest imports, resolved through lexical bindings. The canonical plugin suite
passes 48 tests without cache; real Oxlint parsing confirms four expected
violations while preserving shadowed parameter controls.

Import resolution also preserves `bun:test` declarations and their aliases.
The canonical plugin suite passes 49 tests without cache; actual Oxlint parsing
rejects Bun skips and disabled deadlines while accepting bounded deadlines.

## Full integration lane restructuring (W1)

The full integration matrix is defined once in `go-integration.yml`. CI owns main
pushes, nightly, manual and opt-in PR runs. Release PRs call the same workflow from
Release, and `Dry-Run Build Validation` requires all integration shards to succeed.
The CI heavy aggregator explicitly allows that release-owned call instead of
starting a duplicate matrix. Main CI runs already use non-cancelling concurrency.

Three shards own daemon, two own globaldb and three own all other integration
packages. Each group uses duration-weighted LPT assignment and independent cache
keys. Every package receives an exact top-level test regex. Discovery compares the
actual Go-selected test files with and without the integration tag, including
external test packages and platform constraints. It parses declarations without
compiling test binaries. Untagged unit tests stay in the race unit lane. Functions
matched by the canonical runtime/nightly E2E plans stay in those lanes, including
transport and harness contracts; their package alone does not exclude other tests.

Integration keeps race detection and `-parallel=4`, disabling checkptr only for
modernc as the unit lane already does. The weekly full checkptr audit is unchanged.
Globaldb already had current-schema copies and historical prefix templates; W1
uses the current-family template for the ordinary reopen case. Production open
bootstraps that family once per package, including the telemetry sibling and its
identity; raw SQL prefix snapshots remain historical fixtures. Migration-specific
cases still replay their migration tails, including the historical destructive-cut
case. That full historical replay runs before its parallel sibling upgrades so
CPU contention cannot consume its five-minute operation budget. No assertion or
operation deadline is relaxed.

The integration census accepts completed timing events from partial/failed matrices
and successful runs in either CI or Release, including same-repository PRs. It
retains separate artifact directories because group-local shard filenames repeat.
Unobserved weights remain intact. Qualified test timings supersede package-based
bootstrap weights as receipts accumulate; unit timings are not substituted.
The initial package weights come from completed package summaries in all eight
shards of main Release run 43245729994. Split-package seconds are summed across
shards; they include the old duplicate unit work and are bootstrap weights, not
predictions for the new tagged-only lane.

The baseline job wall times (including setup) are 41.40 / 50.34 / 97.43 minutes
(minimum / median / maximum), totaling 490.85 runner-minutes. This uses the complete
job durations rather than the analysis's approximate execution-only estimate.

The first complete PR run, [43293868078](https://github.com/compozy/compozy/actions/runs/43293868078),
on `9e59a1bcde14d9e1393cf033cef7fd8bb2ad4f2e` passed all eight shards:

| Full job wall time, including setup | Minimum | Median | Maximum | Runner-minutes |
| --- | ---: | ---: | ---: | ---: |
| Main baseline 43245729994 | 41.40 min | 50.34 min | 97.43 min | 490.85 |
| PR 43293868078 | 13.07 min | 22.23 min | 29.03 min | 174.20 |

This single completed sample reduces total runner time by 64.5% and the slowest
shard by 70.2%; it is not a repeated benchmark or a forecast for main cache hits.
The 75 package invocations report total wall time separately from test
execution time: the slowest invocation took 21m45s in total, with 13m28s reported
for test execution. `go test -timeout` applies to the test binary, not compilation.
Those receipts support a 60-minute job ceiling and a 20-minute integration test
binary timeout. The E2E runner retains its separate 45-minute timeout. Per-test
operation deadlines, race detection and preservation assertions are unchanged.

Validation is CI-only by operator direction. PR #1617 records current-head CI
and reviewer evidence; the timeout reduction requires a new full CI run. Native
tools, hooks,
configuration, public APIs/CLI, user/workspace data, the official product skill and
product UI/QA scenarios are unaffected; delivery instructions change with the lane.
