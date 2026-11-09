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
