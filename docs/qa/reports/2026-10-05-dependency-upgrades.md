# Dependency upgrades and main CI repair

Status: local verification passed. Final-head CI is tracked by the pull request.

## Scope

Upgrade Go dependencies alongside the existing Bun dependency update, migrate changed APIs,
repair failing main checks, and deliver a PR with green checks on its final head.

## Confirmed compatibility changes

- Go 1.27.1 is required by Tailscale 1.104.0; update local tooling, CI and release builders together.
- CEL 0.32 uses `cel.dev/cel-go`; migrate every production import.
- Bubble Tea 2 and Bubbles 2 use `charm.land`, `tea.KeyPressMsg` and `tea.View`.
  The existing CLI wizard transition suite retains its behavioral assertions.
- ULID consumers use the current v2 module.
- gVisor's default branch contains Bazel templates. Resolve its Go distribution from `@go`,
  as required by the upstream Go build instructions.
- TypeScript 7 exposes a different compiler API. Keep native TypeScript 7 for typechecking;
  retain TypeScript 5.9.3 for openapi-typescript 7.13's declared `^5.x` compiler API peer.
  The embedded generator uses `ts.factory`, which TypeScript 7 no longer exports.
- Vitest 5 no longer reads the global Jest matcher declarations. Register jest-dom matchers
  through `expect.extend` and augment Vitest's current `Matchers<R, T>` interface.
- MSW 3 replaces `onUnhandledRequest` with `onUnhandledFrame`; retain strict API checks
  through its HTTP frame adapter and regenerate the browser worker.
- Align assistant-ui core/store/tap/stream with React 0.15; remove obsolete store/tap
  patches whose lifecycle fixes are included upstream.
- Go 1.27 enables modernize diagnostics for promoted-field literals. Apply the formatter
  fixes mechanically across the existing source/tests without changing assertions.
- Lucide 1.52 renamed six profile icon slugs. Decode saved/input names to their canonical
  equivalents at the profile boundary; preserve IDs, colors and emoji. Regenerate the catalog.

## Main failure evidence

- Run `37379680579`, head `fb78851b276875fe7c9317388df4254c20eedd5d`: Go lint rejects
  golines formatting in `registry_removal.go` and `marketplace_lifecycle_test.go`.
- The remaining jobs in main run `37379680579`, including all E2E lanes, passed.
- Release `37371298653` had its release-PR job cancelled; the later release run
  `37379680587` succeeded. No current release-job defect was observed.
- Scheduled checkptr run `37265410608`: `TestGlobalAutomationRunProfileMigrationTail`
  exceeded its three-minute context while applying migration 98. The upgraded SQLite
  passes the same test with full race/checkptr in 103.200s in isolation. PR audit
  `37391087126` still exceeded the deadline at migration 120 when four instrumented tests
  ran concurrently; its other seven shards passed. The entire store suite passes with
  `GOMAXPROCS=2`, race, full checkptr and `-parallel=1` in 497.451s. The audit now serializes
  independent tests while preserving internal concurrency, instrumentation, all cases,
  and the original deadlines/assertions. The ordinary lane retains `-parallel=4`.

## Research

Context7 was queried for Vitest, TypeScript and CEL. Exa was attempted but `EXA_API_KEY` is
absent from both the process and project environment. Official upstream sources were used:

- https://go.dev/doc/go1.27
- https://vitest.dev/guide/migration/
- https://github.com/microsoft/typescript/blob/main/packages/typescript/package.json
- https://github.com/cel-expr/cel-go
- https://github.com/charmbracelet/bubbletea/blob/main/UPGRADE_GUIDE_V2.md
- https://github.com/charmbracelet/bubbles/blob/main/UPGRADE_GUIDE_V2.md
- https://gvisor.dev/contributing/
- https://github.com/mswjs/msw/releases/tag/v3.0.0
- https://motion.dev/docs/react-upgrade-guide

## Verification

- PR E2E Web exposed a reproducible 12-window restoration regression: 537.9 ms and
  505.1 ms on Linux against the unchanged 500 ms limit. Window bodies now mount at
  deferred priority after their frames commit. A paired CPU-throttled diagnostic measured
  550.7 ms before and 488.9 ms after; drag and peer convergence assertions also passed.
  The diagnostic instrumentation was removed; the canonical E2E remains unchanged.
  The official `COMPOZY_E2E_WEB_SHARD=2/4 make test-e2e-web` rerun passed all 83
  tests in 14.2 minutes: restore 76.1 ms, no drag long tasks above 50 ms, and peer
  convergence within its existing limit. The local gate also passed all affected lanes.
