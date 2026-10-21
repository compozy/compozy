## What & why

Desktop updates could remain staged indefinitely or close the application before the native installer was ready. Users then had to cancel the staged operation in Settings and download a new app manually. This change repairs the installation lifecycle, stalled-operation recovery, app/runtime coordination and retry action.

The intended desktop-owned interaction is one Update action: download and verify the app bundle, finish native staging while the window remains available, let the native installer restart the app, then observe the installed app and ready runtime. A failed download or native staging attempt exposes **Retry update** directly.

Authored and verified by Codex using an isolated worktree. The implementation baseline was `b1008af26`; the initial implementation commit is `c3e787382`.

## Investigation and confirmed causes

| Finding                                                          | Evidence                                                                                          | Change                                                                                                        |
| ---------------------------------------------------------------- | ------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------- |
| Ordinary shutdown could beat native staging                      | Real Electron BrowserWindow reproduction showed the last window closing before Squirrel was ready | Prepare the native installer before destructive cleanup and give it ownership of quit                         |
| A persisted watchdog did not interrupt a live blocked waiter     | Blocked-I/O and late-completion regressions in the existing consumer suite                        | Execute phase deadlines, propagate cancellation and reject late results                                       |
| A failed acquisition could consume the watcher revision          | Filesystem watcher reproduction                                                                   | Acknowledge only accepted revisions so the existing polling loop retries transient failures                   |
| The app plan could disagree with MacUpdater under Rosetta        | Pinned updater implementation and Go architecture-selection regressions                           | Select the app asset using Apple Silicon hardware identity and disable incompatible differential download     |
| A joint update replaced a desktop-owned runtime before its owner | Verified planner regression                                                                       | Normalize eligible app-plus-runtime intent to one app bundle and let existing bootstrap provision its runtime |
| App completion could precede daemon readiness                    | Real local daemon HTTP/discovery fixtures                                                         | Wait for published daemon identity, compatible version and a matching runtime snapshot                        |
| A settled failure had diagnostics but no direct retry action     | Existing Web presentation and menubar regressions                                                 | Retain the failure and expose Retry update when the release is eligible and the operation channel is free     |

The operator's desktop log also contained 22 instances of `The runtime update operation is invalid.` on 2026-10-20. The released beta.35 parser rejects runtime journal phases that beta.36/37 already support. This is corroborating historical evidence, not a new parser fix in this PR: the canceled journal and executing app version were unavailable, so the incident cannot be reconstructed uniquely. The current shutdown and liveness defects were reproduced independently.

## Reverse engineering and competitor comparison

- **Grok Bot 0.75.0:** REA analyzed the installed Electron/ASAR artifact without launching or modifying it. Artifact hashes, exact packaged JavaScript and machine-readable evidence are recorded. Parsing covered 759 JavaScript files with no parse failures; semantic graph coverage is partial, and nine unpacked native integrity discrepancies remain explicitly untrusted. Its update flow separates download, bounded native staging, installer-owned quit and recovery.
- **Synara:** inspected revision `299fd573b1d6520c323a87c78d30be74e87e53e1`. Background preparation, bounded phases, durable update identity and native quit ownership informed the lifecycle comparison.
- **T3 Code:** inspected revision `5b2ee393c3d6576bcf44c5d2b74565770943e189`. It has both a local confirmation flow and a combined countdown flow, with explicit native installation lifecycle handling.
- **MonoCode:** inspected revision `115f0e97c4f869188b5868cc4e9969d6f1a3f972`. The Tauri sidebar performs download/install/relaunch from one action; Settings can add confirmation. Its error visibility also has limitations.

These source inspections establish implementation patterns, not measured competitor reliability. Detailed citations are recorded in:

- [Research summary](../docs/research/desktop-update-flow/README.md)
- [Grok Bot analysis](../docs/research/desktop-update-flow/grok-bot.md) and [REA evidence JSON](../docs/research/desktop-update-flow/grok-rea-evidence.json)
- [Competitor comparison](../docs/research/desktop-update-flow/competitors.md)
- [Compozy causal reproductions](../docs/research/desktop-update-flow/compozy-causes.md)
- [App-owned planner and CLI evidence](../docs/research/desktop-update-flow/compozy-app-owned-plan.md)

## Implementation

### Desktop download, staging and shutdown

- Verify the planned version and artifact before installation, retain digest verification, and propagate AbortSignal to the downloader cancellation token.
- Add explicit native preparation that waits for readiness, error or cancellation before cleanup. Installation requires successful preparation.
- Keep ordinary app/window shutdown from winning once the installer owns quit; restore ordinary ownership when preparation fails.
- Bound checking/downloading with a 60-second inactivity deadline and a 10-minute absolute deadline, native staging with five minutes, and final cleanup/handoff with 30 seconds. Only increasing download progress refreshes inactivity.
- Renew the operation lease every 30 seconds during preparation, drain renewal before cleanup, and fence late asynchronous results after cancellation.
- Preserve persisted recovery deadlines for restart recovery. Terminal failures remain acknowledged; transient pre-acquisition failures remain retryable.
- Use Electron's Rosetta detection for differential-download policy and remove the superseded internal cross-architecture helper and its redundant test.

### Planner, runtime ownership and CLI completion

