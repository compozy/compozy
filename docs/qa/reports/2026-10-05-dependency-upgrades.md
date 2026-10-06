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