- Review remediation: the composer now suppresses only the pre-hydration observation,
  retaining edits committed before hydration is observed. The existing real-runtime
  session-thread suite passes all 147 tests; the new regression fails on the previous
  implementation because the persisted draft retains the old text.
- Published compressed artifacts are read with the installer's 50 MiB bound. The catalog
  publisher race suite passes, including rejection of an oversized existing artifact.
- PR CI exposed the video's obsolete TypeScript `baseUrl` option. Its alias now uses
  an explicit relative path and its typecheck explicitly selects the native compiler;
  root Turbo video lint/typecheck passes with TypeScript 7.
- `go build ./...`: passed after resolving gVisor's Go distribution.
- Profile lifecycle and CLI wizard suites with race: passed (11.754s / 2.379s).
- Full-checkptr migration regression: passed (103.200s), original context and assertions intact.
- Root Turbo tests for UI, desktop and React SDK: passed.
- Web typecheck passed. The first full test pass exposed four regressions; all 162 tests
  in the three affected suites now pass after repairing asynchronous draft hydration and
  awaiting MSW queue responses without changing behavioral assertions. Full rerun passed:
  693 files and 7,009 tests after review remediation.
- The extension registry test-shape heuristic flags existing top-level assertions; the only
  change there is the ULID v2 import, so unrelated test reshaping is outside this migration.
- Final `make gate`: passed (integration, Mage, Go lint/race tests, codegen, all JS lanes).
  Go lint reports zero issues; root Turbo reports 26 successful tasks. Final-head CI is owned
  by the pull request.
- Final React Doctor: 100/100, no issues across 32 changed files.
- Catalog publisher suite with race: passed, including changed-content publication; lint: zero issues.
  Real `publish` and `validate` commands also preserve Batuta/Herdr metadata and artifact bytes. Go 1.27 changes gzip output; reuse
  published bytes only when decompressed archive hashes match the newly generated package.
- Real isolated CLI/API replay: all six legacy icons create canonical profiles, color-only
  updates retain IDs/icons, and HTTP reads agree. Interactive PTY install completes
  provider/model/review, including Escape navigation. Lab teardown reports clean.
- Lab evidence: `/Users/pedronauck/dev/qa-labs/compozy-dependency-upgrades-20261005-231426-852288-lab/qa-artifacts/qa/`.

- Production Web/site builds and typechecks: passed through root Turbo. Web emits CSS
  optimizer warnings for valid `::highlight` selectors and bundler size/import advisories;
  the same advisories occur in the successful frontend job `111998525465` on main.
  These are recorded separately from zero-warning lint.
- Production-browser composer replay: exact Unicode/repeated spaces survive project remount,
  subsequent editing and full reload. Durable evidence is under
  `docs/qa/evidence/2026-10-05-dependency-upgrades/`; both isolated labs report clean teardown.

- Both targeted QA audits passed with strict enforcement and clean teardown. Audit and teardown
  receipts are stored beside the browser and profile evidence.

## Release-validation follow-up

Release PR #688, head `b95d7e9cb`, exposed an obsolete failure injection in the
existing disconnected-bell browser scenario. CI job `112111431739` shows the
notification-ledger endpoint returning HTTP 200 while the test interrupts the old
session-catalog source. The bell correctly remains connected. The existing E2E now
interrupts its actual ledger endpoint and observes the failed request before checking
the unchanged warning assertion and timeout. This bell correction changes verification only; native
tools, public contracts, hooks, configuration, persisted data, official skills, and
production behavior are unchanged.

Two additional release integration failures reproduced locally:

- The 500-event SSE replay delivered every expected event but its reader failed to join
  in all ten runs. A goroutine dump located the reader in Go 1.27's HTTP response-drain
  EOF handshake after concurrent body closure. HTTP and UDS test requests now own a
  cancellable context; closing their bodies cancels the request before closing the
  transport body. All ten replay runs pass with the original two-second join deadline,
  ten-second replay deadline, event count, and cursor-order assertions unchanged.
- Lane cancellation queued a generic `stop` cleanup before terminal run-agent settlement.
  The idempotent cleanup insert consequently preserved the wrong cause. Production now
  captures the addressed session identities, settles terminal bindings, then queues any
  remaining generic cleanup. The existing real-SQLite regression passes three repetitions;
  it additionally verifies that the returned session identity is retained.

Follow-up verification:

- Official Web E2E shard 1: 79 passed, three existing skips (13.7 minutes), including
  the corrected disconnected-ledger scenario with its original assertion.
