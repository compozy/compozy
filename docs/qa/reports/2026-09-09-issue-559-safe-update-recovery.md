# Issue 559: safe runtime update recovery

## Scope and charter

Dora updates a used local installation, waits through slow startup, and recovers a genuine boot failure without losing state. Bruno opens an older desktop shell against a newer runtime. Targeted fault injection uses an isolated home and the existing restart/update paths; it does not claim provider validation.

| Journey | Status | Evidence |
| --- | --- | --- |
| Runtime replacement preserves migrated state after failure | Pass | Published beta.22 home upgraded from global schema 99 to 107; actual replacement exit retained the new binary and incompatible backup; recovery preserved the workspace |
| Slow detached start and restart eventually report readiness | Pass | Existing CLI/daemon suites and a real replacement paused for 30 seconds before readiness |
| Older desktop preserves a newer runtime | Pass (owning suites) | Bootstrap version policy and hash-verified installed provenance; no full older-app/newer-runtime UI walk claimed |
| Staged app recovery does not depend on runtime readiness | Pass | Packaged macOS Electron scenario observed bootstrap `error` and app operation `applying`; fixture teardown passed |
| Recovered runtime clears historical rollback projection | Pass | Existing projection suite covers equal and newer recovered versions |

## Cross-surface impact

- Native tools: no IDs, schemas, or permissions change. Settings restart and update HTTP/UDS routes retain their payloads; CLI observes the host-local restart journal while the daemon is offline.
- Extensibility/hooks/config: no config keys, defaults, hook ordering, extension contracts, or provider credentials change. Required boot work still precedes readiness.
- Workspace isolation and user state: the update journal and binary are host-global. No SQLite migration, schema version, workspace data, or released migration bytes change. A post-swap failure preserves the replacement and backup because any persisted stream may already have migrated.
- Official skill: desktop recovery guidance is updated in `skills/compozy/references/desktop.md`.
- Web/Docs: existing Settings update/restart projections consume unchanged status contracts. The desktop starts its app update consumer before bootstrap succeeds. Public recovery documentation and affected QA scenarios co-ship, including the intentional slow-start pending/cancellation contract in `APP-start-installed-daemon`.

## Verification log

- Initial `TestCoordinatorRuntimeLifecycle` regression failed because health failure restored the old binary and interrupted-swap recovery archived `rolled-back`.
- After the first production correction, the owning coordinator suite passed with `CGO_ENABLED=1 go test -race ./internal/update -run TestCoordinatorRuntimeLifecycle -count=1`.
- Lab: `.cache/issue-559-qa/compozy-issue-559-recovery-20260909-160037-168457-lab/qa-artifacts/qa/bootstrap-manifest.json`.

## Runtime evidence and limits

The lab used the existing `desktop/e2e/fixtures/runtimeupdate` coordinator, production detached restart helper, actual SQLite stores, and public CLI. The fixture stages the verified replacement directly; this walk does not retest release signature/download verification.

1. Verified the published `v0.3.0-beta.22` macOS archive against its release checksum, started it in the manifest home, and registered workspace `ws_194a2b87cce1e40a` (`recovery-preserved-workspace`). The baseline reported global schema 99.
2. Applied the current implementation (fixture runtime version `0.3.1`) and observed global schema 107. The first attempt to pause startup missed the text log marker and completed normally; it is migration evidence only.
3. Repeated replacement with the corrected marker, suspended the owned replacement for 30 seconds after its schema stage, verified restart stayed `starting`, resumed it, and observed `finalized`/`updated`, runtime `running`, and the preserved workspace. `slow-update-result.json` records this run; both binaries in this second run have the same hash.
4. Recreated the incompatible beta.22 backup in the isolated fixture and killed only the replacement after its schema stage. The coordinator archived `failed`, retained the new executable (SHA-256 `6b2057c6b08f07ba2b80185d2d604a375aef22d55bd3641dbb97e99b6fb0ae27`) and beta.22 backup (`3486080e2c59fd3852de47d793d10e5579cc68ee9060bbf31d74e71e88d8c0c0`), then `compozy daemon start -o json` recovered with the workspace intact. See `failed-update-result.json`, `failure-recovery-start.json`, and `failure-recovery-workspaces.json` in the lab QA directory.
5. Targeted manifest teardown completed with `clean: true`. No operator home or credentials were changed, and no provider session was used. User-installed skill validation warnings were present during boot; no provider-health claim is made.

Readiness observation is intentionally unbounded until ready, actual exit, or caller cancellation. It observes liveness, not migration progress. A stalled live child is neither killed nor marked ready. CLI cancellation reports the PID and releases the startup mutation lock; the owning test reacquires that same lock and verifies no termination. Provisioning completes before startup; the updater releases its swap lock before waiting. The detached restart helper holds neither mutation lock while observing and exits on readiness, actual exit, or its own cancellation; canceling a CLI does not cancel that helper. Periodic diagnostics make the continued wait visible.