- Validate the original requested targets before normalizing a verified, unmanaged desktop-owned joint update to the app target. Caller inputs are not mutated; explicit runtime-only and independent/managed behavior is preserved.
- Probe Apple Silicon hardware with the existing command boundary and a bounded timeout for Darwin/amd64 app selection. Runtime archive selection still follows the runtime architecture; cancellation remains observable.
- After app installation, wait for the actual local daemon's published identity, readiness and compatible version. Confirm the runtime snapshot matches status, retain a compatible newer runtime, and report restart truth from observation.
- Preserve verified app success if runtime readiness fails. Closed-app staging retains its existing behavior.

### Web, documentation and native test fixtures

- Add Retry update to the menubar and General Settings for an eligible settled app failure; preserve error visibility and refresh release eligibility on retry.
- Update the official desktop skill, getting-started and operations documentation, two existing QA journeys and six affected scenario contracts.
- Build distinct baseline/candidate runtimes in the existing native fixture and assert actual daemon readiness after installation.
- Isolate native Squirrel identity/cache using a worktree-specific E2E bundle ID; derive the existing ad-hoc signature requirement from that ID.
- Preserve process state and desktop logs on restarted-shell teardown failure. No timeout or behavior assertion was weakened.

## How you verified it

Regressions were added to existing owning suites after reproducing the defects. No standalone duplicate test suite or production dependency was introduced.

| Verification                                         | Result                                                                                                                            |
| ---------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------- |
| Root `make gate` and current-tree `make gate-status` | All affected local lanes passed: Go lint/race tests, code generation, desktop and Web lint/typecheck/tests                        |
| Desktop suite                                        | 191 tests in 22 files passed                                                                                                      |
| Web suite                                            | 7,262 tests in 687 files passed                                                                                                   |
| Existing native macOS rehearsal                      | 8 tests passed in 4.4 minutes, including real replacement/relaunch from app/runtime 0.3.0 to 0.3.1                                |
| Native recovery                                      | Expired handoff retains old-version truth, displays Retry update, then a fresh staged attempt installs and starts the new runtime |
| Adjacent native boundaries                           | Shell shutdown/relaunch, updating during bootstrap failure, and rejection of an unverified bundled runtime passed                 |
| Go focused regressions                               | Update package and owning CLI suites passed with the race detector; default/integration lint reported zero issues                 |
| Windows compilation boundary                         | `GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build ./internal/update` passed                                                       |
| React Doctor locally                                 | 100/100, no issues                                                                                                                |
| Root Turborepo Web build/typecheck                   | Passed                                                                                                                            |
| Pre-commit tasks                                     | Passed using pinned Bun 1.4.0 and Go 1.27.1 tooling, with lint-staged stash/revert behavior disabled                              |

Native command, invoked from the repository root under the existing verification machine lock:

```sh
bun run --cwd desktop test:e2e --grep 'E2E-016|E2E-017|E2E-007|A staged app update'
```

Lint reports zero warnings/errors. Existing Web `act(...)` notices, expected failure-path console errors, experimental SQLite warnings and build diagnostics are documented; the complete test/build logs are not claimed to be warning-free.

The [verification report](../docs/qa/reports/2026-10-21-desktop-update-flow.md) records intermediate failures, corrections, environment isolation, local evidence paths and teardown. Screenshots and large logs/binaries remain local ignored artifacts. The full six-scenario contracts retain `untested` status because the targeted rehearsal is narrower than the complete release journeys.

## Impact

- **User-facing:** one app-bundle update and native restart for desktop-owned joint intent; direct retry after a settled failure; truthful CLI runtime completion.
- **CLI/HTTP/UDS/native tools:** existing update entry points and operation schema remain in use. No new verb, flag, route, DTO, `compozy__*` ID or journal format.
- **State/config/extensibility:** no database migration, config key, hook or extension SDK change. Existing ownership/eligibility boundaries remain enforced.
- **Web/Docs/official skill:** retry behavior and desktop update instructions are updated with the implementation.
- **Isolation:** research is read-only against the installed Grok app; native validation uses fixture apps/homes/ports and a distinct native updater identity.
- **Dependencies:** none added; no second updater or update service introduced.

The owning [cross-surface impact audit](../docs/_memory/change-impact.md) records these surfaces and the affected QA contracts.

## Remaining release acceptance and limitations

- Native fixtures use local ad-hoc signatures and a seeded verified operation. Both app versions contain the repaired code. They prove native replacement and runtime bootstrap, not the production GitHub/Sigstore/notarization chain or an already shipped old updater consuming a signed candidate.
- A real Rosetta installation, Linux AppImage update and complete browser/CLI apply journey are not claimed as executed. Broader charter branches, including active-agent shutdown and install-directory permission failures, remain outside this targeted rehearsal.
- An already installed broken updater cannot receive its own fix before it installs new code. Affected historical versions may need one recovery install or a separately verified bridge.
- Download/native-staging failures remain recoverable before cleanup. An exceptional failure during final destructive cleanup can still require relaunch.
- This PR does not publish a release. Local evidence does not substitute for current-head CI, CodeRabbit, Greptile or React Doctor review results.

---

- [x] `make gate` passed for the implementation; this description is editorial.
- [x] Changed behavior is covered by existing owning regression suites and a real native run.
- [x] Agent authorship, verification evidence and limitations are disclosed above.
- [ ] All applicable required CI checks are green on the current PR head.
- [ ] CodeRabbit and Greptile findings have evidence-backed dispositions and no unresolved actionable threads.
- [ ] React Doctor is green on the current PR head.
- [ ] Squash merge is performed only after the current head passes the delivery checks above.