- HTTP and UDS stream/SSE integration suites with race: passed (11.489s / 5.481s).
- Existing goal-binding, cancellation atomicity, and terminal-settlement authority
  integration suites with race: passed (5.679s after final helper organization).
- Test-shape checks pass for the HTTP helper and goal-binding suite. The UDS suite
  retains 14 pre-existing top-level assertion findings; only its request helper changes.
- Final follow-up `make gate`: passed. Go lint reports zero issues; API/store race
  suites pass (996s overall), and the Web lint/typecheck/test/codegen lane passes.

## Desktop restoration and native diagnostic shortcut follow-up

PR #694 Web shard 2 measured 516.9 ms for the unchanged 500 ms twelve-window
restoration budget. CPU profiling identified a transient empty desktop: the snapshot
arrived before layout configuration, so the window layer mounted the empty-state
composer before replacing it with restored frames. The layer now waits for its
existing configuration readiness signal before exposing desktops. No persisted
layout, window geometry, or performance threshold changes.

The existing interaction-hook suite owns this readiness invariant. Its new regression
fails against the original hook (one failure, 56 passes) and all 57 tests pass with
the correction through root Turbo. A real browser profile at five-times CPU throttling
measured 494.9 ms before and 406.9 ms after; these are single local measurements,
not Linux CI proof. The official Web shard 2 rerun remains pending.

Main CI desktop job `112109695891` failed the second diagnostic shortcut in E2E-034.
Detached DevTools can own native focus, while Electron requires the target window to
be focused before `sendInputEvent`. The existing packaged test now focuses the product
window and observes native focus before sending that second shortcut. The production
shortcut and all security assertions and deadlines are unchanged. The focused packaged
macOS scenario passes three repetitions (20.5 seconds); its baseline also passed
locally, so Linux confirmation remains owned by the next CI run. This focused run
does not verify the scenario's separate physical clipboard journey.

Final local gate for the restoration and native-focus corrections: all affected lanes
passed, including Desktop and Web lint/typecheck/tests. React Doctor reports 100/100
with no issues. Official Web shard 2 is still running; final CI is pending.

## Loop-record navigation verification follow-up

Main CI job `112163156959` failed the existing Tasks reveal-filter journey after
the browser URL changed to a Loop run but before the Loop window appeared. The
trace shows the next Tasks dock click correctly issuing `window.close` with
`minimize: true`; the preceding run-page screenshot still shows Tasks. The scenario
now also asserts that the existing `loop-run-detail-content` is visible inside the
Loops window before returning. All original assertions and deadlines remain.
This strengthens navigation evidence without changing production behavior. The
local gate passed, including Web lint, typecheck, tests and codegen. The official
Web shard 4 verified this Tasks journey, but finished with 71 passes and two
terminal fixture failures described below.

## Terminal fixture verification follow-up

The local official Web shard 4 exposed two fixture defects in the existing terminal
E2E suite. E2E-001 inherited Fish 4.9.3, but its piped PTY did not answer the required
primary-device-attributes query. Instrumentation showed human input admission followed
by idle fallback after roughly 300 ms, then the shell's authenticated command marker
about ten seconds later. The unchanged scenario failed ten of ten repetitions. An
isolated real PTY answered DA1 and observed the first Fish prompt in 56 ms.