Focused race suites passed for update lifecycle/projection/provenance, CLI wait/cancellation/local journal observation, and restart cancellation. The existing integration command `CGO_ENABLED=1 go test -race -tags integration ./internal/daemon -run 'TestBootMarksRestartOperationReadyAfterFreshDaemonInfo|TestRelaunchHelperFailurePersistsAfterOldDaemonExit' -count=1` passed. Desktop root Turborepo `test typecheck --filter=@compozy/desktop` passed 137 tests; root affected desktop/web builds completed. Windows runtime cross-build passed. Subsequent heavy checks use `GOMAXPROCS=4`, `COMPOZY_GO_TEST_P=1`, `COMPOZY_GO_LINT_CONCURRENCY=2`, and pinned tools through `mise exec`.

## Packaged desktop evidence

`mise exec -- bun run --cwd desktop build:e2e-update` built the instrumented baseline and next-version packages. Under the shared machine verification lock, `bunx playwright test --config playwright.config.ts --grep 'A staged app update starts'` from `desktop/` passed (1 test, 17.1 seconds). The isolated fixture made `compozy.db` a directory to force a real boot failure and held the update download so no native installer/relaunch could escape the fixture home. The app reported `error` while the staged operation reached `applying`. Screenshot: `.tmp/playwright/desktop-results/__tests__-updates-A-staged-9d6fc-untime-bootstrap-is-failing/update-during-bootstrap-failure.png`.

The first attempt exposed a read-before-publication race in the new assertion; the second reached both product conditions and exposed the fixture's assumption that every test has a running daemon during teardown. The existing fixture now verifies the lock PID has exited when no discovery record was published. The final run passed all assertions and cleanup. It neither changes launch-session environment nor performs a native installer relaunch. The existing installer/relaunch scenarios remain the owners of that unchanged behavior.

## Review remediation: staged bundle integrity

Greptile identified that the early update consumer could invoke its transition client before bootstrap verified the runtime bundle. The existing packaged update suite reproduced this with a substituted executable that wrote a marker: the marker existed on the initial implementation even though bootstrap rejected the bundle.

Startup now verifies the bundle before starting the update consumer, while retaining BootstrapRunner's own verification at its execution boundary. A verified bundle can still process a staged app update when daemon boot fails. Rebuilt baseline/next packages and reran `bunx playwright test --config playwright.config.ts --grep 'A staged app update'` under the shared verification lock: both scenarios passed (13.3 seconds), including no execution marker for the tampered bundle and clean fixture teardown.

The initial commit passed the full local affected gate and strict QA audit. Per the user's subsequent delivery direction, follow-up commits retain focused behavioral checks and hook validation while the full gates run in PR CI.

## Delivery tracking

Runtime recovery and the packaged app recovery slice passed with the limits above. The pull request records the final local gate, checked head, required CI, and CodeRabbit/Greptile dispositions. The lab's machine-readable strict audit is finalized with local gate evidence before delivery; `teardown.json` records `clean: true`.


## CodeRabbit follow-up

Desktop shutdown and installer handoff now cancel and await the bootstrap observer before cleanup. The detached runtime remains alive, and canceled bootstrap callbacks cannot publish a product window after cleanup. The existing shell E2E covers quitting during an unfinished bootstrap flow and verifies the same daemon PID remains running. Staged-update polling uses an explicit 30-second bound, and failed-boot fixture cleanup signals the isolated lab's lock PID before verifying exit.

The CLI journal suite also covers the missing-local-record remote fallback. The restart integration's final observer wait has a 10-second deadline. A subsequent fully booted daemon reconciles older `starting` restart observations under its exclusive home lock, while preserving its own current restart operation. Cancellation alone still does not declare a live replacement failed. No schema, migration numbering, or public restart DTO changes were needed.

The requested global readiness deadlines were not adopted: readiness windows are intentionally diagnostic, with caller cancellation and actual exit as termination boundaries. This behavior and lock/helper lifetimes were explicitly reviewed with the controller before publication. An elapsed deadline cannot establish migration failure; a stalled live process remains pending with PID diagnostics. The retain-runtime failure policy remains independent of this observation contract.


The follow-up CLI race test passed (2.391 seconds); the final daemon boot integration passed with `-race -tags integration -p=1 -parallel=4` (6.120 seconds), including reconciliation of a valid abandoned restart despite corrupt historical metadata. Corrupt history is reported without blocking boot or deleting the record. CodeRabbit withdrew and resolved both deadline findings after reviewing the documented contract and existing cancellation/exit/lock evidence.


Final packaged follow-up: the owner `build:e2e-update` generator completed, then Playwright ran under the shared machine verification lock. Four scenarios passed in 25.9 seconds: bootstrap quit preserving the daemon (2.7s), packaged security/DevTools (11.0s), staged update during boot failure (10.9s), and rejected unverified runtime execution (0.9s). All fixture teardown completed. The preceding Linux CI head had passed 27/28 desktop scenarios but lost the Electron debugging connection in E2E-034; its job log and artifact were inspected. The same security scenario passed locally on macOS, so Linux current-head CI remains required rather than claiming a proven cross-platform fix for that isolated failure.
