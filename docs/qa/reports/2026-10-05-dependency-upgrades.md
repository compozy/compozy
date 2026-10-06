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
  passes the same test with full race/checkptr in 103.200s; Linux workflow verification
  is run separately on the PR branch. The original deadline and assertions are unchanged.

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
  693 files and 7,008 tests.
- The extension registry test-shape heuristic flags existing top-level assertions; the only
  change there is the ULID v2 import, so unrelated test reshaping is outside this migration.
- Final `make gate`: passed (integration, Mage, Go lint/race tests, codegen, all JS lanes).
  Go lint reports zero issues; root Turbo reports 26 successful tasks. Final-head CI is owned
  by the pull request.
- Final React Doctor: 100/100, no issues across 30 changed files.
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