The interactive CLI fixture now answers DA1, including queries split across output
chunks, as a basic VT100 terminal. This supplies the terminal peer required by
[Fish's terminal contract](https://fishshell.com/docs/current/terminal-compatibility.html).
It does not change journal attribution, shell selection, assertions or deadlines.
E2E-014 completed its TUI assertions but failed teardown because `go run` downloaded
a read-only Go toolchain/module cache into the isolated operator home. The fixture
now builds the same real TUI using the test runner's Go environment before launching
the binary inside that home.

Both existing scenarios passed three focused repetitions each (six passes, 45.3s)
with the diagnostic daemon. The instrumentation was removed from source before
validation and is not part of the change. The official production-binary shard rerun
and final CI remain pending. The canonical invariants are human attribution after
interactive CLI input (E2E-001) and real alternate-screen rendering, resize, watcher
agreement and primary-screen restoration with clean teardown (E2E-014).

## Profile recovery guidance retry follow-up

PR #695 CI job `112172947814` failed E2E-031 after the recovery tooltip had appeared
with the correct operation ID. The trace shows the next registration retry clearing
the last client error at the same time as the status/tooltip unmounted, even though
the unavailable profile had not recovered. The registration store now retains the
last error through automatic, visibility-resume and explicit retry transitions. Success
clears it; workspace/profile rebinding still creates a fresh state.

Three cases added to the existing registration hook suite fail against the prior
production transitions (three failures, nine passes). The original E2E remains unchanged;
five local baseline repetitions passed, so local timing alone does not reproduce the
Linux CI failure. The deterministic hook regression owns the lifetime invariant.
All 12 owning hook tests pass after the correction. The final local gate passed,
including Web lint with zero warnings/errors, typecheck, tests and codegen. React
Doctor reports 100/100. Official Web shards 3/4 and final-head CI remain pending.
The prior PR head also completed Linux Web shard 4 with 73 passes (16.4m), including
the corrected Tasks navigation; that evidence does not qualify the newer changes.

## SQLite commit acknowledgement and roster fixture follow-up

Release PR #688 head `6e89b3ff` failed Go race shard 2 (`112218483055`): the completed
native-tool event did not persist within the existing post-unlock deadline. Thirty focused
macOS baseline repetitions passed, but a concurrent two-CPU reproduction exposed a related
production failure: COMMIT completed durably, the driver returned context deadline exceeded,
rollback reported no active transaction, and the persistence retry inserted a second event.
A Linux/arm64 race reproduction also reported two events instead of one.

The shared write boundary now checks cancellation after its mutation fence, then executes
COMMIT without caller cancellation. It preserves the actual commit result, as the SQLite
driver's transaction Commit implementation does. Admission, callback cancellation and the
existing rollback path remain in place. The existing TestExecuteWrite suite adds a real-SQLite
case with cancellation injected only at the driver I/O boundary after commit, plus a case
that proves cancellation observed at the fence still rolls back. The first regression fails
against the previous production implementation. Both cases pass ten repetitions after repair.
The original daemon contention test and its assertions/deadlines remain unchanged.

Runtime job `112218483056` separately failed roster journey E2E-005 after both attempts hit
the reused lifecycle fixture's two-second action deadline. The second session completed ACP
startup in 197 ms, then its creation hook took 558 ms; the deadline arrived before a successful
prompt result. This read-layer invariant has no two-second session-start SLA. Its dedicated
fixture blocks until actual cancellation, uses a ten-second action budget for real session
setup and returns the existing successful response on retry. The two-attempt roster assertions
and 45-second overall journey deadline remain unchanged. The separate timeout lifecycle E2E
retains its original two-second deadline and three-second delayed first response.

The complete TestExecuteWrite suite passes (16.293s), the new boundary cases pass ten repetitions
(1.183s), and the unchanged tool-event suite passes 30 repetitions on macOS (20.123s) and Linux
with two CPUs (21.167s), all with the race detector. The real-daemon roster journey passes
three repetitions (50.927s). The existing ACP-subprocess/SQLite managed skill contention
integration passes (5.869s), proving same-session recovery alongside health writes. Lint
reports zero issues. The final official `make test-e2e-runtime` passes all 313 tests:
231 daemon, 21 HTTP, 49 UDS, eight harness and four remote Gateway CLI tests. It includes
the corrected roster fixture and the unchanged two-second timeout lifecycle scenario.
The final local `make gate` passes Go lint with zero issues and all affected Go race suites.
Current-head PR CI remains pending.

## Profile entry during restart reconciliation

Release Nightly job `112266831267`, head `b8cd86cb`, passed 288 browser scenarios with
three existing skips and failed the Unicode zsh scenario before terminal assertions.
Its retained trace shows the pre-restart desktop being accepted by the reload helper;
the screen then entered the root route error boundary with a TanStack `CancelledError`.
The helper and original E2E assertions remain unchanged during the production repair.

The route's profile precondition can join an in-flight selection query. Reconciliation
silently cancels that query and replaces it; TanStack's joined-fetch path exposes the old
promise's cancellation to the route. The existing route-preloading suite reproduces this
with a real QueryClient/router and adapter-boundary deferred responses: the original
production code fails the new regression while 52 existing tests pass. Route entry now
follows a live replacement after silent cancellation, preserving the authoritative profile
read and propagation of genuine failures. The final local gate passes all affected lanes,
including 7,014 Web tests, typecheck and lint with zero warnings/errors. The owning route
suite passes all 53 tests. The production Web build passes through root Turbo and React
Doctor reports 100/100. Both original zsh prompt scenarios pass three repetitions each
against the real daemon and current production Web build (six passes, 2.8 minutes).
Current-head PR CI remains pending.
